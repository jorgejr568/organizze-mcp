# Organizze MCP — OpenAPI Parity Pass 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the remaining gap between our domain/MCP types and `openapi.yaml` for transactions, and document the surprising installment-amount semantics on the `create_transaction` tool so callers stop being burned by it.

**Architecture:** Pure additive field passes — no behaviour change on the wire. We add optional pointer fields with `omitempty` so absent values stay absent on the JSON body; we extend the response struct with an attachments slice; we clarify one tool description; we add tests at every layer that exercises these fields.

**Tech Stack:** Go, `encoding/json`, `net/http/httptest`, the existing `mcpsdk` and `domain.ErrValidation` machinery.

---

## OpenAPI Gap Audit (source of truth: `openapi.yaml`)

| Spec location | Field | Status in code |
|---|---|---|
| `Transaction.attachments` (response, lines 683–689, 718) | `attachments []string` | **missing** from `domain.Transaction` |
| `TransactionInput.credit_card_id` (lines 759–764) | `credit_card_id *int32 nullable` | **missing** from `domain.CreateTransactionParams` |
| `TransactionInput.credit_card_invoice_id` (lines 765–770) | `credit_card_invoice_id *int32 nullable` | **missing** from `domain.CreateTransactionParams` |
| `UpdateTransaction.credit_card_id` (lines 1561–1566) | `credit_card_id *int32 nullable` | **missing** from `domain.UpdateTransactionParams` |
| Installment `amount_cents` semantics | (no spec hint; empirically: total across installments) | **undocumented** on `create_transaction` tool description |

Out of scope (already correct):
- `account_type` on response — we keep it; spec is silent but the field is returned by the live API.
- `attachments_count` — already present (`AttachmentsCount int`).
- `recurrence_id`, `recurring`, installment indices — already present as read-only.

## File Structure

| File | Responsibility | Action |
|---|---|---|
| `internal/domain/transaction.go` | Domain types + JSON tags for the Organizze wire shape | Modify: add 4 fields |
| `internal/usecase/transaction.go` | Service validation | No change (new fields are optional pointers; no validation needed) |
| `internal/adapter/organizze/transaction_repository.go` | HTTP layer | No change (params are forwarded verbatim) |
| `internal/adapter/organizze/transaction_repository_test.go` | Wire-shape tests | Modify: assert new fields land in/are absent from JSON body correctly |
| `internal/adapter/organizze/testdata/transaction.json` | JSON-shape fixture | Modify: add an `attachments` array so the decode roundtrip exercises it |
| `internal/adapter/organizze/jsonshape_test.go` | Fixture decode assertions | Modify: assert `attachments` decodes |
| `internal/adapter/mcp/tools_transactions.go` | MCP tool inputs + descriptions | Modify: add 3 input fields, plumb into params, clarify installment description |
| `internal/adapter/mcp/tools_transactions_test.go` | MCP handler tests | Modify: cover new fields |
| `internal/usecase/transaction_test.go` | Service unit tests | Modify: pass new fields through |
| `CHANGELOG.md` | Release notes | Add `[Unreleased]` entry |

## Task Decomposition

Each task is a self-contained TDD slice: failing test → minimal implementation → green → commit.

---

### Task 1: `Transaction.Attachments` response field

