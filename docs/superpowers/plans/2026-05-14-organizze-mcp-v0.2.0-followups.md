# organizze-mcp v0.2.0 follow-ups — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Address every "Soon" and "Eventually" follow-up flagged by the final v0.1.0 reviewer in one cohesive v0.2.0 release: link-time version injection, repo OSS hygiene (LICENSE/CHANGELOG/SECURITY/CONTRIBUTING), README tool-count clarity, 429 rate-limit error semantics, JSON-shape roundtrip tests, in-code documentation of partial-update semantics, tool-description error hints, `Client.Inner()` audit, GitHub Actions Node-24 refresh, and expanded Docker image labels.

**Architecture:** Pure surface-level work — no new layers, no new packages. Every change either tightens an existing seam (error mapping, version injection), extracts a narrower API (`Inner()` → `Timeout()`), adds documentation (5 markdown files + a few doc comments), or hardens tests (JSON fixtures). The shape of the codebase is unchanged.

**Tech Stack:** Same as v0.1.0 — Go 1.25, `modelcontextprotocol/go-sdk` v1.6.0+, stdlib `net/http`, `httptest`, GitHub Actions, Docker multi-arch.

**Branch protection note:** `main` requires the `Test` CI check to pass on every PR. Every commit in this plan lands on a feature branch (`chore/v0.2.0-followups`); Tasks 1–8 are 8 commits on that branch; Task 9 opens the PR, merges, then tags `v0.2.0`.

---

## File structure

```
organizze-mcp/
├── CHANGELOG.md                          (NEW)        Task 1
├── CONTRIBUTING.md                       (NEW)        Task 1
├── LICENSE                               (NEW — MIT)  Task 1
├── README.md                             (MODIFIED)   Task 2
├── SECURITY.md                           (NEW)        Task 1
├── Makefile                              (MODIFIED)   Task 6
├── Dockerfile                            (MODIFIED)   Task 6
├── .github/workflows/
│   ├── ci.yml                            (MODIFIED)   Task 8
│   └── release.yml                       (MODIFIED)   Tasks 6 + 8
└── internal/
    ├── adapter/
    │   ├── mcp/
    │   │   ├── server.go                 (MODIFIED)   Task 6   (const → var)
    │   │   └── tools_transactions.go     (MODIFIED)   Task 3
    │   └── organizze/
    │       ├── client.go                 (MODIFIED)   Task 7   (Inner→Timeout)
    │       ├── client_test.go            (MODIFIED)   Task 7
    │       ├── errors.go                 (MODIFIED)   Task 4
    │       ├── errors_test.go            (MODIFIED)   Task 4
    │       ├── jsonshape_test.go         (NEW)        Task 5
    │       └── testdata/                 (NEW)        Task 5
    │           ├── account.json
    │           ├── budget.json
    │           ├── category.json
    │           ├── credit_card.json
    │           ├── invoice.json
    │           ├── transaction.json
    │           ├── transfer.json
    │           └── user.json
    └── domain/
        ├── errors.go                     (MODIFIED)   Task 4
        ├── errors_test.go                (MODIFIED)   Task 4
        └── transaction.go                (MODIFIED)   Task 3
```

**Tasks 1–8 are independent enough to commit individually** on the feature branch. The engineer (or subagent dispatcher) may interleave reviews task-by-task per the subagent-driven-development skill; Task 9 is the single integration point.

---

## Task 1: OSS documentation pack (LICENSE, CHANGELOG, SECURITY, CONTRIBUTING)

