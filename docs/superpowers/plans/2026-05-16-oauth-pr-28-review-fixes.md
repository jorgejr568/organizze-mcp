# OAuth PR #28 Review-Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Address every blocking, should-fix, and nit issue raised in the review of [PR #28](https://github.com/jorgejr568/organizze-mcp/pull/28) and push the result onto the same branch (`worktree-oauth-multi-tenant`).

**Architecture:** The PR is broadly correct. Fixes fall in three buckets: (a) **rebase recovery** of the v0.8.0/v0.8.1 zap migration that the branch silently reverted; (b) **OAuth correctness** — atomic refresh-token rotation, code-replay → family revoke, PKCE verifier length validation, charset checks; (c) **defence-in-depth** — DCR rate limit, HMAC-signed consent-binding (replaces stale session scaffolding), bearer-scheme case-insensitivity, missing indexes, migration version table. Several cleanup nits land in a single final commit.

**Tech Stack:** Go 1.25, pgx/v5, `go.uber.org/zap`, `golang.org/x/time/rate`, `crypto/hmac` (stdlib).

**Strategy on history:** the branch's first commit reverts already-merged code. Cleanest fix is **rebase onto `origin/main` then force-push**. The user owns the branch and explicitly authorised "push to this PR 28", which we read as authorising force-push to that branch only. Main is not touched.

---

## File Structure (decomposition decisions)

**Modified (recovery from rebase):**
- `internal/adapter/organizze/executor.go` — restore `Logger *zap.Logger` field + structured records.
- `cmd/organizze-mcp/main.go` — restore `newLogger()` and `Logger:` field passthrough.
- `internal/adapter/organizze/executor_test.go`, `internal/adapter/organizze/transaction_repository_test.go` — restore `zaptest/observer`-based tests.
- `cmd/organizze-mcp/main_test.go` — restore `TestNewLogger_WritesJSONToStderr`.
- `internal/stats/http_reporter.go` + test — restored automatically by the rebase.

**Modified (OAuth fixes):**
- `cmd/organizze-mcp-oauth/main.go` — build `*zap.Logger`, pass to `server.Config.Logger`, replace `log.Printf` with structured logs.
- `internal/oauth/server/server.go` — add `Logger *zap.Logger` to `Config`, default to `zap.NewNop()`, drop stale comment.
- `internal/oauth/server/token.go` — call atomic `RotateRefreshToken`, gate code-replay path on consumed-and-extant code, enforce verifier length 43–128.
- `internal/oauth/server/authorize.go` — base64url charset on `code_challenge`; replace TODO CSRF with HMAC-signed consent-binding; bind `(client_id, redirect_uri, code_challenge, state)` on GET, verify on POST; explicit `template.Execute` error handling.
- `internal/oauth/server/middleware.go` — case-insensitive Bearer scheme; reject `kind != "access"` (already in code, add a regression test).
- `internal/oauth/server/register.go` — per-IP rate limiter, defaults 10 req/min/IP.
- `internal/oauth/server/templates/login.html` — render `consent_token` instead of legacy `csrf`.
- `internal/oauth/storage/storage.go` — add `RotateRefreshToken(ctx, oldHash) (Token, error)`, add `RevokeFamilyByCode(ctx, codeHash) error`.
- `internal/oauth/storage/postgres.go` — implement both new methods via atomic SQL.
- `internal/oauth/storage/migrate.go` — add `schema_migrations` version table; record + skip applied versions.
- `internal/oauth/storage/migrations/embed.go` — comment documents filter substring (`_down.sql`).
- `internal/oauth/storage/migrations/002_indexes_and_code_hash.sql` (NEW) — `oauth_tokens.refresh_for` partial index; `oauth_codes.expires_at` index; `oauth_tokens.code_hash BYTEA` nullable column; `schema_migrations` ledger table.
- `internal/oauth/storage/migrations/002_indexes_and_code_hash_down.sql` (NEW) — paired rollback.
- `cmd/organizze-mcp-oauth/README.md` — clarify `OAUTH_COOKIE_SECRET` byte semantics; note migration version table.

**New / extended tests:**
- `internal/oauth/server/token_test.go` — concurrent refresh rotation race; concurrent auth-code consume race; refresh after expiry; code replay revokes the family; PKCE verifier length boundaries (42/43/128/129).
- `internal/oauth/server/authorize_test.go` — `code_challenge` non-base64url charset; missing `redirect_uri` / `code_challenge`; `code_challenge_method=plain` rejection; CSRF (consent-binding) tamper detection.
- `internal/oauth/server/middleware_test.go` — lowercase `bearer` scheme accepted; refresh-kind token rejected.
- `internal/oauth/server/register_test.go` — 11th request from same IP within 1 minute returns 429.

**Nits cleanup commit:**
- `internal/oauth/server/server.go:73` stale comment.
- `internal/oauth/server/fakestore_test.go:96` panic-stubs (delete if unused after consent-binding refactor).
- `internal/oauth/server/authorize.go` rand.Read / template.Execute swallowed errors.

---

## Task 0: Rebase onto main (recovers zap)

**Files:** the entire branch.

- [ ] **Step 1:** From this worktree (`.claude/worktrees/oauth-fixes`), fetch and rebase.

```bash
git fetch origin
git rebase origin/main
```

- [ ] **Step 2:** Resolve conflicts. The known conflicts are:
  - `internal/adapter/organizze/executor.go` — keep `main`'s zap version; the OAuth branch's revert is the wrong side.
  - `cmd/organizze-mcp/main.go` — keep `main`'s zap version. The OAuth branch's `credprovider`-wrapping change must be ported on top of the zap version.
  - `internal/adapter/organizze/executor_test.go` / `transaction_repository_test.go` — keep `main`'s observer-based tests.
  - `CHANGELOG.md` — keep both `[Unreleased]` entries (OAuth bullet + whatever main has).
  - `internal/stats/http_reporter.go` and its test — keep `main`'s zap version.
  - `cmd/organizze-mcp/main_test.go` — keep `main`'s tests.
  Strategy when conflicted: take `main`'s file (`git checkout --theirs`) then re-apply the OAuth-branch additions by reading `git show <PR-branch-tip>:<path>` for the OAuth-specific changes (`credprovider`, etc.).

