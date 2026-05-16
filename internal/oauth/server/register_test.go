package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
	body := `{"client_name":"ChatGPT","redirect_uris":["https://chat.openai.com/cb"]}`
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
	if _, ok := got["client_secret"]; ok {
		t.Errorf("public client should not receive a client_secret: %v", got)
	}
	if len(fs.clients) != 1 {
		t.Errorf("store has %d clients", len(fs.clients))
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
