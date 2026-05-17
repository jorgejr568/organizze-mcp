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

	"go.uber.org/zap"
	"golang.org/x/time/rate"

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

	// CookieSecret signs the session cookie (HMAC-SHA256) and the
	// authorize-flow consent binding. Required at runtime; minimum 32 raw bytes.
	CookieSecret []byte

	// Logger receives structured records for handler-level events
	// (rate-limit hits, template-render errors, etc.). Defaults to
	// zap.NewNop() so tests need not provide one.
	Logger *zap.Logger
}

// Server is the http.Handler implementation.
type Server struct {
	cfg        Config
	mux        *http.ServeMux
	dcrLimiter *ipRateLimiter
	sessions   *sessionManager
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
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	cfg.Logger = cfg.Logger.Named("oauth")
	s := &Server{
		cfg: cfg,
		mux: http.NewServeMux(),
		// DCR is an unauthenticated write surface — cap each source IP at
		// 10 registrations per minute (burst 10). The 10_000-IP cap bounds
		// memory in case a real bot fans out across a /24.
		dcrLimiter: newIPRateLimiter(rate.Every(6*time.Second), 10, 10_000),
		// Browser-session cookies let a returning user skip re-entering their
		// Organizze credentials on subsequent authorize flows. Cookie TTL
		// mirrors the SessionTTL setting; server-side oauth_sessions rows
		// gate expiry independently so a leaked cookie cannot be replayed
		// once the DB row is gone.
		sessions: newSessionManager(cfg.CookieSecret, cfg.SessionTTL),
	}
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
	s.mux.HandleFunc("/oauth/authorize", s.handleAuthorize)
	s.mux.HandleFunc("/oauth/token", s.handleToken)
	s.mux.HandleFunc("/oauth/revoke", s.handleRevoke)
}
