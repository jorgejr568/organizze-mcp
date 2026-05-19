# OAuth confidential-client support (Perplexity DCR compatibility) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the OAuth Authorization Server in `internal/oauth/server/` issue a `client_secret` on every new Dynamic Client Registration (RFC 7591), and authenticate confidential clients at the token endpoint (RFC 6749 §2.3.1) using `client_secret_basic` or `client_secret_post`. Unblocks Perplexity, which rejects DCR responses without a secret (`DCR_CLIENT_SECRET_REQUIRED`).

**Architecture:** The persistence layer was already shaped for confidential clients — `storage.Client.ClientSecretHash []byte` and the nullable `oauth_clients.client_secret_hash` column already exist (`internal/oauth/storage/storage.go:23`, `internal/oauth/storage/migrations/001_init.sql:23`). Only the HTTP layer treats every client as public. The fix: (1) the DCR handler generates a random 32-byte secret, persists `sha256(secret)`, and returns the secret in the response; (2) a small `authenticateClient` helper in a new `internal/oauth/server/clientauth.go` resolves the client from Basic-auth-or-form and verifies the secret with constant-time compare; (3) both `grantAuthorizationCode` and `grantRefreshToken` call the helper instead of trusting the form `client_id`. Backward compat: clients persisted with `ClientSecretHash = nil` (every row registered before this change) remain valid public/PKCE-only clients — the helper short-circuits secret verification when the stored hash is nil. PKCE remains mandatory for both. Discovery advertises `["client_secret_basic", "client_secret_post", "none"]`.

**Tech Stack:** Go 1.23+, `net/http`, `crypto/sha256`, `crypto/subtle`, `encoding/base64`, `crypto/rand`. No new dependencies; no schema migration.

---

## File Structure

| Path | Status | Responsibility |
| ---- | ------ | -------------- |
| `internal/oauth/server/register.go` | Modify | Generate secret + hash; include `client_secret`, `client_secret_expires_at`, `client_id_issued_at`, `token_endpoint_auth_method: "client_secret_basic"` in the DCR response. |
| `internal/oauth/server/register_test.go` | Modify | Flip the "no client_secret" assertion; assert secret is returned + persisted hash equals `sha256(secret)`. |
| `internal/oauth/server/clientauth.go` | Create | `authenticateClient(ctx, store, r) (clientID string, err error)` — parses Basic-auth-or-form, looks up the client, constant-time-compares the secret hash, allows nil-hash (public) clients to pass without a secret. |
| `internal/oauth/server/clientauth_test.go` | Create | Unit tests for the helper — Basic-vs-form, missing/wrong secret, public-client passthrough, id-mismatch when Basic and form both present. |
| `internal/oauth/server/token.go` | Modify | Replace direct `r.PostForm.Get("client_id")` with `authenticateClient(...)` in both grant branches. |
| `internal/oauth/server/token_test.go` | Modify | Add confidential-client tests for both grants (Basic + form, missing, wrong secret, refresh). Existing public-client tests stay unchanged. |
| `internal/oauth/server/discovery.go` | Modify | Advertise `["client_secret_basic", "client_secret_post", "none"]` in `token_endpoint_auth_methods_supported`. |
| `internal/oauth/server/discovery_test.go` | Modify | Update the auth-methods assertion. |
| `CHANGELOG.md` | Modify | Add an `## [Unreleased]` entry under `### Fixed` (Perplexity) and `### Changed` (DCR now issues a secret). |

Out of scope: `internal/oauth/server/revoke.go` client auth (RFC 7009 §2.1 SHOULD, not MUST; no MCP client requires it today). Storage and migrations: zero changes.

---

## Task 1: DCR endpoint issues `client_secret`

**Files:**
- Modify: `internal/oauth/server/register.go`
- Modify: `internal/oauth/server/register_test.go`

- [ ] **Step 1: Write the failing tests**

Replace the body of `TestRegister_HappyPath` in `internal/oauth/server/register_test.go` (existing lines 24–46) with the version below. Add the new `TestRegister_PersistsSecretHash` immediately after it.

```go
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
```

Add the imports the new test needs (`context` is already used elsewhere in the package; the file currently uses `bytes`, `encoding/json`, `net/http`, `net/http/httptest`, `testing` — add `context` and `github.com/jorgejr568/organizze-mcp/internal/oauth/storage`):

