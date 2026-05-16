// Command consumer drains the stats SQS queue into Postgres. It runs as a
// long-running container (e.g. on ECS / Fargate / k8s) — not a Lambda.
//
// The poll loop uses long polling (WaitTimeSeconds=20, batch size 10 by
// default), hands each batch to handler.Process, and deletes each
// successful message individually. Failed messages are left in the queue
// so SQS re-delivers them once their visibility timeout expires.
//
// Throughput knobs (worker count, batch size, long-poll budget, transient
// receive backoff, pgxpool max connections) are env-driven; see
// cmd/consumer/README.md for the full table.
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jorgejr568/organizze-mcp/cmd/consumer/internal/handler"
	"github.com/jorgejr568/organizze-mcp/cmd/consumer/internal/store"
)

const (
	pingTimeout = 5 * time.Second
	ackTimeout  = 5 * time.Second // grace period for the Delete phase after SIGTERM
)

// config is the resolved, validated runtime configuration. Internal to
// main — operators tune it via env vars; loadConfig owns the parsing.
type config struct {
	QueueURL       string
	DSN            string
	Workers        int
	BatchSize      int32 // SQS API uses int32
	WaitSeconds    int32
	ReceiveBackoff time.Duration
	PGPoolMaxConns int32 // 0 = leave the pgx default
}

func main() {
	cfg := loadConfig()

	// Log the resolved tunables so operators can verify what the pod is
	// actually running with. DSN and queue URL get their own lines to
	// keep credentials/identifiers off the tunables line.
	log.Printf("consumer: workers=%d batch=%d wait=%ds backoff=%ds pg_max_conns=%d",
		cfg.Workers, cfg.BatchSize, cfg.WaitSeconds, int(cfg.ReceiveBackoff/time.Second), cfg.PGPoolMaxConns)
	log.Printf("consumer: polling %s", cfg.QueueURL)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		log.Fatalf("pgxpool.ParseConfig: %v", err)
	}
	if cfg.PGPoolMaxConns > 0 {
		poolCfg.MaxConns = cfg.PGPoolMaxConns
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		log.Fatalf("pgxpool.NewWithConfig: %v", err)
	}
	defer pool.Close()

	// Fail fast on a bad DSN: ping with a short deadline at startup
	// instead of crashing on the first message.
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	if err := pool.Ping(pingCtx); err != nil {
		cancel()
		log.Fatalf("postgres ping: %v", err)
	}
	cancel()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}
	sqsClient := sqs.NewFromConfig(awsCfg)

	// One shared handler across workers: Handler is stateless modulo the
	// pgxpool-backed store, and the store's INSERT is idempotent via
	// ON CONFLICT DO NOTHING, so parallel calls are safe.
	h := &handler.Handler{
		Store: store.New(pool),
		Log:   log.Default(),
	}

	var wg sync.WaitGroup
	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			log.Printf("consumer: worker %d/%d started", idx+1, cfg.Workers)
			if err := pollLoop(ctx, sqsClient, h, cfg); err != nil && !errors.Is(err, context.Canceled) {
				// A pollLoop returning a non-Canceled error is unusual but
				// not fatal to the process — other workers keep draining.
				log.Printf("worker %d: %v", idx+1, err)
			}
		}(i)
	}

	<-ctx.Done()
	log.Print("consumer: shutting down, waiting for in-flight batches")
	wg.Wait()
	log.Print("consumer: shutdown complete")
}

// pollLoop is a single worker's long-running loop. Returns when ctx is
// cancelled. Multiple instances share the SQS client and handler — SQS
// distributes messages across pollers naturally with no coordination.
func pollLoop(ctx context.Context, sqsClient *sqs.Client, h *handler.Handler, cfg config) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		out, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(cfg.QueueURL),
			MaxNumberOfMessages: cfg.BatchSize,
			WaitTimeSeconds:     cfg.WaitSeconds,
		})
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			// Transient SQS error: log and continue. A short backoff
			// keeps us from hammering the API if it's persistently broken.
			log.Printf("consumer: receive failed: %v", err)
			if !sleep(ctx, cfg.ReceiveBackoff) {
				return ctx.Err()
			}
			continue
		}
		if len(out.Messages) == 0 {
			continue // long-poll returned no work; loop immediately
		}

		records := toRecords(out.Messages)
		res := h.Process(ctx, records)

		failed := map[string]struct{}{}
		for _, id := range res.FailedMessageIDs {
			failed[id] = struct{}{}
		}

		// Use a fresh, short-deadlined context for the Delete phase so
		// successful persists still get acked when the parent ctx fires
		// (SIGTERM mid-batch). Without this, every Delete would fail with
		// context.Canceled, the rows would stay in the queue, and the
		// next start-up would see them redelivered — safe due to ON
		// CONFLICT DO NOTHING, but noisy and avoidable.
		ackCtx, ackCancel := context.WithTimeout(context.WithoutCancel(ctx), ackTimeout)
		for _, m := range out.Messages {
			if _, bad := failed[aws.ToString(m.MessageId)]; bad {
				continue // leave for redelivery
			}
			if _, err := sqsClient.DeleteMessage(ackCtx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(cfg.QueueURL),
				ReceiptHandle: m.ReceiptHandle,
			}); err != nil {
				log.Printf("[%s] delete failed: %v", aws.ToString(m.MessageId), err)
			}
		}
		ackCancel()
	}
}

func toRecords(msgs []sqstypes.Message) []handler.Record {
	out := make([]handler.Record, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, handler.Record{
			MessageID: aws.ToString(m.MessageId),
			Body:      []byte(aws.ToString(m.Body)),
		})
	}
	return out
}

// sleep is ctx-cancellable. Returns false if ctx fired before the duration elapsed.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var %s", key)
	}
	return v
}

// loadConfig parses + validates every env var the consumer cares about.
// Required vars use mustEnv; tuning vars use envInt with safe defaults
// matching the v0.8.0 single-worker behaviour. Any parse error or
// out-of-range value terminates startup before we touch SQS or Postgres.
func loadConfig() config {
	dsn := mustEnv("STATS_DATABASE_URL")
	queueURL := mustEnv("STATS_QUEUE_URL")

	workers := envInt("STATS_POLL_WORKERS", 1, 1, 32)
	batch := envInt("STATS_POLL_BATCH_SIZE", 10, 1, 10)
	wait := envInt("STATS_POLL_WAIT_SECONDS", 20, 0, 20)
	backoff := envInt("STATS_RECEIVE_BACKOFF_SECONDS", 2, 0, 60)
	pgMax := envInt("STATS_PG_POOL_MAX_CONNS", 0, 0, 100)

	return config{
		QueueURL:       queueURL,
		DSN:            dsn,
		Workers:        workers,
		BatchSize:      int32(batch),
		WaitSeconds:    int32(wait),
		ReceiveBackoff: time.Duration(backoff) * time.Second,
		PGPoolMaxConns: int32(pgMax),
	}
}

// envInt reads an int env var. Empty/unset → def. Non-integer or out of
// [min, max] → log.Fatalf with a message naming the var, the supplied
// value, and the allowed range.
func envInt(key string, def, min, max int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		log.Fatalf("%s: not an integer (%q)", key, raw)
	}
	if n < min || n > max {
		log.Fatalf("%s=%d out of range [%d, %d]", key, n, min, max)
	}
	return n
}
