// Package stats holds the MCP server's tool-call telemetry. The reporter
// is intentionally narrow (one method) so the MCP adapter doesn't depend
// on the wire format. The HTTPReporter in this same package owns the
// wire shape and the background drain.
package stats

// Event is the JSON document POSTed to the ingest endpoint.
//
// Only non-sensitive fields are ever populated — tool name (the public
// MCP tool identifier), timing, status, and a coarse error class. Tool
// arguments, return values, account IDs, and free-text error messages
// are deliberately omitted; new fields must be approved against this
// non-sensitivity rule.
type Event struct {
	V          int    `json:"v"`
	Type       string `json:"type"`
	Timestamp  string `json:"ts"`
	Server     string `json:"server"`
	Version    string `json:"version"`
	Transport  string `json:"transport"`
	Tool       string `json:"tool"`
	DurationMs int64  `json:"duration_ms"`
	Status     string `json:"status"`
	ErrorClass string `json:"error_class,omitempty"`
}

// Reporter is the narrow seam the MCP adapter depends on.
//
// Implementations MUST NOT block — callers (tool-call wrappers) are
// on the request hot path. If the reporter cannot accept the event
// immediately it should drop it, not wait.
type Reporter interface {
	RecordToolCall(toolName, status, errorClass string, durationMs int64)
}

// NoopReporter is the default when stats are opted-out or when no
// ingest URL/token is configured at build time and not overridden
// at runtime.
type NoopReporter struct{}

func (NoopReporter) RecordToolCall(_, _, _ string, _ int64) {}