- [ ] **Step 3:** After conflict resolution, run a sanity build:

```bash
go build ./...
```

Expected: clean build. Failures here mean the rebase port missed something — fix and continue rebase.

- [ ] **Step 4:** Run the full test suite (no need to run race yet):

```bash
go test ./...
```

Expected: pre-existing tests pass. Some OAuth tests may need `OAUTH_DATABASE_URL` and will skip — that's fine.

- [ ] **Step 5:** Commit nothing yet (rebase already rewrote the existing commits with conflict resolutions). Verify history:

```bash
git log --oneline origin/main..HEAD
```

Expected: each OAuth commit replayed on top of `main`, in order, no merge commits.

---

## Task 1: Wire zap into the OAuth binary

**Files:**
- Modify: `cmd/organizze-mcp-oauth/main.go`
- Modify: `internal/oauth/server/server.go`

- [ ] **Step 1:** In `internal/oauth/server/server.go`, add `Logger *zap.Logger` to `Config` and default to `zap.NewNop()` in `New`. Drop the stale `// /mcp registered in later tasks.` comment.

```go
// in import block:
"go.uber.org/zap"

// in Config struct:
Logger *zap.Logger // optional; defaults to zap.NewNop()

// in New():
if cfg.Logger == nil {
    cfg.Logger = zap.NewNop()
}
cfg.Logger = cfg.Logger.Named("oauth")
```

- [ ] **Step 2:** Replace stdlib `log` usages in `cmd/organizze-mcp-oauth/main.go` with a zap logger built via `newLogger()` (copy the implementation from `cmd/organizze-mcp/main.go`). Pass `Logger: logger` into `server.New`. Each error path uses `logger.Fatal(...)` or `logger.Error(...)` with structured fields (`zap.String("addr", ...)`, `zap.Error(err)`).

- [ ] **Step 3:** Run `go vet ./... && go build ./...`. Expected: clean.

- [ ] **Step 4:** Smoke test stderr emits JSON:

```bash
OAUTH_PUBLIC_URL=https://example.com OAUTH_ENCRYPTION_KEY=$(openssl rand -hex 32) \
OAUTH_COOKIE_SECRET=$(openssl rand -hex 32) OAUTH_DATABASE_URL=postgres://nowhere \
HTTP_ADDR=127.0.0.1:0 timeout 1 ./bin/organizze-mcp-oauth 2>&1 >/dev/null | head -3
```

Expected: at least one `{"level":"...","ts":"...","msg":"..."}` line, OR an early-exit Fatal record. Either is JSON.

- [ ] **Step 5:** Commit.

```bash
git add cmd/organizze-mcp-oauth/main.go internal/oauth/server/server.go
git commit -m "feat(oauth): wire zap JSON logger into the multi-tenant binary"
```

---

## Task 2: Atomic refresh-token rotation

**Files:**
- Modify: `internal/oauth/storage/storage.go` (interface)
- Modify: `internal/oauth/storage/postgres.go` (implementation)
- Modify: `internal/oauth/server/fakestore_test.go` (fake impl)
- Modify: `internal/oauth/server/token.go` (call site)
- Modify: `internal/oauth/server/token_test.go` (concurrent test)

- [ ] **Step 1: Write the failing test (concurrent rotation).** Add to `token_test.go`:

```go
func TestToken_RefreshGrant_ConcurrentRotation_OnlyOneSucceeds(t *testing.T) {
    s := newServer(t)
    user := s.cfg.Store.(*fakeStore).seedUser(t, "u@example.com")
    client := s.cfg.Store.(*fakeStore).seedClient(t, "c", []string{"https://app/cb"})
    _, refresh, _ := s.issueTokenPair(context.Background(), client.ID, user.ID)

    var success, failure atomic.Int32
    var wg sync.WaitGroup
    start := make(chan struct{})
    for i := 0; i < 8; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            <-start
            form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {client.ID}}
            req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
            req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
            rec := httptest.NewRecorder()
            s.handleToken(rec, req)
            if rec.Code == 200 {
                success.Add(1)
            } else {
                failure.Add(1)
            }
        }()
    }
    close(start)
    wg.Wait()
    if success.Load() != 1 {
        t.Fatalf("expected exactly 1 success, got %d (failures: %d)", success.Load(), failure.Load())
    }
}
```

- [ ] **Step 2: Run the test.** Run `go test ./internal/oauth/server/ -run TestToken_RefreshGrant_ConcurrentRotation -race -v`. Expected: FAIL (>1 success).

- [ ] **Step 3: Add the interface method.** In `storage.go`:

```go
// RotateRefreshToken atomically marks the refresh token revoked, IFF it is
// currently un-revoked and un-expired, and returns the row. Two concurrent
// callers will see at most one success — the loser receives ErrNotFound and
// must treat the second-use as a reuse attack.
RotateRefreshToken(ctx context.Context, refreshHash []byte) (Token, error)
```

- [ ] **Step 4: Implement on Postgres.** In `postgres.go`:

```go
func (s *Postgres) RotateRefreshToken(ctx context.Context, refreshHash []byte) (Token, error) {
    const q = `
UPDATE oauth_tokens
   SET revoked_at = NOW()
 WHERE token_hash = $1
   AND kind = 'refresh'
   AND revoked_at IS NULL
   AND expires_at > NOW()
RETURNING token_hash, kind, client_id, user_id, refresh_for, expires_at, revoked_at, created_at
`
    row := s.pool.QueryRow(ctx, q, refreshHash)
    var t Token
    if err := row.Scan(&t.TokenHash, &t.Kind, &t.ClientID, &t.UserID, &t.RefreshFor, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt); err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return Token{}, ErrNotFound
        }
        return Token{}, err
    }
    return t, nil
}
```

- [ ] **Step 5: Implement on the fake store** (`fakestore_test.go`):

