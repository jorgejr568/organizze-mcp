package organizze

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

func TestAPIError_Error_FormatsStatusAndMessage(t *testing.T) {
	e := &APIError{StatusCode: 422, Message: "validation failed"}
	if got := e.Error(); got != "organizze: 422 validation failed" {
		t.Errorf("Error() = %q", got)
	}
}

func TestAPIError_MapsToDomainSentinels(t *testing.T) {
	cases := []struct {
		status   int
		sentinel error
	}{
		{http.StatusNotFound, domain.ErrNotFound},
		{http.StatusUnauthorized, domain.ErrUnauthorized},
		{http.StatusForbidden, domain.ErrUnauthorized},
		{http.StatusUnprocessableEntity, domain.ErrValidation},
		{http.StatusBadRequest, domain.ErrValidation},
		{http.StatusTooManyRequests, domain.ErrRateLimited},
		{http.StatusInternalServerError, domain.ErrUpstream},
	}
	for _, c := range cases {
		err := &APIError{StatusCode: c.status}
		if !errors.Is(err, c.sentinel) {
			t.Errorf("status %d should map to %v; got Is=false", c.status, c.sentinel)
		}
	}
}

func TestAPIError_UnknownStatusMapsToUpstream(t *testing.T) {
	err := &APIError{StatusCode: 418}
	if !errors.Is(err, domain.ErrUpstream) {
		t.Error("unknown status should map to ErrUpstream")
	}
}

func TestAPIError_429MapsToRateLimited(t *testing.T) {
	err := &APIError{StatusCode: http.StatusTooManyRequests}
	if !errors.Is(err, domain.ErrRateLimited) {
		t.Error("429 should map to ErrRateLimited")
	}
	if errors.Is(err, domain.ErrUpstream) {
		t.Error("429 should NOT also map to ErrUpstream (one sentinel per status)")
	}
}
