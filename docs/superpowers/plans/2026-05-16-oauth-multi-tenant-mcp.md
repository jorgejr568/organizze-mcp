# OAuth Multi-Tenant MCP Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a second binary (`cmd/organizze-mcp-oauth/`) that exposes the existing MCP toolset over Streamable HTTP behind a self-hosted OAuth 2.1 Authorization Server, so each ChatGPT user authenticates with their own Organizze credentials instead of the operator embedding a single set in env vars. The original `cmd/organizze-mcp/` binary (stdio + plain HTTP, single-tenant via env vars) is unchanged.

**Architecture:**
- Two binaries in one Go module — they share `internal/domain`, `internal/usecase`, `internal/adapter/organizze`, `internal/adapter/mcp`. New OAuth-specific packages live under `internal/oauth/`. The single-tenant binary does not import the new packages, so its runtime artifact is unchanged.
- `RequestExecutor` is refactored to call a `CredentialsProvider func(ctx) (email, apiKey, userAgent, error)` per request, so the same executor (and all repositories above it) works for both binaries — single-tenant plugs in a static provider; OAuth plugs in a ctx-reading one.
- The OAuth binary hosts the MCP endpoint at `/mcp`, the discovery docs at `/.well-known/oauth-protected-resource` and `/.well-known/oauth-authorization-server`, and the OAuth endpoints (`/oauth/register`, `/oauth/authorize`, `/oauth/token`, `/oauth/revoke`). On the first authorize, the user enters their Organizze email + API key into a small HTML form. The server validates them against Organizze, encrypts the API key with AES-GCM, persists, and issues access + refresh tokens. Subsequent authorizes from the same browser reuse a session cookie.
- Postgres for storage (pgx, same as `cmd/consumer/`). Migrations applied out-of-band via `make migrate-up` (mirrors the consumer pattern). Schema lives at `internal/oauth/storage/migrations/001_init.sql`.

**Tech Stack:**
- Go 1.25 (same as the rest of the repo)
- `github.com/jackc/pgx/v5` (Postgres driver; same as `cmd/consumer/`)
- `github.com/modelcontextprotocol/go-sdk` v1.6.0 (existing)
- stdlib only for OAuth — no `fosite`/`oauth2-server` library. The MCP-required subset (PKCE-S256 auth code + refresh + DCR + two `.well-known` docs) is small and we want to own the code.
- stdlib `crypto/aes` + `crypto/cipher` (AES-256-GCM) for at-rest Organizze API-key encryption
- stdlib `html/template` for the consent page
- `github.com/stretchr/testify` is NOT used — match repo style (raw `testing.T` + manual asserts)

---

## File Structure

**Created:**
```
cmd/organizze-mcp-oauth/
  main.go                                  # composition root; wires storage → oauth server → MCP
  main_test.go                             # end-to-end OAuth-then-tool integration test
  Dockerfile                               # multi-stage build for the new binary
  README.md                                # operator-facing setup notes

internal/config/oauth_config.go            # OAuth-binary-specific env parsing
internal/config/oauth_config_test.go

internal/oauth/storage/
  storage.go                               # interfaces + record types (User, Client, Code, Token, Session)
  errors.go                                # ErrNotFound, ErrConflict
  crypto.go                                # AES-GCM seal/open + SHA-256 token hashing
  crypto_test.go
  postgres.go                              # pgx-backed implementation of Store
  postgres_test.go                         # integration test (skipped without OAUTH_DATABASE_URL)
  migrate.go                               # embed migrations + ApplyMigrations(ctx, pool)
  migrate_test.go
  migrations/
    001_init.sql
    embed.go                               # //go:embed migrations/*.sql

internal/oauth/credprovider/
  credprovider.go                          # CredentialsProvider type + Static + FromContext + WithCredentials
  credprovider_test.go

internal/oauth/server/
  server.go                                # http.Handler with all routes mounted
  discovery.go                             # /.well-known/* handlers
  discovery_test.go
  register.go                              # POST /oauth/register (DCR)
  register_test.go
  authorize.go                             # GET + POST /oauth/authorize, including login form
  authorize_test.go
  token.go                                 # POST /oauth/token (authorization_code, refresh_token)
  token_test.go
  revoke.go                                # POST /oauth/revoke
  revoke_test.go
  middleware.go                            # bearer-auth middleware for /mcp
  middleware_test.go
  session.go                               # signed-cookie browser session
  session_test.go
  templates/
    login.html                             # consent + creds entry form

CHANGELOG.md                               # (modify) new Unreleased entry
Makefile                                   # (modify) oauth-build, oauth-migrate-up targets
```

**Modified:**
```
internal/adapter/organizze/executor.go     # accept CredentialsProvider; resolve creds per request
internal/adapter/organizze/executor_test.go
cmd/organizze-mcp/main.go                  # build a Static credentials provider from env config
go.mod / go.sum                            # add pgx/v5
```

**Untouched (verify after refactor):**
- `internal/domain/*`
- `internal/usecase/*`
- `internal/adapter/organizze/*_repository*.go` (they consume `*RequestExecutor` interfaces, not the struct directly — refactor must preserve method signatures)
- `internal/adapter/mcp/*` other than the executor's call sites
- `cmd/ingest/`, `cmd/consumer/`

---

## Task 1: Add pgx dependency and scaffold the new binary skeleton

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `cmd/organizze-mcp-oauth/main.go`
- Modify: `Makefile`

- [ ] **Step 1: Add the pgx v5 dependency**

Run: `go get github.com/jackc/pgx/v5@latest`
Expected: `go.mod` updated with `github.com/jackc/pgx/v5 vX.Y.Z` in the require block; `go.sum` updated.

- [ ] **Step 2: Create the new binary's `main.go` with a stub that just prints and exits cleanly**

Create `cmd/organizze-mcp-oauth/main.go`:
```go
// Command organizze-mcp-oauth is the multi-tenant variant of organizze-mcp.
// It hosts an OAuth 2.1 Authorization Server and serves the same MCP toolset
// over Streamable HTTP, resolving each caller's Organizze credentials from
// the validated bearer token instead of process-wide env vars.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "organizze-mcp-oauth:", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("organizze-mcp-oauth: scaffolding — wire-up follows in later tasks")
	return nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./cmd/organizze-mcp-oauth/`
Expected: exits with no output, produces `organizze-mcp-oauth` in the repo root.

- [ ] **Step 4: Add Makefile targets for the new binary**

Append to `Makefile` (look at the existing `build:` target for style):
```makefile
.PHONY: oauth-build oauth-migrate-up oauth-migrate-down

oauth-build:
	go build -o bin/organizze-mcp-oauth ./cmd/organizze-mcp-oauth

oauth-migrate-up:
	@test -n "$$OAUTH_DATABASE_URL" || (echo "OAUTH_DATABASE_URL must be set" && exit 1)
	psql "$$OAUTH_DATABASE_URL" -v ON_ERROR_STOP=1 -f internal/oauth/storage/migrations/001_init.sql

oauth-migrate-down:
	@test -n "$$OAUTH_DATABASE_URL" || (echo "OAUTH_DATABASE_URL must be set" && exit 1)
	psql "$$OAUTH_DATABASE_URL" -c "DROP TABLE IF EXISTS oauth_tokens, oauth_codes, oauth_clients, oauth_sessions, oauth_users CASCADE;"
```

- [ ] **Step 5: Verify the Makefile target builds**

Run: `make oauth-build`
Expected: produces `bin/organizze-mcp-oauth`.

Run: `rm bin/organizze-mcp-oauth ./organizze-mcp-oauth`
Expected: cleanup, no output.

- [ ] **Step 6: Run the full test + lint + build to confirm nothing else broke**

Run: `make test && make lint && make build`
Expected: all three succeed (`make build` still produces the original `bin/organizze-mcp`).

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum cmd/organizze-mcp-oauth/main.go Makefile
git commit -m "$(cat <<'EOF'
chore(oauth): scaffold organizze-mcp-oauth binary

Stub-only entry point and pgx/v5 dependency; subsequent tasks build
the storage, OAuth AS, and MCP wiring on top.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Credentials provider abstraction

**Files:**
- Create: `internal/oauth/credprovider/credprovider.go`
- Create: `internal/oauth/credprovider/credprovider_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/oauth/credprovider/credprovider_test.go`:
```go
package credprovider

import (
	"context"
	"errors"
	"testing"
)

func TestStatic_ReturnsValues(t *testing.T) {
	p := Static("e@x.com", "key", "UA")
	email, apiKey, ua, err := p(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if email != "e@x.com" || apiKey != "key" || ua != "UA" {
		t.Errorf("got %q,%q,%q", email, apiKey, ua)
	}
}

func TestFromContext_MissingErrors(t *testing.T) {
	_, _, _, err := FromContext(context.Background())
	if !errors.Is(err, ErrNoCredentials) {
		t.Errorf("expected ErrNoCredentials, got %v", err)
	}
}

func TestWithCredentialsThenFromContext(t *testing.T) {
	ctx := WithCredentials(context.Background(), Credentials{
		Email: "e@x.com", APIKey: "k", UserAgent: "UA",
	})
	email, apiKey, ua, err := FromContext(ctx)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if email != "e@x.com" || apiKey != "k" || ua != "UA" {
		t.Errorf("got %q,%q,%q", email, apiKey, ua)
	}
}
```

- [ ] **Step 2: Run tests, expect compile failure**

Run: `go test ./internal/oauth/credprovider/...`
Expected: build errors — package doesn't exist yet.

- [ ] **Step 3: Implement the package**

Create `internal/oauth/credprovider/credprovider.go`:
```go
// Package credprovider supplies per-request Organizze credentials to the
// RequestExecutor. The single-tenant binary uses Static; the OAuth binary
// uses FromContext after bearer-auth middleware has populated the context.
package credprovider

import (
	"context"
	"errors"
)

// ErrNoCredentials means the request context did not carry credentials.
// In the OAuth binary this should be impossible past the bearer middleware;
// surfacing it means a tool was invoked outside the authenticated path.
var ErrNoCredentials = errors.New("credprovider: no credentials in context")

// Credentials is the per-request triple the Organizze API needs.
type Credentials struct {
	Email     string
	APIKey    string
	UserAgent string
}

// CredentialsProvider resolves credentials for a single outbound call.
// Implementations must be safe for concurrent use.
type CredentialsProvider func(ctx context.Context) (email, apiKey, userAgent string, err error)

// Static returns a provider that always yields the given values.
func Static(email, apiKey, userAgent string) CredentialsProvider {
	return func(_ context.Context) (string, string, string, error) {
		return email, apiKey, userAgent, nil
	}
}

// FromContext reads credentials that WithCredentials placed on ctx.
func FromContext(ctx context.Context) (email, apiKey, userAgent string, err error) {
	c, ok := ctx.Value(ctxKey{}).(Credentials)
	if !ok {
		return "", "", "", ErrNoCredentials
	}
	return c.Email, c.APIKey, c.UserAgent, nil
}

// WithCredentials returns a child ctx carrying c.
func WithCredentials(ctx context.Context, c Credentials) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

type ctxKey struct{}
```

- [ ] **Step 4: Run tests, expect pass**

Run: `go test ./internal/oauth/credprovider/... -v`
Expected: all three tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/oauth/credprovider/
git commit -m "$(cat <<'EOF'
feat(oauth): add credprovider abstraction

Introduces CredentialsProvider, Static, FromContext, and WithCredentials
so the executor refactor in the next task can decouple credential sourcing
from executor construction without touching repositories or services.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Refactor RequestExecutor to resolve credentials per request

**Files:**
- Modify: `internal/adapter/organizze/executor.go`
- Modify: `internal/adapter/organizze/executor_test.go`
- Modify: `cmd/organizze-mcp/main.go`

This task changes the executor's construction signature. The repositories that consume it (`*_repository.go`) only call `Get/Post/Put/Delete` — their bodies don't change. The composition root in `cmd/organizze-mcp/main.go` does change, to wrap the existing env-loaded fields in a `credprovider.Static`.

- [ ] **Step 1: Add a new failing test asserting the executor reads creds per request**

