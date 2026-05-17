// Package credprovider supplies per-request Organizze credentials to the
// RequestExecutor. The single-tenant binary uses Static; the OAuth binary
// uses FromContext after bearer-auth middleware has populated the context.
package credprovider

import (
	"context"
	"errors"
)

// ErrNoCredentials means the request context did not carry credentials.
// In the OAuth binary this should be impossible past the bearer middleware;
// surfacing it means a tool was invoked outside the authenticated path.
var ErrNoCredentials = errors.New("credprovider: no credentials in context")

// Credentials is the per-request triple the Organizze API needs.
type Credentials struct {
	Email     string
	APIKey    string
	UserAgent string
}

// CredentialsProvider resolves credentials for a single outbound call.
// Implementations must be safe for concurrent use.
type CredentialsProvider func(ctx context.Context) (email, apiKey, userAgent string, err error)

// Static returns a provider that always yields the given values.
func Static(email, apiKey, userAgent string) CredentialsProvider {
	return func(_ context.Context) (string, string, string, error) {
		return email, apiKey, userAgent, nil
	}
}

// FromContext reads credentials that WithCredentials placed on ctx.
func FromContext(ctx context.Context) (email, apiKey, userAgent string, err error) {
	c, ok := ctx.Value(ctxKey{}).(Credentials)
	if !ok {
		return "", "", "", ErrNoCredentials
	}
	return c.Email, c.APIKey, c.UserAgent, nil
}

// WithCredentials returns a child ctx carrying c.
func WithCredentials(ctx context.Context, c Credentials) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

type ctxKey struct{}
