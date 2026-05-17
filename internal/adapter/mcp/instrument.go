package mcp

import (
	"context"
	"errors"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
	"github.com/jorgejr568/organizze-mcp/internal/stats"
)

// instrumentation bundles per-tool-call telemetry: the structured logger
// for stderr-side observability and the reporter for the off-box stats
// pipeline. The two share the same non-sensitive vocabulary (tool name,
// status, error_class, duration) — never tool arguments or return values.
type instrumentation struct {
	reporter stats.Reporter
	logger   *zap.Logger
}

func (i instrumentation) normalize() instrumentation {
	if i.reporter == nil {
		i.reporter = stats.NoopReporter{}
	}
	if i.logger == nil {
		i.logger = zap.NewNop()
	}
	return i
}

// instrument wraps a tool handler with timing + error classification +
// fire-and-forget stats reporting + a structured log line per call. Split
// out from addInstrumentedTool so unit tests can exercise the wrapped
// handler directly without going through the MCP server registration.
func instrument[In, Out any](name string, inst instrumentation, h mcpsdk.ToolHandlerFor[In, Out]) mcpsdk.ToolHandlerFor[In, Out] {
	inst = inst.normalize()
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, in In) (*mcpsdk.CallToolResult, Out, error) {
		start := time.Now()
		res, out, err := h(ctx, req, in)
		durationMs := time.Since(start).Milliseconds()
		status := "ok"
		errorClass := classifyError(err)
		if err != nil {
			status = "error"
		}
		inst.reporter.RecordToolCall(name, status, errorClass, durationMs)

		// Structured log fields stay aligned with the stats vocabulary —
		// no tool arguments, no return values, no free-text error
		// messages. Operators get a per-call timeline in their container
		// log stream; deeper diagnostics live in the stats DB.
		fields := []zap.Field{
			zap.String("tool", name),
			zap.String("status", status),
			zap.Int64("duration_ms", durationMs),
		}
		if errorClass != "" {
			fields = append(fields, zap.String("error_class", errorClass))
		}
		if status == "ok" {
			inst.logger.Info("tool call", fields...)
		} else {
			inst.logger.Warn("tool call failed", fields...)
		}
		return res, out, err
	}
}

// addInstrumentedTool is a drop-in replacement for mcpsdk.AddTool that
// wires the handler through `instrument`. Tool registration files
// (tools_*.go) call this instead of mcpsdk.AddTool directly.
func addInstrumentedTool[In, Out any](s *mcpsdk.Server, inst instrumentation, t *mcpsdk.Tool, h mcpsdk.ToolHandlerFor[In, Out]) {
	mcpsdk.AddTool(s, t, instrument(t.Name, inst, h))
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