Append to `internal/adapter/organizze/executor_test.go`:
```go
func TestExecutor_ResolvesCredentialsPerRequest(t *testing.T) {
	var gotAuths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuths = append(gotAuths, r.Header.Get("Authorization"))
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(srv.Close)

	calls := 0
	provider := func(ctx context.Context) (string, string, string, error) {
		calls++
		if calls == 1 {
			return "first@x.com", "key1", "UA", nil
		}
		return "second@x.com", "key2", "UA", nil
	}

	exec, err := NewRequestExecutor(RequestExecutorOptions{
		HTTPClient:  NewClient(ClientOptions{}),
		BaseURL:     srv.URL,
		Credentials: provider,
	})
	if err != nil {
		t.Fatalf("NewRequestExecutor: %v", err)
	}
	for i := 0; i < 2; i++ {
		var out struct{}
		if err := exec.Get(context.Background(), "/x", &out); err != nil {
			t.Fatalf("Get: %v", err)
		}
	}
	if len(gotAuths) != 2 || gotAuths[0] == gotAuths[1] {
		t.Errorf("expected two distinct Authorization headers, got %v", gotAuths)
	}
}

func TestExecutor_ReturnsProviderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Errorf("upstream should not be called when provider errors")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	exec, err := NewRequestExecutor(RequestExecutorOptions{
		HTTPClient:  NewClient(ClientOptions{}),
		BaseURL:     srv.URL,
		Credentials: func(context.Context) (string, string, string, error) {
			return "", "", "", errors.New("boom")
		},
	})
	if err != nil {
		t.Fatalf("NewRequestExecutor: %v", err)
	}
	if err := exec.Get(context.Background(), "/x", nil); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected boom error, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests, expect compile failure**

Run: `go test ./internal/adapter/organizze/ -run TestExecutor_Resolves -v`
Expected: build error — `RequestExecutorOptions` has no `Credentials` field.

- [ ] **Step 3: Refactor the executor**

Edit `internal/adapter/organizze/executor.go`:

Replace the `RequestExecutorOptions` struct:
```go
// RequestExecutorOptions configures a RequestExecutor.
type RequestExecutorOptions struct {
	HTTPClient HTTPClient // required
	BaseURL    string     // required (no trailing slash)

	// Credentials resolves the per-request Basic-Auth pair and User-Agent.
	// Required. Single-tenant callers wrap their env-derived values in
	// credprovider.Static; the OAuth binary uses credprovider.FromContext
	// after bearer middleware has populated the context.
	Credentials credprovider.CredentialsProvider

	LogRequests bool
	LogWriter   io.Writer
}
```

Replace the `RequestExecutor` struct:
```go
type RequestExecutor struct {
	client      HTTPClient
	baseURL     string
	credentials credprovider.CredentialsProvider
	logRequests bool
	logWriter   io.Writer
}
```

Replace `NewRequestExecutor`:
```go
func NewRequestExecutor(opts RequestExecutorOptions) (*RequestExecutor, error) {
	switch {
	case opts.HTTPClient == nil:
		return nil, errors.New("organizze: HTTPClient is required")
	case opts.BaseURL == "":
		return nil, errors.New("organizze: BaseURL is required")
	case opts.Credentials == nil:
		return nil, errors.New("organizze: Credentials provider is required")
	}
	w := opts.LogWriter
	if w == nil {
		w = os.Stderr
	}
	return &RequestExecutor{
		client:      opts.HTTPClient,
		baseURL:     opts.BaseURL,
		credentials: opts.Credentials,
		logRequests: opts.LogRequests,
		logWriter:   w,
	}, nil
}
```

In `do`, replace the lines that set Basic auth and User-Agent (currently lines 115–116):
```go
	email, apiKey, ua, err := e.credentials(ctx)
	if err != nil {
		return fmt.Errorf("organizze: resolve credentials: %w", err)
	}
	req.SetBasicAuth(email, apiKey)
	req.Header.Set("User-Agent", ua)
```

Add the import:
```go
import (
	// existing imports ...
	"github.com/jorgejr568/organizze-mcp/internal/oauth/credprovider"
)
```

- [ ] **Step 4: Update the existing executor tests that constructed via the old fields**

Find `newTestExecutor` in `internal/adapter/organizze/executor_test.go` and update it (and any other test using `Email`/`APIKey`/`UserAgent` directly) to wrap the literal values in `credprovider.Static`:
```go
exec, _ := NewRequestExecutor(RequestExecutorOptions{
	HTTPClient:  NewClient(ClientOptions{}),
	BaseURL:     srv.URL,
	Credentials: credprovider.Static("e@x.com", "test-key", "Test (test@example.com)"),
})
```

Also update `TestNewRequestExecutor_RejectsMissingRequired` to construct cases that test the new validation rule (missing `Credentials`) and remove the cases that tested the removed string fields. The new table:
```go
cases := []RequestExecutorOptions{
	{HTTPClient: c, BaseURL: ""},                                                  // missing BaseURL
	{HTTPClient: c, BaseURL: "https://x"},                                         // missing Credentials
	{HTTPClient: nil, BaseURL: "https://x", Credentials: credprovider.Static("e", "k", "ua")},
}
```

Add the import to the test file:
```go
"github.com/jorgejr568/organizze-mcp/internal/oauth/credprovider"
```

- [ ] **Step 5: Update the single-tenant composition root**

Edit `cmd/organizze-mcp/main.go`'s `buildServer` (lines 56–67). Replace the executor construction with:
```go
exec, err := organizze.NewRequestExecutor(organizze.RequestExecutorOptions{
	HTTPClient:  httpClient,
	BaseURL:     cfg.BaseURL,
	Credentials: credprovider.Static(cfg.Email, cfg.APIKey, cfg.UserAgent),
	LogRequests: cfg.LogRequests,
})
```

Add the import:
```go
"github.com/jorgejr568/organizze-mcp/internal/oauth/credprovider"
```

- [ ] **Step 6: Run all tests**

Run: `go test ./... -count=1`
Expected: all pass. Pay special attention to `internal/adapter/organizze/` (the executor change is here) and `internal/adapter/mcp/` integration test (it should keep working because it uses the executor through the same interface).

- [ ] **Step 7: Run race + full verification**

Run: `make test && make lint && make build`
Expected: all green; `bin/organizze-mcp` still produced.

- [ ] **Step 8: Commit**

```bash
git add internal/adapter/organizze/executor.go internal/adapter/organizze/executor_test.go cmd/organizze-mcp/main.go
git commit -m "$(cat <<'EOF'
refactor(executor): resolve credentials per request via CredentialsProvider

Replaces the executor's Email/APIKey/UserAgent fields with a context-aware
CredentialsProvider so the OAuth binary can inject per-request credentials
without touching repositories or services. Single-tenant callers wrap
their env-loaded values in credprovider.Static — behavior is unchanged.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Postgres schema + migrations file

**Files:**
- Create: `internal/oauth/storage/migrations/001_init.sql`
- Create: `internal/oauth/storage/migrations/embed.go`

- [ ] **Step 1: Write the schema**

Create `internal/oauth/storage/migrations/001_init.sql`:
```sql
-- 001_init.sql
-- Schema for the multi-tenant OAuth Authorization Server.
--
-- All token columns store SHA-256 hashes, not raw tokens — a DB compromise
-- must not equal active-session takeover. The Organizze API key is stored
-- AES-GCM-encrypted; the encryption key is held only in the operator's env
-- (OAUTH_ENCRYPTION_KEY), so a DB-only leak cannot recover plaintext keys.

BEGIN;

CREATE TABLE IF NOT EXISTS oauth_users (
    id                       BIGSERIAL PRIMARY KEY,
    organizze_email          TEXT NOT NULL UNIQUE,
    organizze_api_key_cipher BYTEA NOT NULL,
    organizze_api_key_nonce  BYTEA NOT NULL,
    user_agent               TEXT NOT NULL,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS oauth_clients (
    id                   TEXT PRIMARY KEY,             -- public client_id
    client_secret_hash   BYTEA,                        -- NULL for public clients (PKCE-only)
    client_name          TEXT NOT NULL,
    redirect_uris        JSONB NOT NULL,               -- ["https://...", ...]
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS oauth_sessions (
    id           TEXT PRIMARY KEY,                     -- random; the cookie value
    user_id      BIGINT NOT NULL REFERENCES oauth_users(id) ON DELETE CASCADE,
    expires_at   TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS oauth_sessions_expires_idx ON oauth_sessions (expires_at);

CREATE TABLE IF NOT EXISTS oauth_codes (
    code_hash             BYTEA PRIMARY KEY,
    client_id             TEXT NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    user_id               BIGINT NOT NULL REFERENCES oauth_users(id) ON DELETE CASCADE,
    redirect_uri          TEXT NOT NULL,
    code_challenge        TEXT NOT NULL,
    code_challenge_method TEXT NOT NULL CHECK (code_challenge_method = 'S256'),
    scope                 TEXT NOT NULL DEFAULT '',
    expires_at            TIMESTAMPTZ NOT NULL,
    consumed_at           TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS oauth_tokens (
    token_hash   BYTEA PRIMARY KEY,
    kind         TEXT NOT NULL CHECK (kind IN ('access', 'refresh')),
    client_id    TEXT NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    user_id      BIGINT NOT NULL REFERENCES oauth_users(id) ON DELETE CASCADE,
    refresh_for  BYTEA REFERENCES oauth_tokens(token_hash) ON DELETE SET NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS oauth_tokens_expires_idx ON oauth_tokens (expires_at);
CREATE INDEX IF NOT EXISTS oauth_tokens_user_idx    ON oauth_tokens (user_id);

COMMIT;
```

- [ ] **Step 2: Create the embed wrapper**

Create `internal/oauth/storage/migrations/embed.go`:
```go
// Package migrations embeds the OAuth-server SQL files so the binary
// can ship them and ApplyMigrations can run them. SQL files are the
// source of truth; this file is just the embed bridge.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```

- [ ] **Step 3: Verify the embed compiles**

Run: `go build ./internal/oauth/storage/migrations/`
Expected: no output, success.

- [ ] **Step 4: Commit**

```bash
git add internal/oauth/storage/migrations/
git commit -m "$(cat <<'EOF'
feat(oauth): add Postgres schema for the OAuth AS

Five tables: oauth_users, oauth_clients, oauth_sessions, oauth_codes,
oauth_tokens. Tokens stored as SHA-256 hashes; Organizze API keys
stored AES-GCM-encrypted. Embedded for in-binary migration.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Crypto helpers (AES-GCM + SHA-256 hashing)

**Files:**
- Create: `internal/oauth/storage/crypto.go`
- Create: `internal/oauth/storage/crypto_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/oauth/storage/crypto_test.go`:
```go
package storage

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func mustKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

func TestSealOpenRoundTrip(t *testing.T) {
	c, err := NewCipher(mustKey(t))
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	plain := []byte("organizze-api-key-very-secret")
	ciphertext, nonce, err := c.Seal(plain)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Contains(ciphertext, plain) {
		t.Error("ciphertext contains plaintext substring")
	}
	got, err := c.Open(ciphertext, nonce)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("Open got %q, want %q", got, plain)
	}
}

func TestOpenWithWrongNonceFails(t *testing.T) {
	c, _ := NewCipher(mustKey(t))
	ct, _, _ := c.Seal([]byte("x"))
	badNonce := make([]byte, 12)
	if _, err := c.Open(ct, badNonce); err == nil {
		t.Error("expected error opening with wrong nonce")
	}
}

func TestNewCipherRejectsBadKeyLength(t *testing.T) {
	if _, err := NewCipher([]byte("short")); err == nil {
		t.Error("expected error for 5-byte key")
	}
}

func TestHashTokenIsDeterministicAnd32Bytes(t *testing.T) {
	a := HashToken("hello")
	b := HashToken("hello")
	c := HashToken("world")
	if !bytes.Equal(a, b) {
		t.Error("HashToken not deterministic")
	}
	if bytes.Equal(a, c) {
		t.Error("HashToken collisions on different inputs")
	}
	if len(a) != 32 {
		t.Errorf("HashToken length = %d, want 32", len(a))
	}
}
```

- [ ] **Step 2: Run tests, expect failure**

Run: `go test ./internal/oauth/storage/ -run TestSeal -v`
Expected: build errors — package files don't exist.

- [ ] **Step 3: Implement crypto helpers**

Create `internal/oauth/storage/crypto.go`:
```go
package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
)

// Cipher seals and opens secrets with AES-256-GCM. The 32-byte key is the
// OAUTH_ENCRYPTION_KEY env value (hex-decoded). Losing the key means every
// stored Organizze api_key becomes unreadable — back it up.
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher returns a Cipher bound to key (must be exactly 32 bytes).
func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("oauth/storage: encryption key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("oauth/storage: aes.NewCipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("oauth/storage: cipher.NewGCM: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// Seal encrypts plaintext, returning (ciphertext, nonce). Nonce is freshly
// random per call — DO NOT reuse a nonce with the same key.
func (c *Cipher) Seal(plaintext []byte) (ciphertext, nonce []byte, err error) {
	nonce = make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("oauth/storage: read nonce: %w", err)
	}
	return c.aead.Seal(nil, nonce, plaintext, nil), nonce, nil
}

// Open decrypts ciphertext using the given nonce.
func (c *Cipher) Open(ciphertext, nonce []byte) ([]byte, error) {
	if len(nonce) != c.aead.NonceSize() {
		return nil, errors.New("oauth/storage: nonce length mismatch")
	}
	out, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("oauth/storage: gcm.Open: %w", err)
	}
	return out, nil
}

// HashToken returns the SHA-256 hash of token, used as the primary-key
// column for oauth_codes and oauth_tokens. The raw token never lands in
// the DB, so a DB leak cannot replay sessions.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
```

- [ ] **Step 4: Run tests, expect pass**

Run: `go test ./internal/oauth/storage/ -v`
Expected: 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/oauth/storage/crypto.go internal/oauth/storage/crypto_test.go
git commit -m "$(cat <<'EOF'
feat(oauth/storage): AES-GCM cipher + SHA-256 token hashing

Cipher wraps AES-256-GCM with a fresh nonce per Seal. HashToken
gives us deterministic primary keys for oauth_codes/oauth_tokens
without storing raw token bytes.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Storage interfaces + record types + ApplyMigrations

**Files:**
- Create: `internal/oauth/storage/storage.go`
- Create: `internal/oauth/storage/errors.go`
- Create: `internal/oauth/storage/migrate.go`
- Create: `internal/oauth/storage/migrate_test.go`

- [ ] **Step 1: Define record types and the Store interface**

Create `internal/oauth/storage/errors.go`:
```go
package storage