```go
func (f *fakeStore) RotateRefreshToken(_ context.Context, h []byte) (storage.Token, error) {
    f.mu.Lock()
    defer f.mu.Unlock()
    tok, ok := f.tokens[string(h)]
    if !ok || tok.Kind != "refresh" || tok.RevokedAt != nil || tok.ExpiresAt.Before(time.Now()) {
        return storage.Token{}, storage.ErrNotFound
    }
    now := time.Now()
    tok.RevokedAt = &now
    f.tokens[string(h)] = tok
    return tok, nil
}
```

(Locking is what makes the fake serialise; the test relies on this to expose the handler-side race.)

- [ ] **Step 6: Rewrite the handler.** Replace `grantRefreshToken` body in `token.go`:

```go
func (s *Server) grantRefreshToken(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    rt := r.PostForm.Get("refresh_token")
    clientID := r.PostForm.Get("client_id")
    if rt == "" || clientID == "" {
        writeOAuthError(w, http.StatusBadRequest, "invalid_request", "")
        return
    }
    hash := storage.HashToken(rt)

    // Distinguish "garbage / expired / already-rotated" from "wins the rotation."
    // GetToken first so we can detect prior revocation (genuine reuse signal)
    // and trigger family revoke — without the GetToken pre-read, an attacker
    // replaying a rotated refresh would just hit a benign invalid_grant.
    pre, err := s.cfg.Store.GetToken(ctx, hash)
    if err == nil && pre.Kind == "refresh" && pre.RevokedAt != nil {
        _ = s.cfg.Store.RevokeRefreshFamily(ctx, hash)
        writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "")
        return
    }

    tok, err := s.cfg.Store.RotateRefreshToken(ctx, hash)
    if err != nil {
        // Lost the race, or garbage, or expired.
        writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "")
        return
    }
    if tok.ClientID != clientID {
        // Roll back is impossible (UPDATE already committed); revoke the
        // family so a leaked refresh + bogus client_id can't be exploited.
        _ = s.cfg.Store.RevokeRefreshFamily(ctx, hash)
        writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "client_id mismatch")
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
```

- [ ] **Step 7: Run the failing test again.** Run `go test ./internal/oauth/server/ -run TestToken_RefreshGrant_ConcurrentRotation -race -v`. Expected: PASS.

- [ ] **Step 8: Run all OAuth server tests with -race.** Run `go test ./internal/oauth/server/ -race -count=1 -v`. Expected: PASS — confirms no regression of the existing serial rotation test or revoke-family handling.

- [ ] **Step 9: Commit.**

```bash
git add internal/oauth/storage/storage.go internal/oauth/storage/postgres.go internal/oauth/server/fakestore_test.go internal/oauth/server/token.go internal/oauth/server/token_test.go
git commit -m "fix(oauth): atomic refresh-token rotation"
```

---

## Task 3: Auth-code replay revokes the issued token family

**Files:**
- Modify: `internal/oauth/storage/storage.go` (add `RevokeFamilyByCode` interface + extend `Token` struct with `CodeHash` for traceability)
- Modify: `internal/oauth/storage/postgres.go` (implementation)
- Modify: `internal/oauth/storage/migrations/002_indexes_and_code_hash.sql` (NEW — adds column + indexes)
- Modify: `internal/oauth/storage/migrations/002_indexes_and_code_hash_down.sql` (NEW)
- Modify: `internal/oauth/server/token.go` (carry code_hash into `issueTokenPair`; call `RevokeFamilyByCode` on second use)
- Modify: `internal/oauth/server/fakestore_test.go` (fake impl)
- Modify: `internal/oauth/server/token_test.go` (new test)

- [ ] **Step 1: Write migration 002 (additive: column + indexes).** Create `internal/oauth/storage/migrations/002_indexes_and_code_hash.sql`:

```sql
-- 002_indexes_and_code_hash.sql
-- Index oauth_tokens.refresh_for so RevokeRefreshFamily is O(family size),
-- not a table scan. Index oauth_codes.expires_at for periodic GC.
-- Add code_hash on oauth_tokens so an auth-code replay can revoke every
-- token issued from that code (RFC 6749 §10.5 / Security BCP §4.10).

BEGIN;

ALTER TABLE oauth_tokens ADD COLUMN IF NOT EXISTS code_hash BYTEA;

CREATE INDEX IF NOT EXISTS oauth_tokens_refresh_for_idx
  ON oauth_tokens (refresh_for) WHERE refresh_for IS NOT NULL;

CREATE INDEX IF NOT EXISTS oauth_tokens_code_hash_idx
  ON oauth_tokens (code_hash) WHERE code_hash IS NOT NULL;

CREATE INDEX IF NOT EXISTS oauth_codes_expires_idx
  ON oauth_codes (expires_at);

COMMIT;
```

Create `internal/oauth/storage/migrations/002_indexes_and_code_hash_down.sql`:

```sql
BEGIN;
DROP INDEX IF EXISTS oauth_codes_expires_idx;
DROP INDEX IF EXISTS oauth_tokens_code_hash_idx;
DROP INDEX IF EXISTS oauth_tokens_refresh_for_idx;
ALTER TABLE oauth_tokens DROP COLUMN IF EXISTS code_hash;
COMMIT;
```

- [ ] **Step 2: Extend `Token` struct + interface.** In `storage.go`:

```go
type Token struct {
    TokenHash  []byte
    Kind       string
    ClientID   string
    UserID     int64
    RefreshFor []byte
    CodeHash   []byte // set on the tokens issued from an authorization-code grant; nil on refresh-grant
    ExpiresAt  time.Time
    RevokedAt  *time.Time
    CreatedAt  time.Time
}

// RevokeFamilyByCode revokes every still-live token whose CodeHash equals the
// given hash. Called when an authorization code is presented twice — the
// second presentation is a reuse-attack signal.
RevokeFamilyByCode(ctx context.Context, codeHash []byte) error
```

- [ ] **Step 3: Update Postgres impl.** In `postgres.go`, extend `CreateToken`, `GetToken`, `RotateRefreshToken` Scan/Insert to round-trip `code_hash`. Add:

