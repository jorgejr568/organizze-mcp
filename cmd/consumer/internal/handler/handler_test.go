package handler

import (
	"context"
	"errors"
	"io"
	"log"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

type call struct {
	messageID string
	payload   []byte
}

type fakeStore struct {
	calls    []call
	errByMsg map[string]error // optional: return err for specific message IDs
}

func (s *fakeStore) Insert(_ context.Context, messageID string, payload []byte) error {
	s.calls = append(s.calls, call{messageID: messageID, payload: append([]byte(nil), payload...)})
	if err, ok := s.errByMsg[messageID]; ok {
		return err
	}
	return nil
}

func newHandler(t *testing.T, store StatsStore) *Handler {
	t.Helper()
	return &Handler{
		Store: store,
		Log:   log.New(io.Discard, "", 0),
	}
}

func sqsRec(id, body string) events.SQSMessage {
	return events.SQSMessage{MessageId: id, Body: body}
}

func TestHandle_EmptyEvent_NoFailures(t *testing.T) {
	store := &fakeStore{}
	h := newHandler(t, store)
	resp, err := h.Handle(context.Background(), events.SQSEvent{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.BatchItemFailures) != 0 {
		t.Fatalf("expected 0 failures, got %d", len(resp.BatchItemFailures))
	}
	if len(store.calls) != 0 {
		t.Fatalf("expected 0 store calls, got %d", len(store.calls))
	}
}

func TestHandle_SingleRecord_PersistsAndReportsNoFailures(t *testing.T) {
	store := &fakeStore{}
	h := newHandler(t, store)

	body := `{"stat":"page_view","count":3}`
	resp, err := h.Handle(context.Background(), events.SQSEvent{
		Records: []events.SQSMessage{sqsRec("msg-1", body)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.BatchItemFailures) != 0 {
		t.Fatalf("expected 0 failures, got %d (%v)", len(resp.BatchItemFailures), resp.BatchItemFailures)
	}
	if len(store.calls) != 1 {
		t.Fatalf("expected 1 store call, got %d", len(store.calls))
	}
	if store.calls[0].messageID != "msg-1" {
		t.Fatalf("messageID: got %q want msg-1", store.calls[0].messageID)
	}
	if string(store.calls[0].payload) != body {
		t.Fatalf("payload: got %q want %q", store.calls[0].payload, body)
	}
}

func TestHandle_PartialFailure_ReportsOnlyFailedMessages(t *testing.T) {
	storeErr := errors.New("connection refused")
	store := &fakeStore{
		errByMsg: map[string]error{"msg-bad": storeErr},
	}
	h := newHandler(t, store)

	resp, err := h.Handle(context.Background(), events.SQSEvent{
		Records: []events.SQSMessage{
			sqsRec("msg-1", `{"a":1}`),
			sqsRec("msg-bad", `{"b":2}`),
			sqsRec("msg-3", `{"c":3}`),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.BatchItemFailures) != 1 {
		t.Fatalf("expected 1 failure, got %d (%v)", len(resp.BatchItemFailures), resp.BatchItemFailures)
	}
	if got := resp.BatchItemFailures[0].ItemIdentifier; got != "msg-bad" {
		t.Fatalf("failure ItemIdentifier: got %q want msg-bad", got)
	}

	// The handler must NOT short-circuit on a record failure — every record
	// in the batch should have been attempted.
	if len(store.calls) != 3 {
		t.Fatalf("expected 3 store attempts, got %d", len(store.calls))
	}
}

// Sentinel: fakeStore satisfies the interface at compile time.
var _ StatsStore = (*fakeStore)(nil)