```go
import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/oauth/server/ -run TestRegister -v`
Expected: `TestRegister_HappyPath` and `TestRegister_PersistsSecretHash` FAIL — current handler does not return `client_secret` or persist a hash.

- [ ] **Step 3: Implement the secret issuance**

Replace the body of `handleRegister` and the `registerResponse` struct in `internal/oauth/server/register.go`. Drop nothing else — keep the existing `writeOAuthError`, request struct, and rate-limit / redirect_uri validation as-is.

Replace lines 26–76 (`registerResponse` through end of `handleRegister`) plus add `newClientSecret` next to `newPublicID`:

```go
type registerResponse struct {
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
	ClientSecretExpiresAt   int64    `json:"client_secret_expires_at"` // 0 = never expires (RFC 7591 §3.2.1)
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOAuthError(w, http.StatusMethodNotAllowed, "invalid_request", "POST required")
		return
	}
	if !s.dcrLimiter.allow(clientIP(r)) {
		writeOAuthError(w, http.StatusTooManyRequests, "rate_limited", "too many client registrations from this IP")
		return
	}
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "malformed JSON")
		return
	}
	if len(req.RedirectURIs) == 0 {
		writeOAuthError(w, http.StatusBadRequest, "invalid_redirect_uri", "at least one redirect_uri required")
		return
	}
	for _, u := range req.RedirectURIs {
		if !strings.HasPrefix(u, "https://") {
			writeOAuthError(w, http.StatusBadRequest, "invalid_redirect_uri", "must be https")
			return
		}
	}
	id := newPublicID()
	secret := newClientSecret()
	if err := s.cfg.Store.CreateClient(r.Context(), storage.Client{
		ID:               id,
		ClientSecretHash: storage.HashToken(secret),
		ClientName:       req.ClientName,
		RedirectURIs:     req.RedirectURIs,
	}); err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "")
		return
	}
	writeJSON(w, http.StatusCreated, registerResponse{
		ClientID:                id,
		ClientSecret:            secret,
		ClientIDIssuedAt:        s.cfg.Now().Unix(),
		ClientSecretExpiresAt:   0,
		ClientName:              req.ClientName,
		RedirectURIs:            req.RedirectURIs,
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "client_secret_basic",
	})
}

// newPublicID returns a 192-bit URL-safe random identifier for OAuth clients.
func newPublicID() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// newClientSecret returns a 256-bit URL-safe random secret suitable for
// client_secret_basic. The hash (sha256) is what we persist; the plaintext
// is shown to the registering caller exactly once.
func newClientSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/oauth/server/ -run TestRegister -v`
Expected: all four `TestRegister_*` cases PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/oauth/server/register.go internal/oauth/server/register_test.go
git commit -m "$(cat <<'EOF'
feat(oauth/server): DCR issues client_secret (RFC 7591 §3.2.1)

Every new dynamic client registration now receives a random 32-byte
client_secret in the response, and the sha256 of that secret is persisted
in oauth_clients.client_secret_hash. The response also carries
client_id_issued_at, client_secret_expires_at: 0 (never expires), and
token_endpoint_auth_method: "client_secret_basic".

Motivation: Perplexity's MCP client rejects DCR responses without a
client_secret with DCR_CLIENT_SECRET_REQUIRED. RFC 7591 permits public
clients (the prior behavior), but several MCP clients enforce confidential
clients in practice.

Backward compat with already-registered clients is preserved by the next
commit, which keeps the token endpoint accepting PKCE-only auth when the
stored hash is nil.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: `authenticateClient` helper (Basic + form, public-client passthrough)

**Files:**
- Create: `internal/oauth/server/clientauth.go`
- Create: `internal/oauth/server/clientauth_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/oauth/server/clientauth_test.go`:

```go
package server

import (
	"context"
	"encoding/base64"
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
	if _, err := authenticateClient(r.Context(), fs, r); err == nil {
		t.Fatal("expected error, got nil")
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
	if _, err := authenticateClient(r.Context(), fs, r); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAuthenticateClient_UnknownClientRejected(t *testing.T) {
	fs := newFakeStore()
	r := newTokenReq(t, url.Values{"client_id": {"does-not-exist"}}, "", "")
	if _, err := authenticateClient(r.Context(), fs, r); err == nil {
		t.Fatal("expected error, got nil")
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
	if _, err := authenticateClient(r.Context(), fs, r); err == nil {
		t.Fatal("expected error on Basic-vs-form id mismatch, got nil")
	}
}

func TestAuthenticateClient_MissingClientIDRejected(t *testing.T) {
	fs := newFakeStore()
	r := newTokenReq(t, url.Values{}, "", "")
	if _, err := authenticateClient(r.Context(), fs, r); err == nil {
		t.Fatal("expected error when no client_id present, got nil")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/oauth/server/ -run TestAuthenticateClient -v`