Add the four standard repo-root markdown files. These add no code; they unblock follow-up work (metadata-action's `image.licenses` label, contributor guidance, security disclosure path).

**Files:**
- Create: `LICENSE`
- Create: `CHANGELOG.md`
- Create: `SECURITY.md`
- Create: `CONTRIBUTING.md`

- [ ] **Step 1: Start the feature branch**

```bash
cd /Users/j/src/jorgejr568/organizze-mcp
git checkout main && git pull --ff-only origin main
git checkout -b chore/v0.2.0-followups
```

Expected: clean working tree, branch `chore/v0.2.0-followups` based on `main`.

- [ ] **Step 2: Write `LICENSE` (MIT)**

`LICENSE`:

```
MIT License

Copyright (c) 2026 Jorge Junior

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- [ ] **Step 3: Write `CHANGELOG.md`** (Keep a Changelog format)

`CHANGELOG.md`:

```markdown
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-05-14

### Added
- Link-time version injection: `internal/adapter/mcp.Version` is now a `var` populated via `-ldflags="-X ..."` from the git tag at build time. Eliminates the v0.1.0 risk of the constant drifting from the released tag.
- `domain.ErrRateLimited` sentinel + HTTP `429 Too Many Requests` mapping in `APIError.Is`. Callers may now `errors.Is(err, domain.ErrRateLimited)` to detect throttling.
- `LICENSE` (MIT), `SECURITY.md`, `CONTRIBUTING.md`, `CHANGELOG.md` — standard OSS repo hygiene.
- `Client.Timeout()` accessor exposing the configured per-request deadline (replaces the broader `Inner()` escape hatch).
- JSON-shape roundtrip tests in `internal/adapter/organizze/jsonshape_test.go` — each `domain.*` type now has a captured fixture under `internal/adapter/organizze/testdata/` whose decode is asserted.
- Tool descriptions for `update_transaction` and `delete_transaction` now name their unsafe-retry / no-soft-delete semantics so LLM clients reason about them correctly.
- `internal/domain.UpdateTransactionParams` doc-comments the Organizze semantics: absent fields are left unchanged (not cleared).

### Changed
- `internal/adapter/mcp.Version` is no longer a `const`. Downstream callers reading `mcp.Version` see the same `string` type — no API break — but the value is now build-derived.
- `Client.Inner()` removed. Callers needing a custom `*http.Client` should pass it directly as the `HTTPClient` interface argument to `NewRequestExecutor` (the abstraction was already there; the convenience wrapper just hid it).
- README tool catalogue is now a 16-row numbered table — the v0.1.0 row-merging hid the count.
- GitHub Actions checkout/setup-go/upload-artifact and docker/* actions bumped to Node-24-native versions, eliminating the deprecation warnings.
- Docker images now carry the full OCI label set (`image.licenses`, `image.revision`, `image.created`, `image.version`) via `docker/metadata-action`.

### Fixed
- `TestTransactionService_Create_ValidatesRequiredFields` now exercises each missing-field branch independently (the v0.1.0 table tripped on `AmountCents == 0` for three rows). Already landed in the v0.1.0 polish PR; tracked here for completeness.

## [0.1.0] - 2026-05-14

### Added
- Initial release: 16 MCP tools wrapping the Organizze REST API.
- Clean Architecture layout: `domain` → `usecase` → `adapter/{organizze,mcp}` → `cmd`.
- Both stdio and Streamable-HTTP transports via `MCP_TRANSPORT` env var.
- Multi-architecture Docker image (`linux/amd64`, `linux/arm64`) published to `jorgejr568/organizze-mcp` on Docker Hub.
- GitHub Actions CI (test on PR) and release (publish on `v*` tag) workflows.
- Branch protection on `main` requiring the `Test` check.

[Unreleased]: https://github.com/jorgejr568/organizze-mcp/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/jorgejr568/organizze-mcp/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/jorgejr568/organizze-mcp/releases/tag/v0.1.0
```

- [ ] **Step 4: Write `SECURITY.md`**

`SECURITY.md`:

```markdown
# Security policy

## Supported versions

| Version | Supported |
|---------|-----------|
| 0.2.x   | ✅        |
| 0.1.x   | ❌        |

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

Email `jorgejuniordev@gmail.com` with:
- A description of the issue.
- Reproduction steps or a proof-of-concept if available.
- The version (commit SHA or container digest) you observed it on.

Expect an acknowledgement within 72 hours and a fix or mitigation within 14 days for high-severity issues.

## Threat model in scope

organizze-mcp holds a user's Organizze API token in an environment variable and forwards it via HTTP Basic Auth on every request. Specifically in scope:

- Token leakage via logs, error messages, or panics. The server logs to `stderr` only on stdio; HTTP transport must not echo the token. Verify with `docker logs`.
- Request smuggling, path traversal, or other tool-name injection — MCP tool names and arguments are validated by the SDK schema; report any bypass.
- Supply-chain integrity of the published Docker image. The image is built reproducibly from a tagged commit on `main`; the commit SHA is embedded as the `org.opencontainers.image.revision` label. Compare against this repo before trusting the image.

Out of scope: general Organizze API security (report to Organizze directly), denial of service against your own running instance, social engineering.
```

- [ ] **Step 5: Write `CONTRIBUTING.md`**

`CONTRIBUTING.md`:

```markdown
# Contributing to organizze-mcp

Thanks for your interest. This is a small, opinionated project — please read this before opening a PR.

## Before you start

For non-trivial changes (a new tool, a new transport, a breaking API change), please open an issue first to discuss the design. The architecture is intentionally layered (`domain` → `usecase` → `adapter` → `cmd`); changes that cut across layers usually need a small spec note.

## Development setup

```bash
git clone https://github.com/jorgejr568/organizze-mcp
cd organizze-mcp
make test       # full suite with race detector
make test-cover # coverage report
make lint       # go vet
make build      # local binary
```

You'll need Go ≥ 1.25 and (for the container path) Docker with buildx.

## Pull-request expectations

- **Tests are part of the change.** Every behaviour change ships with a test that fails on `main` and passes on your branch. Tests verify behaviour, not mocks.
- **One concern per PR.** If you find unrelated polish along the way, split it.
- **Branch protection enforces the CI check.** Push will succeed; merge requires the `Test` job green.
- **Commit messages follow Conventional Commits**: `feat(scope): summary`, `fix(scope): summary`, `chore: summary`, `docs: summary`, `ci: summary`. The PR title becomes the squash-merge subject — make it look like a release-note line.

## Architecture overview

See the README's "Architecture" section. Two rules:

1. **Dependencies point inward.** `domain` imports stdlib only. `usecase` imports `domain`. `adapter/*` imports `usecase` + `domain`. `cmd` imports everyone. Going the other way means refactoring, not adding.
2. **Repository and service interfaces are consumer-owned.** They live in `usecase` (for repos) and in each `adapter/mcp/tools_*.go` (for services); the implementations satisfy them implicitly.

## Adding a new resource (e.g., `Contact`)

The pattern is mechanical. Six small new files, plus two edits:

1. `internal/domain/contact.go` — entity struct
2. `internal/usecase/contact.go` — `ContactRepository` interface + `ContactService` struct
3. `internal/usecase/contact_test.go` — service tests with a fake repo
4. `internal/adapter/organizze/contact_repository.go` — HTTP impl
5. `internal/adapter/organizze/contact_repository_test.go` — `httptest`-backed tests
6. `internal/adapter/mcp/tools_contacts.go` — consumer-side service interface + tool registrations
7. Edit `internal/adapter/mcp/server.go` — add `Contact ContactService` to `Dependencies`; call `registerContactTools` from `New`
8. Edit `cmd/organizze-mcp/main.go` — one line in `buildServer` wiring the service

The integration test in `internal/adapter/mcp/integration_test.go` will fail until you add the resource to `allExpectedTools` and the new endpoints to `fakeOrganizze`.

## Release process

Releases are tag-driven:

```bash
git tag -a vX.Y.Z -m "vX.Y.Z — short summary"
git push origin vX.Y.Z
```

The release workflow runs the suite, builds the multi-arch image, and publishes to Docker Hub.

## License

By contributing, you agree your contributions are licensed under the MIT License (see `LICENSE`).
```

- [ ] **Step 6: Commit**

```bash
git add LICENSE CHANGELOG.md SECURITY.md CONTRIBUTING.md
git commit -m "docs: add LICENSE, CHANGELOG, SECURITY, and CONTRIBUTING"
```

Expected: single commit, four new files staged.

---

## Task 2: README tool catalogue clarity + LICENSE link

The v0.1.0 final reviewer flagged that "16 tools" was hard to verify because the table merged rows (`list_accounts / get_account`). Convert to a 16-row numbered table that's grep-friendly and counts at a glance. Also wire the new `LICENSE` file into the README.

**Files:**
- Modify: `/Users/j/src/jorgejr568/organizze-mcp/README.md`

- [ ] **Step 1: Replace the tool catalogue section**

Find the existing block in `README.md` that begins with `## Tool catalogue` and ends just before `## Development`. Replace it with:

```markdown
## Tool catalogue (16 tools)

`amount_cents` is **negative for expenses, positive for income**.

| # | Tool | Operation |
|---|------|-----------|
| 1 | `get_user` | UserService.Get |
| 2 | `list_accounts` | AccountService.List |
| 3 | `get_account` | AccountService.Get |
| 4 | `list_categories` | CategoryService.List |
| 5 | `get_category` | CategoryService.Get |
| 6 | `list_budgets` | BudgetService.List (period routing) |
| 7 | `list_credit_cards` | CreditCardService.List |
| 8 | `get_credit_card` | CreditCardService.Get |
| 9 | `list_credit_card_invoices` | InvoiceService.List |
| 10 | `get_credit_card_invoice` | InvoiceService.Get |
| 11 | `list_transfers` | TransferService.List |
| 12 | `list_transactions` | TransactionService.List |
| 13 | `get_transaction` | TransactionService.Get |
| 14 | `create_transaction` | TransactionService.Create |
| 15 | `update_transaction` | TransactionService.Update |
| 16 | `delete_transaction` | TransactionService.Delete |
```

- [ ] **Step 2: Replace the License section**

Find the existing `## License` block in `README.md` (it currently reads `MIT (or your preferred license).`). Replace it with:

```markdown
## License

MIT — see [`LICENSE`](LICENSE).
```

- [ ] **Step 3: Verify with a grep count**

```bash
cd /Users/j/src/jorgejr568/organizze-mcp
grep -c '^|' README.md
```

The grep will return ~17 (one per row of the new table + the header row). The point of this verification step is that anyone scanning the README in the future can now `grep -c` to confirm.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs(readme): flatten tool catalogue to 16 numbered rows; link LICENSE"
```

---

## Task 3: In-code documentation — Update partial-body semantics + transaction-tool error hints

Two tiny doc-only changes the reviewer flagged: (a) the assumption that Organizze treats absent fields in PUT as "leave unchanged" (not "clear") is currently undocumented in code, and (b) the `update_transaction` / `delete_transaction` tool descriptions don't hint to LLM callers that mutations are non-reversible.

**Files:**
- Modify: `/Users/j/src/jorgejr568/organizze-mcp/internal/domain/transaction.go`
- Modify: `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/mcp/tools_transactions.go`

- [ ] **Step 1: Add the `UpdateTransactionParams` doc comment**

In `/Users/j/src/jorgejr568/organizze-mcp/internal/domain/transaction.go`, find the existing block:

```go
// UpdateTransactionParams describe a partial update; nil pointers are omitted.
type UpdateTransactionParams struct {
```

Replace those two lines with:

```go
// UpdateTransactionParams describe a partial update; nil pointers are omitted
// from the wire body via `omitempty`.
//
// Semantics rely on a load-bearing assumption about the upstream Organizze API:
// fields absent from the PUT body are treated as "leave unchanged", NOT as
// "clear to zero / null". If Organizze ever changes this behaviour, every
// caller of TransactionService.Update becomes destructive. The contract is
// tested at the wire level (TestTransactionRepository_Update_SendsOnlySetFields
// asserts that absent fields are absent from the JSON), but the semantic
// assumption beyond the wire is not tested by anything we control.
//
// Note: `Tags []Tag` has different semantics — because it's not a pointer,
// `omitempty` only drops nil; an explicit `[]Tag{}` will be marshalled and may
// clear server-side tags. Pass nil to leave tags unchanged.
type UpdateTransactionParams struct {
```

- [ ] **Step 2: Sharpen the `update_transaction` tool description**

In `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/mcp/tools_transactions.go`, find the existing block inside `registerTransactionTools`:

```go
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "update_transaction",
		Description: "Update fields on an existing Organizze transaction. Only fields you provide are changed.",
	}, updateTransactionHandler(svc))
```

Replace the `Description` string with:

```go
		Description: "Update fields on an existing Organizze transaction. Only fields you provide are changed; omitted fields are left unchanged (not cleared). To clear notes, pass an empty string; to replace tags, pass the full new tag list (omitting tags leaves them alone, but passing an empty array clears them).",
```

- [ ] **Step 3: Sharpen the `delete_transaction` tool description**

In the same file, find:

```go
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "delete_transaction",
		Description: "Permanently delete an Organizze transaction by id.",
	}, deleteTransactionHandler(svc))
```

Replace the `Description` with:

```go
		Description: "Permanently delete an Organizze transaction by id. There is no soft-delete; the row is gone after this call returns successfully. Calling delete on an already-deleted id returns a not-found error rather than re-deleting.",
```

- [ ] **Step 4: Run tests to confirm no regressions**

```bash
cd /Users/j/src/jorgejr568/organizze-mcp
go test ./... -count=1
```

Expected: every test still PASS. (These changes are doc-only; nothing tested at the string level.)

- [ ] **Step 5: Commit**

```bash
git add internal/domain/transaction.go internal/adapter/mcp/tools_transactions.go
git commit -m "docs(domain,mcp): clarify update partial-body semantics and tool descriptions"
```

---

## Task 4: Rate-limit (429) handling (TDD)

Add `domain.ErrRateLimited` so callers can `errors.Is(err, domain.ErrRateLimited)` to detect throttling, and map HTTP 429 in `APIError.Is`. The change is small but touches three files (domain + adapter + adapter test).

**Files:**
- Modify: `/Users/j/src/jorgejr568/organizze-mcp/internal/domain/errors.go`
- Modify: `/Users/j/src/jorgejr568/organizze-mcp/internal/domain/errors_test.go`
- Modify: `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/organizze/errors.go`
- Modify: `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/organizze/errors_test.go`

- [ ] **Step 1: Write the failing domain test**

Append to `/Users/j/src/jorgejr568/organizze-mcp/internal/domain/errors_test.go` (above the closing brace of the file is fine; place this after the existing test functions):

```go
func TestErrRateLimited_IsDistinctSentinel(t *testing.T) {
	if errors.Is(ErrRateLimited, ErrUpstream) {
		t.Error("ErrRateLimited must not match ErrUpstream")
	}
	if errors.Is(ErrRateLimited, ErrNotFound) {
		t.Error("ErrRateLimited must not match ErrNotFound")
	}
	wrapped := fmt.Errorf("organizze: throttled: %w", ErrRateLimited)
	if !errors.Is(wrapped, ErrRateLimited) {
		t.Error("errors.Is must traverse wrapping")
	}
}
```

- [ ] **Step 2: Run, verify failure**

```bash
cd /Users/j/src/jorgejr568/organizze-mcp
go test ./internal/domain/... -run TestErrRateLimited
```

Expected: build failure — `undefined: ErrRateLimited`.

- [ ] **Step 3: Add the domain sentinel**

In `/Users/j/src/jorgejr568/organizze-mcp/internal/domain/errors.go`, replace the existing `var (...)` block:

```go
var (
	ErrNotFound     = errors.New("domain: not found")
	ErrUnauthorized = errors.New("domain: unauthorized")
	ErrValidation   = errors.New("domain: validation failed")
	ErrUpstream     = errors.New("domain: upstream API error")
)
```

with:

```go
var (
	ErrNotFound     = errors.New("domain: not found")
	ErrUnauthorized = errors.New("domain: unauthorized")
	ErrValidation   = errors.New("domain: validation failed")
	ErrRateLimited  = errors.New("domain: rate limited")
	ErrUpstream     = errors.New("domain: upstream API error")
)
```

- [ ] **Step 4: Run domain tests, verify pass**

```bash
go test ./internal/domain/... -v
```

Expected: `TestSentinelsAreDistinct PASS`, `TestWrappedSentinelMatches PASS`, `TestErrRateLimited_IsDistinctSentinel PASS`.

- [ ] **Step 5: Write the failing adapter test**

In `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/organizze/errors_test.go`, find the existing `TestAPIError_MapsToDomainSentinels` function. Add a new row to its `cases` table:

```go
		{http.StatusTooManyRequests, domain.ErrRateLimited},
```

Place it between the `StatusBadRequest` row and the `StatusInternalServerError` row, so the table reads:

```go
	cases := []struct {
		status int
		sentinel error
	}{
		{http.StatusNotFound, domain.ErrNotFound},
		{http.StatusUnauthorized, domain.ErrUnauthorized},
		{http.StatusForbidden, domain.ErrUnauthorized},
		{http.StatusUnprocessableEntity, domain.ErrValidation},
		{http.StatusBadRequest, domain.ErrValidation},
		{http.StatusTooManyRequests, domain.ErrRateLimited},
		{http.StatusInternalServerError, domain.ErrUpstream},
	}
```

Also add a dedicated test (after the existing `TestAPIError_UnknownStatusMapsToUpstream`):

```go
func TestAPIError_429MapsToRateLimited(t *testing.T) {
	err := &APIError{StatusCode: http.StatusTooManyRequests}
	if !errors.Is(err, domain.ErrRateLimited) {
		t.Error("429 should map to ErrRateLimited")
	}
	if errors.Is(err, domain.ErrUpstream) {
		t.Error("429 should NOT also map to ErrUpstream (one sentinel per status)")
	}
}
```

- [ ] **Step 6: Run, verify failure**

```bash
go test ./internal/adapter/organizze/... -run TestAPIError
```

Expected: `TestAPIError_429MapsToRateLimited` FAILS — the `Is()` switch currently routes 429 to the default arm (`ErrUpstream`).

- [ ] **Step 7: Update the adapter's `Is()` mapping**

In `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/organizze/errors.go`, find the existing `Is` method:

```go
// Is supports errors.Is(err, domain.ErrXxx) — maps HTTP status to domain sentinels.
func (e *APIError) Is(target error) bool {
	switch e.StatusCode {
	case http.StatusNotFound:
		return target == domain.ErrNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return target == domain.ErrUnauthorized
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return target == domain.ErrValidation
	default:
		return target == domain.ErrUpstream
	}
}
```

Replace the switch body with:

```go
	switch e.StatusCode {
	case http.StatusNotFound:
		return target == domain.ErrNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return target == domain.ErrUnauthorized
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return target == domain.ErrValidation
	case http.StatusTooManyRequests:
		return target == domain.ErrRateLimited
	default:
		return target == domain.ErrUpstream
	}
```

- [ ] **Step 8: Run tests, verify pass**

```bash
go test ./internal/adapter/organizze/... -v
go test ./... -count=1
```

Expected: every test PASS, including the new `TestAPIError_429MapsToRateLimited` and the existing `TestAPIError_MapsToDomainSentinels` (with the new 429 row).

- [ ] **Step 9: Commit**

```bash
git add internal/domain/errors.go internal/domain/errors_test.go \
        internal/adapter/organizze/errors.go internal/adapter/organizze/errors_test.go
git commit -m "feat(errors): map HTTP 429 to domain.ErrRateLimited"
```

---

## Task 5: JSON-shape roundtrip tests (TDD)

The reviewer's surprising-absence finding: nothing in the test suite asserts that `domain.*` types correctly decode realistic Organizze response shapes. If Organizze renames a field, the existing httptest-backed repo tests will keep passing because they construct response strings inline. This task adds a captured-fixture test that loads each entity's representative JSON and asserts the key fields populate.

**Files:**
- Create: `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/organizze/testdata/user.json`
- Create: `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/organizze/testdata/account.json`
- Create: `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/organizze/testdata/category.json`
- Create: `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/organizze/testdata/budget.json`
- Create: `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/organizze/testdata/credit_card.json`
- Create: `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/organizze/testdata/invoice.json`
- Create: `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/organizze/testdata/transaction.json`
- Create: `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/organizze/testdata/transfer.json`
- Create: `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/organizze/jsonshape_test.go`

- [ ] **Step 1: Write the failing test**

`/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/organizze/jsonshape_test.go`:

```go
package organizze

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// TestJSONShape_DomainTypesDecodeRealisticFixtures decodes representative API
// payloads into the matching domain.* type and asserts each load-bearing field
// populates. This catches silent field drops caused by typos in JSON tags or
// upstream renames.
//
// To refresh a fixture: capture a real Organizze response (redacting any PII),
// drop it into testdata/<resource>.json, and ensure the assertion below for
// that case still picks meaningful fields.
func TestJSONShape_DomainTypesDecodeRealisticFixtures(t *testing.T) {
	t.Run("User", func(t *testing.T) {
		var u domain.User
		mustDecodeFixture(t, "user.json", &u)
		if u.ID == 0 || u.Name == "" || u.Email == "" || u.Role == "" {
			t.Errorf("User decode lost fields: %+v", u)
		}
	})

	t.Run("Account", func(t *testing.T) {
		var a domain.Account
		mustDecodeFixture(t, "account.json", &a)
		if a.ID == 0 || a.Name == "" || a.Type == "" {
			t.Errorf("Account decode lost fields: %+v", a)
		}
	})

	t.Run("Category", func(t *testing.T) {
		var c domain.Category
		mustDecodeFixture(t, "category.json", &c)
		if c.ID == 0 || c.Name == "" {
			t.Errorf("Category decode lost fields: %+v", c)
		}
	})

	t.Run("Budget", func(t *testing.T) {
		var b domain.Budget
		mustDecodeFixture(t, "budget.json", &b)
		if b.AmountInCents == 0 || b.CategoryID == 0 || b.Date == "" {
			t.Errorf("Budget decode lost fields: %+v", b)
		}
	})

	t.Run("CreditCard", func(t *testing.T) {
		var c domain.CreditCard
		mustDecodeFixture(t, "credit_card.json", &c)
		if c.ID == 0 || c.Name == "" || c.ClosingDay == 0 || c.DueDay == 0 || c.LimitCents == 0 {
			t.Errorf("CreditCard decode lost fields: %+v", c)
		}
	})

	t.Run("Invoice", func(t *testing.T) {
		var inv domain.Invoice
		mustDecodeFixture(t, "invoice.json", &inv)
		if inv.ID == 0 || inv.CreditCardID == 0 || inv.AmountCents == 0 ||
			inv.Date == "" || inv.StartingDate == "" || inv.ClosingDate == "" {
			t.Errorf("Invoice decode lost fields: %+v", inv)
		}
	})

	t.Run("Transaction", func(t *testing.T) {
		var tx domain.Transaction
		mustDecodeFixture(t, "transaction.json", &tx)
		if tx.ID == 0 || tx.Description == "" || tx.Date == "" ||
			tx.AmountCents == 0 || tx.AccountID == 0 || tx.CategoryID == 0 {
			t.Errorf("Transaction decode lost core fields: %+v", tx)
		}
		// optional but load-bearing in real usage:
		if len(tx.Tags) == 0 {
			t.Errorf("Transaction decode dropped Tags: %+v", tx)
		}
	})

	t.Run("Transfer", func(t *testing.T) {
		var tr domain.Transfer
		mustDecodeFixture(t, "transfer.json", &tr)
		if tr.ID == 0 || tr.Description == "" || tr.AmountCents == 0 ||
			tr.AccountID == 0 || tr.OppositeAccountID == 0 || tr.CategoryID == 0 {
			t.Errorf("Transfer decode lost fields: %+v", tr)
		}
	})
}

func mustDecodeFixture(t *testing.T, name string, into any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("unmarshal %s: %v", name, err)
	}
}
```

- [ ] **Step 2: Run, verify failure**

```bash
cd /Users/j/src/jorgejr568/organizze-mcp
go test ./internal/adapter/organizze/... -run TestJSONShape -v
```

Expected: failure with `read fixture user.json: open ... no such file or directory` — the testdata files don't exist yet.

- [ ] **Step 3: Create the fixture files**

These fixtures mirror the JSON shapes documented in https://github.com/organizze/api-doc and used by the existing repo tests. They are minimal but realistic — enough to populate every assertion above.

`internal/adapter/organizze/testdata/user.json`:

```json
{
  "id": 3,
  "name": "Jorge Junior",
  "email": "jorge@example.com",
  "role": "admin"
}
```

`internal/adapter/organizze/testdata/account.json`:

```json
{
  "id": 1,
  "name": "Itau Checking",
  "description": "Primary",
  "type": "checking",
  "default": true,
  "archived": false,
  "created_at": "2024-01-15T08:30:00Z",
  "updated_at": "2026-05-10T14:22:00Z"
}
```

`internal/adapter/organizze/testdata/category.json`:

```json
{
  "id": 10,
  "name": "Food",
  "color": "#ff0000",
  "parent_id": null
}
```

`internal/adapter/organizze/testdata/budget.json`:

```json
{
  "amount_in_cents": 50000,
  "category_id": 10,
  "date": "2026-05-01",
  "activity_type": 1,
  "total": 12000,
  "predicted_total": 30000,
  "percentage": "24"
}
```

`internal/adapter/organizze/testdata/credit_card.json`:

```json
{
  "id": 1,
  "name": "Nubank Platinum",
  "description": null,
  "card_network": "visa",
  "closing_day": 20,
  "due_day": 27,
  "limit_cents": 500000,
  "kind": "credit_card",
  "archived": false,
  "default": true,
  "created_at": "2023-06-01T00:00:00Z",
  "updated_at": "2026-04-20T09:15:00Z"
}
```

`internal/adapter/organizze/testdata/invoice.json`:

```json
{
  "id": 100,
  "date": "2026-05-27",
  "starting_date": "2026-04-21",
  "closing_date": "2026-05-20",
  "amount_cents": 120000,
  "payment_amount_cents": 0,
  "balance_cents": 120000,
  "previous_balance_cents": 0,
  "credit_card_id": 1,
  "transactions": [],
  "payments": []
}
```

`internal/adapter/organizze/testdata/transaction.json`:

```json
{
  "id": 555,
  "description": "Coffee at Octavio",
  "date": "2026-05-12",
  "paid": true,
  "amount_cents": -1500,
  "total_installments": 1,
  "installment": 1,
  "recurring": false,
  "account_id": 1,
  "account_type": "Account",
  "category_id": 10,
  "contact_id": null,
  "notes": "Receipt #4321",
  "attachments_count": 0,
  "credit_card_id": null,
  "credit_card_invoice_id": null,
  "paid_credit_card_id": null,
  "paid_credit_card_invoice_id": null,
  "oposite_transaction_id": null,
  "oposite_account_id": null,
  "recurrence_id": null,
  "tags": [{"name": "coffee"}, {"name": "weekday"}],
  "created_at": "2026-05-12T07:42:11Z",
  "updated_at": "2026-05-12T07:42:11Z"
}
```

`internal/adapter/organizze/testdata/transfer.json`:

```json
{
  "id": 800,
  "description": "Move savings",
  "date": "2026-05-01",
  "paid": true,
  "amount_cents": 250000,
  "account_id": 1,
  "oposite_account_id": 2,
  "oposite_transaction_id": 801,
  "category_id": 99,
  "notes": null,
  "recurrence_id": null
}
```

- [ ] **Step 4: Run tests, verify pass**

```bash
go test ./internal/adapter/organizze/... -run TestJSONShape -v
go test ./... -count=1
```

Expected: each of the 8 subtests PASS, full suite green.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/organizze/testdata/ internal/adapter/organizze/jsonshape_test.go
git commit -m "test(organizze): JSON-shape roundtrip fixtures for every domain type"
```

---

## Task 6: Link-time version injection

`mcp.Version` is currently `const Version = "0.1.0"` — a literal that doesn't move when you tag a release. Convert it to a build-time-injected `var` and update the three build paths (Makefile, Dockerfile, release.yml) to set it.

**Files:**
- Modify: `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/mcp/server.go`
- Modify: `/Users/j/src/jorgejr568/organizze-mcp/Makefile`
- Modify: `/Users/j/src/jorgejr568/organizze-mcp/Dockerfile`
- Modify: `/Users/j/src/jorgejr568/organizze-mcp/.github/workflows/release.yml`

- [ ] **Step 1: Convert `Version` from const to var**

In `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/mcp/server.go`, find:

```go
// Version is reported via the MCP Implementation block on handshake.
const Version = "0.1.0"
```

Replace with:

```go
// Version is reported via the MCP Implementation block on handshake.
//
// Set at link time via:
//
//	go build -ldflags="-X 'github.com/jorgejr568/organizze-mcp/internal/adapter/mcp.Version=<value>'"
//
// Defaults to "dev" for unstamped builds (go run, go test, IDE builds).
var Version = "dev"
```

- [ ] **Step 2: Run existing tests to confirm `Version` is still a `string` consumers see**

```bash
cd /Users/j/src/jorgejr568/organizze-mcp
go test ./... -count=1
```

Expected: every test PASS. (Nothing in the suite reads the literal value of `Version`; both `cmd/main.go` and any consumer just use the variable as a `string`.)

- [ ] **Step 3: Wire the Makefile**

In `/Users/j/src/jorgejr568/organizze-mcp/Makefile`, find:

```makefile
BINARY := organizze-mcp

build:
	go build -o bin/$(BINARY) ./cmd/organizze-mcp
```

Replace with:

```makefile
BINARY := organizze-mcp
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X 'github.com/jorgejr568/organizze-mcp/internal/adapter/mcp.Version=$(VERSION)'

build:
	go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) ./cmd/organizze-mcp
```

- [ ] **Step 4: Wire the Dockerfile**

In `/Users/j/src/jorgejr568/organizze-mcp/Dockerfile`, find the build-stage `RUN`:

```dockerfile
COPY . .

ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/organizze-mcp ./cmd/organizze-mcp
```

Replace with:

```dockerfile
COPY . .

ARG TARGETARCH
ARG VERSION=dev
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath \
      -ldflags="-s -w -X 'github.com/jorgejr568/organizze-mcp/internal/adapter/mcp.Version=${VERSION}'" \
      -o /out/organizze-mcp ./cmd/organizze-mcp
```

- [ ] **Step 5: Wire the release workflow**

In `/Users/j/src/jorgejr568/organizze-mcp/.github/workflows/release.yml`, find the `Build and push` step inside the `docker` job:

```yaml
      - name: Build and push
        uses: docker/build-push-action@v6
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

Insert a `Determine version` step *before* it (between the existing `Extract metadata` step and `Build and push`), then add a `build-args` block to `Build and push`. The resulting region should read:

```yaml
      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: jorgejr568/organizze-mcp
          tags: |
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=semver,pattern={{major}}
            type=raw,value=latest

      - name: Determine version
        id: version
        run: echo "value=${GITHUB_REF_NAME#v}" >> "$GITHUB_OUTPUT"

      - name: Build and push
        uses: docker/build-push-action@v6
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          build-args: |
            VERSION=${{ steps.version.outputs.value }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

`GITHUB_REF_NAME` is `v0.2.0` for a tag push; `${GITHUB_REF_NAME#v}` strips the leading `v`, yielding `0.2.0`, which matches Docker Hub's semver tag.

- [ ] **Step 6: Local build to confirm the stamped binary prints the right version**

```bash
cd /Users/j/src/jorgejr568/organizze-mcp
make build
strings bin/organizze-mcp | grep -E '^(dev|v?[0-9]+\.[0-9]+\.[0-9]+)' | head -3
```

Expected: at least one line that looks like a version (it'll be `dev` or a `git describe` output like `v0.1.0-3-gabcd123`). If you see `0.1.0` (the old literal), the change didn't land.

Also run the smoke flow to confirm the startup log shows the new value:

```bash
ORGANIZZE_API_KEY=x ORGANIZZE_EMAIL=x@x.com ORGANIZZE_USER_AGENT='Test (x@x.com)' \
  ORGANIZZE_BASE_URL=http://127.0.0.1:1 \
  timeout 1 ./bin/organizze-mcp </dev/null 2>&1 | head -1
```

Expected: stderr line like `organizze-mcp vv0.1.0-... starting on stdio` (or `dev` if you're not in a git checkout with tags). The version replaces the old hardcoded `0.1.0`.

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/mcp/server.go Makefile Dockerfile .github/workflows/release.yml
git commit -m "feat(build): inject mcp.Version via -ldflags from git tag"
```

---

## Task 7: Audit `Client.Inner()` — replace with narrower `Timeout()` accessor

`Client.Inner() *http.Client` is the broadest possible escape hatch and is used by exactly one test. Replace it with `Timeout() time.Duration`, which is the only thing that test actually needs. Callers who genuinely need transport customisation can pass their own `*http.Client` directly as `HTTPClient` (it satisfies the interface).

**Files:**
- Modify: `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/organizze/client.go`
- Modify: `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/organizze/client_test.go`

- [ ] **Step 1: Update the failing test first**

In `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/organizze/client_test.go`, find:

```go
func TestNewClient_AppliesDefaultTimeout(t *testing.T) {
	c := NewClient(ClientOptions{})
	if c.Inner().Timeout == 0 {
		t.Error("default timeout should be non-zero")
	}
}
```

Replace the function body with:

```go
func TestNewClient_AppliesDefaultTimeout(t *testing.T) {
	c := NewClient(ClientOptions{})
	if c.Timeout() != defaultTimeout {
		t.Errorf("Timeout() = %v, want defaultTimeout (%v)", c.Timeout(), defaultTimeout)
	}
}

func TestNewClient_HonorsCustomTimeout(t *testing.T) {
	c := NewClient(ClientOptions{Timeout: 7 * time.Second})
	if c.Timeout() != 7*time.Second {
		t.Errorf("Timeout() = %v, want 7s", c.Timeout())
	}
}
```

- [ ] **Step 2: Run, verify failure**

```bash
cd /Users/j/src/jorgejr568/organizze-mcp
go test ./internal/adapter/organizze/... -run TestNewClient
```

Expected: build failure — `c.Timeout undefined (type *Client has no field or method Timeout)`.

- [ ] **Step 3: Replace `Inner()` with `Timeout()` in client.go**

In `/Users/j/src/jorgejr568/organizze-mcp/internal/adapter/organizze/client.go`, find:

```go
// Inner exposes the underlying *http.Client for advanced callers that need to
// configure transports (proxies, TLS) directly. Most code should not use this.
func (c *Client) Inner() *http.Client {
	return c.inner
}
```

Replace with:

```go
// Timeout returns the per-request deadline this Client was configured with.
// Callers that need custom transports (proxies, TLS, retries) should construct
// their own *http.Client and pass it as the HTTPClient argument to
// NewRequestExecutor — Client is only the default-settings convenience.
func (c *Client) Timeout() time.Duration {
	return c.inner.Timeout
}
```

- [ ] **Step 4: Run all tests, verify pass**

```bash
go test ./internal/adapter/organizze/... -v
go test ./... -count=1
```

Expected: every test PASS, including the new `TestNewClient_HonorsCustomTimeout`. No callsites of the removed `Inner()` method exist outside the now-updated test (verify with `grep -rn '\.Inner()' /Users/j/src/jorgejr568/organizze-mcp` — should return nothing).

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/organizze/client.go internal/adapter/organizze/client_test.go
git commit -m "refactor(organizze): replace Client.Inner with narrower Timeout accessor"
```

---

## Task 8: CI/release workflow refresh — Node-24 action versions + verify expanded labels

GitHub Actions deprecated Node 20 on 2025-09-19; from 2026-06-02 it becomes the default runtime even for actions still pinned to v3/v4 that internally use Node 20. Bump every action to its Node-24-native major version. Also confirm `metadata-action`'s default label set now includes `image.licenses` (which works because Task 1 added a `LICENSE` file).

**Files:**
- Modify: `/Users/j/src/jorgejr568/organizze-mcp/.github/workflows/ci.yml`
- Modify: `/Users/j/src/jorgejr568/organizze-mcp/.github/workflows/release.yml`

- [ ] **Step 1: Discover current Node-24-native major versions**

The "right" versions move; resolve them at execution time:

```bash
for repo in actions/checkout actions/setup-go actions/upload-artifact \
           docker/setup-qemu-action docker/setup-buildx-action \
           docker/login-action docker/metadata-action docker/build-push-action; do
  latest=$(gh api "repos/$repo/releases/latest" --jq '.tag_name' 2>/dev/null || echo "?")
  echo "$repo -> $latest"
done
```

Expected output (representative — actual values at execution time may differ):

```
actions/checkout         -> v5
actions/setup-go         -> v6
actions/upload-artifact  -> v5
docker/setup-qemu-action -> v3
docker/setup-buildx-action -> v3
docker/login-action      -> v3
docker/metadata-action   -> v5
docker/build-push-action -> v6
```

Record the latest major version for each.

- [ ] **Step 2: Update `.github/workflows/ci.yml`**

In `/Users/j/src/jorgejr568/organizze-mcp/.github/workflows/ci.yml`, update the action versions discovered in Step 1. The current file pins `actions/checkout@v4`, `actions/setup-go@v5`, `actions/upload-artifact@v4`. Bump each to the latest major from Step 1's output — most likely:

- `actions/checkout@v4` → `actions/checkout@v5`
- `actions/setup-go@v5` → `actions/setup-go@v6`
- `actions/upload-artifact@v4` → `actions/upload-artifact@v5`

> **If `gh api` shows a different latest major** for any action, use that instead. Don't blind-bump; pin to what's actually released. If the latest major is incompatible with the action's documented usage in the file, stay on the previous major and add `env: { FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: "true" }` at the workflow level as the fallback.

- [ ] **Step 3: Update `.github/workflows/release.yml`**

Same bumping pass on `release.yml`. Most likely:

- `actions/checkout@v4` → `actions/checkout@v5` (used twice — once in `test` job, once in `docker` job)
- `actions/setup-go@v5` → `actions/setup-go@v6`
- `docker/setup-qemu-action@v3` → unchanged unless Step 1 shows a higher major
- `docker/setup-buildx-action@v3` → unchanged unless Step 1 shows a higher major
- `docker/login-action@v3` → unchanged unless Step 1 shows a higher major
- `docker/metadata-action@v5` → unchanged unless Step 1 shows a higher major
- `docker/build-push-action@v6` → unchanged unless Step 1 shows a higher major

- [ ] **Step 4: (No new fixture file needed for label coverage)**

`docker/metadata-action` automatically emits `org.opencontainers.image.licenses` whenever the repo has a `LICENSE` file detectable by the GitHub API — Task 1 already added it. No workflow change is required to surface it; the `labels: ${{ steps.meta.outputs.labels }}` line already in `release.yml`'s `Build and push` step picks it up.

To verify after the next release, run:

```bash
docker buildx imagetools inspect jorgejr568/organizze-mcp:latest --raw | jq '.manifests[0].annotations'
```

Expected (after Task 9 ships): includes `"org.opencontainers.image.licenses": "MIT"`, `"org.opencontainers.image.revision": "<commit-sha>"`, `"org.opencontainers.image.created": "<iso8601>"`, `"org.opencontainers.image.version": "0.2.0"`.

- [ ] **Step 5: Lint workflows locally (best-effort)**

`act` and `actionlint` are optional dependencies. If you have either, run it; otherwise rely on the post-push CI run as the validator:

```bash
which actionlint && actionlint .github/workflows/ci.yml .github/workflows/release.yml || true
```

Expected: no output (no issues) if `actionlint` is installed; otherwise the command is a no-op.

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/ci.yml .github/workflows/release.yml
git commit -m "ci: bump GitHub Actions to Node-24-native major versions"
```

---

## Task 9: Tag and ship v0.2.0

Open the PR for the feature branch, watch CI go green, merge to `main`, then tag `v0.2.0` and verify the release flow.

This task is sequential operations against the GitHub API; no new files.

- [ ] **Step 1: Push the feature branch and open the PR**

```bash
cd /Users/j/src/jorgejr568/organizze-mcp
git push -u origin chore/v0.2.0-followups
gh pr create --base main \
  --title "release: v0.2.0 follow-ups (docs, error semantics, link-time version, workflow refresh)" \
  --body "$(cat <<'PRBODY'
Addresses every "Soon" and "Eventually" item from the v0.1.0 final review.

## Changes

- **Docs:** add `LICENSE` (MIT), `CHANGELOG.md`, `SECURITY.md`, `CONTRIBUTING.md`.
- **README:** flatten tool catalogue to 16 numbered rows so the count is verifiable at a glance; link the new LICENSE.
- **In-code docs:** spell out the Organizze partial-update assumption on `UpdateTransactionParams`; surface unsafe-retry / no-soft-delete semantics in the `update_transaction` and `delete_transaction` tool descriptions.
- **Error semantics:** new `domain.ErrRateLimited` sentinel; HTTP 429 now maps to it through `APIError.Is`.
- **Tests:** new `internal/adapter/organizze/jsonshape_test.go` decodes captured fixtures into every `domain.*` type and asserts core fields populate — catches silent field drops from upstream renames.
- **Build:** `mcp.Version` is now a `var` populated via `-ldflags="-X ..."` from the git tag. Wired through the Makefile, Dockerfile (`ARG VERSION`), and the release workflow (`build-args`).
- **API surface:** `Client.Inner() *http.Client` replaced with `Client.Timeout() time.Duration` — only the single test ever used it, and the narrower API matches what the test actually needed.
- **CI/CD:** bump every action to its Node-24-native major version so the workflows survive the June 2026 deprecation. `metadata-action` now emits `image.licenses` because Task 1 added a LICENSE.

## Verification

- `go test ./... -count=1` green locally.
- `make build` produces a binary whose stderr startup log shows the version from `git describe`.
- The CI `Test` check on this PR must be green before merge (branch protection enforces it).

## Release flow

Once merged to `main`:

\`\`\`
git tag -a v0.2.0 -m "v0.2.0 — followups"
git push origin v0.2.0
\`\`\`

The release workflow builds the multi-arch image with `VERSION=0.2.0` injected and publishes to Docker Hub.
PRBODY
)"
```

- [ ] **Step 2: Wait for CI**

```bash
gh pr checks --watch
```

Expected: the `Test` job concludes `success` in ~1–2 minutes. If it fails, inspect with `gh run view --log`.

- [ ] **Step 3: Squash-merge**

```bash
gh pr merge --squash --delete-branch
```

Expected: PR closes, feature branch is deleted both remotely and locally. `main` advances by one commit.

- [ ] **Step 4: Pull, tag, and push the v0.2.0 release tag**

```bash
git checkout main
git pull --ff-only origin main
git tag -a v0.2.0 -m "v0.2.0 — followups (docs, errors, build, workflows)"
git push origin v0.2.0
```

Expected: the tag appears on GitHub; the `Release` workflow starts within a few seconds.

- [ ] **Step 5: Watch the release workflow**

```bash
gh run watch
```

Expected: `Test` (~30s) passes, `Build and push Docker image` (~4–8 min via multi-arch QEMU) succeeds. Both jobs end green.

- [ ] **Step 6: Verify the published image carries the new version + labels**

```bash
docker pull jorgejr568/organizze-mcp:0.2.0
docker pull jorgejr568/organizze-mcp:latest
docker buildx imagetools inspect jorgejr568/organizze-mcp:0.2.0 --raw \
  | jq '.manifests[] | select(.platform.architecture != null) | {arch: .platform.architecture, labels: .annotations}'
```

Expected:
- Pulls succeed.
- Each platform (`amd64`, `arm64`) carries annotations including `"org.opencontainers.image.licenses": "MIT"`, `"org.opencontainers.image.revision": "<sha>"`, `"org.opencontainers.image.created": "<iso8601>"`, `"org.opencontainers.image.version": "0.2.0"`.
- Running the image briefly should log `organizze-mcp v0.2.0 starting on stdio` (because the binary's `Version` was stamped from `${VERSION}` build arg):

```bash
docker run --rm -i \
  -e ORGANIZZE_API_KEY=x -e ORGANIZZE_EMAIL=x@x.com \
  -e "ORGANIZZE_USER_AGENT=Test (x@x.com)" \
  -e ORGANIZZE_BASE_URL=http://127.0.0.1:1 \
  jorgejr568/organizze-mcp:0.2.0 </dev/null 2>&1 | head -1
```

Expected: `organizze-mcp v0.2.0 starting on stdio`. If you see `dev`, the `VERSION` build-arg didn't propagate — re-check Task 6 Step 5.

---

## Self-Review Notes

**Spec coverage:**
- [x] Version injection at link time → Task 6.
- [x] LICENSE file → Task 1 Step 2.
- [x] README "16 tools" count clarity → Task 2.
- [x] CHANGELOG → Task 1 Step 3.
- [x] SECURITY → Task 1 Step 4.
- [x] CONTRIBUTING → Task 1 Step 5.
- [x] Rate-limit (429) handling with `domain.ErrRateLimited` → Task 4.
- [x] JSON roundtrip tests against real Organizze response shapes → Task 5.
- [x] Document Update partial-body semantics in code → Task 3 Step 1.
- [x] Tool descriptions error-semantics hints → Task 3 Steps 2–3.
- [x] `Client.Inner()` audit → Task 7 (decision: replace with narrower `Timeout()`).
- [x] GitHub Actions Node-20 deprecation refresh → Task 8.
- [x] Docker image labels expansion (`revision`, `created`, `licenses`) → Task 8 Step 4 (passive: `metadata-action` emits them automatically once `LICENSE` exists, which Task 1 provides).

**Type consistency:**
- `domain.ErrRateLimited` is used in both `domain/errors.go` (declaration), `domain/errors_test.go` (distinctness test), `adapter/organizze/errors.go` (status mapping), and `adapter/organizze/errors_test.go` (mapping test). Naming and `errors.New("domain: rate limited")` text are consistent across.
- `mcp.Version` is the same identifier (`var Version string`) in all four touch points (declaration, Makefile ldflag, Dockerfile ldflag, release-workflow build-arg).
- `Client.Timeout()` replaces `Client.Inner()` everywhere — verified by the `grep -rn '\.Inner()'` step in Task 7. No stale callsite.

**Placeholder scan:** every step has the actual content to apply — no "TBD", no "implement appropriately", no `// ...` in code blocks. Step 1 of Task 8 has a templated `gh api` call whose output drives Step 2–3, but the exact substitutions are spelled out and the fallback path (env var + stay-on-major) is documented inline.

**Tests added/strengthened:**
- `TestErrRateLimited_IsDistinctSentinel` (domain): proves the new sentinel doesn't collide.
- `TestAPIError_429MapsToRateLimited` + new row in `TestAPIError_MapsToDomainSentinels` (adapter): wire mapping.
- `TestJSONShape_DomainTypesDecodeRealisticFixtures` (adapter): 8 subtests, one per `domain.*` type.
- `TestNewClient_AppliesDefaultTimeout` updated to use `Timeout()`; `TestNewClient_HonorsCustomTimeout` added for the custom-value branch.
