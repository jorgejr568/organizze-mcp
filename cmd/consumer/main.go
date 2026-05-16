// Command consumer drains the stats SQS queue into Postgres. It runs as a
// long-running container (e.g. on ECS / Fargate / k8s) — not a Lambda.
//
// The poll loop uses long polling (WaitTimeSeconds=20, batch size 10),
// hands each batch to handler.Process, and deletes each successful
// message individually. Failed messages are left in the queue so SQS
// re-delivers them once their visibility timeout expires.
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

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
	logger := mustLogger()
	defer func() { _ = logger.Sync() }()

	dsn := mustEnv(logger, "STATS_DATABASE_URL")
	queueURL := mustEnv(logger, "STATS_QUEUE_URL")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		logger.Fatal("pgxpool.New", zap.Error(err))
	}
	defer pool.Close()

	// Fail fast on a bad DSN: ping with a short deadline at startup
	// instead of crashing on the first message.
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	if err := pool.Ping(pingCtx); err != nil {
		cancel()
		logger.Fatal("postgres ping", zap.Error(err))
	}
	cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Fatal("aws config", zap.Error(err))
	}
	sqsClient := sqs.NewFromConfig(cfg)

	h := &handler.Handler{
		Store: store.New(pool),
		Log:   logger,
	}

	logger.Info("consumer polling", zap.String("queue_url", queueURL))
	if err := pollLoop(ctx, logger, sqsClient, h, queueURL); err != nil && !errors.Is(err, context.Canceled) {
		logger.Fatal("poll loop", zap.Error(err))
	}
	logger.Info("consumer shutdown complete")
}

// mustLogger returns the production JSON logger. Container runtimes (ECS / Fargate /
// k8s) ship stderr to their respective log aggregators, which parse JSON-encoded
// records into structured fields.
func mustLogger() *zap.Logger {
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{"stderr"}
	cfg.ErrorOutputPaths = []string{"stderr"}
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	l, err := cfg.Build()
	if err != nil {
		panic("build logger: " + err.Error())
	}
	return l
}

// pollLoop is the long-running loop. Returns when ctx is cancelled.
func pollLoop(ctx context.Context, logger *zap.Logger, sqsClient *sqs.Client, h *handler.Handler, queueURL string) error {
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
			logger.Error("receive failed", zap.Error(err))
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
		for _, m := range out.Messages {
			if _, bad := failed[aws.ToString(m.MessageId)]; bad {
				continue // leave for redelivery
			}
			if _, err := sqsClient.DeleteMessage(ackCtx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(queueURL),
				ReceiptHandle: m.ReceiptHandle,
			}); err != nil {
				logger.Error("delete failed",
					zap.String("message_id", aws.ToString(m.MessageId)),
					zap.Error(err),
				)
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

func mustEnv(logger *zap.Logger, key string) string {
	v := os.Getenv(key)
	if v == "" {
		logger.Fatal("missing required env var", zap.String("key", key))
	}
	return v
}