Expected: every case fails with `undefined: authenticateClient` (compilation error).

- [ ] **Step 3: Implement the helper**

Create `internal/oauth/server/clientauth.go`:

```go
package server

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

// errInvalidClient signals an RFC 6749 §5.2 invalid_client. Handlers map it
// to a 400 with error=invalid_client; we don't expose the specific reason
// (missing secret vs. wrong secret vs. unknown client) to avoid leaking
// which client IDs exist.
var errInvalidClient = errors.New("invalid_client")

// authenticateClient resolves and authenticates the OAuth client for a token
// or revoke request. It accepts client_secret_basic (preferred) and
// client_secret_post (form fields). Clients whose stored ClientSecretHash is
// nil are public clients (registered before this server issued secrets, or
// future explicit-none registrations) and are accepted without a secret —
// PKCE is the binding mechanism for those, enforced by the caller.
//
// The form on r must already be parsed (r.ParseForm() called by the handler).
func authenticateClient(ctx context.Context, store storage.Store, r *http.Request) (string, error) {
	basicID, basicSecret, hasBasic := r.BasicAuth()
	formID := r.PostForm.Get("client_id")
	formSecret := r.PostForm.Get("client_secret")

	var clientID, presentedSecret string
	switch {
	case hasBasic:
		clientID = basicID
		presentedSecret = basicSecret
		// If the form also carries a client_id, RFC 6749 §2.3.1 says only
		// one method may be used per request. Tolerate a duplicate that
		// matches, reject a contradiction.
		if formID != "" && formID != basicID {
			return "", errInvalidClient
		}
	default:
		clientID = formID
		presentedSecret = formSecret
	}
	if clientID == "" {
		return "", errInvalidClient
	}

	client, err := store.GetClient(ctx, clientID)
	if err != nil {
		return "", errInvalidClient
	}

	if client.ClientSecretHash == nil {
		// Public client — PKCE is the binding factor (verified by the
		// authorization-code grant path). No secret expected; presence
		// of a stray client_secret is ignored.
		return clientID, nil
	}

	if presentedSecret == "" {
		return "", errInvalidClient
	}
	if subtle.ConstantTimeCompare(storage.HashToken(presentedSecret), client.ClientSecretHash) != 1 {
		return "", errInvalidClient
	}
	return clientID, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/oauth/server/ -run TestAuthenticateClient -v`
Expected: all eight `TestAuthenticateClient_*` cases PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/oauth/server/clientauth.go internal/oauth/server/clientauth_test.go
git commit -m "$(cat <<'EOF'
feat(oauth/server): authenticateClient helper for token endpoint

Resolves the OAuth client for /oauth/token (and any future endpoint that
needs client auth) from Basic auth or form params, then constant-time-
compares the presented secret against the stored sha256 hash. Clients
whose stored ClientSecretHash is nil are treated as public (PKCE-only) —
preserves backward compat with already-registered ChatGPT/Claude clients
that predate the DCR-issues-secret change.

RFC 6749 §2.3.1: rejects requests that supply both Basic auth and a
contradictory form client_id. RFC 7235: returns a generic invalid_client
sentinel rather than distinguishing unknown-id from wrong-secret, so the
endpoint cannot be used as a client-id oracle.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Wire `authenticateClient` into both token grants

**Files:**
- Modify: `internal/oauth/server/token.go`
- Modify: `internal/oauth/server/token_test.go`

- [ ] **Step 1: Write the failing tests**

Append these test functions to the bottom of `internal/oauth/server/token_test.go`. They use the existing `seedClientRecord` / `storageClient` helpers and `issueCode` / `postForm` helpers from that file. The new ones build their own confidential-client records in-place.