import "errors"

var (
	ErrNotFound = errors.New("oauth/storage: not found")
	ErrConflict = errors.New("oauth/storage: conflict")
)
```

Create `internal/oauth/storage/storage.go`:
```go
// Package storage is the persistence layer for the OAuth Authorization Server.
// Store is the single interface every server handler consumes; postgres.go is
// the only concrete implementation in this iteration.
package storage

import (
	"context"
	"time"
)

type User struct {
	ID              int64
	OrganizzeEmail  string
	APIKeyCipher    []byte
	APIKeyNonce     []byte
	UserAgent       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Client struct {
	ID               string
	ClientSecretHash []byte // nil for public/PKCE-only clients
	ClientName       string
	RedirectURIs     []string
	CreatedAt        time.Time
}

type Session struct {
	ID        string
	UserID    int64
	ExpiresAt time.Time
	CreatedAt time.Time
}

type AuthCode struct {
	CodeHash            []byte
	ClientID            string
	UserID              int64
	RedirectURI         string
	CodeChallenge       string
	CodeChallengeMethod string // always "S256"
	Scope               string
	ExpiresAt           time.Time
	ConsumedAt          *time.Time
}

type Token struct {
	TokenHash   []byte
	Kind        string // "access" or "refresh"
	ClientID    string
	UserID      int64
	RefreshFor  []byte // for access tokens: hash of the issuing refresh token; nil otherwise
	ExpiresAt   time.Time
	RevokedAt   *time.Time
	CreatedAt   time.Time
}

// Store is the persistence contract for the OAuth AS. Methods return
// ErrNotFound or ErrConflict where applicable. Implementations must be
// safe for concurrent use.
type Store interface {
	// Users
	UpsertUserByEmail(ctx context.Context, u User) (User, error)
	GetUser(ctx context.Context, id int64) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)

	// Clients
	CreateClient(ctx context.Context, c Client) error
	GetClient(ctx context.Context, id string) (Client, error)

	// Sessions
	CreateSession(ctx context.Context, s Session) error
	GetSession(ctx context.Context, id string) (Session, error)
	DeleteSession(ctx context.Context, id string) error

	// Authorization codes
	CreateAuthCode(ctx context.Context, ac AuthCode) error
	ConsumeAuthCode(ctx context.Context, codeHash []byte) (AuthCode, error)

	// Tokens
	CreateToken(ctx context.Context, tok Token) error
	GetToken(ctx context.Context, tokenHash []byte) (Token, error)
	RevokeToken(ctx context.Context, tokenHash []byte) error
	RevokeRefreshFamily(ctx context.Context, refreshHash []byte) error
}
```

- [ ] **Step 2: Write a failing test for ApplyMigrations**

Create `internal/oauth/storage/migrate_test.go`:
```go
package storage

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// requireDB skips the test if OAUTH_DATABASE_URL is not set.
func requireDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("OAUTH_DATABASE_URL")
	if dsn == "" {
		t.Skip("OAUTH_DATABASE_URL not set; skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestApplyMigrationsIsIdempotent(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	if err := ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("first ApplyMigrations: %v", err)
	}
	if err := ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("second ApplyMigrations: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables WHERE table_name = 'oauth_users'`).Scan(&n); err != nil {
		t.Fatalf("schema check: %v", err)
	}
	if n != 1 {
		t.Errorf("oauth_users table count = %d, want 1", n)
	}
}
```

- [ ] **Step 3: Run, expect compile failure**

Run: `go test ./internal/oauth/storage/ -run TestApplyMigrations -v`
Expected: build error — `ApplyMigrations` undefined.

- [ ] **Step 4: Implement ApplyMigrations**

Create `internal/oauth/storage/migrate.go`:
```go
package storage

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage/migrations"
)

// ApplyMigrations runs every embedded *.sql file in lexical order inside
// a single transaction per file. The migrations themselves are guarded
// with CREATE TABLE IF NOT EXISTS, so re-runs are no-ops.
func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("oauth/storage: read embed: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("oauth/storage: read %s: %w", name, err)
		}
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("oauth/storage: apply %s: %w", name, err)
		}
	}
	return nil
}
```

- [ ] **Step 5: Run, expect pass (or skip if no DB)**

Run: `go test ./internal/oauth/storage/ -run TestApplyMigrations -v`
Expected: PASS if `OAUTH_DATABASE_URL` is set; otherwise SKIP. Either is acceptable.

- [ ] **Step 6: Commit**

```bash
git add internal/oauth/storage/storage.go internal/oauth/storage/errors.go internal/oauth/storage/migrate.go internal/oauth/storage/migrate_test.go
git commit -m "$(cat <<'EOF'
feat(oauth/storage): Store interface + record types + ApplyMigrations

Defines the persistence contract every OAuth server handler will consume,
plus the embedded-migration applier. Concrete pgx implementation lands
in the next task.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Postgres implementation of Store

**Files:**
- Create: `internal/oauth/storage/postgres.go`
- Create: `internal/oauth/storage/postgres_test.go`

This task is by far the longest; the test file exercises every method. All tests require `OAUTH_DATABASE_URL` and `requireDB` (defined in Task 6) skips otherwise — execute against a real Postgres before merging.

- [ ] **Step 1: Write the failing tests (one Run sub-test per method group)**

Create `internal/oauth/storage/postgres_test.go`:
```go
package storage

import (
	"context"
	"errors"
	"testing"
	"time"
)

// resetSchema wipes all five OAuth tables. Call after migrations are applied.
func resetSchema(t *testing.T, s *Postgres) {
	t.Helper()
	ctx := context.Background()
	for _, tbl := range []string{"oauth_tokens", "oauth_codes", "oauth_sessions", "oauth_clients", "oauth_users"} {
		if _, err := s.pool.Exec(ctx, "TRUNCATE TABLE "+tbl+" RESTART IDENTITY CASCADE"); err != nil {
			t.Fatalf("truncate %s: %v", tbl, err)
		}
	}
}

func newStore(t *testing.T) *Postgres {
	t.Helper()
	pool := requireDB(t)
	ctx := context.Background()
	if err := ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s := NewPostgres(pool)
	resetSchema(t, s)
	return s
}

func TestPostgres_Users(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	u, err := s.UpsertUserByEmail(ctx, User{
		OrganizzeEmail: "a@b.com",
		APIKeyCipher:   []byte{1, 2},
		APIKeyNonce:    []byte{3, 4},
		UserAgent:      "UA",
	})
	if err != nil {
		t.Fatalf("UpsertUserByEmail (insert): %v", err)
	}
	if u.ID == 0 {
		t.Error("expected ID assigned")
	}

	u2, err := s.UpsertUserByEmail(ctx, User{
		OrganizzeEmail: "a@b.com",
		APIKeyCipher:   []byte{9, 9},
		APIKeyNonce:    []byte{8, 8},
		UserAgent:      "UA2",
	})
	if err != nil {
		t.Fatalf("UpsertUserByEmail (update): %v", err)
	}
	if u2.ID != u.ID {
		t.Errorf("upsert created a new row: %d vs %d", u2.ID, u.ID)
	}
	if u2.APIKeyCipher[0] != 9 || u2.UserAgent != "UA2" {
		t.Errorf("upsert did not overwrite: %+v", u2)
	}

	got, err := s.GetUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.OrganizzeEmail != "a@b.com" {
		t.Errorf("GetUser email = %q", got.OrganizzeEmail)
	}

	if _, err := s.GetUser(ctx, 9999); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPostgres_ClientsAndSessions(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	u, _ := s.UpsertUserByEmail(ctx, User{OrganizzeEmail: "x@x.com", APIKeyCipher: []byte{1}, APIKeyNonce: []byte{2}, UserAgent: "UA"})

	if err := s.CreateClient(ctx, Client{ID: "cli-1", ClientName: "ChatGPT", RedirectURIs: []string{"https://chat.openai.com/cb"}}); err != nil {
		t.Fatalf("CreateClient: %v", err)
	}
	c, err := s.GetClient(ctx, "cli-1")
	if err != nil {
		t.Fatalf("GetClient: %v", err)
	}
	if c.ClientName != "ChatGPT" || len(c.RedirectURIs) != 1 {
		t.Errorf("client = %+v", c)
	}

	sess := Session{ID: "sess-1", UserID: u.ID, ExpiresAt: time.Now().Add(time.Hour).UTC()}
	if err := s.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	got, err := s.GetSession(ctx, "sess-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.UserID != u.ID {
		t.Errorf("session userID = %d", got.UserID)
	}
	if err := s.DeleteSession(ctx, "sess-1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := s.GetSession(ctx, "sess-1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestPostgres_AuthCode_ConsumeOnce(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	u, _ := s.UpsertUserByEmail(ctx, User{OrganizzeEmail: "x@x.com", APIKeyCipher: []byte{1}, APIKeyNonce: []byte{2}, UserAgent: "UA"})
	_ = s.CreateClient(ctx, Client{ID: "cli", ClientName: "X", RedirectURIs: []string{"https://cb"}})

	hash := HashToken("the-code")
	ac := AuthCode{
		CodeHash: hash, ClientID: "cli", UserID: u.ID,
		RedirectURI: "https://cb", CodeChallenge: "abc", CodeChallengeMethod: "S256",
		ExpiresAt: time.Now().Add(5 * time.Minute).UTC(),
	}
	if err := s.CreateAuthCode(ctx, ac); err != nil {
		t.Fatalf("CreateAuthCode: %v", err)
	}
	got, err := s.ConsumeAuthCode(ctx, hash)
	if err != nil {
		t.Fatalf("ConsumeAuthCode: %v", err)
	}
	if got.UserID != u.ID {
		t.Errorf("consumed user = %d", got.UserID)
	}
	if _, err := s.ConsumeAuthCode(ctx, hash); !errors.Is(err, ErrNotFound) {
		t.Errorf("double consume should fail, got %v", err)
	}
}

func TestPostgres_Tokens_RevokeAndFamily(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	u, _ := s.UpsertUserByEmail(ctx, User{OrganizzeEmail: "x@x.com", APIKeyCipher: []byte{1}, APIKeyNonce: []byte{2}, UserAgent: "UA"})
	_ = s.CreateClient(ctx, Client{ID: "cli", ClientName: "X", RedirectURIs: []string{"https://cb"}})

	refresh := Token{
		TokenHash: HashToken("rt-1"), Kind: "refresh", ClientID: "cli", UserID: u.ID,
		ExpiresAt: time.Now().Add(24 * time.Hour).UTC(),
	}
	if err := s.CreateToken(ctx, refresh); err != nil {
		t.Fatalf("CreateToken refresh: %v", err)
	}
	access := Token{
		TokenHash: HashToken("at-1"), Kind: "access", ClientID: "cli", UserID: u.ID,
		RefreshFor: refresh.TokenHash,
		ExpiresAt:  time.Now().Add(time.Hour).UTC(),
	}
	if err := s.CreateToken(ctx, access); err != nil {
		t.Fatalf("CreateToken access: %v", err)
	}

	got, err := s.GetToken(ctx, access.TokenHash)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if got.UserID != u.ID || got.RevokedAt != nil {
		t.Errorf("token = %+v", got)
	}
	if err := s.RevokeRefreshFamily(ctx, refresh.TokenHash); err != nil {
		t.Fatalf("RevokeRefreshFamily: %v", err)
	}
	rev, _ := s.GetToken(ctx, refresh.TokenHash)
	if rev.RevokedAt == nil {
		t.Error("refresh not revoked")
	}
	revA, _ := s.GetToken(ctx, access.TokenHash)
	if revA.RevokedAt == nil {
		t.Error("descendant access not revoked")
	}
}
```

- [ ] **Step 2: Run, expect failure**

Run: `go test ./internal/oauth/storage/ -run TestPostgres -v`
Expected: build error — `Postgres` / `NewPostgres` undefined.

- [ ] **Step 3: Implement the Postgres store**

Create `internal/oauth/storage/postgres.go`:
```go
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres is a pgx-backed implementation of Store.
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres wraps an existing connection pool. The caller owns the pool.
func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

// --- users ---

func (s *Postgres) UpsertUserByEmail(ctx context.Context, u User) (User, error) {
	const q = `
INSERT INTO oauth_users (organizze_email, organizze_api_key_cipher, organizze_api_key_nonce, user_agent)
VALUES ($1, $2, $3, $4)
ON CONFLICT (organizze_email) DO UPDATE
SET organizze_api_key_cipher = EXCLUDED.organizze_api_key_cipher,
    organizze_api_key_nonce  = EXCLUDED.organizze_api_key_nonce,
    user_agent               = EXCLUDED.user_agent,
    updated_at               = NOW()
