package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

func newServerWithFakeStore(t *testing.T) (*Server, *fakeStore) {
	t.Helper()
	fs := newFakeStore()
	srv := New(Config{
		PublicURL:         "https://mcp.example.com",
		Store:             fs,
		CookieSecret:      []byte("secret"),
		ValidateOrganizze: func(_ context.Context, _, _, _ string) error { return nil },
	})
	return srv, fs
}

func TestRegister_HappyPath(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	body := `{"client_name":"Perplexity","redirect_uris":["https://www.perplexity.ai/cb"]}`
	req := httptest.NewRequest(http.MethodPost, "/oauth/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["client_id"] == "" || got["client_id"] == nil {
		t.Errorf("missing client_id: %v", got)
	}
	secret, _ := got["client_secret"].(string)
	if secret == "" {
		t.Errorf("missing client_secret (Perplexity DCR_CLIENT_SECRET_REQUIRED): %v", got)
	}
	if got["token_endpoint_auth_method"] != "client_secret_basic" {
		t.Errorf("token_endpoint_auth_method = %v, want client_secret_basic", got["token_endpoint_auth_method"])
	}
	// RFC 7591 §3.2.1: 0 means the secret does not expire.
	if v, ok := got["client_secret_expires_at"].(float64); !ok || v != 0 {
		t.Errorf("client_secret_expires_at = %v, want 0", got["client_secret_expires_at"])
	}
	if _, ok := got["client_id_issued_at"].(float64); !ok {
		t.Errorf("missing client_id_issued_at: %v", got)
	}
	if len(fs.clients) != 1 {
		t.Errorf("store has %d clients", len(fs.clients))
	}
}

func TestRegister_PersistsSecretHash(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	body := `{"client_name":"X","redirect_uris":["https://app.example.com/cb"]}`
	req := httptest.NewRequest(http.MethodPost, "/oauth/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	id := got["client_id"].(string)
	secret := got["client_secret"].(string)

	stored, err := fs.GetClient(context.Background(), id)
	if err != nil {
		t.Fatalf("GetClient: %v", err)
	}
	wantHash := storage.HashToken(secret)
	if !bytes.Equal(stored.ClientSecretHash, wantHash) {
		t.Errorf("stored hash %x != sha256(secret) %x", stored.ClientSecretHash, wantHash)
	}
	if len(stored.ClientSecretHash) != 32 {
		t.Errorf("stored hash length = %d, want 32", len(stored.ClientSecretHash))
	}
}

func TestRegister_RejectsMissingRedirectURIs(t *testing.T) {
	srv, _ := newServerWithFakeStore(t)
	body := `{"client_name":"X"}`
	req := httptest.NewRequest(http.MethodPost, "/oauth/register", bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestRegister_RejectsNonHTTPSRedirect(t *testing.T) {
	srv, _ := newServerWithFakeStore(t)
	body := `{"client_name":"X","redirect_uris":["http://evil.example.com/cb"]}`
	req := httptest.NewRequest(http.MethodPost, "/oauth/register", bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestRegister_RateLimitedAfterBurst(t *testing.T) {
	srv, _ := newServerWithFakeStore(t)
	body := `{"client_name":"x","redirect_uris":["https://app.example.com/cb"]}`
	var created, ratelimited int
	for i := 0; i < 30; i++ {
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "1.2.3.4:12345"
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		switch rec.Code {
		case http.StatusCreated:
			created++
		case http.StatusTooManyRequests:
			ratelimited++
		}
	}
	if created == 0 || ratelimited == 0 {
		t.Fatalf("expected mix: created=%d 429=%d", created, ratelimited)
	}
}