```go
func seedConfidentialClient(t *testing.T, fs *fakeStore) (id, secret, redirect string) {
	t.Helper()
	id = "conf-client"
	secret = "super-secret-32-bytes-or-whatever"
	redirect = "https://chat.example.com/cb"
	if err := fs.CreateClient(context.Background(), storage.Client{
		ID:               id,
		ClientName:       "Perplexity",
		ClientSecretHash: storage.HashToken(secret),
		RedirectURIs:     []string{redirect},
	}); err != nil {
		t.Fatalf("CreateClient: %v", err)
	}
	return
}

func postFormWithBasic(srv *Server, form url.Values, basicID, basicSecret string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if basicID != "" || basicSecret != "" {
		auth := basicID + ":" + basicSecret
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestToken_ConfidentialClient_AuthorizationCode_BasicAuth_Success(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	id, secret, redirect := seedConfidentialClient(t, fs)
	verifier := "verifier-that-is-long-enough-12345678901234567"
	// issueCode hard-codes RedirectURI to https://chat.example.com/cb,
	// which matches seedConfidentialClient's redirect.
	code, _ := issueCode(t, fs, id, verifier)

	rec := postFormWithBasic(srv, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirect},
		"code_verifier": {verifier},
	}, id, secret)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestToken_ConfidentialClient_AuthorizationCode_PostAuth_Success(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	id, secret, redirect := seedConfidentialClient(t, fs)
	verifier := "verifier-that-is-long-enough-12345678901234567"
	code, _ := issueCode(t, fs, id, verifier)

	rec := postForm(srv, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirect},
		"client_id":     {id},
		"client_secret": {secret},
		"code_verifier": {verifier},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestToken_ConfidentialClient_MissingSecret_InvalidClient(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	id, _, redirect := seedConfidentialClient(t, fs)
	verifier := "verifier-that-is-long-enough-12345678901234567"
	code, _ := issueCode(t, fs, id, verifier)

	rec := postForm(srv, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirect},
		"client_id":     {id},
		"code_verifier": {verifier},
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "invalid_client" {
		t.Errorf("error = %v, want invalid_client", body["error"])
	}
}

func TestToken_ConfidentialClient_WrongSecret_InvalidClient(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	id, _, redirect := seedConfidentialClient(t, fs)
	verifier := "verifier-that-is-long-enough-12345678901234567"
	code, _ := issueCode(t, fs, id, verifier)

	rec := postFormWithBasic(srv, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirect},
		"code_verifier": {verifier},
	}, id, "this-is-not-the-secret")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestToken_ConfidentialClient_RefreshGrant_RequiresSecret(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	id, secret, redirect := seedConfidentialClient(t, fs)
	verifier := "verifier-that-is-long-enough-12345678901234567"
	code, _ := issueCode(t, fs, id, verifier)

	// Mint a token pair via the confidential code grant.
	rec := postFormWithBasic(srv, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirect},
		"code_verifier": {verifier},
	}, id, secret)
	if rec.Code != http.StatusOK {
		t.Fatalf("seed exchange status = %d body=%s", rec.Code, rec.Body.String())
	}
	var pair map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &pair)
	refresh := pair["refresh_token"].(string)

	// Refresh without secret must fail.
	rec = postForm(srv, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refresh},
		"client_id":     {id},
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (refresh without secret on confidential client)", rec.Code)
	}

	// Refresh with secret must succeed.
	rec = postFormWithBasic(srv, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refresh},
	}, id, secret)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d body=%s (refresh with secret should succeed)", rec.Code, rec.Body.String())
	}
}
```