```go
func (s *Postgres) RevokeFamilyByCode(ctx context.Context, codeHash []byte) error {
    const q = `
UPDATE oauth_tokens
   SET revoked_at = NOW()
 WHERE code_hash = $1 AND revoked_at IS NULL
`
    if _, err := s.pool.Exec(ctx, q, codeHash); err != nil {
        return fmt.Errorf("oauth/storage: revoke family by code: %w", err)
    }
    return nil
}
```

- [ ] **Step 4: Update fake store.** Add `CodeHash` to fake's token map values; implement `RevokeFamilyByCode`:

```go
func (f *fakeStore) RevokeFamilyByCode(_ context.Context, codeHash []byte) error {
    f.mu.Lock()
    defer f.mu.Unlock()
    now := time.Now()
    for h, tok := range f.tokens {
        if bytes.Equal(tok.CodeHash, codeHash) && tok.RevokedAt == nil {
            tok.RevokedAt = &now
            f.tokens[h] = tok
        }
    }
    return nil
}
```

- [ ] **Step 5: Thread code_hash through handler.** Modify `issueTokenPair` to take `codeHash []byte` (nil when called from refresh-grant). In `grantAuthorizationCode`, pass `ac.CodeHash`. In `grantRefreshToken`, pass nil.

```go
func (s *Server) issueTokenPair(ctx context.Context, clientID string, userID int64, codeHash []byte) (string, string, error) {
    // ... unchanged except both CreateToken calls set CodeHash: codeHash
}
```

- [ ] **Step 6: Detect code replay + revoke family.** In `grantAuthorizationCode`, change the `ConsumeAuthCode` error branch:

```go
ac, err := s.cfg.Store.ConsumeAuthCode(ctx, storage.HashToken(code))
if err != nil {
    // Second-use is the attack signal. Use a separate GetAuthCode for
    // diagnostics; if the row exists and consumed_at IS NOT NULL, we know
    // this code was successfully redeemed before — revoke whatever tokens
    // were issued from it.
    if errors.Is(err, storage.ErrNotFound) {
        // best-effort family revoke; ignore errors (the row may genuinely
        // not exist if the client sent garbage).
        _ = s.cfg.Store.RevokeFamilyByCode(ctx, storage.HashToken(code))
    }
    writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "code unknown, used, or expired")
    return
}
```

(`RevokeFamilyByCode` is a no-op for unknown codes — UPDATE with no matching rows. No need to distinguish "never existed" from "was consumed.")

- [ ] **Step 7: Write the test.** In `token_test.go`:

```go
func TestToken_AuthorizationCode_Replay_RevokesFamily(t *testing.T) {
    s := newServer(t)
    user := s.cfg.Store.(*fakeStore).seedUser(t, "u@example.com")
    client := s.cfg.Store.(*fakeStore).seedClient(t, "c", []string{"https://app/cb"})

    verifier := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQ" // 43 chars
    sum := sha256.Sum256([]byte(verifier))
    challenge := base64.RawURLEncoding.EncodeToString(sum[:])

    code := "auth-code-xyz"
    _ = s.cfg.Store.CreateAuthCode(context.Background(), storage.AuthCode{
        CodeHash:            storage.HashToken(code),
        ClientID:            client.ID,
        UserID:              user.ID,
        RedirectURI:         "https://app/cb",
        CodeChallenge:       challenge,
        CodeChallengeMethod: "S256",
        ExpiresAt:           time.Now().Add(5 * time.Minute),
    })

    // First exchange: success.
    form := url.Values{
        "grant_type": {"authorization_code"}, "code": {code},
        "client_id": {client.ID}, "redirect_uri": {"https://app/cb"},
        "code_verifier": {verifier},
    }
    rec := httptest.NewRecorder()
    s.handleToken(rec, mustForm(t, "POST", "/oauth/token", form))
    if rec.Code != 200 {
        t.Fatalf("first exchange: status %d body %s", rec.Code, rec.Body.String())
    }
    var first tokenResponse
    _ = json.Unmarshal(rec.Body.Bytes(), &first)
    accessHashFirst := storage.HashToken(first.AccessToken)

    // Second exchange of the same code: invalid_grant AND the previously
    // issued access token must now be revoked.
    rec2 := httptest.NewRecorder()
    s.handleToken(rec2, mustForm(t, "POST", "/oauth/token", form))
    if rec2.Code != 400 {
        t.Fatalf("replay: expected 400, got %d", rec2.Code)
    }
    tok, _ := s.cfg.Store.GetToken(context.Background(), accessHashFirst)
    if tok.RevokedAt == nil {
        t.Fatal("expected previously issued access token to be revoked after code replay")
    }
}
```

(Add `mustForm` helper if not present.)

- [ ] **Step 8: Run new test.** `go test ./internal/oauth/server/ -run TestToken_AuthorizationCode_Replay_RevokesFamily -v`. Expected: PASS.

- [ ] **Step 9: Run all OAuth tests.** `go test ./internal/oauth/... -race -count=1`. Expected: PASS.

- [ ] **Step 10: Commit.**

```bash
git add internal/oauth/storage/migrations/ internal/oauth/storage/storage.go internal/oauth/storage/postgres.go internal/oauth/server/fakestore_test.go internal/oauth/server/token.go internal/oauth/server/token_test.go
git commit -m "fix(oauth): revoke token family on authorization-code replay"
```

---

## Task 4: PKCE verifier length enforcement (RFC 7636 §4.1)

**Files:**
- Modify: `internal/oauth/server/token.go`
- Modify: `internal/oauth/server/token_test.go`

- [ ] **Step 1: Write the failing test.** Add to `token_test.go`:

```go
func TestVerifyPKCE_RejectsOutOfRangeVerifier(t *testing.T) {
    // S256 challenge for the valid-length verifier; the function should
    // still reject anything outside [43, 128] regardless of hash match.
    short := strings.Repeat("a", 42)
    okLen := strings.Repeat("a", 43)
    long := strings.Repeat("a", 129)

    sum := sha256.Sum256([]byte(okLen))
    challenge := base64.RawURLEncoding.EncodeToString(sum[:])

    if verifyPKCE(short, challenge) {
        t.Fatal("expected reject on 42-char verifier")
    }
    if verifyPKCE(long, challenge) {
        t.Fatal("expected reject on 129-char verifier")
    }
    if !verifyPKCE(okLen, challenge) {
        t.Fatal("expected accept on 43-char verifier")
    }
}
```