RETURNING id, organizze_email, organizze_api_key_cipher, organizze_api_key_nonce, user_agent, created_at, updated_at
`
	row := s.pool.QueryRow(ctx, q, u.OrganizzeEmail, u.APIKeyCipher, u.APIKeyNonce, u.UserAgent)
	var got User
	if err := row.Scan(&got.ID, &got.OrganizzeEmail, &got.APIKeyCipher, &got.APIKeyNonce, &got.UserAgent, &got.CreatedAt, &got.UpdatedAt); err != nil {
		return User{}, fmt.Errorf("upsert user: %w", err)
	}
	return got, nil
}

func (s *Postgres) GetUser(ctx context.Context, id int64) (User, error) {
	const q = `SELECT id, organizze_email, organizze_api_key_cipher, organizze_api_key_nonce, user_agent, created_at, updated_at FROM oauth_users WHERE id = $1`
	row := s.pool.QueryRow(ctx, q, id)
	var u User
	if err := row.Scan(&u.ID, &u.OrganizzeEmail, &u.APIKeyCipher, &u.APIKeyNonce, &u.UserAgent, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return u, nil
}

func (s *Postgres) GetUserByEmail(ctx context.Context, email string) (User, error) {
	const q = `SELECT id, organizze_email, organizze_api_key_cipher, organizze_api_key_nonce, user_agent, created_at, updated_at FROM oauth_users WHERE organizze_email = $1`
	row := s.pool.QueryRow(ctx, q, email)
	var u User
	if err := row.Scan(&u.ID, &u.OrganizzeEmail, &u.APIKeyCipher, &u.APIKeyNonce, &u.UserAgent, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return u, nil
}

// --- clients ---

func (s *Postgres) CreateClient(ctx context.Context, c Client) error {
	uris, err := json.Marshal(c.RedirectURIs)
	if err != nil {
		return fmt.Errorf("marshal redirect uris: %w", err)
	}
	const q = `INSERT INTO oauth_clients (id, client_secret_hash, client_name, redirect_uris) VALUES ($1, $2, $3, $4)`
	if _, err := s.pool.Exec(ctx, q, c.ID, c.ClientSecretHash, c.ClientName, uris); err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	return nil
}

func (s *Postgres) GetClient(ctx context.Context, id string) (Client, error) {
	const q = `SELECT id, client_secret_hash, client_name, redirect_uris, created_at FROM oauth_clients WHERE id = $1`
	row := s.pool.QueryRow(ctx, q, id)
	var c Client
	var uris []byte
	if err := row.Scan(&c.ID, &c.ClientSecretHash, &c.ClientName, &uris, &c.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Client{}, ErrNotFound
		}
		return Client{}, err
	}
	if err := json.Unmarshal(uris, &c.RedirectURIs); err != nil {
		return Client{}, fmt.Errorf("unmarshal redirect uris: %w", err)
	}
	return c, nil
}

// --- sessions ---

func (s *Postgres) CreateSession(ctx context.Context, sess Session) error {
	const q = `INSERT INTO oauth_sessions (id, user_id, expires_at) VALUES ($1, $2, $3)`
	if _, err := s.pool.Exec(ctx, q, sess.ID, sess.UserID, sess.ExpiresAt); err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (s *Postgres) GetSession(ctx context.Context, id string) (Session, error) {
	const q = `SELECT id, user_id, expires_at, created_at FROM oauth_sessions WHERE id = $1 AND expires_at > NOW()`
	row := s.pool.QueryRow(ctx, q, id)
	var sess Session
	if err := row.Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt, &sess.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrNotFound
		}
		return Session{}, err
	}
	return sess, nil
}

func (s *Postgres) DeleteSession(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM oauth_sessions WHERE id = $1`, id)
	return err
}

// --- auth codes ---

func (s *Postgres) CreateAuthCode(ctx context.Context, ac AuthCode) error {
	const q = `
INSERT INTO oauth_codes (code_hash, client_id, user_id, redirect_uri, code_challenge, code_challenge_method, scope, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`
	if _, err := s.pool.Exec(ctx, q, ac.CodeHash, ac.ClientID, ac.UserID, ac.RedirectURI, ac.CodeChallenge, ac.CodeChallengeMethod, ac.Scope, ac.ExpiresAt); err != nil {
		return fmt.Errorf("create auth code: %w", err)
	}
	return nil
}

func (s *Postgres) ConsumeAuthCode(ctx context.Context, codeHash []byte) (AuthCode, error) {
	const q = `
UPDATE oauth_codes
SET consumed_at = NOW()
WHERE code_hash = $1 AND consumed_at IS NULL AND expires_at > NOW()
RETURNING code_hash, client_id, user_id, redirect_uri, code_challenge, code_challenge_method, scope, expires_at, consumed_at
`
	row := s.pool.QueryRow(ctx, q, codeHash)
	var ac AuthCode
	if err := row.Scan(&ac.CodeHash, &ac.ClientID, &ac.UserID, &ac.RedirectURI, &ac.CodeChallenge, &ac.CodeChallengeMethod, &ac.Scope, &ac.ExpiresAt, &ac.ConsumedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AuthCode{}, ErrNotFound
		}
		return AuthCode{}, err
	}
	return ac, nil
}

// --- tokens ---

func (s *Postgres) CreateToken(ctx context.Context, tok Token) error {
	const q = `
INSERT INTO oauth_tokens (token_hash, kind, client_id, user_id, refresh_for, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
`
	if _, err := s.pool.Exec(ctx, q, tok.TokenHash, tok.Kind, tok.ClientID, tok.UserID, tok.RefreshFor, tok.ExpiresAt); err != nil {
		return fmt.Errorf("create token: %w", err)
	}
	return nil
}

func (s *Postgres) GetToken(ctx context.Context, tokenHash []byte) (Token, error) {
	const q = `SELECT token_hash, kind, client_id, user_id, refresh_for, expires_at, revoked_at, created_at FROM oauth_tokens WHERE token_hash = $1`
	row := s.pool.QueryRow(ctx, q, tokenHash)
	var t Token
	if err := row.Scan(&t.TokenHash, &t.Kind, &t.ClientID, &t.UserID, &t.RefreshFor, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Token{}, ErrNotFound
		}
		return Token{}, err
	}
	return t, nil
}