Add `"encoding/base64"` to the import block at the top of the file (the package already imports `encoding/json` and others; `encoding/base64` is missing).

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/oauth/server/ -run TestToken_ConfidentialClient -v`
Expected: all five cases fail — the handler currently ignores `client_secret` entirely and accepts the request as-if-public.

- [ ] **Step 3: Refactor token.go to call authenticateClient**

Edit `internal/oauth/server/token.go`. Replace the body of `grantAuthorizationCode` (current lines 41–85) and `grantRefreshToken` (current lines 87–134) so the early `client_id` read goes through `authenticateClient`. The PKCE / code-replay / rotation logic is unchanged. Two changes per function.

Replace `grantAuthorizationCode`:

```go
func (s *Server) grantAuthorizationCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clientID, err := authenticateClient(ctx, s.cfg.Store, r)
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client", "")
		return
	}
	code := r.PostForm.Get("code")
	redirectURI := r.PostForm.Get("redirect_uri")
	verifier := r.PostForm.Get("code_verifier")
	if code == "" || verifier == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "missing required field")
		return
	}
	codeHash := storage.HashToken(code)
	ac, err := s.cfg.Store.ConsumeAuthCode(ctx, codeHash)
	if err != nil {
		// Code unknown, used, or expired. If the code was previously consumed
		// (re-presentation = reuse signal per RFC 6749 §10.5 / Security BCP
		// §4.10), every token issued from it must be revoked. The store call
		// is a no-op when no tokens reference this code, so we can fire it
		// unconditionally on ErrNotFound without distinguishing replay from
		// garbage.
		if errors.Is(err, storage.ErrNotFound) {
			_ = s.cfg.Store.RevokeFamilyByCode(ctx, codeHash)
		}
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "code unknown, used, or expired")
		return
	}
	if ac.ClientID != clientID || ac.RedirectURI != redirectURI {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "client_id or redirect_uri mismatch")
		return
	}
	if !verifyPKCE(verifier, ac.CodeChallenge) {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "PKCE verifier mismatch")
		return
	}
	access, refresh, err := s.issueTokenPair(ctx, clientID, ac.UserID, codeHash)
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "")
		return
	}
	writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.cfg.AccessTokenTTL.Seconds()),
		RefreshToken: refresh,
	})
}
```

Replace `grantRefreshToken`:

```go
func (s *Server) grantRefreshToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clientID, err := authenticateClient(ctx, s.cfg.Store, r)
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client", "")
		return
	}
	rt := r.PostForm.Get("refresh_token")
	if rt == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "")
		return
	}
	hash := storage.HashToken(rt)

	// Pre-read to detect prior-revocation as the reuse-attack signal. The
	// pre-read + RotateRefreshToken pair is safe under concurrency: rotation
	// is what's atomic; the pre-read only adds the family-revoke branch for
	// already-revoked rows. Garbage / expired tokens take the same flat
	// invalid_grant exit as a lost rotation race.
	if pre, err := s.cfg.Store.GetToken(ctx, hash); err == nil && pre.Kind == "refresh" && pre.RevokedAt != nil {
		_ = s.cfg.Store.RevokeRefreshFamily(ctx, hash)
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "")
		return
	}

	tok, err := s.cfg.Store.RotateRefreshToken(ctx, hash)
	if err != nil {
		// Lost the race, or garbage, or expired. RFC 6749-compliant
		// invalid_grant — do NOT revoke the family because we can't
		// distinguish unknown-token from rotated-by-someone-else.
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "")
		return
	}
	if tok.ClientID != clientID {
		// Mismatched client — revoke the family defensively since a leaked
		// refresh token + bogus client_id is a strong attack signal.
		_ = s.cfg.Store.RevokeRefreshFamily(ctx, hash)
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "client_id mismatch")
		return
	}
	access, refresh, err := s.issueTokenPair(ctx, clientID, tok.UserID, nil)
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "")
		return
	}
	writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.cfg.AccessTokenTTL.Seconds()),
		RefreshToken: refresh,
	})
}
```

- [ ] **Step 4: Run all server tests to verify nothing else regressed**

Run: `go test ./internal/oauth/server/ -run TestToken -v`
Expected: every `TestToken_*` case PASSES, including the new five confidential-client cases and every pre-existing public-client case (`TestToken_AuthorizationCode_Success`, `TestToken_RefreshGrant_Rotates`, the concurrency and replay cases). The existing tests pass because `seedClientRecord` creates clients with `ClientSecretHash = nil`, which the helper treats as public.

- [ ] **Step 5: Commit**

```bash
git add internal/oauth/server/token.go internal/oauth/server/token_test.go
git commit -m "$(cat <<'EOF'
feat(oauth/server): authenticate confidential clients on /oauth/token

grantAuthorizationCode and grantRefreshToken now resolve client_id via
authenticateClient (RFC 6749 §2.3.1, Basic + form). Clients with a
non-nil stored ClientSecretHash MUST present the matching secret; clients
with nil hash (the public/PKCE-only path, including every row registered
before Task 1) continue to authenticate via PKCE alone.

invalid_client is returned without distinguishing missing-secret from
wrong-secret from unknown-client, so the endpoint cannot be used as a
client-id oracle.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Advertise the new auth methods in discovery

**Files:**
- Modify: `internal/oauth/server/discovery.go`
- Modify: `internal/oauth/server/discovery_test.go`

- [ ] **Step 1: Write the failing test**

