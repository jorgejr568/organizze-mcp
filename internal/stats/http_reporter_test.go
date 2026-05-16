package stats

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestHTTPReporter_PostsEventToIngestURL(t *testing.T) {
	var (
		mu       sync.Mutex
		got      Event
		gotToken string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ev Event
		token := r.Header.Get("X-Ingest-Token")
		if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
			t.Errorf("decode body: %v", err)
		}
		mu.Lock()
		got = ev
		gotToken = token
		mu.Unlock()
		w.WriteHeader(202)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := NewHTTPReporter(ctx, srv.URL, "shh-token", "1.2.3", "stdio", 8, zap.NewNop())

	r.RecordToolCall("list_transactions", "ok", "", 17)

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return got.Tool != ""
	})

	mu.Lock()
	defer mu.Unlock()
	if got.Tool != "list_transactions" {
		t.Fatalf("tool: got %q", got.Tool)
	}
	if got.Status != "ok" || got.ErrorClass != "" {
		t.Fatalf("status/error_class: got %q/%q", got.Status, got.ErrorClass)
	}
	if got.Version != "1.2.3" || got.Transport != "stdio" {
		t.Fatalf("version/transport: got %q/%q", got.Version, got.Transport)
	}
	if got.DurationMs != 17 {
		t.Fatalf("duration: got %d", got.DurationMs)
	}
	if got.Type != "tool_call" || got.V != 1 {
		t.Fatalf("type/v: got %q/%d", got.Type, got.V)
	}
	if got.Server != "organizze-mcp" {
		t.Fatalf("server: got %q", got.Server)
	}
	if gotToken != "shh-token" {
		t.Fatalf("token header: got %q", gotToken)
	}
}

func TestHTTPReporter_RecordIsNonBlockingWhenServerIsSlow(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
		w.WriteHeader(202)
	}))
	defer srv.Close()
	defer close(block)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := NewHTTPReporter(ctx, srv.URL, "tok", "v", "t", 4, zap.NewNop())

	// Record should not block even though the server is hanging on the
	// in-flight POST. Time 100 invocations and assert wall-clock budget.
	start := time.Now()
	for i := 0; i < 100; i++ {
		r.RecordToolCall("t", "ok", "", 1)
	}
	if d := time.Since(start); d > 200*time.Millisecond {
		t.Fatalf("Record took too long: %v (expected < 200ms)", d)
	}
}

func TestHTTPReporter_DropsEventsWhenBufferFull(t *testing.T) {
	core, logs := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)

	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer srv.Close()
	defer close(block)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Buffer size 2 + drain goroutine holding one inflight = 3 absorbed
	// before drops begin.
	r := NewHTTPReporter(ctx, srv.URL, "tok", "v", "t", 2, logger)

	for i := 0; i < 50; i++ {
		r.RecordToolCall("t", "ok", "", 1)
	}

	waitFor(t, 1*time.Second, func() bool {
		return logs.FilterMessage("dropped tool-call event").Len() > 0
	})
	if logs.FilterMessage("dropped tool-call event").Len() == 0 {
		t.Fatalf("expected drop warning in log; entries:\n%v", logs.All())
	}
}

func TestHTTPReporter_LogsWhenIngestReturnsError(t *testing.T) {
	core, logs := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := NewHTTPReporter(ctx, srv.URL, "tok", "v", "t", 4, logger)
	r.RecordToolCall("t", "ok", "", 1)

	waitFor(t, 2*time.Second, func() bool {
		return logs.FilterMessage("ingest non-success").Len() > 0
	})
	entries := logs.FilterMessage("ingest non-success").All()
	if len(entries) == 0 {
		t.Fatalf("expected ingest non-success log; entries:\n%v", logs.All())
	}
	gotStatus, _ := entries[0].ContextMap()["status"].(int64)
	if gotStatus != 500 {
		t.Fatalf("status field: got %v want 500", gotStatus)
	}
}

func waitFor(t *testing.T, max time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}
