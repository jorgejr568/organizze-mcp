// Package handler implements the consumer's per-batch persistence logic.
// The consumer's main loop receives SQS messages, builds a slice of
// Records, hands them to Handler.Process, and uses the returned set of
// failed message IDs to decide which messages to leave for redelivery.
//
// The handler is intentionally decoupled from any specific runtime
// (Lambda, container, test) — callers translate their event source into
// []Record and consume []FailedMessageID. This package depends only on
// the StatsStore interface; the production pgx-backed implementation
// lives in cmd/consumer/internal/store.
package handler

import (
	"context"
	"log"
)

// StatsStore is the narrow persistence interface the handler depends on.
// PGStore (in cmd/consumer/internal/store) is the production implementation;
// tests substitute a fake.
type StatsStore interface {
	// Insert persists a single message body. Implementations MUST be
	// idempotent — duplicate messageID is not an error, it is a no-op.
	Insert(ctx context.Context, messageID string, payload []byte) error
}

// Record is one message to persist. MessageID is the SQS message ID
// (used as the idempotency key by the store). Body is the raw JSON
// payload forwarded by the ingest Lambda.
type Record struct {
	MessageID string
	Body      []byte
}

// Result captures which records failed to persist. Callers use this
// to decide which SQS messages to keep for redelivery (anything in
// FailedMessageIDs) vs. delete (anything not in it).
type Result struct {
	FailedMessageIDs []string
}

// Handler holds the runtime dependencies that survive across batches.
type Handler struct {
	Store StatsStore
	Log   *log.Logger
}

// Process iterates the batch, calling Store.Insert for each record.
// Every record is attempted — a failure on one does not short-circuit
// the rest. Failures are logged with the message ID as the correlation
// key and collected into Result.FailedMessageIDs.
func (h *Handler) Process(ctx context.Context, records []Record) Result {
	logger := h.Log
	if logger == nil {
		logger = log.Default()
	}

	var failed []string
	for _, rec := range records {
		if err := h.Store.Insert(ctx, rec.MessageID, rec.Body); err != nil {
			logger.Printf("[%s] store insert failed: %v", rec.MessageID, err)
			failed = append(failed, rec.MessageID)
			continue
		}
		logger.Printf("[%s] persisted (%d bytes)", rec.MessageID, len(rec.Body))
	}
	return Result{FailedMessageIDs: failed}
}
