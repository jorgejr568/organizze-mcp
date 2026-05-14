// Package domain holds the entities, value objects, and sentinel errors that
// cross layer boundaries. It depends on nothing outside the standard library.
package domain

import "errors"

// Sentinel errors. Outer layers wrap these so callers can match with errors.Is.
var (
	ErrNotFound     = errors.New("domain: not found")
	ErrUnauthorized = errors.New("domain: unauthorized")
	ErrValidation   = errors.New("domain: validation failed")
	ErrUpstream     = errors.New("domain: upstream API error")
)
