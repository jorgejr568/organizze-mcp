package organizze

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// APIError is returned by the executor for any non-2xx Organizze response.
// Use errors.As to inspect details; errors.Is matches it to domain sentinels.
type APIError struct {
	StatusCode int    // HTTP status
	Message    string // best-effort message pulled from the JSON body
	Body       string // raw body (truncated to 1 KiB)
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("organizze: %d %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("organizze: %d", e.StatusCode)
}

// Is supports errors.Is(err, domain.ErrXxx) — maps HTTP status to domain sentinels.
func (e *APIError) Is(target error) bool {
	switch e.StatusCode {
	case http.StatusNotFound:
		return target == domain.ErrNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return target == domain.ErrUnauthorized
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return target == domain.ErrValidation
	default:
		return target == domain.ErrUpstream
	}
}

// parseAPIError reads a non-2xx response and constructs an APIError.
func parseAPIError(resp *http.Response) *APIError {
	const maxBody = 1 << 10
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	apiErr := &APIError{StatusCode: resp.StatusCode, Body: string(raw)}

	var payload map[string]any
	if json.Unmarshal(raw, &payload) == nil {
		if m, ok := payload["message"].(string); ok && m != "" {
			apiErr.Message = m
		} else if m, ok := payload["error"].(string); ok && m != "" {
			apiErr.Message = m
		}
	}
	return apiErr
}
