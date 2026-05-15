package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoad_DefaultsAndRequiredFields(t *testing.T) {
	t.Setenv("ORGANIZZE_API_KEY", "k")
	t.Setenv("ORGANIZZE_EMAIL", "e@x.com")
	t.Setenv("ORGANIZZE_USER_AGENT", "App (e@x.com)")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIKey != "k" || cfg.Email != "e@x.com" || cfg.UserAgent != "App (e@x.com)" {
		t.Fatalf("required: %+v", cfg)
	}
	if cfg.Transport != "stdio" {
		t.Errorf("Transport default = %q", cfg.Transport)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr default = %q", cfg.HTTPAddr)
	}
	if cfg.BaseURL != "https://api.organizze.com.br/rest/v2" {
		t.Errorf("BaseURL default = %q", cfg.BaseURL)
	}
	if cfg.HTTPTimeout != 30*time.Second {
		t.Errorf("HTTPTimeout default = %v", cfg.HTTPTimeout)
	}
}

func TestLoad_MissingRequiredFails(t *testing.T) {
	t.Setenv("ORGANIZZE_API_KEY", "")
	t.Setenv("ORGANIZZE_EMAIL", "")
	t.Setenv("ORGANIZZE_USER_AGENT", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"ORGANIZZE_API_KEY", "ORGANIZZE_EMAIL", "ORGANIZZE_USER_AGENT"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err missing %q: %v", want, err)
		}
	}
}

func TestLoad_RejectsUnknownTransport(t *testing.T) {
	t.Setenv("ORGANIZZE_API_KEY", "k")
	t.Setenv("ORGANIZZE_EMAIL", "e@x.com")
	t.Setenv("ORGANIZZE_USER_AGENT", "ua")
	t.Setenv("MCP_TRANSPORT", "grpc")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "MCP_TRANSPORT") {
		t.Fatalf("want MCP_TRANSPORT error, got %v", err)
	}
}

func TestLoad_AcceptsHTTPAndCustomTimeout(t *testing.T) {
	t.Setenv("ORGANIZZE_API_KEY", "k")
	t.Setenv("ORGANIZZE_EMAIL", "e@x.com")
	t.Setenv("ORGANIZZE_USER_AGENT", "ua")
	t.Setenv("MCP_TRANSPORT", "HTTP")
	t.Setenv("MCP_HTTP_ADDR", ":9000")
	t.Setenv("ORGANIZZE_HTTP_TIMEOUT", "10s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Transport != "http" || cfg.HTTPAddr != ":9000" || cfg.HTTPTimeout != 10*time.Second {
		t.Errorf("got %+v", cfg)
	}
}

func TestLoad_RejectsInvalidTimeout(t *testing.T) {
	t.Setenv("ORGANIZZE_API_KEY", "k")
	t.Setenv("ORGANIZZE_EMAIL", "e@x.com")
	t.Setenv("ORGANIZZE_USER_AGENT", "ua")
	t.Setenv("ORGANIZZE_HTTP_TIMEOUT", "not-a-duration")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "ORGANIZZE_HTTP_TIMEOUT") {
		t.Fatalf("want timeout error, got %v", err)
	}
}

func TestLoad_LogRequests_DefaultsOff(t *testing.T) {
	t.Setenv("ORGANIZZE_API_KEY", "k")
	t.Setenv("ORGANIZZE_EMAIL", "e@x.com")
	t.Setenv("ORGANIZZE_USER_AGENT", "ua")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LogRequests {
		t.Errorf("LogRequests = true, want false when env var is unset")
	}
}

func TestLoad_LogRequests_OnWhenSetToOne(t *testing.T) {
	t.Setenv("ORGANIZZE_API_KEY", "k")
	t.Setenv("ORGANIZZE_EMAIL", "e@x.com")
	t.Setenv("ORGANIZZE_USER_AGENT", "ua")
	t.Setenv("ORGANIZZE_LOG_REQUESTS", "1")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.LogRequests {
		t.Errorf("LogRequests = false, want true when ORGANIZZE_LOG_REQUESTS=1")
	}
}

func TestLoad_LogRequests_OffForNonOneValues(t *testing.T) {
	t.Setenv("ORGANIZZE_API_KEY", "k")
	t.Setenv("ORGANIZZE_EMAIL", "e@x.com")
	t.Setenv("ORGANIZZE_USER_AGENT", "ua")
	for _, v := range []string{"true", "yes", "0", "false", "on", ""} {
		t.Setenv("ORGANIZZE_LOG_REQUESTS", v)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load(%q): %v", v, err)
		}
		if cfg.LogRequests {
			t.Errorf("LogRequests = true for ORGANIZZE_LOG_REQUESTS=%q, want false", v)
		}
	}
}