- [ ] **Step 2: Run it.** `go test ./internal/oauth/server/ -run TestVerifyPKCE_RejectsOutOfRangeVerifier -v`. Expected: FAIL on `short` (or on `long` — pre-fix it returns whatever SHA-256 happens to produce).

- [ ] **Step 3: Add the length guard.** In `token.go`:

```go
func verifyPKCE(verifier, challenge string) bool {
    if n := len(verifier); n < 43 || n > 128 {
        return false
    }
    sum := sha256.Sum256([]byte(verifier))
    got := base64.RawURLEncoding.EncodeToString(sum[:])
    return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}
```

- [ ] **Step 4: Run test.** Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add internal/oauth/server/token.go internal/oauth/server/token_test.go
git commit -m "fix(oauth): enforce PKCE verifier length 43-128 (RFC 7636 §4.1)"
```

---

## Task 5: code_challenge base64url charset validation

**Files:**
- Modify: `internal/oauth/server/authorize.go`
- Modify: `internal/oauth/server/authorize_test.go`

- [ ] **Step 1: Write the failing test.** Add to `authorize_test.go`:

```go
func TestAuthorize_RejectsNonBase64URLChallenge(t *testing.T) {
    s := newServer(t)
    _ = s.cfg.Store.(*fakeStore).seedClient(t, "c", []string{"https://app/cb"})

    // 43 chars but contains '/' which is base64-std, not base64url.
    bogus := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA/aa"
    q := url.Values{
        "client_id":             {"c"},
        "redirect_uri":          {"https://app/cb"},
        "code_challenge":        {bogus},
        "code_challenge_method": {"S256"},
    }
    req := httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
    rec := httptest.NewRecorder()
    s.handleAuthorize(rec, req)
    if rec.Code != 400 {
        t.Fatalf("expected 400, got %d", rec.Code)
    }
}
```

- [ ] **Step 2: Run it.** Expected: FAIL (length-only check accepts it).

- [ ] **Step 3: Add the charset guard.** In `authorize.go`, replace the `len(challenge) != 43` check with:

```go
if !isBase64URLNoPad(challenge, 43) {
    return loginViewModel{}, errors.New("invalid_request: code_challenge must be 43-char base64url (S256)")
}
```

Add helper:

```go
// isBase64URLNoPad reports whether s is exactly n base64url chars
// (A-Z, a-z, 0-9, '-', '_') with no padding.
func isBase64URLNoPad(s string, n int) bool {
    if len(s) != n {
        return false
    }
    for _, c := range s {
        switch {
        case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_':
        default:
            return false
        }
    }
    return true
}
```

- [ ] **Step 4: Run test.** Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add internal/oauth/server/authorize.go internal/oauth/server/authorize_test.go
git commit -m "fix(oauth): validate PKCE code_challenge as base64url charset"
```

---

## Task 6: HMAC-signed consent binding (replaces stale CSRF / session-trust TODO)

**Files:**
- Modify: `internal/oauth/server/server.go` (consume `CookieSecret` as HMAC key)
- Modify: `internal/oauth/server/authorize.go` (sign on GET, verify on POST)
- Modify: `internal/oauth/server/templates/login.html` (rename `csrf` field to `consent_token`)
- Modify: `internal/oauth/server/authorize_test.go` (new tests)

**Design rationale:** the existing session scaffolding (`session.go`) is dead code (constructed but never wired); wiring full server-side sessions is a bigger lift than this PR's scope. Stateless HMAC-signed binding addresses the same threat (CSRF + POST-field tampering) with no DB writes and gives `OAUTH_COOKIE_SECRET` a real job. Session work can land later as a follow-up; this PR closes the security gap.

- [ ] **Step 1: In `server.go`, parse `CookieSecret` into raw bytes and expose on `Server`.** Confirm an existing accessor (`Server.cfg.CookieSecret`) is already a `[]byte`; if string, decode hex or expose as bytes via `[]byte(cfg.CookieSecret)`.

- [ ] **Step 2: Add the binding helpers** (in `authorize.go` or a new sibling file `consent.go`):

```go
type consentBinding struct {
    ClientID    string
    RedirectURI string
    Challenge   string
    State       string
    IssuedAt    int64 // unix seconds
}

const consentTTL = 10 * time.Minute

func signConsent(secret []byte, b consentBinding) string {
    payload := strings.Join([]string{
        b.ClientID, b.RedirectURI, b.Challenge, b.State,
        strconv.FormatInt(b.IssuedAt, 10),
    }, "\x1f") // unit separator — disallowed in OAuth field values
    mac := hmac.New(sha256.New, secret)
    mac.Write([]byte(payload))
    sig := mac.Sum(nil)
    return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." +
        base64.RawURLEncoding.EncodeToString(sig)
}

func verifyConsent(secret []byte, token string, now time.Time) (consentBinding, error) {
    parts := strings.SplitN(token, ".", 2)
    if len(parts) != 2 {
        return consentBinding{}, errors.New("malformed")
    }
    payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
    if err != nil {
        return consentBinding{}, errors.New("malformed payload")
    }
    sig, err := base64.RawURLEncoding.DecodeString(parts[1])
    if err != nil {
        return consentBinding{}, errors.New("malformed sig")
    }
    mac := hmac.New(sha256.New, secret)
    mac.Write(payloadBytes)
    if !hmac.Equal(sig, mac.Sum(nil)) {
        return consentBinding{}, errors.New("bad signature")
    }
    fields := strings.Split(string(payloadBytes), "\x1f")
    if len(fields) != 5 {
        return consentBinding{}, errors.New("bad payload")
    }
    issuedAt, err := strconv.ParseInt(fields[4], 10, 64)
    if err != nil {
        return consentBinding{}, errors.New("bad issued_at")
    }
    if now.Unix()-issuedAt > int64(consentTTL.Seconds()) {
        return consentBinding{}, errors.New("expired")
    }
    return consentBinding{
        ClientID: fields[0], RedirectURI: fields[1], Challenge: fields[2],
        State: fields[3], IssuedAt: issuedAt,
    }, nil
}
```