func (s *Postgres) RevokeToken(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, `UPDATE oauth_tokens SET revoked_at = NOW() WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash)
	return err
}

// RevokeRefreshFamily revokes the refresh token and every access token issued
// from it. Called when a refresh-token reuse is detected, or when the user
// explicitly logs out.
func (s *Postgres) RevokeRefreshFamily(ctx context.Context, refreshHash []byte) error {
	const q = `
UPDATE oauth_tokens
SET revoked_at = NOW()
WHERE revoked_at IS NULL
  AND (token_hash = $1 OR refresh_for = $1)
`
	_, err := s.pool.Exec(ctx, q, refreshHash)
	return err
}
```

- [ ] **Step 4: Run integration tests against a real Postgres**

Run: `OAUTH_DATABASE_URL=postgres://localhost:5432/organizze_oauth_test go test ./internal/oauth/storage/ -v`
Expected: all PASS. (If you don't have a local Postgres, spin one up: `docker run --rm -p 5432:5432 -e POSTGRES_PASSWORD=test -e POSTGRES_DB=organizze_oauth_test postgres:16`.)

- [ ] **Step 5: Run the full test suite to ensure non-storage tests still skip cleanly**

Run: `go test ./...`
Expected: all non-OAuth tests PASS; OAuth-storage tests SKIP if `OAUTH_DATABASE_URL` isn't set.

- [ ] **Step 6: Commit**

```bash
git add internal/oauth/storage/postgres.go internal/oauth/storage/postgres_test.go
git commit -m "$(cat <<'EOF'
feat(oauth/storage): pgx-backed Store implementation

Covers users, clients, sessions, auth codes, and the token family
(with refresh-family revocation). Integration tests skip when
OAUTH_DATABASE_URL is unset so the suite remains DB-free by default.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Discovery endpoints (`.well-known`)

**Files:**
- Create: `internal/oauth/server/server.go` (skeleton — populated incrementally)
- Create: `internal/oauth/server/discovery.go`
- Create: `internal/oauth/server/discovery_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/oauth/server/discovery_test.go`:
```go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProtectedResourceMetadata(t *testing.T) {
	h := New(Config{PublicURL: "https://mcp.example.com"})
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["resource"] != "https://mcp.example.com/mcp" {
		t.Errorf("resource = %v", got["resource"])
	}
	servers, _ := got["authorization_servers"].([]any)
	if len(servers) != 1 || servers[0] != "https://mcp.example.com" {
		t.Errorf("authorization_servers = %v", servers)
	}
}

func TestAuthorizationServerMetadata(t *testing.T) {
	h := New(Config{PublicURL: "https://mcp.example.com"})
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for k, want := range map[string]string{
		"issuer":                 "https://mcp.example.com",
		"authorization_endpoint": "https://mcp.example.com/oauth/authorize",
		"token_endpoint":         "https://mcp.example.com/oauth/token",
		"registration_endpoint":  "https://mcp.example.com/oauth/register",
		"revocation_endpoint":    "https://mcp.example.com/oauth/revoke",
	} {
		if got[k] != want {
			t.Errorf("%s = %v, want %s", k, got[k], want)
		}
	}
	codeMethods, _ := got["code_challenge_methods_supported"].([]any)
	if len(codeMethods) != 1 || codeMethods[0] != "S256" {
		t.Errorf("code_challenge_methods_supported = %v", codeMethods)
	}
}
```

- [ ] **Step 2: Run, expect compile failure**

Run: `go test ./internal/oauth/server/ -run TestProtectedResourceMetadata -v`
Expected: build error — package files don't exist.

- [ ] **Step 3: Create the server skeleton**

Create `internal/oauth/server/server.go`:
```go
// Package server hosts the OAuth 2.1 Authorization Server endpoints and the
// bearer middleware that fronts the MCP handler. Construct with New, then
// either ServeHTTP directly (tests) or mount the handler in a real
// *http.Server (cmd/organizze-mcp-oauth/main.go).
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

// Config holds compile-time-injected dependencies and tunables.
type Config struct {
	// PublicURL is the externally reachable origin (no trailing slash),
	// e.g. "https://mcp.example.com". Used to render absolute URLs in
	// discovery documents and the redirect_uri match.
	PublicURL string

	// Store is the persistence layer. Required at runtime; tests that
	// only exercise discovery may omit it.
	Store storage.Store

	// Cipher seals/unseals the Organizze API key. Required at runtime.
	Cipher *storage.Cipher

	// ValidateOrganizze checks an email+key pair against the live Organizze
	// API and returns nil on success. Required at runtime; tests inject a
	// fake.
	ValidateOrganizze func(ctx context.Context, email, apiKey, userAgent string) error

	// Now returns the current wall-clock time. Defaults to time.Now.UTC.
	Now func() time.Time

	// AccessTokenTTL defaults to 1h. RefreshTokenTTL defaults to 30d.
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	// SessionTTL defaults to 24h.
	SessionTTL time.Duration

	// CookieSecret signs the session cookie (HMAC-SHA256). Required at runtime.
	CookieSecret []byte
}

// Server is the http.Handler implementation.
type Server struct {
	cfg Config
	mux *http.ServeMux
}

// New constructs a Server with all routes mounted.
func New(cfg Config) *Server {
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	if cfg.AccessTokenTTL == 0 {
		cfg.AccessTokenTTL = time.Hour
	}
	if cfg.RefreshTokenTTL == 0 {
		cfg.RefreshTokenTTL = 30 * 24 * time.Hour
	}
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = 24 * time.Hour
	}
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/.well-known/oauth-protected-resource", s.handleProtectedResource)
	s.mux.HandleFunc("/.well-known/oauth-authorization-server", s.handleAuthorizationServer)
	// /oauth/register, /authorize, /token, /revoke, /mcp registered in later tasks.
}
```

- [ ] **Step 4: Implement discovery**

Create `internal/oauth/server/discovery.go`:
```go
package server

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleProtectedResource(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]any{
		"resource":              s.cfg.PublicURL + "/mcp",
		"authorization_servers": []string{s.cfg.PublicURL},
		"bearer_methods_supported": []string{"header"},
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAuthorizationServer(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]any{
		"issuer":                                s.cfg.PublicURL,
		"authorization_endpoint":                s.cfg.PublicURL + "/oauth/authorize",
		"token_endpoint":                        s.cfg.PublicURL + "/oauth/token",
		"registration_endpoint":                 s.cfg.PublicURL + "/oauth/register",
		"revocation_endpoint":                   s.cfg.PublicURL + "/oauth/revoke",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none", "client_secret_basic"},
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
```

- [ ] **Step 5: Run discovery tests, expect pass**

Run: `go test ./internal/oauth/server/ -run "TestProtectedResource|TestAuthorization" -v`
Expected: both PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/oauth/server/
git commit -m "$(cat <<'EOF'
feat(oauth/server): discovery endpoints + server skeleton

GET /.well-known/oauth-protected-resource and
GET /.well-known/oauth-authorization-server return the metadata
ChatGPT needs to drive Dynamic Client Registration and PKCE.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Dynamic Client Registration

**Files:**
- Create: `internal/oauth/server/register.go`
- Create: `internal/oauth/server/register_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/oauth/server/register_test.go`:
```go
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
		PublicURL:    "https://mcp.example.com",
		Store:        fs,
		CookieSecret: []byte("secret"),
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
```

And the in-memory fake store (`internal/oauth/server/fakestore_test.go`):
```go
package server

import (
	"context"
	"sync"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

type fakeStore struct {
	mu       sync.Mutex
	users    map[int64]storage.User
	emails   map[string]int64
	clients  map[string]storage.Client
	sessions map[string]storage.Session
	codes    map[string]storage.AuthCode
	tokens   map[string]storage.Token
	nextID   int64
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		users: map[int64]storage.User{}, emails: map[string]int64{},
		clients: map[string]storage.Client{}, sessions: map[string]storage.Session{},
		codes: map[string]storage.AuthCode{}, tokens: map[string]storage.Token{},
	}
}

// Implement every method on storage.Store using the in-memory maps.
// For brevity, only the methods exercised in this and later tests are shown;
// fill in the rest with minimal implementations as you reach tasks that need them.

func (f *fakeStore) CreateClient(_ context.Context, c storage.Client) error {
	f.mu.Lock(); defer f.mu.Unlock()
	if _, ok := f.clients[c.ID]; ok { return storage.ErrConflict }
	f.clients[c.ID] = c
	return nil
}
func (f *fakeStore) GetClient(_ context.Context, id string) (storage.Client, error) {
	f.mu.Lock(); defer f.mu.Unlock()
	c, ok := f.clients[id]
	if !ok { return storage.Client{}, storage.ErrNotFound }
	return c, nil
}

// (Stub the remaining methods on storage.Store with `panic("unused in this test")`
// or full implementations as they become needed in Tasks 10–13.)
```

> Engineer note: when a later task needs another method, add it to `fakeStore` then. Don't pre-implement everything.

- [ ] **Step 2: Run, expect compile failure**

Run: `go test ./internal/oauth/server/ -run TestRegister -v`
Expected: build error or fail — `handleRegister` undefined.

- [ ] **Step 3: Implement DCR**

Create `internal/oauth/server/register.go`:
```go
package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

type registerRequest struct {
	ClientName   string   `json:"client_name"`
	RedirectURIs []string `json:"redirect_uris"`
}

type registerResponse struct {
	ClientID     string   `json:"client_id"`
	ClientName   string   `json:"client_name"`
	RedirectURIs []string `json:"redirect_uris"`
	GrantTypes   []string `json:"grant_types"`
	ResponseTypes []string `json:"response_types"`
	TokenEndpointAuthMethod string `json:"token_endpoint_auth_method"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid_client_metadata: malformed JSON", http.StatusBadRequest)
		return
	}
	if len(req.RedirectURIs) == 0 {
		http.Error(w, "invalid_redirect_uri: at least one redirect_uri required", http.StatusBadRequest)
		return
	}
	for _, u := range req.RedirectURIs {
		if !strings.HasPrefix(u, "https://") {
			http.Error(w, "invalid_redirect_uri: must be https", http.StatusBadRequest)
			return
		}
	}
	id := newPublicID()
	if err := s.cfg.Store.CreateClient(r.Context(), storage.Client{
		ID:           id,
		ClientName:   req.ClientName,
		RedirectURIs: req.RedirectURIs,
	}); err != nil {
		http.Error(w, "server_error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, registerResponse{
		ClientID:                id,
		ClientName:              req.ClientName,
		RedirectURIs:            req.RedirectURIs,
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "none",
	})
}

func newPublicID() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
```

Add to `routes()` in `server.go`:
```go
s.mux.HandleFunc("/oauth/register", s.handleRegister)
```

- [ ] **Step 4: Run register tests, expect pass**

Run: `go test ./internal/oauth/server/ -run TestRegister -v`
Expected: 3 PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/oauth/server/register.go internal/oauth/server/register_test.go internal/oauth/server/fakestore_test.go internal/oauth/server/server.go
git commit -m "$(cat <<'EOF'
feat(oauth/server): POST /oauth/register (Dynamic Client Registration)

Public-client only (no client_secret issued; PKCE is required). Rejects
non-HTTPS redirect_uris, which is required for ChatGPT to accept the
server as a valid OAuth AS.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Session cookie helpers

**Files:**
- Create: `internal/oauth/server/session.go`
- Create: `internal/oauth/server/session_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/oauth/server/session_test.go`:
```go
package server

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestSession_RoundTrip(t *testing.T) {
	sm := newSessionManager([]byte("k"), time.Hour)
	w := httptest.NewRecorder()
	sm.write(w, "sess-1")
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName {
		t.Fatalf("cookies = %+v", cookies)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(cookies[0])
	got, ok := sm.read(r)
	if !ok || got != "sess-1" {
		t.Errorf("read = %q, ok=%v", got, ok)
	}
}

func TestSession_TamperedFails(t *testing.T) {
	sm := newSessionManager([]byte("k"), time.Hour)
	w := httptest.NewRecorder()
	sm.write(w, "sess-1")
	cookies := w.Result().Cookies()

	cookies[0].Value = cookies[0].Value[:len(cookies[0].Value)-2] + "xx"
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(cookies[0])
	if _, ok := sm.read(r); ok {
		t.Error("expected tampered cookie to be rejected")
	}
}
```

- [ ] **Step 2: Run, expect failure**

Run: `go test ./internal/oauth/server/ -run TestSession -v`
Expected: build errors — symbols undefined.

- [ ] **Step 3: Implement**

Create `internal/oauth/server/session.go`:
```go
package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

const sessionCookieName = "organizze_oauth_session"

type sessionManager struct {
	secret []byte
	ttl    time.Duration
}

func newSessionManager(secret []byte, ttl time.Duration) *sessionManager {
	return &sessionManager{secret: secret, ttl: ttl}
}

// write sets a signed cookie carrying sessionID.
func (m *sessionManager) write(w http.ResponseWriter, sessionID string) {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(sessionID))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	value := sessionID + "." + sig
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(m.ttl.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// read returns the sessionID after verifying the HMAC.
func (m *sessionManager) read(r *http.Request) (string, bool) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}
	parts := strings.SplitN(c.Value, ".", 2)
	if len(parts) != 2 {
		return "", false
	}
	sessionID, sig := parts[0], parts[1]
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(sessionID))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return "", false
	}
	return sessionID, true
}

// clear deletes the cookie at the client.
func (m *sessionManager) clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/",
		MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})
}
```

- [ ] **Step 4: Run tests, expect pass**

Run: `go test ./internal/oauth/server/ -run TestSession -v`
Expected: both PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/oauth/server/session.go internal/oauth/server/session_test.go
git commit -m "$(cat <<'EOF'
feat(oauth/server): HMAC-signed session cookie helpers

write/read/clear an HttpOnly+Secure+SameSite=Lax cookie that carries
the storage session ID. Tampered cookies are rejected via HMAC-SHA256.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: Authorization endpoint (GET form + POST submission)

**Files:**
- Create: `internal/oauth/server/templates/login.html`
- Create: `internal/oauth/server/authorize.go`
- Create: `internal/oauth/server/authorize_test.go`

The `/oauth/authorize` endpoint has two methods:
- **GET**: validate query params, render the consent + creds entry form (or, if a valid session cookie exists, render a one-click confirm).
- **POST**: process form submission, validate Organizze creds against the real API via `cfg.ValidateOrganizze`, upsert user, create auth code, redirect to `redirect_uri?code=...&state=...`.

- [ ] **Step 1: Write the HTML template**

Create `internal/oauth/server/templates/login.html`:
```html
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Authorize {{.ClientName}}</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, sans-serif; max-width: 480px; margin: 4em auto; padding: 0 1em; color: #222; }
    h1 { font-size: 1.4em; }
    label { display: block; margin-top: 1em; font-weight: 600; }
    input[type=email], input[type=password] { width: 100%; padding: 0.6em; border: 1px solid #ccc; border-radius: 4px; box-sizing: border-box; }
    button { margin-top: 1.5em; padding: 0.7em 1.2em; background: #1a73e8; color: white; border: 0; border-radius: 4px; cursor: pointer; }
    .err { color: #b00020; margin-top: 1em; }
    .hint { color: #666; font-size: 0.9em; margin-top: 0.4em; }
  </style>
</head>
<body>
  <h1>Authorize <strong>{{.ClientName}}</strong></h1>
  <p>{{.ClientName}} is requesting access to your Organizze account through this MCP server.</p>
  {{if .Error}}<div class="err">{{.Error}}</div>{{end}}
  <form method="POST" action="/oauth/authorize">
    <input type="hidden" name="csrf"          value="{{.CSRF}}">
    <input type="hidden" name="client_id"     value="{{.ClientID}}">
    <input type="hidden" name="redirect_uri"  value="{{.RedirectURI}}">
    <input type="hidden" name="state"         value="{{.State}}">
    <input type="hidden" name="code_challenge" value="{{.CodeChallenge}}">
    <input type="hidden" name="code_challenge_method" value="{{.CodeChallengeMethod}}">
    <label for="email">Organizze email</label>
    <input id="email" type="email" name="email" required autocomplete="email">
    <label for="api_key">Organizze API key</label>
    <input id="api_key" type="password" name="api_key" required autocomplete="off">
    <div class="hint">Get your key from organizze.com.br → Settings → API.</div>
    <label for="user_agent">App identifier (e.g. <code>my-name (me@example.com)</code>)</label>
    <input id="user_agent" type="text" name="user_agent" required>
    <button type="submit">Authorize</button>
  </form>
</body>
</html>
```

- [ ] **Step 2: Write failing tests for authorize**

Create `internal/oauth/server/authorize_test.go`:
```go
package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func seedClient(t *testing.T, fs *fakeStore) {
	t.Helper()
	_ = fs.CreateClient(context.Background(), seedClientRecord())
}
func seedClientRecord() (c struct{ ID, Name string; URIs []string }) {
	c.ID = "client-abc"
	c.Name = "ChatGPT"
	c.URIs = []string{"https://chat.example.com/cb"}
	return
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
		"code_challenge":        {"abc"},
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
		"code_challenge":        {"abc"},
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
		"code_challenge":        {"abc"},
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
	srv.cfg.ValidateOrganizze = func(context.Context, string, string, string) error { return errors.New("401 unauthorized") }
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))

	form := url.Values{
		"client_id":             {c.ID},
		"redirect_uri":          {c.URIs[0]},
		"state":                 {"xyz"},
		"code_challenge":        {"abc"},
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
```

Add to `fakestore_test.go` (as needed by these tests — extend the in-memory map methods):
```go
func (f *fakeStore) UpsertUserByEmail(_ context.Context, u storage.User) (storage.User, error) {
	f.mu.Lock(); defer f.mu.Unlock()
	if id, ok := f.emails[u.OrganizzeEmail]; ok {
		u.ID = id
		f.users[id] = u
		return u, nil
	}
	f.nextID++
	u.ID = f.nextID
	f.users[u.ID] = u
	f.emails[u.OrganizzeEmail] = u.ID
	return u, nil
}
func (f *fakeStore) GetUser(_ context.Context, id int64) (storage.User, error) {
	f.mu.Lock(); defer f.mu.Unlock()
	if u, ok := f.users[id]; ok { return u, nil }
	return storage.User{}, storage.ErrNotFound
}
func (f *fakeStore) GetUserByEmail(_ context.Context, e string) (storage.User, error) {
	f.mu.Lock(); defer f.mu.Unlock()
	if id, ok := f.emails[e]; ok { return f.users[id], nil }
	return storage.User{}, storage.ErrNotFound
}
func (f *fakeStore) CreateAuthCode(_ context.Context, ac storage.AuthCode) error {
	f.mu.Lock(); defer f.mu.Unlock()
	f.codes[string(ac.CodeHash)] = ac
	return nil
}
```

And helpers:
```go
func storageClient(c struct{ ID, Name string; URIs []string }) storage.Client {
	return storage.Client{ID: c.ID, ClientName: c.Name, RedirectURIs: c.URIs}
}
func mustTestCipher(t *testing.T) *storage.Cipher {
	t.Helper()
	key := make([]byte, 32)
	c, err := storage.NewCipher(key) // zero key fine for tests
	if err != nil { t.Fatal(err) }
	return c
}
```

- [ ] **Step 2: Run, expect failure**

Run: `go test ./internal/oauth/server/ -run TestAuthorize -v`
Expected: build errors — `handleAuthorize` undefined.

- [ ] **Step 3: Implement the authorize handler**

Create `internal/oauth/server/authorize.go`:
```go
package server

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"time"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

//go:embed templates/login.html
var loginFS embed.FS

var loginTpl = template.Must(template.ParseFS(loginFS, "templates/login.html"))

type loginViewModel struct {
	ClientID            string
	ClientName          string
	RedirectURI         string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	CSRF                string
	Error               string
}

func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.authorizeGET(w, r)
	case http.MethodPost:
		s.authorizePOST(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) authorizeGET(w http.ResponseWriter, r *http.Request) {
	vm, err := s.parseAuthorizeParams(r.Context(), r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	renderLogin(w, vm)
}

func (s *Server) authorizePOST(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	vm, err := s.parseAuthorizeParams(r.Context(), r.PostForm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	email := r.PostForm.Get("email")
	apiKey := r.PostForm.Get("api_key")
	userAgent := r.PostForm.Get("user_agent")
	if email == "" || apiKey == "" || userAgent == "" {
		vm.Error = "All fields are required."
		renderLogin(w, vm)
		return
	}

	if err := s.cfg.ValidateOrganizze(r.Context(), email, apiKey, userAgent); err != nil {
		vm.Error = "Invalid Organizze credentials: " + err.Error()
		renderLogin(w, vm)
		return
	}

	cipher, nonce, err := s.cfg.Cipher.Seal([]byte(apiKey))
	if err != nil {
		http.Error(w, "server_error", http.StatusInternalServerError)
		return
	}
	user, err := s.cfg.Store.UpsertUserByEmail(r.Context(), storage.User{
		OrganizzeEmail: email,
		APIKeyCipher:   cipher,
		APIKeyNonce:    nonce,
		UserAgent:      userAgent,
	})
	if err != nil {
		http.Error(w, "server_error", http.StatusInternalServerError)
		return
	}

	code := newRandomToken()
	if err := s.cfg.Store.CreateAuthCode(r.Context(), storage.AuthCode{
		CodeHash:            storage.HashToken(code),
		ClientID:            vm.ClientID,
		UserID:              user.ID,
		RedirectURI:         vm.RedirectURI,
		CodeChallenge:       vm.CodeChallenge,
		CodeChallengeMethod: vm.CodeChallengeMethod,
		ExpiresAt:           s.cfg.Now().Add(5 * time.Minute),
	}); err != nil {
		http.Error(w, "server_error", http.StatusInternalServerError)
		return
	}

	q := url.Values{"code": {code}, "state": {vm.State}}
	http.Redirect(w, r, vm.RedirectURI+"?"+q.Encode(), http.StatusSeeOther)
}

// parseAuthorizeParams validates the OAuth params and returns a populated
// view model (without Error). The same parsing is reused by GET and POST.
func (s *Server) parseAuthorizeParams(ctx context.Context, q url.Values) (loginViewModel, error) {
	clientID := q.Get("client_id")
	if clientID == "" {
		return loginViewModel{}, errors.New("invalid_request: client_id required")
	}
	client, err := s.cfg.Store.GetClient(ctx, clientID)
	if err != nil {
		return loginViewModel{}, errors.New("invalid_client: unknown client_id")
	}
	redirectURI := q.Get("redirect_uri")
	if !contains(client.RedirectURIs, redirectURI) {
		return loginViewModel{}, errors.New("invalid_redirect_uri: not registered for this client")
	}
	if q.Get("response_type") != "code" && q.Get("response_type") != "" {
		return loginViewModel{}, errors.New("unsupported_response_type")
	}
	method := q.Get("code_challenge_method")
	if method == "" {
		method = "S256"
	}
	if method != "S256" {
		return loginViewModel{}, errors.New("invalid_request: only S256 supported")
	}
	if q.Get("code_challenge") == "" {
		return loginViewModel{}, errors.New("invalid_request: PKCE code_challenge required")
	}
	return loginViewModel{
		ClientID:            clientID,
		ClientName:          client.ClientName,
		RedirectURI:         redirectURI,
		State:               q.Get("state"),
		CodeChallenge:       q.Get("code_challenge"),
		CodeChallengeMethod: method,
	}, nil
}

func renderLogin(w http.ResponseWriter, vm loginViewModel) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = loginTpl.Execute(w, vm)
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func newRandomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
```

Add to `routes()` in `server.go`:
```go
s.mux.HandleFunc("/oauth/authorize", s.handleAuthorize)
```

- [ ] **Step 4: Run authorize tests, expect pass**

Run: `go test ./internal/oauth/server/ -run TestAuthorize -v`
Expected: 4 PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/oauth/server/authorize.go internal/oauth/server/authorize_test.go internal/oauth/server/templates/ internal/oauth/server/fakestore_test.go internal/oauth/server/server.go
git commit -m "$(cat <<'EOF'
feat(oauth/server): /oauth/authorize (GET form + POST submission)

GET validates client+redirect+PKCE params and renders the consent +
credentials-entry form. POST validates Organizze credentials against
the live API, encrypts the api_key, upserts the user, issues a
short-lived auth code, and redirects to the registered redirect_uri.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 12: Token endpoint

**Files:**
- Create: `internal/oauth/server/token.go`
- Create: `internal/oauth/server/token_test.go`

Handles two grants:
- `grant_type=authorization_code` — exchanges code + PKCE verifier for an access + refresh token pair.
- `grant_type=refresh_token` — rotates: revokes the old refresh, returns a new access + refresh.

- [ ] **Step 1: Write failing tests**

Create `internal/oauth/server/token_test.go`:
```go
package server

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

func issueCode(t *testing.T, srv *Server, fs *fakeStore, clientID string, codeVerifier string) (code string, userID int64) {
	t.Helper()
	user, _ := fs.UpsertUserByEmail(context.Background(), storage.User{
		OrganizzeEmail: "u@x.com", APIKeyCipher: []byte{1}, APIKeyNonce: []byte{2}, UserAgent: "UA",
	})
	sum := sha256.Sum256([]byte(codeVerifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	code = newRandomToken()
	_ = fs.CreateAuthCode(context.Background(), storage.AuthCode{
		CodeHash:            storage.HashToken(code),
		ClientID:            clientID,
		UserID:              user.ID,
		RedirectURI:         "https://chat.example.com/cb",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
		ExpiresAt:           time.Now().Add(5 * time.Minute).UTC(),
	})
	return code, user.ID
}

func postForm(srv *Server, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestToken_AuthorizationCode_Success(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))
	verifier := "verifier-1234567890123456789012345678901234567890"
	code, _ := issueCode(t, srv, fs, c.ID, verifier)

	rec := postForm(srv, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.URIs[0]},
		"client_id":     {c.ID},
		"code_verifier": {verifier},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["access_token"] == "" || got["refresh_token"] == "" {
		t.Errorf("missing tokens: %v", got)
	}
	if got["token_type"] != "Bearer" {
		t.Errorf("token_type = %v", got["token_type"])
	}
}

func TestToken_AuthorizationCode_RejectsWrongVerifier(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))
	code, _ := issueCode(t, srv, fs, c.ID, "right-verifier-that-is-long-enough-12345")
	rec := postForm(srv, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.URIs[0]},
		"client_id":     {c.ID},
		"code_verifier": {"wrong-verifier-that-is-long-enough-12345"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestToken_AuthorizationCode_SingleUse(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))
	verifier := "verifier-that-is-long-enough-1234567890123"
	code, _ := issueCode(t, srv, fs, c.ID, verifier)
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.URIs[0]},
		"client_id":     {c.ID},
		"code_verifier": {verifier},
	}
	if rec := postForm(srv, form); rec.Code != http.StatusOK { t.Fatalf("first = %d", rec.Code) }
	if rec := postForm(srv, form); rec.Code != http.StatusBadRequest {
		t.Errorf("second use status = %d", rec.Code)
	}
}

func TestToken_RefreshGrant_Rotates(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))
	verifier := "verifier-that-is-long-enough-1234567890123"
	code, _ := issueCode(t, srv, fs, c.ID, verifier)
	rec := postForm(srv, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.URIs[0]},
		"client_id":     {c.ID},
		"code_verifier": {verifier},
	})
	var first map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &first)
	oldRefresh := first["refresh_token"].(string)

	rec = postForm(srv, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {oldRefresh},
		"client_id":     {c.ID},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh status = %d body=%s", rec.Code, rec.Body.String())
	}
	var second map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &second)
	if second["refresh_token"] == oldRefresh {
		t.Error("refresh_token did not rotate")
	}

	rec = postForm(srv, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {oldRefresh},
		"client_id":     {c.ID},
	})
	if rec.Code == http.StatusOK {
		t.Error("old refresh should be revoked after rotation")
	}
}
```

Extend `fakestore_test.go` with the missing methods (token CRUD, code consumption, family revocation). Mirror the Postgres semantics:
```go
func (f *fakeStore) ConsumeAuthCode(_ context.Context, h []byte) (storage.AuthCode, error) {
	f.mu.Lock(); defer f.mu.Unlock()
	ac, ok := f.codes[string(h)]
	if !ok || ac.ConsumedAt != nil || ac.ExpiresAt.Before(time.Now()) {
		return storage.AuthCode{}, storage.ErrNotFound
	}
	now := time.Now().UTC()
	ac.ConsumedAt = &now
	f.codes[string(h)] = ac
	return ac, nil
}
func (f *fakeStore) CreateToken(_ context.Context, tok storage.Token) error {
	f.mu.Lock(); defer f.mu.Unlock()
	f.tokens[string(tok.TokenHash)] = tok
	return nil
}
func (f *fakeStore) GetToken(_ context.Context, h []byte) (storage.Token, error) {
	f.mu.Lock(); defer f.mu.Unlock()
	t, ok := f.tokens[string(h)]
	if !ok { return storage.Token{}, storage.ErrNotFound }
	return t, nil
}
func (f *fakeStore) RevokeToken(_ context.Context, h []byte) error {
	f.mu.Lock(); defer f.mu.Unlock()
	t, ok := f.tokens[string(h)]
	if !ok { return nil }
	now := time.Now().UTC(); t.RevokedAt = &now
	f.tokens[string(h)] = t
	return nil
}
func (f *fakeStore) RevokeRefreshFamily(_ context.Context, h []byte) error {
	f.mu.Lock(); defer f.mu.Unlock()
	for k, t := range f.tokens {
		if string(t.TokenHash) == string(h) || (t.RefreshFor != nil && string(t.RefreshFor) == string(h)) {
			now := time.Now().UTC(); t.RevokedAt = &now
			f.tokens[k] = t
		}
	}
	return nil
}
// CreateSession / GetSession / DeleteSession: stub with panic for now if untouched.
```

- [ ] **Step 2: Run, expect failure**

Run: `go test ./internal/oauth/server/ -run TestToken -v`
Expected: build errors — `handleToken` undefined.

- [ ] **Step 3: Implement token handler**

Create `internal/oauth/server/token.go`:
```go
package server

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope,omitempty"`
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		s.grantAuthorizationCode(w, r)
	case "refresh_token":
		s.grantRefreshToken(w, r)
	default:
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "")
	}
}

func (s *Server) grantAuthorizationCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	code := r.PostForm.Get("code")
	clientID := r.PostForm.Get("client_id")
	redirectURI := r.PostForm.Get("redirect_uri")
	verifier := r.PostForm.Get("code_verifier")
	if code == "" || clientID == "" || verifier == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "missing required field")
		return
	}
	ac, err := s.cfg.Store.ConsumeAuthCode(ctx, storage.HashToken(code))
	if err != nil {
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
	access, refresh, err := s.issueTokenPair(ctx, clientID, ac.UserID)
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

func (s *Server) grantRefreshToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rt := r.PostForm.Get("refresh_token")
	clientID := r.PostForm.Get("client_id")
	if rt == "" || clientID == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "")
		return
	}
	hash := storage.HashToken(rt)
	tok, err := s.cfg.Store.GetToken(ctx, hash)
	if err != nil || tok.Kind != "refresh" || tok.RevokedAt != nil || tok.ExpiresAt.Before(s.cfg.Now()) {
		// Reuse detection: revoke the whole family.
		_ = s.cfg.Store.RevokeRefreshFamily(ctx, hash)
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "")
		return
	}
	if tok.ClientID != clientID {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "client_id mismatch")
		return
	}
	if err := s.cfg.Store.RevokeRefreshFamily(ctx, hash); err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "")
		return
	}
	access, refresh, err := s.issueTokenPair(ctx, clientID, tok.UserID)
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

