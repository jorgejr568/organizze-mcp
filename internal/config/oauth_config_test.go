package config

import (
	"strings"
	"testing"
)

func TestLoadOAuth_RejectsSingleTenantEnv(t *testing.T) {
	t.Setenv("OAUTH_DATABASE_URL", "postgres://x")
	t.Setenv("OAUTH_PUBLIC_URL", "https://x")
	t.Setenv("OAUTH_ENCRYPTION_KEY", strings.Repeat("0", 64))
	t.Setenv("OAUTH_COOKIE_SECRET", strings.Repeat("c", 32))
	t.Setenv("ORGANIZZE_API_KEY", "must-not-be-set")
	if _, err := LoadOAuth(); err == nil {
		t.Error("expected error when ORGANIZZE_API_KEY is set")
	}
}

func TestLoadOAuth_RequiresAllVars(t *testing.T) {
	required := []string{"OAUTH_DATABASE_URL", "OAUTH_PUBLIC_URL", "OAUTH_ENCRYPTION_KEY", "OAUTH_COOKIE_SECRET"}
	for _, missing := range required {
		t.Setenv("OAUTH_DATABASE_URL", "postgres://x")
		t.Setenv("OAUTH_PUBLIC_URL", "https://x")
		t.Setenv("OAUTH_ENCRYPTION_KEY", strings.Repeat("0", 64))
		t.Setenv("OAUTH_COOKIE_SECRET", strings.Repeat("c", 32))
		t.Setenv("ORGANIZZE_API_KEY", "")
		t.Setenv(missing, "")
		if _, err := LoadOAuth(); err == nil {
			t.Errorf("missing %s should error", missing)
		}
	}
}

func TestLoadOAuth_RejectsBadEncryptionKey(t *testing.T) {
	t.Setenv("OAUTH_DATABASE_URL", "postgres://x")
	t.Setenv("OAUTH_PUBLIC_URL", "https://x")
	t.Setenv("OAUTH_ENCRYPTION_KEY", "not-hex")
	t.Setenv("OAUTH_COOKIE_SECRET", strings.Repeat("c", 32))
	t.Setenv("ORGANIZZE_API_KEY", "")
	if _, err := LoadOAuth(); err == nil {
		t.Error("expected hex-decode error")
	}
}

func TestLoadOAuth_RejectsShortCookieSecret(t *testing.T) {
	t.Setenv("OAUTH_DATABASE_URL", "postgres://x")
	t.Setenv("OAUTH_PUBLIC_URL", "https://x")
	t.Setenv("OAUTH_ENCRYPTION_KEY", strings.Repeat("0", 64))
	t.Setenv("OAUTH_COOKIE_SECRET", "too-short")
	t.Setenv("ORGANIZZE_API_KEY", "")
	if _, err := LoadOAuth(); err == nil {
		t.Error("expected error for short cookie secret")
	}
}
