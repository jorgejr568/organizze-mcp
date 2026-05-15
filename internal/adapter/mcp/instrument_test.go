package mcp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type recordedCall struct {
	tool       string
	status     string
	errorClass string
	durationMs int64
}

type recordingReporter struct {
	mu    sync.Mutex
	calls []recordedCall
}

func (r *recordingReporter) RecordToolCall(tool, status, errorClass string, durationMs int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, recordedCall{tool, status, errorClass, durationMs})
}

func (r *recordingReporter) last() recordedCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) == 0 {
		return recordedCall{}
	}
	return r.calls[len(r.calls)-1]
}

type fakeIn struct{}
type fakeOut struct{ N int }

func handlerReturning(err error) mcpsdk.ToolHandlerFor[fakeIn, fakeOut] {
	return func(_ context.Context, _ *mcpsdk.CallToolRequest, _ fakeIn) (*mcpsdk.CallToolResult, fakeOut, error) {
		return &mcpsdk.CallToolResult{}, fakeOut{N: 7}, err
	}
}

func TestInstrument_Success_RecordsOk(t *testing.T) {
	r := &recordingReporter{}
	wrapped := instrument("greet", r, handlerReturning(nil))

	_, out, err := wrapped(context.Background(), &mcpsdk.CallToolRequest{}, fakeIn{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.N != 7 {
		t.Fatalf("output: got %+v", out)
	}

	got := r.last()
	if got.tool != "greet" || got.status != "ok" || got.errorClass != "" {
		t.Fatalf("unexpected recorded call: %+v", got)
	}
	if got.durationMs < 0 {
		t.Fatalf("negative duration: %d", got.durationMs)
	}
}

func TestInstrument_ValidationError_RecordsValidation(t *testing.T) {
	r := &recordingReporter{}
	wrapped := instrument("create_transaction", r, handlerReturning(
		fmt.Errorf("%w: amount_cents must be non-zero", domain.ErrValidation),
	))

	_, _, err := wrapped(context.Background(), &mcpsdk.CallToolRequest{}, fakeIn{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("wrapped handler must propagate the error: got %v", err)
	}
	if got := r.last(); got.status != "error" || got.errorClass != "validation" {
		t.Fatalf("unexpected recorded call: %+v", got)
	}
}

func TestInstrument_RateLimitedError_RecordsRateLimited(t *testing.T) {
	r := &recordingReporter{}
	wrapped := instrument("list_accounts", r, handlerReturning(
		fmt.Errorf("%w: try again", domain.ErrRateLimited),
	))
	_, _, _ = wrapped(context.Background(), &mcpsdk.CallToolRequest{}, fakeIn{})
	if got := r.last(); got.errorClass != "rate_limited" {
		t.Fatalf("error_class: got %q want rate_limited", got.errorClass)
	}
}

func TestInstrument_ContextCanceled_RecordsContextCanceled(t *testing.T) {
	r := &recordingReporter{}
	wrapped := instrument("any", r, handlerReturning(context.Canceled))
	_, _, _ = wrapped(context.Background(), &mcpsdk.CallToolRequest{}, fakeIn{})
	if got := r.last(); got.errorClass != "context_canceled" {
		t.Fatalf("error_class: got %q want context_canceled", got.errorClass)
	}
}

func TestInstrument_UnknownError_RecordsUnknown(t *testing.T) {
	r := &recordingReporter{}
	wrapped := instrument("any", r, handlerReturning(errors.New("boom")))
	_, _, _ = wrapped(context.Background(), &mcpsdk.CallToolRequest{}, fakeIn{})
	if got := r.last(); got.errorClass != "unknown" {
		t.Fatalf("error_class: got %q want unknown", got.errorClass)
	}
}

func TestClassifyError_NilIsEmpty(t *testing.T) {
	if got := classifyError(nil); got != "" {
		t.Fatalf("classifyError(nil): got %q want \"\"", got)
	}
}
