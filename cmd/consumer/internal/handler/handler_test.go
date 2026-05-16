package handler

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
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
		Log:   zap.NewNop(),
	}
}

func rec(id, body string) Record {
	return Record{MessageID: id, Body: []byte(body)}
}

func TestProcess_EmptyBatch_NoFailures(t *testing.T) {
	store := &fakeStore{}
	h := newHandler(t, store)
	res := h.Process(context.Background(), nil)
	if len(res.FailedMessageIDs) != 0 {
		t.Fatalf("expected 0 failures, got %d", len(res.FailedMessageIDs))
	}
	if len(store.calls) != 0 {
		t.Fatalf("expected 0 store calls, got %d", len(store.calls))
	}
}

func TestProcess_SingleRecord_PersistsAndReportsNoFailures(t *testing.T) {
	store := &fakeStore{}
	h := newHandler(t, store)

	body := `{"stat":"page_view","count":3}`
	res := h.Process(context.Background(), []Record{rec("msg-1", body)})

	if len(res.FailedMessageIDs) != 0 {
		t.Fatalf("expected 0 failures, got %v", res.FailedMessageIDs)
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

func TestProcess_PartialFailure_ReportsOnlyFailedMessages(t *testing.T) {
	storeErr := errors.New("connection refused")
	store := &fakeStore{
		errByMsg: map[string]error{"msg-bad": storeErr},
	}
	h := newHandler(t, store)

	res := h.Process(context.Background(), []Record{
		rec("msg-1", `{"a":1}`),
		rec("msg-bad", `{"b":2}`),
		rec("msg-3", `{"c":3}`),
	})

	if len(res.FailedMessageIDs) != 1 {
		t.Fatalf("expected 1 failure, got %d (%v)", len(res.FailedMessageIDs), res.FailedMessageIDs)
	}
	if res.FailedMessageIDs[0] != "msg-bad" {
		t.Fatalf("failure id: got %q want msg-bad", res.FailedMessageIDs[0])
	}

	// The handler must NOT short-circuit on a record failure — every record
	// in the batch should have been attempted.
	if len(store.calls) != 3 {
		t.Fatalf("expected 3 store attempts, got %d", len(store.calls))
	}
}

// Sentinel: fakeStore satisfies the interface at compile time.
var _ StatsStore = (*fakeStore)(nil)
