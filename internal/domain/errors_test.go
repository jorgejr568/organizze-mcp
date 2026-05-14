package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestSentinelsAreDistinct(t *testing.T) {
	if errors.Is(ErrNotFound, ErrUnauthorized) {
		t.Error("ErrNotFound must not match ErrUnauthorized")
	}
	if errors.Is(ErrUnauthorized, ErrValidation) {
		t.Error("ErrUnauthorized must not match ErrValidation")
	}
}

func TestWrappedSentinelMatches(t *testing.T) {
	wrapped := fmt.Errorf("transaction 42: %w", ErrNotFound)
	if !errors.Is(wrapped, ErrNotFound) {
		t.Error("errors.Is must traverse wrapping")
	}
}

func TestErrRateLimited_IsDistinctSentinel(t *testing.T) {
	if errors.Is(ErrRateLimited, ErrUpstream) {
		t.Error("ErrRateLimited must not match ErrUpstream")
	}
	if errors.Is(ErrRateLimited, ErrNotFound) {
		t.Error("ErrRateLimited must not match ErrNotFound")
	}
	wrapped := fmt.Errorf("organizze: throttled: %w", ErrRateLimited)
	if !errors.Is(wrapped, ErrRateLimited) {
		t.Error("errors.Is must traverse wrapping")
	}
}