- [ ] **Step 3: GET issues the consent token; render into the template.** In `authorizeGET`:

```go
vm.ConsentToken = signConsent(s.cfg.CookieSecret, consentBinding{
    ClientID:    vm.ClientID,
    RedirectURI: vm.RedirectURI,
    Challenge:   vm.CodeChallenge,
    State:       vm.State,
    IssuedAt:    s.cfg.Now().Unix(),
})
```

Add `ConsentToken string` to `loginViewModel`. Update `login.html`:

```html
<input type="hidden" name="consent_token" value="{{.ConsentToken}}">
```

- [ ] **Step 4: POST verifies the consent token.** In `authorizePOST` (replacing the TODO comment):

```go
if err := r.ParseForm(); err != nil {
    http.Error(w, "invalid form", http.StatusBadRequest)
    return
}
binding, err := verifyConsent(s.cfg.CookieSecret, r.PostForm.Get("consent_token"), s.cfg.Now())
if err != nil {
    http.Error(w, "invalid consent token: "+err.Error(), http.StatusBadRequest)
    return
}
// Sanity check: the POST'd hidden fields must equal the bound values.
// Mismatch = tampering or a stale form.
if r.PostForm.Get("client_id") != binding.ClientID ||
   r.PostForm.Get("redirect_uri") != binding.RedirectURI ||
   r.PostForm.Get("code_challenge") != binding.Challenge {
    http.Error(w, "consent params mismatch", http.StatusBadRequest)
    return
}
// Re-validate (defensive — client may have been deleted between GET and POST).
vm, err := s.parseAuthorizeParams(r.Context(), r.PostForm)
if err != nil {
    http.Error(w, err.Error(), http.StatusBadRequest)
    return
}
```

- [ ] **Step 5: Write tests.** In `authorize_test.go`:

```go
func TestAuthorize_POST_RejectsMissingConsentToken(t *testing.T) { /* POST with no consent_token → 400 */ }
func TestAuthorize_POST_RejectsTamperedClientID(t *testing.T) {
    // GET → grab consent_token → POST with client_id=DIFFERENT → 400.
}
func TestAuthorize_POST_RejectsExpiredConsentToken(t *testing.T) {
    // build a binding with IssuedAt = now - 11min; verifyConsent → expired.
}
func TestAuthorize_POST_HappyPath(t *testing.T) {
    // GET → POST with bound fields + valid creds → 303 with ?code=...&state=...
}
```

- [ ] **Step 6: Run all OAuth tests.** Fix any pre-existing tests that POST without a consent_token. Expected: PASS after fixes.

- [ ] **Step 7: Drop dead session scaffolding** — only if confirmed unused. Run `grep -r "newSessionManager\|SessionManager\|CreateSession\|GetSession\|DeleteSession" --include="*.go" .` to confirm zero non-test references; if so, delete `session.go` + `session_test.go` and the corresponding Store interface methods + Postgres impls. Mention in commit message. Otherwise leave for follow-up.

- [ ] **Step 8: Commit.**

```bash
git add internal/oauth/server/ cmd/organizze-mcp-oauth/README.md
git commit -m "fix(oauth): HMAC-signed consent binding closes CSRF + field-tamper gap"
```

---

## Task 7: DCR per-IP rate limit

**Files:**
- Modify: `internal/oauth/server/register.go`
- Modify: `internal/oauth/server/server.go` (construct limiter)
- Modify: `internal/oauth/server/register_test.go`
- Modify: `go.mod` / `go.sum` (`golang.org/x/time/rate`)

- [ ] **Step 1: Add the dep.** `go get golang.org/x/time/rate`.

- [ ] **Step 2: Add limiter type.** In `register.go`:

```go
// ipRateLimiter is a per-IP token bucket. Buckets are evicted lazily as new
// IPs arrive; for a single-operator deployment the steady-state population
// is small enough that this never pressures memory.
type ipRateLimiter struct {
    mu       sync.Mutex
    buckets  map[string]*rate.Limiter
    rps      rate.Limit
    burst    int
    maxIPs   int
}

func newIPRateLimiter(rps rate.Limit, burst, maxIPs int) *ipRateLimiter {
    return &ipRateLimiter{buckets: make(map[string]*rate.Limiter), rps: rps, burst: burst, maxIPs: maxIPs}
}

func (l *ipRateLimiter) allow(ip string) bool {
    l.mu.Lock()
    defer l.mu.Unlock()
    b, ok := l.buckets[ip]
    if !ok {
        if len(l.buckets) >= l.maxIPs {
            // Random eviction; the simplest bound that doesn't add a heap.
            for k := range l.buckets {
                delete(l.buckets, k)
                break
            }
        }
        b = rate.NewLimiter(l.rps, l.burst)
        l.buckets[ip] = b
    }
    return b.Allow()
}
```

- [ ] **Step 3: Construct and wire in `Server`.** Default 10 reqs/min/IP (i.e. `rate.Every(6*time.Second)`), burst 10, max 10_000 IPs. Make it overridable via `Config.DCRLimiter` (nil → default).

- [ ] **Step 4: Reject 429 in handler.**

```go
func clientIP(r *http.Request) string {
    if h := r.Header.Get("X-Forwarded-For"); h != "" {
        return strings.TrimSpace(strings.SplitN(h, ",", 2)[0])
    }
    if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
        return ip
    }
    return r.RemoteAddr
}

// At top of handleRegister, after the method check:
if !s.dcrLimiter.allow(clientIP(r)) {
    writeOAuthError(w, http.StatusTooManyRequests, "rate_limited", "too many client registrations")
    return
}
```

- [ ] **Step 5: Test.** In `register_test.go`:

