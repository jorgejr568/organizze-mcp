// Package handler implements the AWS Lambda SQS-event handler for the
// stats consumer. Each invocation receives a batch of SQS messages; the
// handler stores each one and reports per-message failures back to Lambda
// so only failed records get redelivered.
package handler

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/events"
)

// StatsStore is the narrow persistence interface the handler depends on.
// PGStore (in cmd/consumer/internal/store) is the production implementation;
// tests substitute a fake.
type StatsStore interface {
	// Insert persists a single SQS message body. Implementations MUST be
	// idempotent — duplicate messageID is not an error, it is a no-op.
	Insert(ctx context.Context, messageID string, payload []byte) error
}

// Handler holds the runtime dependencies that survive across Lambda
// invocations (cold-start initialization).
type Handler struct {
	Store StatsStore
	Log   *log.Logger
}

func (h *Handler) Handle(ctx context.Context, evt events.SQSEvent) (events.SQSEventResponse, error) {
	logger := h.Log
	if logger == nil {
		logger = log.Default()
	}

	var failures []events.SQSBatchItemFailure
	for _, rec := range evt.Records {
		if err := h.Store.Insert(ctx, rec.MessageId, []byte(rec.Body)); err != nil {
			logger.Printf("[%s] store insert failed: %v", rec.MessageId, err)
			failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: rec.MessageId})
			continue
		}
		logger.Printf("[%s] persisted (%d bytes)", rec.MessageId, len(rec.Body))
	}
	return events.SQSEventResponse{BatchItemFailures: failures}, nil
}