Replace the auth-methods assertion at the bottom of `TestAuthorizationServerMetadata` in `internal/oauth/server/discovery_test.go` (current lines 60–63) with:

```go
	authMethods, _ := got["token_endpoint_auth_methods_supported"].([]any)
	wantAuthMethods := []string{"client_secret_basic", "client_secret_post", "none"}
	if len(authMethods) != len(wantAuthMethods) {
		t.Fatalf("token_endpoint_auth_methods_supported = %v, want %v", authMethods, wantAuthMethods)
	}
	for i, want := range wantAuthMethods {
		if authMethods[i] != want {
			t.Errorf("auth method [%d] = %v, want %s", i, authMethods[i], want)
		}
	}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/oauth/server/ -run TestAuthorizationServerMetadata -v`
Expected: FAIL — current discovery doc only advertises `["none"]`.

- [ ] **Step 3: Update the discovery handler**

In `internal/oauth/server/discovery.go`, replace line 27:

```go
		"token_endpoint_auth_methods_supported": []string{"none"},
```

with:

```go
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post", "none"},
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/oauth/server/ -run TestAuthorizationServerMetadata -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/oauth/server/discovery.go internal/oauth/server/discovery_test.go
git commit -m "$(cat <<'EOF'
feat(oauth/server): advertise client_secret_{basic,post} in discovery

token_endpoint_auth_methods_supported now reports the full set the token
endpoint accepts: client_secret_basic, client_secret_post, and none (for
public clients registered before DCR started issuing secrets). MCP
clients that select an auth method from this list will now see and
choose a confidential one.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: CHANGELOG entry

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Insert the Unreleased entry**

Replace line 8 of `CHANGELOG.md` (the bare `## [Unreleased]`) with the block below. Leave everything from `## [0.9.3] - 2026-05-18` downward untouched.

```markdown
## [Unreleased]

### Fixed
- **`organizze-mcp-oauth` Dynamic Client Registration now returns a `client_secret`, unblocking Perplexity.** Perplexity's MCP client rejects DCR responses without a secret with `{"error_code":"DCR_CLIENT_SECRET_REQUIRED","message":"Dynamic client registration did not return a client_secret"}`. RFC 7591 §3.2.1 permits public clients (the prior behavior), but several MCP clients enforce confidential clients in practice. `POST /oauth/register` now generates a random 32-byte secret, persists `sha256(secret)` in the existing `oauth_clients.client_secret_hash` column, and returns the plaintext secret plus `client_id_issued_at`, `client_secret_expires_at: 0` (never expires), and `token_endpoint_auth_method: "client_secret_basic"`.

### Changed
- **`POST /oauth/token` now authenticates confidential clients.** Both `authorization_code` and `refresh_token` grants accept `client_secret_basic` (HTTP Basic) and `client_secret_post` (form body) per RFC 6749 §2.3.1, verifying the presented secret via constant-time compare against the stored sha256 hash. Clients whose stored `client_secret_hash` is NULL — every row registered before this release — continue to authenticate via PKCE alone, so no re-registration is needed for ChatGPT or Claude installations already in production. `invalid_client` is returned without distinguishing missing-secret from wrong-secret from unknown-client so the endpoint cannot be used as a client-id oracle.
- **`.well-known/oauth-authorization-server` advertises the new methods.** `token_endpoint_auth_methods_supported` is now `["client_secret_basic", "client_secret_post", "none"]` (was `["none"]`).
```

- [ ] **Step 2: Commit**

```bash
git add CHANGELOG.md
git commit -m "$(cat <<'EOF'
docs(changelog): record Perplexity DCR fix + confidential-client support

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Verify, push, open PR

**Files:** none

- [ ] **Step 1: Run the full local verification suite**

Run, in order:

```bash
make test
make lint
make build
```

Expected: every command exits 0. `make test` is `go test ./...`; `make lint` is `go vet ./...`; `make build` writes `bin/organizze-mcp`. Per CI, also mirror the race-test:

```bash
go test ./... -race -count=1
```

Expected: 0 failures, 0 races.

- [ ] **Step 2: Push the branch**

The worktree was created on the auto-named branch `claude/stoic-spence-db7841`. Push it under the AGENTS.md-conventional name in a single step:

```bash
git push -u origin HEAD:feat/oauth-confidential-clients
```

If the local branch is already named `feat/oauth-confidential-clients` (e.g. you ran `git branch -m feat/oauth-confidential-clients` first), drop the `HEAD:` mapping.

- [ ] **Step 3: Open the PR**

```bash
gh pr create --title "feat(oauth): issue client_secret on DCR (Perplexity compat)" --body "$(cat <<'EOF'
## Summary
- `POST /oauth/register` now issues a `client_secret` and returns RFC 7591 §3.2.1 metadata (`client_id_issued_at`, `client_secret_expires_at: 0`, `token_endpoint_auth_method: "client_secret_basic"`).
- `POST /oauth/token` now authenticates confidential clients via `client_secret_basic` / `client_secret_post`. Clients whose stored hash is NULL (every row registered before this PR) continue to authenticate via PKCE alone — no re-registration needed for existing ChatGPT/Claude installations.
- `.well-known/oauth-authorization-server` advertises `["client_secret_basic", "client_secret_post", "none"]`.

