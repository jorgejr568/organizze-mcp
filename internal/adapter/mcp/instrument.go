package mcp

import (
	"context"
	"errors"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
	"github.com/jorgejr568/organizze-mcp/internal/stats"
)

// instrument wraps a tool handler with timing + error classification +
// fire-and-forget stats reporting. It is split out from addInstrumentedTool
// so unit tests can exercise the wrapped handler directly without going
// through the MCP server registration.
func instrument[In, Out any](name string, r stats.Reporter, h mcpsdk.ToolHandlerFor[In, Out]) mcpsdk.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, in In) (*mcpsdk.CallToolResult, Out, error) {
		start := time.Now()
		res, out, err := h(ctx, req, in)
		status := "ok"
		if err != nil {
			status = "error"
		}
		r.RecordToolCall(name, status, classifyError(err), time.Since(start).Milliseconds())
		return res, out, err
	}
}

// addInstrumentedTool is a drop-in replacement for mcpsdk.AddTool that
// wires the handler through `instrument`. Tool registration files
// (tools_*.go) call this instead of mcpsdk.AddTool directly.
func addInstrumentedTool[In, Out any](s *mcpsdk.Server, r stats.Reporter, t *mcpsdk.Tool, h mcpsdk.ToolHandlerFor[In, Out]) {
	if r == nil {
		r = stats.NoopReporter{}
	}
	mcpsdk.AddTool(s, t, instrument(t.Name, r, h))
}

// classifyError maps domain sentinels to a small fixed vocabulary so the
// ingest endpoint never sees a free-text error message (which could leak
// account IDs, descriptions, etc.).
func classifyError(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, domain.ErrValidation):
		return "validation"
	case errors.Is(err, domain.ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "context_canceled"
	default:
		return "unknown"
	}
}
