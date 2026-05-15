// Command consumer drains the stats SQS queue into Postgres. It runs as a
// long-running container (e.g. on ECS / Fargate / k8s) — not a Lambda.
//
// The poll loop uses long polling (WaitTimeSeconds=20, batch size 10),
// hands each batch to handler.Process, and deletes successful messages
// in a single DeleteMessageBatch call. Failed messages are left in the
// queue so SQS re-delivers them once their visibility timeout expires.
//
// Two env vars tune throughput without rebuilding the image:
//   - CONSUMER_POLLERS: number of parallel pollLoop goroutines (default 1).
//   - CONSUMER_INSERT_CONCURRENCY: max concurrent inserts per batch (default 1).
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/jorgejr568/organizze-mcp/cmd/consumer/internal/handler"
	"github.com/jorgejr568/organizze-mcp/cmd/consumer/internal/store"
)

const (
	receiveBatchSize   = 10 // SQS max
	receiveWaitSeconds = 20 // long-poll
	pingTimeout        = 5 * time.Second
	ackTimeout         = 5 * time.Second // grace period for the Delete phase after SIGTERM
)

func main() {
	dsn := mustEnv("STATS_DATABASE_URL")
	queueURL := mustEnv("STATS_QUEUE_URL")

	pollers := envInt("CONSUMER_POLLERS", 1)
	insertConc := envInt("CONSUMER_INSERT_CONCURRENCY", 1)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("pgxpool.New: %v", err)
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

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}
	sqsClient := sqs.NewFromConfig(cfg)

	h := &handler.Handler{
		Store:             store.New(pool),
		Log:               log.Default(),
		InsertConcurrency: insertConc,
	}

	log.Printf("consumer: polling %s (pollers=%d, insert_concurrency=%d)", queueURL, pollers, insertConc)

	g, gctx := errgroup.WithContext(ctx)
	for i := 0; i < pollers; i++ {
		g.Go(func() error {
			return pollLoop(gctx, sqsClient, h, queueURL)
		})
	}
	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("poll loop: %v", err)
	}
	log.Print("consumer: shutdown complete")
}

// pollLoop is the long-running loop. Returns when ctx is cancelled.
func pollLoop(ctx context.Context, sqsClient *sqs.Client, h *handler.Handler, queueURL string) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		out, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: receiveBatchSize,
			WaitTimeSeconds:     receiveWaitSeconds,
		})
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			// Transient SQS error: log and continue. A short backoff
			// keeps us from hammering the API if it's persistently broken.
			log.Printf("consumer: receive failed: %v", err)
			if !sleep(ctx, 2*time.Second) {
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
		var entries []sqstypes.DeleteMessageBatchRequestEntry
		for _, m := range out.Messages {
			if _, bad := failed[aws.ToString(m.MessageId)]; bad {
				continue // leave for redelivery
			}
			// MessageId is guaranteed unique within a ReceiveMessage batch,
			// so it satisfies DeleteMessageBatch's "Id unique within request"
			// requirement without an extra counter.
			entries = append(entries, sqstypes.DeleteMessageBatchRequestEntry{
				Id:            m.MessageId,
				ReceiptHandle: m.ReceiptHandle,
			})
		}
		if len(entries) > 0 {
			deleteRes, err := sqsClient.DeleteMessageBatch(ackCtx, &sqs.DeleteMessageBatchInput{
				QueueUrl: aws.String(queueURL),
				Entries:  entries,
			})
			if err != nil {
				log.Printf("delete batch failed: %v", err)
			} else {
				for _, f := range deleteRes.Failed {
					log.Printf("[%s] delete failed: %s", aws.ToString(f.Id), aws.ToString(f.Message))
				}
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

// envInt reads an integer env var, returning fallback if the var is unset,
// not a valid integer, or <1. Invalid values log a warning so an operator
// who misspells a value sees it in the startup log instead of silently
// getting the default.
func envInt(key string, fallback int) int {
	s := os.Getenv(key)
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		log.Printf("invalid %s=%q, using default %d", key, s, fallback)
		return fallback
	}
	return n
}