## Why
Perplexity's MCP client rejects DCR responses without a `client_secret`:

```
{"detail":{"error_code":"DCR_CLIENT_SECRET_REQUIRED","message":"Dynamic client registration did not return a client_secret"}}
```

RFC 7591 permits public clients, but several real-world MCP clients enforce confidential clients. The persistence layer was already shaped for this (`storage.Client.ClientSecretHash`, nullable `oauth_clients.client_secret_hash`) — only the HTTP layer needed to start issuing and verifying.

## Test Plan
- [x] `go test ./internal/oauth/server/ -v` — every existing test still passes (public-client path is preserved via NULL `ClientSecretHash`).
- [x] New tests cover: DCR returns secret + persists hash; Basic-auth accepted; form-secret accepted; missing secret rejected; wrong secret rejected; refresh-grant requires secret on confidential clients; public clients still work without a secret; discovery advertises the new methods.
- [x] `make test && make lint && make build`
- [x] `go test ./... -race -count=1`

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

- [ ] **Step 4: Wait for CI**

```bash
PR=$(gh pr view --json number --jq '.number')
until [ "$(gh pr view "$PR" --json statusCheckRollup --jq '.statusCheckRollup[0].status')" = "COMPLETED" ]; do sleep 5; done
gh pr view "$PR" --json statusCheckRollup --jq '.statusCheckRollup'
```

Expected: every check `COMPLETED / SUCCESS`.

- [ ] **Step 5: Merge**

```bash
gh pr merge "$PR" --squash --delete-branch
gh pr view "$PR" --json state,mergedAt
```

Expected: `"state":"MERGED"`. The local `--delete-branch` step may emit `fatal: 'main' is already used by worktree at ...` — that's the harness, not GitHub. The PR state above is the source of truth.

- [ ] **Step 6: Exit the worktree**

From the harness, call `ExitWorktree({action: "remove", discard_changes: true})`. The local feature commits were rewritten on `main` by the squash, so their SHAs are stale; discarding is safe.

- [ ] **Step 7: Pull main in the main checkout**

```bash
git checkout main && git pull
```

If a loose copy of `docs/superpowers/plans/2026-05-18-oauth-confidential-clients-for-perplexity.md` exists in main's working tree from before the worktree was created, it will block the pull — `rm` it first.

---

## Notes for the executor

- **Do not touch `internal/oauth/storage/`.** The column, struct field, and `CreateClient` parameter for the secret hash already exist; this PR only flips behavior in the HTTP layer.
- **Do not add a database migration.** `client_secret_hash` is already `BYTEA NULL` in `001_init.sql`.
- **Do not change the revoke endpoint.** RFC 7009 §2.1 says confidential clients SHOULD authenticate to revoke; no MCP client we know of requires it, and the existing endpoint returns 200 even for unknown tokens, so it leaks nothing. Add later if a client demands it.
- **Backward compat is load-bearing.** Every row in `oauth_clients` that exists today has `client_secret_hash = NULL`. The `authenticateClient` helper's `client.ClientSecretHash == nil` branch is what keeps ChatGPT and Claude working after deploy. Do not "simplify" by requiring a secret unconditionally.
- **Release follows the normal AGENTS.md flow.** This PR ships under `## [Unreleased]`; a follow-up `chore/release-vX.Y.Z` PR promotes the header and tags. Patch bump (additive, backward-compatible) is appropriate per AGENTS.md's semver guidance — though the user-visible "Perplexity now works" framing is reasonable as a minor bump too. Defer that to the release PR.
