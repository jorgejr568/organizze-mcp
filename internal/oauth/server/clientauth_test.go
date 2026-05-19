package server

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

// newTokenReq builds a POST /oauth/token request with the given form values,
// optionally setting HTTP Basic auth. The form is parsed before return so the
// helper-under-test can read r.PostForm.
func newTokenReq(t *testing.T, form url.Values, basicID, basicSecret string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if basicID != "" || basicSecret != "" {
		auth := basicID + ":" + basicSecret
		r.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))
	}
	if err := r.ParseForm(); err != nil {
		t.Fatalf("ParseForm: %v", err)
	}
	return r
}

func TestAuthenticateClient_PublicClient_PassesWithoutSecret(t *testing.T) {
	fs := newFakeStore()
	_ = fs.CreateClient(context.Background(), storage.Client{
		ID:           "pub-1",
		RedirectURIs: []string{"https://x.example.com/cb"},
		// ClientSecretHash intentionally nil — legacy public client.
	})
	r := newTokenReq(t, url.Values{"client_id": {"pub-1"}}, "", "")
	id, err := authenticateClient(r.Context(), fs, r)
	if err != nil {
		t.Fatalf("err = %v, want nil for public client", err)
	}
	if id != "pub-1" {
		t.Errorf("id = %q", id)
	}
}

func TestAuthenticateClient_ConfidentialClient_BasicAuthAccepted(t *testing.T) {
	fs := newFakeStore()
	secret := "the-secret-xyz"
	_ = fs.CreateClient(context.Background(), storage.Client{
		ID:               "conf-1",
		ClientSecretHash: storage.HashToken(secret),
		RedirectURIs:     []string{"https://x.example.com/cb"},
	})
	r := newTokenReq(t, url.Values{}, "conf-1", secret)
	id, err := authenticateClient(r.Context(), fs, r)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if id != "conf-1" {
		t.Errorf("id = %q", id)
	}
}

func TestAuthenticateClient_ConfidentialClient_FormSecretAccepted(t *testing.T) {
	fs := newFakeStore()
	secret := "the-secret-xyz"
	_ = fs.CreateClient(context.Background(), storage.Client{
		ID:               "conf-2",
		ClientSecretHash: storage.HashToken(secret),
		RedirectURIs:     []string{"https://x.example.com/cb"},
	})
	r := newTokenReq(t, url.Values{
		"client_id":     {"conf-2"},
		"client_secret": {secret},
	}, "", "")
	id, err := authenticateClient(r.Context(), fs, r)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if id != "conf-2" {
		t.Errorf("id = %q", id)
	}
}

func TestAuthenticateClient_ConfidentialClient_MissingSecretRejected(t *testing.T) {
	fs := newFakeStore()
	_ = fs.CreateClient(context.Background(), storage.Client{
		ID:               "conf-3",
		ClientSecretHash: storage.HashToken("a-secret"),
		RedirectURIs:     []string{"https://x.example.com/cb"},
	})
	r := newTokenReq(t, url.Values{"client_id": {"conf-3"}}, "", "")
	_, err := authenticateClient(r.Context(), fs, r)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errInvalidClient) {
		t.Errorf("err = %v, want errInvalidClient", err)
	}
}

func TestAuthenticateClient_ConfidentialClient_WrongSecretRejected(t *testing.T) {
	fs := newFakeStore()
	_ = fs.CreateClient(context.Background(), storage.Client{
		ID:               "conf-4",
		ClientSecretHash: storage.HashToken("right-secret"),
		RedirectURIs:     []string{"https://x.example.com/cb"},
	})
	r := newTokenReq(t, url.Values{}, "conf-4", "wrong-secret")
	_, err := authenticateClient(r.Context(), fs, r)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errInvalidClient) {
		t.Errorf("err = %v, want errInvalidClient", err)
	}
}

func TestAuthenticateClient_UnknownClientRejected(t *testing.T) {
	fs := newFakeStore()
	r := newTokenReq(t, url.Values{"client_id": {"does-not-exist"}}, "", "")
	_, err := authenticateClient(r.Context(), fs, r)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errInvalidClient) {
		t.Errorf("err = %v, want errInvalidClient", err)
	}
}

func TestAuthenticateClient_BasicAndFormIDMismatchRejected(t *testing.T) {
	// RFC 6749 §2.3.1: a client MUST NOT use more than one method per request.
	// If Basic auth's userid disagrees with form client_id we reject.
	fs := newFakeStore()
	_ = fs.CreateClient(context.Background(), storage.Client{
		ID:               "conf-5",
		ClientSecretHash: storage.HashToken("s"),
		RedirectURIs:     []string{"https://x.example.com/cb"},
	})
	r := newTokenReq(t, url.Values{
		"client_id":     {"conf-OTHER"},
		"client_secret": {"s"},
	}, "conf-5", "s")
	_, err := authenticateClient(r.Context(), fs, r)
	if err == nil {
		t.Fatal("expected error on Basic-vs-form id mismatch, got nil")
	}
	if !errors.Is(err, errInvalidClient) {
		t.Errorf("err = %v, want errInvalidClient", err)
	}
}

func TestAuthenticateClient_MissingClientIDRejected(t *testing.T) {
	fs := newFakeStore()
	r := newTokenReq(t, url.Values{}, "", "")
	_, err := authenticateClient(r.Context(), fs, r)
	if err == nil {
		t.Fatal("expected error when no client_id present, got nil")
	}
	if !errors.Is(err, errInvalidClient) {
		t.Errorf("err = %v, want errInvalidClient", err)
	}
}

func TestAuthenticateClient_EmptyBasicUseridRejected(t *testing.T) {
	// Authorization: Basic <base64(":secret")> parses as ok=true with basicID="".
	// The empty-clientID guard must reject before any store lookup runs.
	fs := newFakeStore()
	r := newTokenReq(t, url.Values{}, "", "some-secret")
	_, err := authenticateClient(r.Context(), fs, r)
	if err == nil {
		t.Fatal("expected error for Basic auth with empty userid, got nil")
	}
	if !errors.Is(err, errInvalidClient) {
		t.Errorf("err = %v, want errInvalidClient", err)
	}
}

func TestAuthenticateClient_BasicAndFormIDMatchTolerated(t *testing.T) {
	// RFC 6749 §2.3.1 "MUST NOT use more than one method" — we tolerate a
	// duplicate form client_id that matches the Basic userid (common client
	// libraries set both defensively); reject only on contradiction.
	fs := newFakeStore()
	secret := "the-secret-xyz"
	_ = fs.CreateClient(context.Background(), storage.Client{
		ID:               "conf-dup",
		ClientSecretHash: storage.HashToken(secret),
		RedirectURIs:     []string{"https://x.example.com/cb"},
	})
	r := newTokenReq(t, url.Values{"client_id": {"conf-dup"}}, "conf-dup", secret)
	id, err := authenticateClient(r.Context(), fs, r)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if id != "conf-dup" {
		t.Errorf("id = %q", id)
	}
}