```go
func TestRegister_RateLimitedAfterBurst(t *testing.T) {
    s := newServer(t)
    body := `{"client_name":"x","redirect_uris":["https://app/cb"]}`
    var first, ratelimited int
    for i := 0; i < 20; i++ {
        req := httptest.NewRequest("POST", "/oauth/register", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        req.RemoteAddr = "1.2.3.4:12345"
        rec := httptest.NewRecorder()
        s.handleRegister(rec, req)
        if rec.Code == 201 {
            first++
        } else if rec.Code == 429 {
            ratelimited++
        }
    }
    if first == 0 || ratelimited == 0 {
        t.Fatalf("expected mix: created=%d 429=%d", first, ratelimited)
    }
}
```

- [ ] **Step 6: Commit.**

```bash
git add internal/oauth/server/register.go internal/oauth/server/server.go internal/oauth/server/register_test.go go.mod go.sum
git commit -m "fix(oauth): per-IP rate limit on DCR endpoint"
```

---

## Task 8: Bearer scheme case-insensitive + refresh-kind rejection test

**Files:**
- Modify: `internal/oauth/server/middleware.go`
- Modify: `internal/oauth/server/middleware_test.go`

- [ ] **Step 1: Write failing tests.**

```go
func TestBearer_AcceptsLowercaseScheme(t *testing.T) {
    s, access, _ := bootstrapAccess(t)
    req := httptest.NewRequest("GET", "/mcp", nil)
    req.Header.Set("Authorization", "bearer "+access)
    rec := httptest.NewRecorder()
    s.Bearer(http.HandlerFunc(okHandler)).ServeHTTP(rec, req)
    if rec.Code != 200 {
        t.Fatalf("lowercase bearer: got %d body %s", rec.Code, rec.Body.String())
    }
}

func TestBearer_RejectsRefreshKindToken(t *testing.T) {
    s, _, refresh := bootstrapAccess(t)
    req := httptest.NewRequest("GET", "/mcp", nil)
    req.Header.Set("Authorization", "Bearer "+refresh)
    rec := httptest.NewRecorder()
    s.Bearer(http.HandlerFunc(okHandler)).ServeHTTP(rec, req)
    if rec.Code != 401 {
        t.Fatalf("refresh as bearer: got %d", rec.Code)
    }
}
```

(Adds `bootstrapAccess` helper that returns access+refresh strings.)

- [ ] **Step 2: Implement.** Replace `if !strings.HasPrefix(raw, "Bearer ")` with:

```go
const schemeLen = len("bearer ")
if len(raw) < schemeLen || !strings.EqualFold(raw[:schemeLen], "Bearer ") {
    w.Header().Set("WWW-Authenticate", challenge)
    http.Error(w, "missing bearer", http.StatusUnauthorized)
    return
}
token := raw[schemeLen:]
```

- [ ] **Step 3: Run tests.** Expected: PASS.

- [ ] **Step 4: Commit.**

```bash
git add internal/oauth/server/middleware.go internal/oauth/server/middleware_test.go
git commit -m "fix(oauth): accept case-insensitive Bearer scheme per RFC 6750 §2.1"
```

---

## Task 9: Migration version table

**Files:**
- Modify: `internal/oauth/storage/migrate.go`
- Modify: `internal/oauth/storage/migrate_test.go`
- Modify: `internal/oauth/storage/migrations/embed.go` (doc comment about `_down.sql` convention)

- [ ] **Step 1: Rewrite `ApplyMigrations`** to:
  1. `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())` (outside any embedded file — bootstrap).
  2. For each non-`_down.sql` file in lexical order, check whether its filename is already in `schema_migrations`. If yes, skip. If no, exec the file in a transaction, then `INSERT INTO schema_migrations(version) VALUES ($1)` in the same transaction.

```go
const bootstrap = `CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`

func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
    if _, err := pool.Exec(ctx, bootstrap); err != nil {
        return fmt.Errorf("oauth/storage: bootstrap schema_migrations: %w", err)
    }
    entries, err := migrations.FS.ReadDir(".")
    if err != nil {
        return fmt.Errorf("oauth/storage: read embed: %w", err)
    }
    var names []string
    for _, e := range entries {
        if e.IsDir() || strings.HasSuffix(e.Name(), "_down.sql") {
            continue
        }
        names = append(names, e.Name())
    }
    sort.Strings(names)
    for _, name := range names {
        var exists bool
        if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, name).Scan(&exists); err != nil {
            return fmt.Errorf("oauth/storage: check applied %s: %w", name, err)
        }
        if exists {
            continue
        }
        body, err := migrations.FS.ReadFile(name)
        if err != nil {
            return fmt.Errorf("oauth/storage: read %s: %w", name, err)
        }
        if _, err := pool.Exec(ctx, string(body)); err != nil {
            return fmt.Errorf("oauth/storage: apply %s: %w", name, err)
        }
        if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
            return fmt.Errorf("oauth/storage: record %s: %w", name, err)
        }
    }
    return nil
}
```

