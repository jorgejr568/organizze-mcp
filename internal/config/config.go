// Package config loads and validates the environment configuration
// for the organizze-mcp server.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config is the resolved runtime configuration.
type Config struct {
	APIKey      string
	Email       string
	UserAgent   string
	BaseURL     string
	HTTPTimeout time.Duration

	Transport string // "stdio" | "http"
	HTTPAddr  string // listen address when Transport == "http"
}

// Load reads configuration from environment variables, applying defaults and
// validating required fields and enum-like values. Errors list every problem.
func Load() (*Config, error) {
	cfg := &Config{
		APIKey:    os.Getenv("ORGANIZZE_API_KEY"),
		Email:     os.Getenv("ORGANIZZE_EMAIL"),
		UserAgent: os.Getenv("ORGANIZZE_USER_AGENT"),
		BaseURL:   os.Getenv("ORGANIZZE_BASE_URL"),
		Transport: strings.ToLower(strings.TrimSpace(os.Getenv("MCP_TRANSPORT"))),
		HTTPAddr:  os.Getenv("MCP_HTTP_ADDR"),
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.organizze.com.br/rest/v2"
	}
	if cfg.Transport == "" {
		cfg.Transport = "stdio"
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}

	timeoutStr := os.Getenv("ORGANIZZE_HTTP_TIMEOUT")
	if timeoutStr == "" {
		cfg.HTTPTimeout = 30 * time.Second
	} else {
		d, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return nil, fmt.Errorf("ORGANIZZE_HTTP_TIMEOUT %q: %w", timeoutStr, err)
		}
		cfg.HTTPTimeout = d
	}

	var missing []string
	if cfg.APIKey == "" {
		missing = append(missing, "ORGANIZZE_API_KEY")
	}
	if cfg.Email == "" {
		missing = append(missing, "ORGANIZZE_EMAIL")
	}
	if cfg.UserAgent == "" {
		missing = append(missing, "ORGANIZZE_USER_AGENT")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	switch cfg.Transport {
	case "stdio", "http":
	default:
		return nil, fmt.Errorf("invalid MCP_TRANSPORT %q (expected stdio or http)", cfg.Transport)
	}

	return cfg, nil
}
