package handler

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"testing"
)

type call struct {
	messageID string
	payload   []byte
}

type fakeStore struct {
	mu       sync.Mutex
	calls    []call
	errByMsg map[string]error // optional: return err for specific message IDs
}

func (s *fakeStore) Insert(_ context.Context, messageID string, payload []byte) error {
	s.mu.Lock()
	s.calls = append(s.calls, call{messageID: messageID, payload: append([]byte(nil), payload...)})
	err, ok := s.errByMsg[messageID]
	s.mu.Unlock()
	if ok {
		return err
	}
	return nil
}

func (s *fakeStore) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func (s *fakeStore) callAt(i int) call {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[i]
}

func newHandler(t *testing.T, store StatsStore) *Handler {
	t.Helper()
	return &Handler{
		Store:             store,
		Log:               log.New(io.Discard, "", 0),
		InsertConcurrency: 1,
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
	if store.callCount() != 0 {
		t.Fatalf("expected 0 store calls, got %d", store.callCount())
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
	if store.callCount() != 1 {
		t.Fatalf("expected 1 store call, got %d", store.callCount())
	}
	got := store.callAt(0)
	if got.messageID != "msg-1" {
		t.Fatalf("messageID: got %q want msg-1", got.messageID)
	}
	if string(got.payload) != body {
		t.Fatalf("payload: got %q want %q", got.payload, body)
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
	if store.callCount() != 3 {
		t.Fatalf("expected 3 store attempts, got %d", store.callCount())
	}
}

// gatedStore blocks each Insert until a token is received on gate, signaling
// arrival on entered. Lets the test wait for all workers to be in flight
// before releasing them, removing the start-up race.
type gatedStore struct {
	gate    chan struct{}
	entered chan struct{}
	track   func()
	untrack func()
}

func (s *gatedStore) Insert(_ context.Context, _ string, _ []byte) error {
	s.track()
	s.entered <- struct{}{}
	<-s.gate
	s.untrack()
	return nil
}

func TestHandle_ConcurrentInserts_RunsUpToLimit(t *testing.T) {
	const limit = 4
	const total = 5
	gate := make(chan struct{})
	entered := make(chan struct{}, total)
	var (
		mu          sync.Mutex
		inflight    int
		maxInflight int
	)
	store := &gatedStore{
		gate:    gate,
		entered: entered,
		track: func() {
			mu.Lock()
			inflight++
			if inflight > maxInflight {
				maxInflight = inflight
			}
			mu.Unlock()
		},
		untrack: func() {
			mu.Lock()
			inflight--
			mu.Unlock()
		},
	}

	h := &Handler{Store: store, Log: log.New(io.Discard, "", 0), InsertConcurrency: limit}

	done := make(chan struct{})
	go func() {
		_ = h.Process(context.Background(), []Record{
			{MessageID: "m1", Body: []byte(`{}`)},
			{MessageID: "m2", Body: []byte(`{}`)},
			{MessageID: "m3", Body: []byte(`{}`)},
			{MessageID: "m4", Body: []byte(`{}`)},
			{MessageID: "m5", Body: []byte(`{}`)},
		})
		close(done)
	}()

	// Wait for `limit` workers to have entered Insert before releasing any.
	// This pins the inflight count to `limit` at observation time without
	// relying on goroutine scheduling.
	for range limit {
		<-entered
	}
	mu.Lock()
	got := maxInflight
	mu.Unlock()
	if got != limit {
		t.Fatalf("maxInflight while gated: got %d want %d (InsertConcurrency)", got, limit)
	}

	// Release them all so Process returns and the test can finish cleanly.
	for range total {
		gate <- struct{}{}
	}
	// Drain the remaining entered signals to keep the channel-send in
	// Insert non-blocking for the 5th worker.
	for range total - limit {
		<-entered
	}
	<-done
}

// Sentinel: fakeStore satisfies the interface at compile time.
var _ StatsStore = (*fakeStore)(nil)
var _ StatsStore = (*gatedStore)(nil)
