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
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jorgejr568/organizze-mcp/cmd/consumer/internal/handler"
	"github.com/jorgejr568/organizze-mcp/cmd/consumer/internal/store"
)

const (
	receiveBatchSize   = 10 // SQS max
	receiveWaitSeconds = 20 // long-poll
	pingTimeout        = 5 * time.Second
)

func main() {
	dsn := mustEnv("STATS_DATABASE_URL")
	queueURL := mustEnv("STATS_QUEUE_URL")

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
		Store: store.New(pool),
		Log:   log.Default(),
	}

	log.Printf("consumer: polling %s", queueURL)
	if err := pollLoop(ctx, sqsClient, h, queueURL); err != nil && !errors.Is(err, context.Canceled) {
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
		for _, m := range out.Messages {
			if _, bad := failed[aws.ToString(m.MessageId)]; bad {
				continue // leave for redelivery
			}
			if _, err := sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(queueURL),
				ReceiptHandle: m.ReceiptHandle,
			}); err != nil {
				log.Printf("[%s] delete failed: %v", aws.ToString(m.MessageId), err)
			}
		}
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
