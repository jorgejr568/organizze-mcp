package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

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

// validConsentToken returns a freshly-signed consent_token bound to the given
// authorize-form fields. Tests use it to POST a form that the real handler
// would have rendered itself on GET.
func validConsentToken(srv *Server, clientID, redirectURI, challenge, state string) string {
	return signConsent(srv.cfg.CookieSecret, consentBinding{
		ClientID:    clientID,
		RedirectURI: redirectURI,
		Challenge:   challenge,
		State:       state,
		IssuedAt:    srv.cfg.Now().Unix(),
	})
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
		"consent_token":         {validConsentToken(srv, c.ID, c.URIs[0], "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", "xyz")},
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
		"consent_token":         {validConsentToken(srv, c.ID, c.URIs[0], "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", "xyz")},
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

func TestAuthorize_GET_RejectsMissingRedirectURI(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))
	q := url.Values{
		"client_id":             {c.ID},
		// no redirect_uri
		"code_challenge":        {"E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"},
		"code_challenge_method": {"S256"},
	}
	req := httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestAuthorize_GET_RejectsMissingCodeChallenge(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))
	q := url.Values{
		"client_id":    {c.ID},
		"redirect_uri": {c.URIs[0]},
		// no code_challenge
	}
	req := httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestAuthorize_GET_RejectsCodeChallengeMethodPlain(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))
	q := url.Values{
		"client_id":             {c.ID},
		"redirect_uri":          {c.URIs[0]},
		"code_challenge":        {"E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"},
		"code_challenge_method": {"plain"},
	}
	req := httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
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

func TestAuthorize_POST_RejectsMissingConsentToken(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))

	form := url.Values{
		"client_id":             {c.ID},
		"redirect_uri":          {c.URIs[0]},
		"state":                 {"xyz"},
		"code_challenge":        {"E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"},
		"code_challenge_method": {"S256"},
		"email":                 {"u@x.com"},
		"api_key":               {"k"},
		"user_agent":            {"ua"},
		// no consent_token
	}
	req := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthorize_POST_RejectsTamperedClientID(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))
	other := storage.Client{ID: "other-client", ClientName: "Evil", RedirectURIs: []string{"https://evil.example.com/cb"}}
	_ = fs.CreateClient(context.Background(), other)

	challenge := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	// Token bound to c.ID, but the POSTed client_id claims to be other.ID.
	tok := validConsentToken(srv, c.ID, c.URIs[0], challenge, "xyz")
	form := url.Values{
		"client_id":             {other.ID}, // tampered
		"redirect_uri":          {c.URIs[0]},
		"state":                 {"xyz"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"consent_token":         {tok},
		"email":                 {"u@x.com"},
		"api_key":               {"k"},
		"user_agent":            {"ua"},
	}
	req := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthorize_ConsentToken_ExpiredRejected(t *testing.T) {
	secret := []byte("supersecretkey-that-is-32-bytes!")
	expired := signConsent(secret, consentBinding{
		ClientID: "c", RedirectURI: "https://app/cb", Challenge: "ch", State: "s",
		IssuedAt: time.Now().Add(-11 * time.Minute).Unix(),
	})
	if _, err := verifyConsent(secret, expired, time.Now()); err == nil {
		t.Fatal("expected expired error")
	}
}

func TestAuthorize_GET_RejectsNonBase64URLCodeChallenge(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))

	// 43 chars but contains '/' (base64-std, not base64url).
	bogus := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA/aa"
	q := url.Values{
		"client_id":             {c.ID},
		"redirect_uri":          {c.URIs[0]},
		"response_type":         {"code"},
		"state":                 {"xyz"},
		"code_challenge":        {bogus},
		"code_challenge_method": {"S256"},
	}
	req := httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}
