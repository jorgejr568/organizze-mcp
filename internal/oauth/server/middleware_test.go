package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/credprovider"
	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

func TestBearer_Missing401WithChallenge(t *testing.T) {
	srv, _ := newServerWithFakeStore(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("next should not be called without bearer")
	})
	h := srv.Bearer(next)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/mcp", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d", rec.Code)
	}
	if !slices.Contains(rec.Header().Values("WWW-Authenticate"), `Bearer resource_metadata="https://mcp.example.com/.well-known/oauth-protected-resource"`) {
		t.Errorf("WWW-Authenticate = %v", rec.Header().Values("WWW-Authenticate"))
	}
}

func TestBearer_HappyPath_InjectsCredentials(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)

	cipher, nonce, _ := srv.cfg.Cipher.Seal([]byte("the-real-api-key"))
	user, _ := fs.UpsertUserByEmail(context.Background(), storage.User{
		OrganizzeEmail: "u@x.com", APIKeyCipher: cipher, APIKeyNonce: nonce, UserAgent: "UA",
	})
	_ = fs.CreateToken(context.Background(), storage.Token{
		TokenHash: storage.HashToken("the-access-token"),
		Kind:      "access", ClientID: "cli", UserID: user.ID,
		ExpiresAt: time.Now().Add(time.Hour),
	})

	var sawEmail, sawKey, sawUA string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e, k, ua, err := credprovider.FromContext(r.Context())
		if err != nil {
			t.Fatalf("FromContext: %v", err)
		}
		sawEmail, sawKey, sawUA = e, k, ua
		w.WriteHeader(http.StatusOK)
	})
	h := srv.Bearer(next)

	req := httptest.NewRequest("GET", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer the-access-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if sawEmail != "u@x.com" || sawKey != "the-real-api-key" || sawUA != "UA" {
		t.Errorf("got %q,%q,%q", sawEmail, sawKey, sawUA)
	}
}

func TestBearer_AcceptsLowercaseScheme(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	cipher, nonce, _ := srv.cfg.Cipher.Seal([]byte("k"))
	user, _ := fs.UpsertUserByEmail(context.Background(), storage.User{
		OrganizzeEmail: "u@x.com", APIKeyCipher: cipher, APIKeyNonce: nonce, UserAgent: "UA",
	})
	_ = fs.CreateToken(context.Background(), storage.Token{
		TokenHash: storage.HashToken("tok-lower"), Kind: "access", ClientID: "cli", UserID: user.ID,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	req := httptest.NewRequest("GET", "/mcp", nil)
	req.Header.Set("Authorization", "bearer tok-lower") // lowercase scheme
	rec := httptest.NewRecorder()
	srv.Bearer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("lowercase bearer status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBearer_RejectsRefreshKindToken(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	cipher, nonce, _ := srv.cfg.Cipher.Seal([]byte("k"))
	user, _ := fs.UpsertUserByEmail(context.Background(), storage.User{
		OrganizzeEmail: "u@x.com", APIKeyCipher: cipher, APIKeyNonce: nonce, UserAgent: "UA",
	})
	_ = fs.CreateToken(context.Background(), storage.Token{
		TokenHash: storage.HashToken("refresh-as-bearer"), Kind: "refresh", ClientID: "cli", UserID: user.ID,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	req := httptest.NewRequest("GET", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer refresh-as-bearer")
	rec := httptest.NewRecorder()
	srv.Bearer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("next should not be called for refresh-kind token")
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestBearer_RejectsRevokedToken(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	cipher, nonce, _ := srv.cfg.Cipher.Seal([]byte("k"))
	user, _ := fs.UpsertUserByEmail(context.Background(), storage.User{
		OrganizzeEmail: "u@x.com", APIKeyCipher: cipher, APIKeyNonce: nonce, UserAgent: "UA",
	})
	now := time.Now().UTC()
	_ = fs.CreateToken(context.Background(), storage.Token{
		TokenHash: storage.HashToken("rev"), Kind: "access", ClientID: "cli", UserID: user.ID,
		ExpiresAt: now.Add(time.Hour), RevokedAt: &now,
	})
	req := httptest.NewRequest("GET", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer rev")
	rec := httptest.NewRecorder()
	srv.Bearer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d", rec.Code)
	}
}