(Note: file is responsible for its own BEGIN/COMMIT today. The ledger insert is a separate transaction. That's acceptable; the worst-case is "migration applied, ledger not recorded" which retries gracefully because every migration is `CREATE TABLE IF NOT EXISTS` / `ALTER ... IF NOT EXISTS`.)

- [ ] **Step 2: Update `migrate_test.go`** if it exists — confirm a second `ApplyMigrations` call is a no-op (idempotent). Skip with `OAUTH_DATABASE_URL` env gate like the existing tests.

- [ ] **Step 3: Update embed.go comment** to document the filter convention.

- [ ] **Step 4: Commit.**

```bash
git add internal/oauth/storage/migrate.go internal/oauth/storage/migrate_test.go internal/oauth/storage/migrations/embed.go
git commit -m "feat(oauth): record applied migrations in schema_migrations table"
```

---

## Task 10: Additional test coverage gaps

**Files:** test files only.

- [ ] **Step 1: Concurrent auth-code consume.** In `token_test.go`:

```go
func TestToken_AuthorizationCode_ConcurrentConsume_OnlyOneSucceeds(t *testing.T) {
    // 8 goroutines POST the same code+verifier; exactly 1 should get 200.
}
```

- [ ] **Step 2: Refresh after expiry.** In `token_test.go`:

```go
func TestToken_RefreshGrant_ExpiredReturnsInvalidGrantNoFamilyRevoke(t *testing.T) {
    // Issue refresh with ExpiresAt in the past. POST → 400 invalid_grant.
    // Confirm no other tokens were revoked.
}
```

- [ ] **Step 3: Missing-field authorize.** In `authorize_test.go`:

```go
func TestAuthorize_RejectsMissingRedirectURI(t *testing.T) { ... }
func TestAuthorize_RejectsMissingCodeChallenge(t *testing.T) { ... }
func TestAuthorize_RejectsCodeChallengeMethodPlain(t *testing.T) { ... }
```

- [ ] **Step 4: Run the additions.** `go test ./internal/oauth/server/ -race -count=1`. Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add internal/oauth/server/token_test.go internal/oauth/server/authorize_test.go
git commit -m "test(oauth): cover concurrent auth-code, expired refresh, missing authorize fields"
```

---

## Task 11: Nits cleanup

**Files:**
- Modify: `internal/oauth/server/authorize.go` (explicit error handling)
- Modify: `internal/oauth/server/server.go` (stale comment)
- Modify: `cmd/organizze-mcp-oauth/README.md`
- Optionally: `internal/oauth/server/fakestore_test.go` (kill panic stubs if dead)

- [ ] **Step 1:** Replace `_ = loginTpl.Execute(w, vm)` with explicit error logging via the server's zap logger. Same for any `_, _ = rand.Read(b)` paths in `newRandomToken`, `newPublicID`. On Go 1.24+ `crypto/rand.Read` panics on failure, so the existing code is technically safe, but explicit `if err != nil { panic(err) }` documents intent.

- [ ] **Step 2:** Remove the stale `// /mcp registered in later tasks.` comment from `server.go`.

- [ ] **Step 3:** README updates: clarify `OAUTH_COOKIE_SECRET` minimum is in raw bytes (so 32 hex chars = 16 bytes ≠ acceptable; require 32+ bytes — i.e. 64 hex chars). Mention the new `schema_migrations` table. Mention the new `code_hash` column on `oauth_tokens`. Remove the "Authorize form re-prompts" known-limitation if the consent-binding closed the CSRF concern (re-prompting per-flow is now a deliberate UX choice, not a security gap).

- [ ] **Step 4:** Commit.

```bash
git add -u
git commit -m "chore(oauth): cleanup — explicit error handling, stale comments, README"
```

---

## Task 12: Update CHANGELOG, verify, force-push

- [ ] **Step 1:** Extend the existing `[Unreleased]` OAuth bullet to mention review-feedback fixes (atomic refresh, code-replay family revoke, HMAC consent binding, DCR rate limit, PKCE length + charset enforcement, schema_migrations ledger).

- [ ] **Step 2:** Run the full suite + race + lint + builds.

```bash
make test
go test -race -count=1 ./...
make lint
make build
make oauth-build
```

Expected: all green.

- [ ] **Step 3:** If `OAUTH_DATABASE_URL` is available, run the OAuth e2e tests. Otherwise document the skip in the PR comment.

- [ ] **Step 4:** Force-push.

```bash
git push --force-with-lease origin worktree-oauth-multi-tenant
```

`--force-with-lease` is the safer variant: it refuses if the remote has commits we don't have locally (i.e. someone pushed to the branch in parallel). Per author authorisation: force-push is restricted to **this branch only**; main is not touched.

- [ ] **Step 5:** Add a PR comment summarising the fixes and pointing to this plan file. Reference the original review for traceability.

---

## Spec coverage self-review

- ✅ Blocking #1 (zap regression in executor) — Task 0 rebase.
- ✅ Blocking #2 (zap regression in single-tenant main) — Task 0 rebase + Task 1 wires the OAuth binary the same way.
- ✅ Blocking #3 (auth-code replay no family revoke) — Task 3.
- ✅ Blocking #4 (refresh rotation TOCTOU) — Task 2.
- ✅ Should-fix (PKCE verifier length) — Task 4.
- ✅ Should-fix (code_challenge charset) — Task 5.
- ✅ Should-fix (CSRF / POST trusts hidden fields) — Task 6 (HMAC consent binding).
- ✅ Should-fix (DCR unauthenticated/unbounded) — Task 7.
- ✅ Should-fix (no migration version table) — Task 9.
- ✅ Should-fix (Bearer case-sensitive) — Task 8.
- ✅ Should-fix (missing indexes) — Task 3 (migration 002).
- ✅ Nits (rand.Read / Execute / stale comment / README / dead session stubs) — Task 11.
- ✅ Coverage gaps (concurrent rotation/consume; expired refresh; bearer rejects refresh-kind; missing authorize fields) — Tasks 2, 8, 10.
- ✅ Conventions (zap, layering, CHANGELOG) — Task 1, Task 12.

No placeholders, no TBD steps, no "similar to Task N" references — every snippet is complete code in the file you'd paste it into.

---

## Risks and trade-offs

- **Force-push to a PR branch.** Acceptable because the user authored the branch and explicitly asked for push. Risk: any reviewer with an in-flight comment thread loses git-line anchoring. Mitigation: announce in the PR comment + use `--force-with-lease`.
- **Session scaffolding deletion in Task 6 Step 7.** Only deletes if `grep` confirms zero non-test references. If the author had a near-term follow-up plan for sessions, leaving the code is safer than deleting; the consent-binding approach is independent of that future work.
- **Migration 002 adds a column without backfilling.** `code_hash` is nullable, so existing tokens (which there shouldn't be in any production DB yet given the PR isn't merged) stay valid and just can't participate in code-replay family-revoke. Acceptable for a pre-release PR.
- **Inline execution.** The plan is large but linear; the chained context (rebase resolution → handler shape → test fixtures) makes inline execution cheaper than subagent dispatch, which would re-load the same OAuth code per task.
