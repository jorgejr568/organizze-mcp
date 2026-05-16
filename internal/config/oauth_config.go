package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// OAuthConfig is the resolved env for cmd/organizze-mcp-oauth. Unlike the
// single-tenant Config, no Organizze credentials live here — they arrive
// per-request via the OAuth bearer token and are injected into the request
// context by the bearer middleware.
type OAuthConfig struct {
	DatabaseURL    string
	PublicURL      string // no trailing slash
	EncryptionKey  []byte // exactly 32 bytes (AES-256-GCM key)
	CookieSecret   []byte
	HTTPAddr       string        // defaults to :8080
	HTTPTimeout    time.Duration // upstream Organizze timeout
	OrganizzeBase  string        // defaults to https://api.organizze.com.br/rest/v2
	AccessTokenTTL time.Duration // defaults to 1h
	RefreshTTL     time.Duration // defaults to 30d
	SessionTTL     time.Duration // defaults to 24h
}

// LoadOAuth reads OAUTH_* environment variables, applies defaults, and
// validates required fields. It explicitly refuses to start when
// ORGANIZZE_API_KEY is set in the environment — the OAuth binary is the
// multi-tenant variant; a process-wide static credential is always a
// misconfiguration here, never an override.
func LoadOAuth() (*OAuthConfig, error) {
	if os.Getenv("ORGANIZZE_API_KEY") != "" {
		return nil, errors.New("ORGANIZZE_API_KEY must NOT be set for organizze-mcp-oauth (multi-tenant binary; creds come from OAuth tokens)")
	}
	cfg := &OAuthConfig{
		DatabaseURL:   os.Getenv("OAUTH_DATABASE_URL"),
		PublicURL:     strings.TrimRight(os.Getenv("OAUTH_PUBLIC_URL"), "/"),
		HTTPAddr:      os.Getenv("MCP_HTTP_ADDR"),
		OrganizzeBase: os.Getenv("ORGANIZZE_BASE_URL"),
	}
	if cfg.OrganizzeBase == "" {
		cfg.OrganizzeBase = "https://api.organizze.com.br/rest/v2"
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "OAUTH_DATABASE_URL")
	}
	if cfg.PublicURL == "" {
		missing = append(missing, "OAUTH_PUBLIC_URL")
	}
	if os.Getenv("OAUTH_ENCRYPTION_KEY") == "" {
		missing = append(missing, "OAUTH_ENCRYPTION_KEY")
	}
	if os.Getenv("OAUTH_COOKIE_SECRET") == "" {
		missing = append(missing, "OAUTH_COOKIE_SECRET")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	key, err := hex.DecodeString(os.Getenv("OAUTH_ENCRYPTION_KEY"))
	if err != nil {
		return nil, fmt.Errorf("OAUTH_ENCRYPTION_KEY: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("OAUTH_ENCRYPTION_KEY: must decode to 32 bytes, got %d", len(key))
	}
	cfg.EncryptionKey = key
	cfg.CookieSecret = []byte(os.Getenv("OAUTH_COOKIE_SECRET"))
	if len(cfg.CookieSecret) < 32 {
		return nil, fmt.Errorf("OAUTH_COOKIE_SECRET: must be at least 32 bytes, got %d", len(cfg.CookieSecret))
	}

	cfg.HTTPTimeout = 30 * time.Second
	cfg.AccessTokenTTL = time.Hour
	cfg.RefreshTTL = 30 * 24 * time.Hour
	cfg.SessionTTL = 24 * time.Hour
	return cfg, nil
}