**Files:**
- Modify: `internal/domain/transaction.go` (add field on `Transaction` struct, ~line 34)
- Modify: `internal/adapter/organizze/testdata/transaction.json` (add `"attachments": ["https://example.com/x.pdf"]` if the fixture exists; otherwise extend `jsonshape_test.go`'s inline fixture)
- Modify: `internal/adapter/organizze/jsonshape_test.go` (assert decoded `Attachments` is non-empty)

- [ ] **Step 1: Inspect the existing fixture/test to know which lever to pull**

Run: `ls internal/adapter/organizze/testdata/ 2>/dev/null && grep -n "Transaction\|attachments" internal/adapter/organizze/jsonshape_test.go`

Two possibilities — branch on what the listing shows:

- If `testdata/transaction.json` exists: edit that fixture (Step 2a).
- If the fixture is inline in `jsonshape_test.go`: edit the inline string (Step 2b).

- [ ] **Step 2a: Add `attachments` to the JSON fixture**

In `internal/adapter/organizze/testdata/transaction.json`, add the top-level key before the closing brace:

```json
  "attachments": ["https://example.com/receipt.pdf"]
```

- [ ] **Step 2b: Alternative — inline fixture in `jsonshape_test.go`**

Locate the Transaction JSON literal and append `,"attachments":["https://example.com/receipt.pdf"]` inside the object.

- [ ] **Step 3: Extend the shape assertion**

In `internal/adapter/organizze/jsonshape_test.go`, find the Transaction roundtrip test and add an assertion:

```go
if len(got.Attachments) != 1 || got.Attachments[0] != "https://example.com/receipt.pdf" {
    t.Errorf("attachments = %v, want [https://example.com/receipt.pdf]", got.Attachments)
}
```

- [ ] **Step 4: Run the test — must FAIL with "Attachments undefined"**

Run: `go test ./internal/adapter/organizze/... -run Transaction -v`
Expected: compile error: `got.Attachments undefined`.

- [ ] **Step 5: Add the field to the domain type**

In `internal/domain/transaction.go`, inside the `Transaction` struct, add (place near `AttachmentsCount` for cohesion):

```go
	Attachments             []string `json:"attachments,omitempty"`
```

- [ ] **Step 6: Run the test — must PASS**

Run: `go test ./internal/adapter/organizze/... -run Transaction -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/domain/transaction.go internal/adapter/organizze/jsonshape_test.go internal/adapter/organizze/testdata/transaction.json
git commit -m "feat(domain): Transaction.Attachments mirrors openapi.yaml attachments[]"
```

---

### Task 2: `CreateTransactionParams.CreditCardID` + `CreditCardInvoiceID`

**Files:**
- Modify: `internal/adapter/organizze/transaction_repository_test.go` (new test for wire shape)
- Modify: `internal/domain/transaction.go` (`CreateTransactionParams`)

- [ ] **Step 1: Write the failing wire-shape test**

Append to `internal/adapter/organizze/transaction_repository_test.go` (after the existing `TestTransactionRepository_Create_OmitsRecurrenceWhenNil`):

```go
func TestTransactionRepository_Create_IncludesCreditCardFields(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":1}`)
	})
	repo := NewTransactionRepository(exec)
	cardID := int64(1287765)
	invoiceID := int64(276)
	_, err := repo.Create(context.Background(), domain.CreateTransactionParams{
		Description: "Coffee", Date: "2026-05-14", AmountCents: -1500,
		AccountID: 1, CategoryID: 10,
		CreditCardID:        &cardID,
		CreditCardInvoiceID: &invoiceID,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if raw["credit_card_id"] != float64(1287765) {
		t.Errorf("credit_card_id = %v, want 1287765", raw["credit_card_id"])
	}
	if raw["credit_card_invoice_id"] != float64(276) {
		t.Errorf("credit_card_invoice_id = %v, want 276", raw["credit_card_invoice_id"])
	}
}

func TestTransactionRepository_Create_OmitsCreditCardFieldsWhenNil(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":1}`)
	})
	repo := NewTransactionRepository(exec)
	_, err := repo.Create(context.Background(), domain.CreateTransactionParams{
		Description: "Coffee", Date: "2026-05-14", AmountCents: -1500,
		AccountID: 1, CategoryID: 10,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	for _, k := range []string{"credit_card_id", "credit_card_invoice_id"} {
		if _, has := raw[k]; has {
			t.Errorf("%s must be omitted when nil; body=%v", k, raw)
		}
	}
}
```

- [ ] **Step 2: Run the tests — must FAIL with "unknown field"**

Run: `go test ./internal/adapter/organizze/... -run CreditCard -v`
Expected: compile error: `unknown field CreditCardID in struct literal`.

- [ ] **Step 3: Add fields to the domain type**

In `internal/domain/transaction.go`, extend `CreateTransactionParams` (after `Tags`, before `Recurrence`):

```go
	CreditCardID        *int64 `json:"credit_card_id,omitempty"`
	CreditCardInvoiceID *int64 `json:"credit_card_invoice_id,omitempty"`
```

The full struct should now have credit-card fields slotted between `Tags` and `Recurrence` so JSON tag groupings stay readable.

- [ ] **Step 4: Run the tests — must PASS**

Run: `go test ./internal/adapter/organizze/... -run CreditCard -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/transaction.go internal/adapter/organizze/transaction_repository_test.go
git commit -m "feat(domain): CreateTransactionParams gains credit_card_id + credit_card_invoice_id"
```

---

### Task 3: `UpdateTransactionParams.CreditCardID`

**Files:**
- Modify: `internal/adapter/organizze/transaction_repository_test.go` (extend update test)
- Modify: `internal/domain/transaction.go` (`UpdateTransactionParams`)

- [ ] **Step 1: Write the failing test**

Append to `internal/adapter/organizze/transaction_repository_test.go`:

```go
func TestTransactionRepository_Update_IncludesCreditCardID(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		_, _ = io.WriteString(w, `{"id":777}`)
	})
	repo := NewTransactionRepository(exec)
	cardID := int64(1287765)
	_, err := repo.Update(context.Background(), 777, domain.UpdateTransactionParams{
		CreditCardID: &cardID,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if raw["credit_card_id"] != float64(1287765) {
		t.Errorf("credit_card_id = %v, want 1287765", raw["credit_card_id"])
	}
}
```

- [ ] **Step 2: Run — must FAIL with "unknown field"**

Run: `go test ./internal/adapter/organizze/... -run Update_IncludesCreditCardID -v`
Expected: compile error.

- [ ] **Step 3: Add the field**

In `internal/domain/transaction.go`, extend `UpdateTransactionParams` (after `Tags`, before `UpdateFuture`):

```go
	CreditCardID *int64  `json:"credit_card_id,omitempty"`
```

- [ ] **Step 4: Run — must PASS**

Run: `go test ./internal/adapter/organizze/... -run Update_IncludesCreditCardID -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/transaction.go internal/adapter/organizze/transaction_repository_test.go
git commit -m "feat(domain): UpdateTransactionParams gains credit_card_id"
```

---

### Task 4: MCP `create_transaction` exposes credit-card fields

**Files:**
- Modify: `internal/adapter/mcp/tools_transactions.go` (input struct + handler plumbing)
- Modify: `internal/adapter/mcp/tools_transactions_test.go` (handler test)

- [ ] **Step 1: Write the failing handler test**

Add to `internal/adapter/mcp/tools_transactions_test.go` (alongside the other `TestCreateTransactionHandler_*`):

```go
func TestCreateTransactionHandler_PlumbsCreditCardFields(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := createTransactionHandler(svc)
	cardID := int64(1287765)
	invoiceID := int64(276)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateTransactionInput{
		Description: "Coffee", Date: "2026-05-14", AmountCents: -1500,
		AccountID: 1, CategoryID: 10,
		CreditCardID:        &cardID,
		CreditCardInvoiceID: &invoiceID,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if svc.created.CreditCardID == nil || *svc.created.CreditCardID != 1287765 {
		t.Errorf("CreditCardID = %v, want 1287765", svc.created.CreditCardID)
	}
	if svc.created.CreditCardInvoiceID == nil || *svc.created.CreditCardInvoiceID != 276 {
		t.Errorf("CreditCardInvoiceID = %v, want 276", svc.created.CreditCardInvoiceID)
	}
}
```

- [ ] **Step 2: Run — must FAIL with "unknown field"**

Run: `go test ./internal/adapter/mcp/... -run PlumbsCreditCardFields -v`
Expected: compile error.

- [ ] **Step 3: Add input fields**

In `internal/adapter/mcp/tools_transactions.go`, extend `CreateTransactionInput` (after `Tags`, before `Recurrence`):

```go
	CreditCardID        *int64 `json:"credit_card_id,omitempty"         jsonschema:"Optional. Bill this transaction to a credit card by ID. Mutually exclusive with installments=null only when the card requires a specific invoice; otherwise pair with credit_card_invoice_id to target a specific invoice."`
	CreditCardInvoiceID *int64 `json:"credit_card_invoice_id,omitempty" jsonschema:"Optional. Pin this transaction to a specific credit-card invoice. Only meaningful together with credit_card_id."`
```

- [ ] **Step 4: Plumb into params in `createTransactionHandler`**

Locate `createTransactionHandler` and update the `domain.CreateTransactionParams` literal to include the two new fields:

```go
		params := domain.CreateTransactionParams{
			Description: in.Description, Date: in.Date, AmountCents: in.AmountCents,
			AccountID: in.AccountID, CategoryID: in.CategoryID, Paid: in.Paid,
			Notes: in.Notes, ContactID: in.ContactID, Tags: in.Tags,
			CreditCardID:        in.CreditCardID,
			CreditCardInvoiceID: in.CreditCardInvoiceID,
		}
```

(Keep the existing `if in.Recurrence != nil` / `if in.Installments != nil` blocks below this literal untouched.)

- [ ] **Step 5: Run — must PASS**

Run: `go test ./internal/adapter/mcp/... -run PlumbsCreditCardFields -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/mcp/tools_transactions.go internal/adapter/mcp/tools_transactions_test.go
git commit -m "feat(mcp): create_transaction exposes credit_card_id and credit_card_invoice_id"
```

---

### Task 5: MCP `update_transaction` exposes credit-card field

**Files:**
- Modify: `internal/adapter/mcp/tools_transactions.go`
- Modify: `internal/adapter/mcp/tools_transactions_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestUpdateTransactionHandler_PlumbsCreditCardID(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := updateTransactionHandler(svc)
	cardID := int64(1287765)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, UpdateTransactionInput{
		ID: 777, CreditCardID: &cardID,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if svc.updated.params.CreditCardID == nil || *svc.updated.params.CreditCardID != 1287765 {
		t.Errorf("params.CreditCardID = %v", svc.updated.params.CreditCardID)
	}
}
```

- [ ] **Step 2: Run — must FAIL with "unknown field"**

Run: `go test ./internal/adapter/mcp/... -run UpdateTransactionHandler_PlumbsCreditCardID -v`
Expected: compile error.

- [ ] **Step 3: Add input field**

In `internal/adapter/mcp/tools_transactions.go`, extend `UpdateTransactionInput` (after `Tags`, before `UpdateFuture`):

```go
	CreditCardID *int64       `json:"credit_card_id,omitempty"  jsonschema:"New credit-card id; pass null to leave unchanged. Pass an explicit value to move the transaction to a card."`
```

- [ ] **Step 4: Plumb into `updateTransactionHandler`'s `domain.UpdateTransactionParams` literal**

```go
		params := domain.UpdateTransactionParams{
			Description: in.Description, Date: in.Date, AmountCents: in.AmountCents,
			AccountID: in.AccountID, CategoryID: in.CategoryID, Paid: in.Paid,
			Notes: in.Notes, ContactID: in.ContactID, Tags: in.Tags,
			CreditCardID: in.CreditCardID,
			UpdateFuture: in.UpdateFuture,
			UpdateAll:    in.UpdateAll,
		}
```

- [ ] **Step 5: Run — must PASS**

Run: `go test ./internal/adapter/mcp/... -run UpdateTransactionHandler_PlumbsCreditCardID -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/mcp/tools_transactions.go internal/adapter/mcp/tools_transactions_test.go
git commit -m "feat(mcp): update_transaction exposes credit_card_id"
```

---

### Task 6: Clarify installment `amount_cents` semantics on the tool description

**Background:** The user empirically discovered Organizze treats `amount_cents` as the *total* across all installments when `installments_attributes` is set, then divides evenly. The OpenAPI does not document this. We do not auto-multiply (that would diverge from the wire); we make it impossible to miss in the description.

**Files:**
- Modify: `internal/adapter/mcp/tools_transactions.go` (description string on the `create_transaction` tool registration)

- [ ] **Step 1: Locate the existing description**

Find the `mcpsdk.AddTool` call for `create_transaction` in `registerTransactionTools` (search for `Name:        "create_transaction"`).

- [ ] **Step 2: Replace the `Description` string**

Replace the existing description with:

```go
		Description: "Create a new Organizze transaction. amount_cents is in cents (negative for expenses, positive for income). " +
			"For a fixed recurring transaction, pass `recurrence` with a `periodicity` (weekly, biweekly, monthly, bimonthly, trimonthly, yearly). " +
			"For a parcelada (installment) transaction, pass `installments` with `periodicity` and `total`; IMPORTANT: when `installments` is set, Organizze treats `amount_cents` as the TOTAL across all installments and divides evenly, so each generated installment will be amount_cents/total. To get per-installment value X with N installments, send amount_cents = X * N. " +
			"`recurrence` and `installments` are mutually exclusive. Bill to a credit card by setting `credit_card_id` (optionally pinned to an invoice via `credit_card_invoice_id`).",
```

- [ ] **Step 3: Verify the build**

Run: `go build ./...`
Expected: no output (clean build).

- [ ] **Step 4: Verify the full suite is still green**

Run: `make test`
Expected: all packages PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/mcp/tools_transactions.go
git commit -m "docs(mcp): create_transaction description documents installment amount=total semantics"
```

---

### Task 7: Usecase + MCP integration coverage

Confirm nothing in the service or integration layer broke. No new code needed — just make sure the fake services in tests don't need updating.

- [ ] **Step 1: Run the full suite**

Run: `make test`
Expected: PASS across all packages.

- [ ] **Step 2: Run lint**

Run: `make lint`
Expected: clean (no `go vet` complaints).

- [ ] **Step 3: Run the build**

Run: `make build`
Expected: binary at `bin/organizze-mcp`.

- [ ] **Step 4: No commit needed if all green**

If any check failed, stop and surface the failure — do not paper over it.

---

### Task 8: CHANGELOG

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Add an `[Unreleased]` entry**

Insert under the existing `## [Unreleased]` header:

```markdown
### Added
- `create_transaction` and `update_transaction` now accept `credit_card_id` (and `create_transaction` also accepts `credit_card_invoice_id`), closing the gap with the OpenAPI `TransactionInput` / `UpdateTransaction` schemas — letting callers create or move transactions on a specific credit card / invoice.
- `Transaction` response now includes `attachments` (array of attachment URLs), mirroring `openapi.yaml`'s `Transaction.attachments`.

### Changed
- `create_transaction` tool description now explicitly documents the Organizze installment-amount semantics: when `installments` is set, `amount_cents` is the TOTAL across all installments (Organizze divides evenly). To get per-installment value `X` with `N` installments, send `amount_cents = X * N`.
```

- [ ] **Step 2: Verify it parses (no markdown lint expected, just eyeball)**

Run: `head -20 CHANGELOG.md`
Expected: the new entry under `## [Unreleased]`.

- [ ] **Step 3: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs(changelog): credit-card fields + installment amount clarification"
```

---

### Task 9: Open PR

- [ ] **Step 1: Push the branch**

```bash
git push -u origin <feature-branch-name>
```

- [ ] **Step 2: Open the PR**

```bash
gh pr create --title "feat: OpenAPI parity pass — credit-card fields + attachments + installment docs" --body "$(cat <<'EOF'
## Summary
- Adds `credit_card_id` to `create_transaction` and `update_transaction`, and `credit_card_invoice_id` to `create_transaction`, matching `openapi.yaml`'s `TransactionInput` / `UpdateTransaction`.
- Adds `attachments []string` to the `Transaction` response, mirroring `openapi.yaml`'s `Transaction.attachments`.
- Clarifies on the `create_transaction` tool description that, for installment plans, Organizze treats `amount_cents` as the *total* across all installments and divides evenly (the surprise that prompted this PR).

## Why
Audit against `openapi.yaml` found three required-by-spec fields not exposed by the MCP. The installment `amount_cents` semantics were also undocumented and caused a real user surprise (sent R$165.80 with total=2, got two R$82.90 installments).

## Test Plan
- [x] `make test` — full suite green; new tests cover wire shape and handler plumbing for every new field.
- [x] `make lint`
- [x] `make build`

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

- [ ] **Step 3: Wait for CI, then squash-merge with `gh pr merge <N> --squash --delete-branch`**

---

### Task 10: Release v0.5.0

This is a minor version bump (additive, no breaking change).

- [ ] **Step 1: Open a release-notes PR**

After the feature PR merges, branch off latest `main`:

```bash
git checkout main && git pull
git checkout -b chore/release-v0.5.0
```

Edit `CHANGELOG.md`: change `## [Unreleased]` to `## [Unreleased]` followed by a new `## [0.5.0] - 2026-05-14` section holding the entries just added.

- [ ] **Step 2: Commit, push, PR, merge**

```bash
git add CHANGELOG.md
git commit -m "docs(changelog): release v0.5.0"
git push -u origin chore/release-v0.5.0
gh pr create --base main --title "release: v0.5.0 (changelog header)" --body "Promotes the [Unreleased] entries from #<PR-N> into the [0.5.0] section."
# wait for CI; merge with --squash --delete-branch
```

- [ ] **Step 3: Tag and push**

```bash
git checkout main && git pull
git tag -a v0.5.0 -m "v0.5.0"
git push origin v0.5.0
```

This triggers the `release.yml` workflow → Docker multi-arch build.

- [ ] **Step 4: Create the GitHub release**

```bash
gh release create v0.5.0 --title "v0.5.0" --notes "$(cat <<'EOF'
## Added
- `create_transaction` and `update_transaction` accept `credit_card_id`; `create_transaction` also accepts `credit_card_invoice_id`.
- `Transaction` response now includes `attachments` (URLs).

## Changed
- `create_transaction` description documents the Organizze installment-amount rule: `amount_cents` is the TOTAL across all installments when `installments` is set.

## Docker
```
docker pull jorgejr568/organizze-mcp:0.5.0
docker pull jorgejr568/organizze-mcp:0.5
docker pull jorgejr568/organizze-mcp:latest
```

**Full changelog:** https://github.com/jorgejr568/organizze-mcp/compare/v0.4.0...v0.5.0
EOF
)"
```

---

## Self-Review

**Spec coverage:**
- `Transaction.attachments` → Task 1 ✓
- `TransactionInput.credit_card_id` → Task 2 + Task 4 ✓
- `TransactionInput.credit_card_invoice_id` → Task 2 + Task 4 ✓
- `UpdateTransaction.credit_card_id` → Task 3 + Task 5 ✓
- Installment amount surprise → Task 6 (docs) + Task 8 (changelog) ✓

**Placeholder scan:** No "TBD"/"implement later"; every code step shows actual Go code; every commit step shows the exact `git` invocation. Step 1 of Task 1 branches on filesystem state because the fixture's existence isn't predictable from this plan's vantage — both branches contain the full code, so the executor can run whichever applies.

**Type consistency:** Field names — `CreditCardID *int64`, `CreditCardInvoiceID *int64`, `Attachments []string` — match between the domain struct, the MCP input struct, and the tests across every task. JSON tags consistently use `omitempty` for the new optional fields.
