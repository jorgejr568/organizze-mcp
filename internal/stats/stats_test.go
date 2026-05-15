package stats

import (
	"encoding/json"
	"testing"
)

func TestEvent_JSONShape_OmitsEmptyErrorClass(t *testing.T) {
	b, err := json.Marshal(Event{
		V: 1, Type: "tool_call", Timestamp: "2026-05-15T00:00:00Z",
		Server: "organizze-mcp", Version: "0.7.0", Transport: "stdio",
		Tool: "list_transactions", DurationMs: 42, Status: "ok",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `"status":"ok"`; !contains(string(b), want) {
		t.Fatalf("expected %q in %s", want, b)
	}
	if contains(string(b), `"error_class"`) {
		t.Fatalf("error_class should be omitted when empty: %s", b)
	}
}

func TestEvent_JSONShape_IncludesErrorClassWhenSet(t *testing.T) {
	b, err := json.Marshal(Event{Status: "error", ErrorClass: "validation"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !contains(string(b), `"error_class":"validation"`) {
		t.Fatalf("expected error_class in %s", b)
	}
}

func TestNoopReporter_DoesNothing(t *testing.T) {
	// Should not panic, should not block.
	r := NoopReporter{}
	for i := 0; i < 1000; i++ {
		r.RecordToolCall("any_tool", "ok", "", 1)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
