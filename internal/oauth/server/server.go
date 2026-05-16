// Package server hosts the OAuth 2.1 Authorization Server endpoints and the
// bearer middleware that fronts the MCP handler. Construct with New, then
// either ServeHTTP directly (tests) or mount the handler in a real
// *http.Server (cmd/organizze-mcp-oauth/main.go).
package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

// Config holds compile-time-injected dependencies and tunables.
type Config struct {
	// PublicURL is the externally reachable origin (no trailing slash),
	// e.g. "https://mcp.example.com". Used to render absolute URLs in
	// discovery documents and the redirect_uri match.
	PublicURL string

	// Store is the persistence layer. Required at runtime; tests that
	// only exercise discovery may omit it.
	Store storage.Store

	// Cipher seals/unseals the Organizze API key. Required at runtime.
	Cipher *storage.Cipher

	// ValidateOrganizze checks an email+key pair against the live Organizze
	// API and returns nil on success. Required at runtime; tests inject a
	// fake.
	ValidateOrganizze func(ctx context.Context, email, apiKey, userAgent string) error

	// Now returns the current wall-clock time. Defaults to time.Now.UTC.
	Now func() time.Time

	// AccessTokenTTL defaults to 1h. RefreshTokenTTL defaults to 30d.
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	// SessionTTL defaults to 24h.
	SessionTTL time.Duration

	// CookieSecret signs the session cookie (HMAC-SHA256). Required at runtime.
	CookieSecret []byte
}

// Server is the http.Handler implementation.
type Server struct {
	cfg Config
	mux *http.ServeMux
}

// New constructs a Server with all routes mounted.
func New(cfg Config) *Server {
	cfg.PublicURL = strings.TrimRight(cfg.PublicURL, "/")
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	if cfg.AccessTokenTTL == 0 {
		cfg.AccessTokenTTL = time.Hour
	}
	if cfg.RefreshTokenTTL == 0 {
		cfg.RefreshTokenTTL = 30 * 24 * time.Hour
	}
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = 24 * time.Hour
	}
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/.well-known/oauth-protected-resource", s.handleProtectedResource)
	s.mux.HandleFunc("/.well-known/oauth-authorization-server", s.handleAuthorizationServer)
	s.mux.HandleFunc("/oauth/register", s.handleRegister)
	// /authorize, /token, /revoke, /mcp registered in later tasks.
}
