package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

func seedClientRecord() (c struct {
	ID, Name string
	URIs     []string
}) {
	c.ID = "client-abc"
	c.Name = "ChatGPT"
	c.URIs = []string{"https://chat.example.com/cb"}
	return
}

func storageClient(c struct {
	ID, Name string
	URIs     []string
}) storage.Client {
	return storage.Client{ID: c.ID, ClientName: c.Name, RedirectURIs: c.URIs}
}

func mustTestCipher(t *testing.T) *storage.Cipher {
	t.Helper()
	key := make([]byte, 32)
	c, err := storage.NewCipher(key) // zero key fine for tests
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAuthorize_GET_RendersForm(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))

	q := url.Values{
		"client_id":             {c.ID},
		"redirect_uri":          {c.URIs[0]},
		"response_type":         {"code"},
		"state":                 {"xyz"},
		"code_challenge":        {"E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"},
		"code_challenge_method": {"S256"},
	}
	req := httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"ChatGPT", "Organizze email", "Organizze API key", c.URIs[0], "xyz"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestAuthorize_GET_RejectsUnknownRedirectURI(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))

	q := url.Values{
		"client_id":             {c.ID},
		"redirect_uri":          {"https://evil.example.com/cb"},
		"response_type":         {"code"},
		"state":                 {"xyz"},
		"code_challenge":        {"E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"},
		"code_challenge_method": {"S256"},
	}
	req := httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthorize_POST_HappyPath_RedirectsWithCode(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	srv.cfg.ValidateOrganizze = func(_ context.Context, e, k, _ string) error {
		if e == "user@x.com" && k == "the-key" {
			return nil
		}
		return errors.New("bad creds")
	}
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))

	form := url.Values{
		"client_id":             {c.ID},
		"redirect_uri":          {c.URIs[0]},
		"state":                 {"xyz"},
		"code_challenge":        {"E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"},
		"code_challenge_method": {"S256"},
		"email":                 {"user@x.com"},
		"api_key":               {"the-key"},
		"user_agent":            {"Me (me@x.com)"},
	}
	req := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, c.URIs[0]+"?") {
		t.Errorf("location = %q", loc)
	}
	u, _ := url.Parse(loc)
	if u.Query().Get("state") != "xyz" || u.Query().Get("code") == "" {
		t.Errorf("query = %s", u.RawQuery)
	}
}

func TestAuthorize_POST_RejectsInvalidOrganizzCreds(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	srv.cfg.ValidateOrganizze = func(context.Context, string, string, string) error {
		return errors.New("401 unauthorized")
	}
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))

	form := url.Values{
		"client_id":             {c.ID},
		"redirect_uri":          {c.URIs[0]},
		"state":                 {"xyz"},
		"code_challenge":        {"E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"},
		"code_challenge_method": {"S256"},
		"email":                 {"user@x.com"},
		"api_key":               {"bad"},
		"user_agent":            {"Me (me@x.com)"},
	}
	req := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	// Re-renders form with error, status 200.
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Invalid Organizze credentials") {
		t.Errorf("body did not contain error: %s", rec.Body.String())
	}
}

func TestAuthorize_GET_RejectsShortCodeChallenge(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))

	q := url.Values{
		"client_id":             {c.ID},
		"redirect_uri":          {c.URIs[0]},
		"response_type":         {"code"},
		"state":                 {"xyz"},
		"code_challenge":        {"abc"},
		"code_challenge_method": {"S256"},
	}
	req := httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}
