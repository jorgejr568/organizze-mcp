package stats

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

// DefaultIngestURL and DefaultIngestToken are injected at link time via
//
//	go build -ldflags="-X 'github.com/jorgejr568/organizze-mcp/internal/stats.DefaultIngestURL=https://...'"
//
// They are intentionally empty for un-ldflagged builds (dev `go build`,
// `go test`, IDE builds). The composition root checks both and falls back
// to NoopReporter if either is missing.
var (
	DefaultIngestURL   = ""
	DefaultIngestToken = ""
)

const (
	defaultHTTPTimeout  = 3 * time.Second
	defaultDropLogEvery = 100
)

// HTTPReporter buffers events into a channel and drains them on one
// background goroutine. RecordToolCall never blocks: if the channel is
// full the event is dropped and the drop count is logged every Nth
// drop to keep log noise bounded.
type HTTPReporter struct {
	url       string
	token     string
	version   string
	transport string
	httpc     *http.Client
	events    chan Event
	log       *log.Logger
	dropped   atomic.Uint64
}

// NewHTTPReporter starts the drain goroutine immediately. The goroutine
// exits when ctx is cancelled; callers do not need to call any Close.
func NewHTTPReporter(ctx context.Context, url, token, version, transport string, bufferSize int, logger *log.Logger) *HTTPReporter {
	if logger == nil {
		logger = log.Default()
	}
	r := &HTTPReporter{
		url:       url,
		token:     token,
		version:   version,
		transport: transport,
		httpc:     &http.Client{Timeout: defaultHTTPTimeout},
		events:    make(chan Event, bufferSize),
		log:       logger,
	}
	go r.drain(ctx)
	return r
}

func (r *HTTPReporter) RecordToolCall(toolName, status, errorClass string, durationMs int64) {
	evt := Event{
		V:          1,
		Type:       "tool_call",
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		Server:     "organizze-mcp",
		Version:    r.version,
		Transport:  r.transport,
		Tool:       toolName,
		DurationMs: durationMs,
		Status:     status,
		ErrorClass: errorClass,
	}
	select {
	case r.events <- evt:
	default:
		n := r.dropped.Add(1)
		if n == 1 || n%defaultDropLogEvery == 0 {
			r.log.Printf("stats: dropped tool-call event (%d total)", n)
		}
	}
}

func (r *HTTPReporter) drain(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-r.events:
			r.send(ctx, evt)
		}
	}
}

func (r *HTTPReporter) send(ctx context.Context, evt Event) {
	body, err := json.Marshal(evt)
	if err != nil {
		r.log.Printf("stats: marshal event: %v", err)
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(body))
	if err != nil {
		r.log.Printf("stats: build request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ingest-Token", r.token)
	resp, err := r.httpc.Do(req)
	if err != nil {
		r.log.Printf("stats: post failed: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		r.log.Printf("stats: ingest responded %d for tool=%s", resp.StatusCode, evt.Tool)
	}
}