func (s *Server) issueTokenPair(ctx context.Context, clientID string, userID int64) (string, string, error) {
	access := newRandomToken()
	refresh := newRandomToken()
	now := s.cfg.Now()
	refreshHash := storage.HashToken(refresh)
	if err := s.cfg.Store.CreateToken(ctx, storage.Token{
		TokenHash: refreshHash,
		Kind:      "refresh",
		ClientID:  clientID,
		UserID:    userID,
		ExpiresAt: now.Add(s.cfg.RefreshTokenTTL),
	}); err != nil {
		return "", "", err
	}
	if err := s.cfg.Store.CreateToken(ctx, storage.Token{
		TokenHash:  storage.HashToken(access),
		Kind:       "access",
		ClientID:   clientID,
		UserID:     userID,
		RefreshFor: refreshHash,
		ExpiresAt:  now.Add(s.cfg.AccessTokenTTL),
	}); err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

func verifyPKCE(verifier, challenge string) bool {
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

func writeOAuthError(w http.ResponseWriter, status int, code, description string) {
	body := map[string]string{"error": code}
	if description != "" {
		body["error_description"] = description
	}
	writeJSON(w, status, body)
}
```

Add to `routes()` in `server.go`:
```go
s.mux.HandleFunc("/oauth/token", s.handleToken)
```

- [ ] **Step 4: Run token tests, expect pass**

Run: `go test ./internal/oauth/server/ -run TestToken -v`
Expected: 4 PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/oauth/server/token.go internal/oauth/server/token_test.go internal/oauth/server/fakestore_test.go internal/oauth/server/server.go
git commit -m "$(cat <<'EOF'
feat(oauth/server): /oauth/token (authorization_code + refresh_token)

PKCE-S256 verification, single-use auth codes, refresh-token rotation
with reuse-detection that revokes the whole family.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 13: Revocation endpoint

**Files:**
- Create: `internal/oauth/server/revoke.go`
- Create: `internal/oauth/server/revoke_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/oauth/server/revoke_test.go`:
```go
package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

func TestRevoke_RemovesRefreshFamily(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)

	refreshHash := storage.HashToken("rt")
	accessHash := storage.HashToken("at")
	_ = fs.CreateToken(context.Background(), storage.Token{TokenHash: refreshHash, Kind: "refresh", ExpiresAt: time.Now().Add(time.Hour)})
	_ = fs.CreateToken(context.Background(), storage.Token{TokenHash: accessHash, Kind: "access", RefreshFor: refreshHash, ExpiresAt: time.Now().Add(time.Hour)})

	body := url.Values{"token": {"rt"}}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	r, _ := fs.GetToken(context.Background(), refreshHash)
	a, _ := fs.GetToken(context.Background(), accessHash)
	if r.RevokedAt == nil || a.RevokedAt == nil {
		t.Error("expected both tokens revoked")
	}
}
```

- [ ] **Step 2: Run, expect failure**

Run: `go test ./internal/oauth/server/ -run TestRevoke -v`
Expected: build error.

- [ ] **Step 3: Implement**

Create `internal/oauth/server/revoke.go`:
```go
package server

import (
	"net/http"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	token := r.PostForm.Get("token")
	if token == "" {
		w.WriteHeader(http.StatusOK)
		return
	}
	hash := storage.HashToken(token)
	// Spec: respond 200 even if the token is unknown. Internally,
	// always revoke the whole refresh family to be safe.
	_ = s.cfg.Store.RevokeRefreshFamily(r.Context(), hash)
	_ = s.cfg.Store.RevokeToken(r.Context(), hash)
	w.WriteHeader(http.StatusOK)
}
```

Add to `routes()` in `server.go`:
```go
s.mux.HandleFunc("/oauth/revoke", s.handleRevoke)
```

- [ ] **Step 4: Run, expect pass**

Run: `go test ./internal/oauth/server/ -run TestRevoke -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/oauth/server/revoke.go internal/oauth/server/revoke_test.go internal/oauth/server/server.go
git commit -m "$(cat <<'EOF'
feat(oauth/server): /oauth/revoke endpoint

RFC 7009-compatible: 200 OK regardless of whether the token was known;
revokes the refresh family to ensure derived access tokens die too.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 14: Bearer middleware that fronts /mcp and threads creds into ctx

**Files:**
- Create: `internal/oauth/server/middleware.go`
- Create: `internal/oauth/server/middleware_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/oauth/server/middleware_test.go`:
```go
package server

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	if !contains(rec.Header().Values("WWW-Authenticate"), `Bearer resource_metadata="https://mcp.example.com/.well-known/oauth-protected-resource"`) {
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
		Kind: "access", ClientID: "cli", UserID: user.ID,
		ExpiresAt: time.Now().Add(time.Hour),
	})

	var sawEmail, sawKey, sawUA string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e, k, ua, err := credprovider.FromContext(r.Context())
		if err != nil { t.Fatalf("FromContext: %v", err) }
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
```

- [ ] **Step 2: Run, expect failure**

Run: `go test ./internal/oauth/server/ -run TestBearer -v`
Expected: build error.

- [ ] **Step 3: Implement**

Create `internal/oauth/server/middleware.go`:
```go
package server

import (
	"net/http"
	"strings"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/credprovider"
	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

// Bearer wraps next with the OAuth resource-server check: requires a valid
// access token in Authorization, looks up the user, decrypts the Organizze
// API key, and places the resolved credentials on the request context.
func (s *Server) Bearer(next http.Handler) http.Handler {
	challenge := `Bearer resource_metadata="` + s.cfg.PublicURL + `/.well-known/oauth-protected-resource"`
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("Authorization")
		if !strings.HasPrefix(raw, "Bearer ") {
			w.Header().Set("WWW-Authenticate", challenge)
			http.Error(w, "missing bearer", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(raw, "Bearer ")
		tok, err := s.cfg.Store.GetToken(r.Context(), storage.HashToken(token))
		if err != nil || tok.Kind != "access" || tok.RevokedAt != nil || tok.ExpiresAt.Before(s.cfg.Now()) {
			w.Header().Set("WWW-Authenticate", challenge)
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		user, err := s.cfg.Store.GetUser(r.Context(), tok.UserID)
		if err != nil {
			http.Error(w, "server_error", http.StatusInternalServerError)
			return
		}
		apiKey, err := s.cfg.Cipher.Open(user.APIKeyCipher, user.APIKeyNonce)
		if err != nil {
			http.Error(w, "server_error", http.StatusInternalServerError)
			return
		}
		ctx := credprovider.WithCredentials(r.Context(), credprovider.Credentials{
			Email: user.OrganizzeEmail, APIKey: string(apiKey), UserAgent: user.UserAgent,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
```

Note: tests use `contains` from `authorize.go`. The signature of `contains` in `authorize.go` is `contains([]string, string) bool` — make sure that's exported (lowercase is fine within the package). Test should call it as a package-internal helper; this works because tests are in the `server` package.

- [ ] **Step 4: Run middleware tests, expect pass**

Run: `go test ./internal/oauth/server/ -run TestBearer -v`
Expected: 3 PASS.

- [ ] **Step 5: Run the full server suite**

Run: `go test ./internal/oauth/server/ -v`
Expected: every test PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/oauth/server/middleware.go internal/oauth/server/middleware_test.go
git commit -m "$(cat <<'EOF'
feat(oauth/server): Bearer middleware fronting /mcp

Validates the access token against the Store, decrypts the user's
Organizze API key, and threads (email, api_key, user_agent) into ctx
via credprovider.WithCredentials. WWW-Authenticate points clients at
the protected-resource metadata for OAuth-driven discovery.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 15: OAuth-binary config + main wiring + end-to-end test

**Files:**
- Create: `internal/config/oauth_config.go`
- Create: `internal/config/oauth_config_test.go`
- Modify: `cmd/organizze-mcp-oauth/main.go`
- Create: `cmd/organizze-mcp-oauth/main_test.go`

- [ ] **Step 1: Write config tests**

Create `internal/config/oauth_config_test.go`:
```go
package config

import (
	"strings"
	"testing"
)

func TestLoadOAuth_RejectsSingleTenantEnv(t *testing.T) {
	t.Setenv("OAUTH_DATABASE_URL", "postgres://x")
	t.Setenv("OAUTH_PUBLIC_URL", "https://x")
	t.Setenv("OAUTH_ENCRYPTION_KEY", strings.Repeat("0", 64))
	t.Setenv("OAUTH_COOKIE_SECRET", "s")
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
		t.Setenv("OAUTH_COOKIE_SECRET", "s")
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
	t.Setenv("OAUTH_COOKIE_SECRET", "s")
	t.Setenv("ORGANIZZE_API_KEY", "")
	if _, err := LoadOAuth(); err == nil {
		t.Error("expected hex-decode error")
	}
}
```

- [ ] **Step 2: Implement OAuth config**

Create `internal/config/oauth_config.go`:
```go
package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// OAuthConfig is the resolved env for cmd/organizze-mcp-oauth.
type OAuthConfig struct {
	DatabaseURL    string
	PublicURL      string        // no trailing slash
	EncryptionKey  []byte        // 32 bytes
	CookieSecret   []byte
	HTTPAddr       string        // default :8080
	HTTPTimeout    time.Duration // upstream Organizze timeout
	OrganizzeBase  string        // default https://api.organizze.com.br/rest/v2
	AccessTokenTTL time.Duration // default 1h
	RefreshTTL     time.Duration // default 30d
	SessionTTL     time.Duration // default 24h
}

func LoadOAuth() (*OAuthConfig, error) {
	if os.Getenv("ORGANIZZE_API_KEY") != "" {
		return nil, errors.New("ORGANIZZE_API_KEY must NOT be set for organizze-mcp-oauth (multi-tenant binary; creds come from OAuth tokens)")
	}
	cfg := &OAuthConfig{
		DatabaseURL:   os.Getenv("OAUTH_DATABASE_URL"),
		PublicURL:     strings.TrimRight(os.Getenv("OAUTH_PUBLIC_URL"), "/"),
		HTTPAddr:      os.Getenv("MCP_HTTP_ADDR"),
		OrganizzeBase: os.Getenv("ORGANIZZE_BASE_URL"),
	}
	if cfg.OrganizzeBase == "" {
		cfg.OrganizzeBase = "https://api.organizze.com.br/rest/v2"
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}

	var missing []string
	if cfg.DatabaseURL == "" { missing = append(missing, "OAUTH_DATABASE_URL") }
	if cfg.PublicURL == "" { missing = append(missing, "OAUTH_PUBLIC_URL") }
	if os.Getenv("OAUTH_ENCRYPTION_KEY") == "" { missing = append(missing, "OAUTH_ENCRYPTION_KEY") }
	if os.Getenv("OAUTH_COOKIE_SECRET") == "" { missing = append(missing, "OAUTH_COOKIE_SECRET") }
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	key, err := hex.DecodeString(os.Getenv("OAUTH_ENCRYPTION_KEY"))
	if err != nil {
		return nil, fmt.Errorf("OAUTH_ENCRYPTION_KEY: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("OAUTH_ENCRYPTION_KEY: must decode to 32 bytes, got %d", len(key))
	}
	cfg.EncryptionKey = key
	cfg.CookieSecret = []byte(os.Getenv("OAUTH_COOKIE_SECRET"))

	cfg.HTTPTimeout = 30 * time.Second
	cfg.AccessTokenTTL = time.Hour
	cfg.RefreshTTL = 30 * 24 * time.Hour
	cfg.SessionTTL = 24 * time.Hour
	return cfg, nil
}
```

- [ ] **Step 3: Run config tests, expect pass**

Run: `go test ./internal/config/ -v`
Expected: existing + new tests PASS.

- [ ] **Step 4: Wire the OAuth binary**

Replace `cmd/organizze-mcp-oauth/main.go` with:
```go
// Command organizze-mcp-oauth is the multi-tenant variant of organizze-mcp.
// See AGENTS.md and cmd/organizze-mcp-oauth/README.md for the operator runbook.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/adapter/mcp"
	"github.com/jorgejr568/organizze-mcp/internal/adapter/organizze"
	"github.com/jorgejr568/organizze-mcp/internal/config"
	"github.com/jorgejr568/organizze-mcp/internal/oauth/credprovider"
	"github.com/jorgejr568/organizze-mcp/internal/oauth/server"
	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
	"github.com/jorgejr568/organizze-mcp/internal/stats"
	"github.com/jorgejr568/organizze-mcp/internal/usecase"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "organizze-mcp-oauth:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadOAuth()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("pgxpool: %w", err)
	}
	defer pool.Close()
	if err := storage.ApplyMigrations(ctx, pool); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	store := storage.NewPostgres(pool)
	cipher, err := storage.NewCipher(cfg.EncryptionKey)
	if err != nil {
		return fmt.Errorf("cipher: %w", err)
	}

	// Upstream Organizze client (shared across all tenants — credentials are per-request).
	httpClient := organizze.NewClient(organizze.ClientOptions{Timeout: cfg.HTTPTimeout})
	exec, err := organizze.NewRequestExecutor(organizze.RequestExecutorOptions{
		HTTPClient:  httpClient,
		BaseURL:     cfg.OrganizzeBase,
		Credentials: credprovider.FromContext,
	})
	if err != nil {
		return fmt.Errorf("executor: %w", err)
	}

	validate := func(ctx context.Context, email, apiKey, ua string) error {
		// Probe with GET /accounts under those creds. Cheap, no side effects.
		validator, err := organizze.NewRequestExecutor(organizze.RequestExecutorOptions{
			HTTPClient:  httpClient,
			BaseURL:     cfg.OrganizzeBase,
			Credentials: credprovider.Static(email, apiKey, ua),
		})
		if err != nil {
			return err
		}
		var ignored []map[string]any
		return validator.Get(ctx, "/accounts", &ignored)
	}

	oauthSrv := server.New(server.Config{
		PublicURL:         cfg.PublicURL,
		Store:             store,
		Cipher:            cipher,
		ValidateOrganizze: validate,
		AccessTokenTTL:    cfg.AccessTokenTTL,
		RefreshTokenTTL:   cfg.RefreshTTL,
		SessionTTL:        cfg.SessionTTL,
		CookieSecret:      cfg.CookieSecret,
	})

	mcpServer := mcp.New(mcp.Dependencies{
		Reporter:    stats.NoopReporter{},
		User:        usecase.NewUserService(organizze.NewUserRepository(exec)),
		Account:     usecase.NewAccountService(organizze.NewAccountRepository(exec)),
		Category:    usecase.NewCategoryService(organizze.NewCategoryRepository(exec)),
		Budget:      usecase.NewBudgetService(organizze.NewBudgetRepository(exec)),
		CreditCard:  usecase.NewCreditCardService(organizze.NewCreditCardRepository(exec)),
		Invoice:     usecase.NewInvoiceService(organizze.NewInvoiceRepository(exec)),
		Transfer:    usecase.NewTransferService(organizze.NewTransferRepository(exec)),
		Transaction: usecase.NewTransactionService(organizze.NewTransactionRepository(exec)),
	})

	mux := http.NewServeMux()
	mux.Handle("/mcp", oauthSrv.Bearer(mcpsdk.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcpsdk.Server { return mcpServer },
		nil,
	)))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/", oauthSrv) // OAuth + .well-known routes

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Printf("organizze-mcp-oauth listening on %s (public_url=%s)", cfg.HTTPAddr, cfg.PublicURL)
		err := httpSrv.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		return err
	}
}
```

- [ ] **Step 5: Write the end-to-end test**

Create `cmd/organizze-mcp-oauth/main_test.go`:
```go
package main

// This test runs the full OAuth flow against an in-memory Postgres-less
// setup: it constructs server.Server with a fakeStore equivalent (re-use
// the one from internal/oauth/server tests is not visible from here, so
// we exercise via the real binary path only when OAUTH_DATABASE_URL is
// set). When DB is absent, this test SKIPS.
//
// The flow:
//   1. POST /oauth/register → client_id
//   2. POST /oauth/authorize (form) with fake Organizze upstream → 303 with code
//   3. POST /oauth/token (authorization_code) → access_token
//   4. POST /mcp with Bearer access_token → list of tools

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jorgejr568/organizze-mcp/internal/adapter/mcp"
	"github.com/jorgejr568/organizze-mcp/internal/adapter/organizze"
	"github.com/jorgejr568/organizze-mcp/internal/oauth/credprovider"
	"github.com/jorgejr568/organizze-mcp/internal/oauth/server"
	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
	"github.com/jorgejr568/organizze-mcp/internal/stats"
	"github.com/jorgejr568/organizze-mcp/internal/usecase"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestEndToEnd_OAuthThenToolCall(t *testing.T) {
	dsn := os.Getenv("OAUTH_DATABASE_URL")
	if dsn == "" {
		t.Skip("OAUTH_DATABASE_URL not set; skipping e2e test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := storage.ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Wipe between runs.
	for _, tbl := range []string{"oauth_tokens", "oauth_codes", "oauth_sessions", "oauth_clients", "oauth_users"} {
		_, _ = pool.Exec(ctx, "TRUNCATE TABLE "+tbl+" RESTART IDENTITY CASCADE")
	}
	store := storage.NewPostgres(pool)

	// Fake Organizze upstream.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/accounts":
			_, _ = io.WriteString(w, `[{"id":1,"name":"Checking","type":"checking"}]`)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(upstream.Close)

	cipher, _ := storage.NewCipher(make([]byte, 32))
	httpClient := organizze.NewClient(organizze.ClientOptions{Timeout: 5 * time.Second})
	exec, _ := organizze.NewRequestExecutor(organizze.RequestExecutorOptions{
		HTTPClient: httpClient, BaseURL: upstream.URL,
		Credentials: credprovider.FromContext,
	})
	validate := func(ctx context.Context, email, apiKey, ua string) error {
		v, _ := organizze.NewRequestExecutor(organizze.RequestExecutorOptions{
			HTTPClient: httpClient, BaseURL: upstream.URL,
			Credentials: credprovider.Static(email, apiKey, ua),
		})
		var ignored []map[string]any
		return v.Get(ctx, "/accounts", &ignored)
	}

	oauthSrv := server.New(server.Config{
		PublicURL: "http://oauth.example.com",
		Store: store, Cipher: cipher, ValidateOrganizze: validate, CookieSecret: []byte("s"),
	})
	mcpServer := mcp.New(mcp.Dependencies{
		Reporter: stats.NoopReporter{},
		Account:  usecase.NewAccountService(organizze.NewAccountRepository(exec)),
		// other deps can be nil-safe for this test if not exercised; otherwise wire fully.
	})
	mux := http.NewServeMux()
	mux.Handle("/mcp", oauthSrv.Bearer(mcpsdk.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcpsdk.Server { return mcpServer }, nil)))
	mux.Handle("/", oauthSrv)
	front := httptest.NewServer(mux)
	t.Cleanup(front.Close)

	// 1. DCR
	dcrBody := `{"client_name":"E2E","redirect_uris":["https://chat.example.com/cb"]}`
	resp, err := http.Post(front.URL+"/oauth/register", "application/json", strings.NewReader(dcrBody))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: %v %d", err, resp.StatusCode)
	}
	var dcr struct{ ClientID string `json:"client_id"` }
	_ = json.NewDecoder(resp.Body).Decode(&dcr)

	// 2. Authorize POST (skipping GET; ChatGPT would render the form in a browser)
	verifier := "the-pkce-verifier-which-is-long-enough-to-pass-checks-1234"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	form := url.Values{
		"client_id":             {dcr.ClientID},
		"redirect_uri":          {"https://chat.example.com/cb"},
		"state":                 {"st"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"email":                 {"e@x.com"},
		"api_key":               {"key"},
		"user_agent":            {"E2E (e@x.com)"},
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err = client.PostForm(front.URL+"/oauth/authorize", form)
	if err != nil || resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("authorize: %v %d", err, resp.StatusCode)
	}
	loc, _ := url.Parse(resp.Header.Get("Location"))
	code := loc.Query().Get("code")
	if code == "" {
		t.Fatalf("no code in %s", resp.Header.Get("Location"))
	}

	// 3. Token exchange
	resp, err = http.PostForm(front.URL+"/oauth/token", url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {"https://chat.example.com/cb"},
		"client_id":     {dcr.ClientID},
		"code_verifier": {verifier},
	})
	if err != nil || resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("token: %v %d %s", err, resp.StatusCode, body)
	}
	var tok struct{ AccessToken string `json:"access_token"` }
	_ = json.NewDecoder(resp.Body).Decode(&tok)
	if tok.AccessToken == "" {
		t.Fatalf("no access_token")
	}

	// 4. /mcp tools/list with the bearer token (raw JSON-RPC against Streamable HTTP)
	body := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	req, _ := http.NewRequest(http.MethodPost, front.URL+"/mcp", body)
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err = http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("mcp call: %v %d %s", err, resp.StatusCode, raw)
	}
}
```

- [ ] **Step 6: Build and run all tests**

Run: `go build ./... && go test ./...`
Expected: builds; OAuth tests SKIP without `OAUTH_DATABASE_URL`. Set the env to run the e2e: `OAUTH_DATABASE_URL=postgres://localhost/organizze_oauth_test go test ./cmd/organizze-mcp-oauth/... -v`.

- [ ] **Step 7: Make full verification pass**

Run: `make test && make lint && make build && make oauth-build`
Expected: all green. Both `bin/organizze-mcp` and `bin/organizze-mcp-oauth` exist.

- [ ] **Step 8: Commit**

```bash
git add internal/config/oauth_config.go internal/config/oauth_config_test.go cmd/organizze-mcp-oauth/main.go cmd/organizze-mcp-oauth/main_test.go
git commit -m "$(cat <<'EOF'
feat(oauth): wire organizze-mcp-oauth binary end-to-end

Composes pgx pool → storage → server.Server → MCP handler. Refuses to
start if ORGANIZZE_API_KEY is set (single-tenant env in a multi-tenant
binary is a misconfig). End-to-end test runs full DCR → authorize →
token → /mcp dance against a fake Organizze upstream; skips without
OAUTH_DATABASE_URL.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 16: Dockerfile + README + CHANGELOG

**Files:**
- Create: `cmd/organizze-mcp-oauth/Dockerfile`
- Create: `cmd/organizze-mcp-oauth/README.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Write the Dockerfile**

Create `cmd/organizze-mcp-oauth/Dockerfile`:
```dockerfile
# syntax=docker/dockerfile:1.7
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath -ldflags="-s -w" \
    -o /out/organizze-mcp-oauth ./cmd/organizze-mcp-oauth

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/organizze-mcp-oauth /usr/local/bin/organizze-mcp-oauth
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/organizze-mcp-oauth"]
```

- [ ] **Step 2: Write the operator README**

Create `cmd/organizze-mcp-oauth/README.md`:
```markdown
# organizze-mcp-oauth

Multi-tenant variant of `organizze-mcp`. Hosts an OAuth 2.1 Authorization
Server alongside the MCP endpoint so each ChatGPT user authenticates with
their own Organizze credentials instead of the operator embedding a single
set in env vars.

## Run

```bash
docker run --rm -p 8080:8080 \
  -e OAUTH_DATABASE_URL=postgres://user:pass@host/organizze_oauth \
  -e OAUTH_PUBLIC_URL=https://your-host.example.com \
  -e OAUTH_ENCRYPTION_KEY=$(openssl rand -hex 32) \
  -e OAUTH_COOKIE_SECRET=$(openssl rand -hex 32) \
  jorgejr568/organizze-mcp-oauth:latest
```

| Env var                | Required | Purpose                                                  |
| ---------------------- | -------- | -------------------------------------------------------- |
| `OAUTH_DATABASE_URL`   | yes      | libpq URI for the OAuth Postgres                          |
| `OAUTH_PUBLIC_URL`     | yes      | Externally reachable origin (no trailing slash, HTTPS)    |
| `OAUTH_ENCRYPTION_KEY` | yes      | Hex-encoded 32 bytes; AES-GCM key for the Organizze API key column |
| `OAUTH_COOKIE_SECRET`  | yes      | HMAC secret for the browser session cookie               |
| `MCP_HTTP_ADDR`        | no       | Listen address, default `:8080`                          |
| `ORGANIZZE_BASE_URL`   | no       | Override Organizze API base                              |
| `ORGANIZZE_API_KEY`    | **must NOT be set** | Single-tenant env; the binary refuses to start with it set |

## Connect from ChatGPT (Developer Mode)

In ChatGPT → Settings → Connectors → Add custom MCP server:
- URL: `https://<your-host>/mcp`
- Auth: OAuth (ChatGPT will auto-discover via the `.well-known` docs)

On first authorize, ChatGPT will open `<your-host>/oauth/authorize` in a
browser tab. Enter the Organizze email + API key + user-agent string and
approve. The server validates the credentials against the live Organizze
API before storing them.

## Operations

- **Encryption key rotation.** Not yet supported in-binary. To rotate, write
  a one-off Go program that opens each `oauth_users` row with the old key
  and re-seals with the new. Bumping `OAUTH_ENCRYPTION_KEY` alone will make
  every stored `api_key` undecryptable — back up the key.
- **Migrations.** Applied automatically at startup. To run manually:
  `OAUTH_DATABASE_URL=... make oauth-migrate-up`.
- **Revoking a user.** `DELETE FROM oauth_users WHERE organizze_email = '…';`
  cascades to sessions, codes, and tokens.
- **Audit.** Bearer-token denials and Organizze validation failures are
  logged at stderr.
```

- [ ] **Step 3: CHANGELOG entry**

Add to `CHANGELOG.md` under `## [Unreleased]`:
```markdown
### Added
- `cmd/organizze-mcp-oauth/`: multi-tenant variant of the MCP server that hosts an OAuth 2.1 Authorization Server. Each ChatGPT user authenticates with their own Organizze credentials; the operator no longer embeds a single API key in env vars. Backed by Postgres (`internal/oauth/storage/`); API keys stored AES-GCM-encrypted under `OAUTH_ENCRYPTION_KEY`. See `cmd/organizze-mcp-oauth/README.md`.

### Changed
- `internal/adapter/organizze.RequestExecutor` now resolves per-request credentials via a `credprovider.CredentialsProvider` callback instead of capturing fixed values at construction. Single-tenant `cmd/organizze-mcp/` callers are unaffected (env values wrapped in `credprovider.Static`).
```

- [ ] **Step 4: Verify and commit**

Run: `make test && make lint && make build && make oauth-build`
Expected: all green.

Run: `docker build -f cmd/organizze-mcp-oauth/Dockerfile -t organizze-mcp-oauth:dev .`
Expected: image built; final stage uses distroless.

```bash
git add cmd/organizze-mcp-oauth/Dockerfile cmd/organizze-mcp-oauth/README.md CHANGELOG.md
git commit -m "$(cat <<'EOF'
docs(oauth): operator README + Dockerfile + CHANGELOG

Dockerfile follows the project's distroless/nonroot pattern. README
documents required env, ChatGPT Developer-Mode hookup, and the
encryption-key rotation gotcha.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Post-execution checklist (for the human merging)

- [ ] Run `make test && make lint && make build && make oauth-build` from a clean checkout.
- [ ] Run the e2e against a throwaway Postgres: `docker run --rm -d -p 5432:5432 -e POSTGRES_PASSWORD=test -e POSTGRES_DB=organizze_oauth_test postgres:16 && OAUTH_DATABASE_URL=postgres://postgres:test@localhost/organizze_oauth_test go test ./cmd/organizze-mcp-oauth/... -v`.
- [ ] Smoke against real Organizze (use the live-api-testing env from AGENTS.md): start the binary, hit `/.well-known/oauth-authorization-server`, register a fake client, walk the authorize+token+mcp flow by hand with `curl`.
- [ ] Verify the single-tenant binary still works: `ORGANIZZE_EMAIL=… ORGANIZZE_API_KEY=… ORGANIZZE_USER_AGENT='X (x@y.com)' MCP_TRANSPORT=stdio ./bin/organizze-mcp`.
- [ ] Decide on release vehicle: this is a `feat:` PR. After merge, when ready for a release, follow the AGENTS.md release workflow (chore/release-vX.Y.0 → tag → manual GH release). The Docker image for the new binary needs its own image-build workflow analogous to `.github/workflows/release.yml` — out of scope for this plan, file as a follow-up.
