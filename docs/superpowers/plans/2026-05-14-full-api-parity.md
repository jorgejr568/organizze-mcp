# Organizze MCP — Full API Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close every gap between the MCP server and `ORGANIZZE_API.md`. After this plan ships, every endpoint, every documented body field, every documented filter, and every documented response field is reachable through MCP tools. v0.4.0 added `recurrence_attributes`; this plan finishes the job (`installments_attributes`, `update_future`/`update_all`, missing endpoints, missing fields on update bodies, broken `replacement_id` transport, missing invoice payment endpoint, expanded Transfer domain model).

**Architecture:** No new layers, no new packages. The existing 3-layer shape (`internal/domain/` → `internal/usecase/` → `internal/adapter/{organizze,mcp}/`) is preserved. The only cross-cutting change is a single capability added to `RequestExecutor` (DELETE-with-body), which unblocks two correctness fixes (`delete_category` replacement, `delete_transaction` recurring flags).

**Tech Stack:** Unchanged. Go ≥ 1.23, stdlib `net/http`/`encoding/json`/`testing`/`net/http/httptest`, `github.com/modelcontextprotocol/go-sdk`.

---

## Audit summary — all gaps tackled by this plan

| # | Resource | Gap | Severity |
|---|---|---|---|
| 1 | (foundation) | `RequestExecutor.Delete` cannot send a request body — blocks fixes for Categories and Transactions | High |
| 2 | Categories | `replacement_id` is sent as a query string, but `ORGANIZZE_API.md:481-489` documents it as JSON body. Replacement is **silently ignored** by Organizze. | **High (functional bug)** |
| 3 | Categories | `delete_category` discards the deleted-category payload the API returns | Low |
| 4 | Accounts | Cannot archive/unarchive an account — `archived` missing on UpdateAccountParams | Medium |
| 5 | Accounts | `delete_account` discards the deleted-account payload | Low |
| 6 | Credit Cards | `update_credit_card` cannot change `limit_cents`, `card_network`, `archived`, `default` | Medium |
| 7 | Credit Cards | `delete_credit_card` discards the deleted-card payload | Low |
| 8 | Invoices | `list_credit_card_invoices` lacks `start_date`/`end_date` — clients capped to current year per `ORGANIZZE_API.md:134` | **High (unreachable data)** |
| 9 | Invoices | `GET /credit_cards/:id/invoices/:invoice_id/payments` (`ORGANIZZE_API.md:884-919`) is not implemented at all | High |
| 10 | Transactions | `installments_attributes` (parcelada) — documented at `ORGANIZZE_API.md:1168-1212`, missing entirely | **High (feature gap)** |
| 11 | Transactions | `update_future` / `update_all` on PUT — documented at `ORGANIZZE_API.md:1216-1235`, missing | High |
| 12 | Transactions | `update_future` / `update_all` on DELETE — documented at `ORGANIZZE_API.md:1266-1281`, missing (needs DELETE-with-body from #1) | High |
| 13 | Transactions | `delete_transaction` discards the deleted-transaction payload | Low |
| 14 | Transfers | Transfer struct missing: `total_installments`, `installment`, `recurring`, `attachments_count`, `credit_card_id`, `credit_card_invoice_id`, `paid_credit_card_id`, `paid_credit_card_invoice_id`, `created_at`, `updated_at`, `tags`, `attachments` (`ORGANIZZE_API.md:1326-1350`) | High |
| 15 | Transfers | `GET /transfers/{id}` (`ORGANIZZE_API.md:1354-1388`) not implemented — no `get_transfer` tool | High |
| 16 | Transfers | `delete_transfer` fabricates `deleted: true` instead of reading API's `"deleted": true` (`ORGANIZZE_API.md:1517`), and discards the deleted-transfer payload | Medium |
| 17 | Cross-cutting | Integration test fake + README catalogue + tool descriptions need updates | required |

Two confirmed non-gaps (do **not** touch in this plan):
- **Users**: only `GET /users/:id` is documented; already fully exposed.
- **Budgets**: all three URL variants (`/budgets`, `/budgets/{year}`, `/budgets/{year}/{month}`) wired; the doc defines no Create/Update/Delete.

**Conscious non-scope clarifications**:
- `RecurrenceID` on `domain.Transaction` (`transaction.go:31`) is present in code but absent from the API doc. The field is harmless — `omitempty` drops it on the wire when the upstream omits it, and it correctly surfaces when present. We leave it as-is.
- `Attachment` is an opaque object in the API doc (always `[]` in the response example). We model `Transfer.Attachments` as `[]json.RawMessage` so the API value round-trips losslessly without inventing a domain shape the doc never specifies.

---

## Final tool catalogue (29 tools; +1 from current 28)

| | | Mutating? |
|---|---|---|
| `get_user` | UserService.Get | no |
| `list_accounts`, `get_account` | AccountService.List/Get | no |
| `create_account`, `update_account`, `delete_account` | AccountService write | **yes** |
| `list_categories`, `get_category` | CategoryService.List/Get | no |
| `create_category`, `update_category`, `delete_category` | CategoryService write | **yes** |
| `list_budgets` | BudgetService.List | no |
| `list_credit_cards`, `get_credit_card` | CreditCardService.List/Get | no |
| `create_credit_card`, `update_credit_card`, `delete_credit_card` | CreditCardService write | **yes** |
| `list_credit_card_invoices`, `get_credit_card_invoice` | InvoiceService.List/Get | no |
| **NEW** `get_credit_card_invoice_payment` | InvoiceService.Payment | no |
| `list_transfers`, **NEW** `get_transfer` | TransferService.List/Get | no |
| `create_transfer`, `update_transfer`, `delete_transfer` | TransferService write | **yes** |
| `list_transactions`, `get_transaction` | TransactionService.List/Get | no |
| `create_transaction`, `update_transaction`, `delete_transaction` | TransactionService write | **yes** |

---

## File map

The plan touches these files. Each task block below names the exact subset.

**Foundation (Task 1)**
- `internal/adapter/organizze/executor.go` — Delete signature broadened.
- `internal/adapter/organizze/executor_test.go` — new wire test.
- All five existing repository callsites of `exec.Delete(ctx, path)` updated to `exec.Delete(ctx, path, nil)`:
  - `internal/adapter/organizze/account_repository.go`
  - `internal/adapter/organizze/category_repository.go`
  - `internal/adapter/organizze/credit_card_repository.go`
  - `internal/adapter/organizze/transaction_repository.go`
  - `internal/adapter/organizze/transfer_repository.go`

**Domain layer touches**
- `internal/domain/account.go` (Task 4)
- `internal/domain/credit_card.go` (Task 6)
- `internal/domain/transaction.go` (Tasks 10, 11, 12)
- `internal/domain/transfer.go` (Tasks 14, 15)
- `internal/domain/filters.go` (Task 8 — new `ListInvoicesFilter`)

**Repository touches**
- `internal/adapter/organizze/category_repository.go` (Task 2)
- `internal/adapter/organizze/invoice_repository.go` (Tasks 8, 9)
- `internal/adapter/organizze/transaction_repository.go` (Task 12)
- `internal/adapter/organizze/transfer_repository.go` (Task 15)
- `internal/adapter/organizze/account_repository.go` (Task 5)
- `internal/adapter/organizze/credit_card_repository.go` (Task 7)
- Plus `_test.go` siblings.

**Usecase touches**
- `internal/usecase/category.go` (Task 3)
- `internal/usecase/account.go` (Task 5)
- `internal/usecase/credit_card.go` (Task 7)
- `internal/usecase/invoice.go` (Tasks 8, 9)
- `internal/usecase/transaction.go` (Tasks 10–13)
- `internal/usecase/transfer.go` (Tasks 15, 16)

**MCP adapter touches**
- `internal/adapter/mcp/tools_categories.go` (Tasks 2, 3)
- `internal/adapter/mcp/tools_accounts.go` (Tasks 4, 5)
- `internal/adapter/mcp/tools_credit_cards.go` (Tasks 6, 7)
- `internal/adapter/mcp/tools_invoices.go` (Tasks 8, 9)
- `internal/adapter/mcp/tools_transactions.go` (Tasks 10–13)
- `internal/adapter/mcp/tools_transfers.go` (Tasks 14, 15, 16)
- `internal/adapter/mcp/server.go` (Task 9 — `InvoiceService` interface widened)
- `internal/adapter/mcp/integration_test.go` (Task 17 — fake + tool list + roundtrip)

**Docs**
- `README.md` (Task 17 — tool catalogue)

---

## Conventions reused throughout this plan

Every delete-output that newly surfaces the deleted resource follows this shape:

```go
type DeleteXOutput struct {
    Deleted bool       `json:"deleted"`
    ID      int64      `json:"id"`
    X       *domain.X  `json:"x,omitempty"` // server's view of the deleted row
}
```

`Deleted` stays for backward compatibility with v0.3.0 clients that already shipped (commit `6aae45a`). The optional `X` field is populated from the API's response body. For DELETEs that return `204 No Content` in the test fake, `X` will be `nil`; against real Organizze it carries the payload.

Every "ParamX" type uses pointer fields when the underlying value is optional — never zero-as-sentinel. Existing `CreateAccountInput.Default bool` (`tools_accounts.go:39`) is the lone exception and stays as-is to avoid breaking v0.3.0 clients; new fields follow the pointer convention.

Every TDD step lists:
- The test code (full, copy-pasteable).
- The exact command to run.
- The expected outcome (PASS / FAIL with reason).

---

# Task 1: Foundation — `RequestExecutor.Delete` with optional body

**Files:**
- Modify: `internal/adapter/organizze/executor.go` (lines 56-74 — broaden `Delete` signature)
- Modify: `internal/adapter/organizze/executor_test.go` (add wire test)
- Modify: 5 callsites in `internal/adapter/organizze/*_repository.go` to pass `nil`

**Why this task first:** Tasks 2 (categories replacement_id) and 12 (transaction delete recurring flags) both need DELETE-with-JSON-body. Doing it once at the executor level keeps the change DRY.

- [ ] **Step 1: Read existing `executor_test.go` for the test style**

Run: `cat internal/adapter/organizze/executor_test.go | head -80`
Expected: see existing wire-level tests using `httptest.NewServer`. Reuse the same helper shape.

- [ ] **Step 2: Write failing test for DELETE-with-body**

Append to `internal/adapter/organizze/executor_test.go`:

```go
func TestRequestExecutor_Delete_WithBody_SendsJSONBodyAndDecodesResponse(t *testing.T) {
	var gotMethod, gotPath, gotCT string
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":42,"deleted":true}`)
	}))
	defer ts.Close()

	exec := newTestExecutor(t, ts.URL)
	body := map[string]any{"replacement_id": 18}
	var out struct {
		ID      int64 `json:"id"`
		Deleted bool  `json:"deleted"`
	}
	if err := exec.Delete(context.Background(), "/categories/6", body, &out); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/categories/6" {
		t.Errorf("path = %q", gotPath)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotCT)
	}
	if string(gotBody) != `{"replacement_id":18}` {
		t.Errorf("body = %q, want {\"replacement_id\":18}", string(gotBody))
	}
	if out.ID != 42 || !out.Deleted {
		t.Errorf("decoded = %+v", out)
	}
}

func TestRequestExecutor_Delete_NilBody_OmitsContentType(t *testing.T) {
	var gotCT string
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	exec := newTestExecutor(t, ts.URL)
	if err := exec.Delete(context.Background(), "/accounts/1", nil, nil); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if gotCT != "" {
		t.Errorf("Content-Type = %q on body-less DELETE, want empty", gotCT)
	}
	if len(gotBody) != 0 {
		t.Errorf("body = %q on body-less DELETE, want empty", string(gotBody))
	}
}
```

If `newTestExecutor` does not exist in `executor_test.go`, add the same helper that other tests in the package use (lift from `testhelper_test.go`).

- [ ] **Step 3: Run the new tests to verify they fail**

Run: `go test ./internal/adapter/organizze -run TestRequestExecutor_Delete -v`
Expected: compile error (`too many arguments in call to exec.Delete`) OR FAIL — the current `Delete(ctx, path)` has arity 2, not 4.

- [ ] **Step 4: Broaden `Delete` to `Delete(ctx, path, body, out)`**

Edit `internal/adapter/organizze/executor.go:71-74`:

```go
// Delete performs a DELETE. If body is non-nil it is JSON-encoded; if out is
// non-nil the response body is decoded into it. Pass (nil, nil) for the
// classic no-body / discard-response case.
func (e *RequestExecutor) Delete(ctx context.Context, path string, body, out any) error {
	return e.do(ctx, http.MethodDelete, path, body, out)
}
```

- [ ] **Step 5: Update the 5 callsites that pass no body**

For each of these files, change `r.exec.Delete(ctx, path)` → `r.exec.Delete(ctx, path, nil, nil)`:

- `internal/adapter/organizze/account_repository.go:55`
- `internal/adapter/organizze/credit_card_repository.go:55`
- `internal/adapter/organizze/transaction_repository.go:75`
- `internal/adapter/organizze/transfer_repository.go:60`
- `internal/adapter/organizze/category_repository.go:64` (will be fully rewritten in Task 2; for now just make it compile)

- [ ] **Step 6: Run the new tests + the whole organizze package**

Run: `go test ./internal/adapter/organizze -v`
Expected: all PASS, including the two new tests from Step 2.

- [ ] **Step 7: Run the full test suite (sanity)**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/adapter/organizze/executor.go internal/adapter/organizze/executor_test.go internal/adapter/organizze/account_repository.go internal/adapter/organizze/category_repository.go internal/adapter/organizze/credit_card_repository.go internal/adapter/organizze/transaction_repository.go internal/adapter/organizze/transfer_repository.go
git commit -m "refactor(organizze): allow DELETE to send a JSON body and decode a response"
```

---

# Task 2: Categories — fix `replacement_id` transport (query → body)

**Files:**
- Modify: `internal/adapter/organizze/category_repository.go:55-65` (Delete)
- Modify: `internal/adapter/organizze/category_repository_test.go` (rewrite the replacement test)

**Why:** `ORGANIZZE_API.md:481-489` shows:
> `DELETE /categories/6` with body `{ "replacement_id": 18 }`
>
> "Ao excluir uma categoria você pode informar uma categoria para substitui-la, todas as movimentações da categoria excluídas serão transferidas para a categoria substituta."

The current code sends `?replacement_id=18` in the query string, which Organizze ignores. Affected transactions silently fall back to the default category.

- [ ] **Step 1: Read the existing repo test for delete**

Run: `grep -n "Delete\|replacement_id" internal/adapter/organizze/category_repository_test.go`
Expected: find the current "replacement_id" test that asserts the query string.

- [ ] **Step 2: Rewrite the failing test to assert body, not query**

Replace the existing `TestCategoryRepository_Delete_*` test(s) in `internal/adapter/organizze/category_repository_test.go` with:

```go
func TestCategoryRepository_Delete_SendsReplacementIDInBody(t *testing.T) {
	var gotPath, gotCT string
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":6,"name":"Marketing","color":"8dd47f","parent_id":null}`)
	}))
	defer ts.Close()

	repo := organizze.NewCategoryRepository(newTestExecutor(t, ts.URL))
	rid := int64(18)
	cat, err := repo.Delete(context.Background(), 6, &rid)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if gotPath != "/categories/6?" {
		t.Errorf("path = %q, want /categories/6 with no query", gotPath)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotCT)
	}
	if string(gotBody) != `{"replacement_id":18}` {
		t.Errorf("body = %q, want {\"replacement_id\":18}", string(gotBody))
	}
	if cat == nil || cat.ID != 6 {
		t.Errorf("returned category = %+v", cat)
	}
}

func TestCategoryRepository_Delete_NilReplacement_SendsNoBody(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()
	repo := organizze.NewCategoryRepository(newTestExecutor(t, ts.URL))
	if _, err := repo.Delete(context.Background(), 6, nil); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(gotBody) != 0 {
		t.Errorf("body = %q on nil replacement, want empty", string(gotBody))
	}
}
```

Note the signature change: `Delete` now returns `(*domain.Category, error)`.

- [ ] **Step 3: Run the tests — verify failure**

Run: `go test ./internal/adapter/organizze -run TestCategoryRepository_Delete -v`
Expected: compile FAIL (`Delete` returns only `error`, not `(*Category, error)`).

- [ ] **Step 4: Update the repository**

Replace `internal/adapter/organizze/category_repository.go:55-65` with:

```go
// Delete issues a DELETE. If replacementID is non-nil, the request body is
// {"replacement_id": ID} which tells Organizze to reassign affected
// transactions to that category (per ORGANIZZE_API.md "Excluir uma categoria").
// Returns the deleted Category as echoed by the API.
func (r *CategoryRepository) Delete(ctx context.Context, id int64, replacementID *int64) (*domain.Category, error) {
	var body any
	if replacementID != nil {
		body = struct {
			ReplacementID int64 `json:"replacement_id"`
		}{ReplacementID: *replacementID}
	}
	var out domain.Category
	if err := r.exec.Delete(ctx, fmt.Sprintf("/categories/%d", id), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
```

Drop the now-unused `net/url` and `strconv` imports from this file.

- [ ] **Step 5: Run tests — should still fail (downstream signature mismatch)**

Run: `go build ./...`
Expected: build FAIL — `usecase.CategoryService.Delete` and the `CategoryWriter` interface still expect `error`. Will be fixed in Task 3.

To keep this task green-on-its-own and minimize cross-task interleaving, we defer the downstream fix until Task 3 — leave this commit only **after** Task 3 is also done (Tasks 2 and 3 land as one commit). Skip to Task 3 now.

---

# Task 3: Categories — surface deleted category in MCP delete output

**Files:**
- Modify: `internal/domain/category.go` (no change needed — already complete)
- Modify: `internal/usecase/category.go:15-19, 53-55` (interface + service signature)
- Modify: `internal/usecase/category_test.go` (update existing Delete tests)
- Modify: `internal/adapter/mcp/tools_categories.go:11-17, 57-60, 102-109, 128-131` (interface + output + handler)
- Modify: `internal/adapter/mcp/tools_categories_test.go` (update Delete handler test)

- [ ] **Step 1: Update the usecase test for Delete**

In `internal/usecase/category_test.go`, find the existing `TestCategoryService_Delete*` cases. Replace the fake `Delete` method on the test repo with the new signature returning `(*domain.Category, error)`, and assert the returned category bubbles through:

```go
// fakeCategoryRepo: extend the existing Delete stub to return the category.
func (f *fakeCategoryRepo) Delete(ctx context.Context, id int64, replacementID *int64) (*domain.Category, error) {
	f.deleteID = id
	f.deleteReplacement = replacementID
	if f.deleteErr != nil {
		return nil, f.deleteErr
	}
	return &domain.Category{ID: id, Name: "deleted-stub"}, nil
}

func TestCategoryService_Delete_ReturnsDeletedCategory(t *testing.T) {
	repo := &fakeCategoryRepo{}
	svc := usecase.NewCategoryService(repo)
	c, err := svc.Delete(context.Background(), 6, nil)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if c == nil || c.ID != 6 || c.Name != "deleted-stub" {
		t.Errorf("returned = %+v", c)
	}
}
```

- [ ] **Step 2: Run the test — verify failure**

Run: `go test ./internal/usecase -run TestCategoryService_Delete -v`
Expected: compile FAIL.

- [ ] **Step 3: Update the usecase**

Edit `internal/usecase/category.go`:

```go
type CategoryWriter interface {
	Create(ctx context.Context, params domain.CreateCategoryParams) (*domain.Category, error)
	Update(ctx context.Context, id int64, params domain.UpdateCategoryParams) (*domain.Category, error)
	Delete(ctx context.Context, id int64, replacementID *int64) (*domain.Category, error)
}
```

and:

```go
func (s *CategoryService) Delete(ctx context.Context, id int64, replacementID *int64) (*domain.Category, error) {
	return s.repo.Delete(ctx, id, replacementID)
}
```

- [ ] **Step 4: Update the MCP test for `delete_category`**

In `internal/adapter/mcp/tools_categories_test.go`, locate the existing `delete_category` test. Update the fake service:

```go
func (f *fakeCategorySvc) Delete(ctx context.Context, id int64, replacementID *int64) (*domain.Category, error) {
	f.deleteID = id
	f.deleteReplacement = replacementID
	return &domain.Category{ID: id, Name: "deleted-stub", Color: "8dd47f"}, nil
}

func TestDeleteCategoryHandler_ReturnsDeletedCategory(t *testing.T) {
	svc := &fakeCategorySvc{}
	out, err := callTool[mcp.DeleteCategoryInput, mcp.DeleteCategoryOutput](
		t, registerCategoryToolsForTest(svc), "delete_category",
		mcp.DeleteCategoryInput{ID: 6},
	)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !out.Deleted || out.ID != 6 {
		t.Errorf("got %+v", out)
	}
	if out.Category == nil || out.Category.ID != 6 || out.Category.Name != "deleted-stub" {
		t.Errorf("Category = %+v", out.Category)
	}
}
```

(If `callTool`/`registerCategoryToolsForTest` helpers do not exist, follow whatever harness `tools_categories_test.go` already uses — match the style without inventing new abstractions.)

- [ ] **Step 5: Run the tests — verify they fail**

Run: `go test ./internal/adapter/mcp -run TestDeleteCategoryHandler -v`
Expected: compile FAIL.

- [ ] **Step 6: Update the MCP layer**

Edit `internal/adapter/mcp/tools_categories.go`:

```go
type CategoryService interface {
	List(ctx context.Context) ([]domain.Category, error)
	Get(ctx context.Context, id int64) (*domain.Category, error)
	Create(ctx context.Context, params domain.CreateCategoryParams) (*domain.Category, error)
	Update(ctx context.Context, id int64, params domain.UpdateCategoryParams) (*domain.Category, error)
	Delete(ctx context.Context, id int64, replacementID *int64) (*domain.Category, error)
}
```

```go
type DeleteCategoryOutput struct {
	Deleted  bool             `json:"deleted"`
	ID       int64            `json:"id"`
	Category *domain.Category `json:"category,omitempty"`
}
```

```go
func deleteCategoryHandler(svc CategoryService) mcpsdk.ToolHandlerFor[DeleteCategoryInput, DeleteCategoryOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in DeleteCategoryInput) (*mcpsdk.CallToolResult, DeleteCategoryOutput, error) {
		c, err := svc.Delete(ctx, in.ID, in.ReplacementID)
		if err != nil {
			return nil, DeleteCategoryOutput{}, err
		}
		return nil, DeleteCategoryOutput{Deleted: true, ID: in.ID, Category: c}, nil
	}
}
```

Update the `delete_category` tool description (`tools_categories.go:128-131`) to:

```go
Description: "Permanently delete an Organizze category by id. Optionally pass replacement_id (numeric id of another category) to reassign affected transactions to that category — without this, Organizze falls back to the default category. The deleted category snapshot is returned in the 'category' field when the API provides one.",
```

- [ ] **Step 7: Run the full suite**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 8: Commit Tasks 2 + 3 together**

```bash
git add internal/adapter/organizze/category_repository.go internal/adapter/organizze/category_repository_test.go internal/usecase/category.go internal/usecase/category_test.go internal/adapter/mcp/tools_categories.go internal/adapter/mcp/tools_categories_test.go
git commit -m "fix(categories): send replacement_id as JSON body on delete; surface deleted category"
```

---

# Task 4: Accounts — add `archived` to update; pointerize Create defaults

**Files:**
- Modify: `internal/domain/account.go:19-32`
- Modify: `internal/adapter/mcp/tools_accounts.go:35-54, 93-115`
- Modify: `internal/usecase/account_test.go` and `tools_accounts_test.go` (extend existing update tests)

- [ ] **Step 1: Write failing test asserting `archived` round-trips on update**

Append to `internal/adapter/organizze/account_repository_test.go`:

```go
func TestAccountRepository_Update_SendsArchivedWhenSet(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":1,"name":"Itaú","type":"checking","archived":true}`)
	}))
	defer ts.Close()
	repo := organizze.NewAccountRepository(newTestExecutor(t, ts.URL))
	archived := true
	a, err := repo.Update(context.Background(), 1, domain.UpdateAccountParams{Archived: &archived})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if string(gotBody) != `{"archived":true}` {
		t.Errorf("body = %q, want {\"archived\":true}", string(gotBody))
	}
	if a == nil || !a.Archived {
		t.Errorf("returned = %+v", a)
	}
}
```

- [ ] **Step 2: Run — verify failure**

Run: `go test ./internal/adapter/organizze -run TestAccountRepository_Update_SendsArchived -v`
Expected: compile FAIL (`Archived` not a field of `UpdateAccountParams`).

- [ ] **Step 3: Add `Archived` to `UpdateAccountParams`**

Edit `internal/domain/account.go:26-32`:

```go
type UpdateAccountParams struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Default     *bool   `json:"default,omitempty"`
	Type        *string `json:"type,omitempty"`
	Archived    *bool   `json:"archived,omitempty"`
}
```

- [ ] **Step 4: Plumb `Archived` through the MCP layer**

Edit `internal/adapter/mcp/tools_accounts.go`:

```go
type UpdateAccountInput struct {
	ID          int64   `json:"id"                    jsonschema:"The numeric Organizze account id to update."`
	Name        *string `json:"name,omitempty"        jsonschema:"New account name."`
	Description *string `json:"description,omitempty" jsonschema:"New description."`
	Default     *bool   `json:"default,omitempty"     jsonschema:"New default flag."`
	Type        *string `json:"type,omitempty"        jsonschema:"New type (checking|savings|other)."`
	Archived    *bool   `json:"archived,omitempty"    jsonschema:"Archive (true) or unarchive (false) the account."`
}
```

And in `updateAccountHandler` (currently `tools_accounts.go:105-115`):

```go
a, err := svc.Update(ctx, in.ID, domain.UpdateAccountParams{
	Name: in.Name, Description: in.Description, Default: in.Default, Type: in.Type, Archived: in.Archived,
})
```

Bump the `update_account` tool description:

```go
Description: "Update fields on an existing Organizze account. Only fields you provide are changed. Set archived=true to archive (or false to unarchive).",
```

- [ ] **Step 5: Run full suite**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/account.go internal/adapter/organizze/account_repository_test.go internal/adapter/mcp/tools_accounts.go
git commit -m "feat(accounts): expose 'archived' on update_account"
```

---

# Task 5: Accounts — surface deleted account in delete output

**Files:**
- Modify: `internal/adapter/organizze/account_repository.go:54-56` (return deleted account)
- Modify: `internal/adapter/organizze/account_repository_test.go` (extend delete test)
- Modify: `internal/usecase/account.go` (interface + service)
- Modify: `internal/usecase/account_test.go`
- Modify: `internal/adapter/mcp/tools_accounts.go:11-17, 66-69, 117-124, 143-146`
- Modify: `internal/adapter/mcp/tools_accounts_test.go`

This mirrors Task 3 exactly for accounts. Use the same `DeleteXOutput` convention.

- [ ] **Step 1: Write failing repository test**

Append to `internal/adapter/organizze/account_repository_test.go`:

```go
func TestAccountRepository_Delete_ReturnsDeletedAccount(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":18,"name":"Itaú","type":"checking","archived":false,"default":true}`)
	}))
	defer ts.Close()
	repo := organizze.NewAccountRepository(newTestExecutor(t, ts.URL))
	a, err := repo.Delete(context.Background(), 18)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if a == nil || a.ID != 18 || a.Name != "Itaú" {
		t.Errorf("returned = %+v", a)
	}
}
```

- [ ] **Step 2: Run — verify failure**

Run: `go test ./internal/adapter/organizze -run TestAccountRepository_Delete_Returns -v`
Expected: compile FAIL (`Delete` returns only `error`).

- [ ] **Step 3: Update repository**

Replace `internal/adapter/organizze/account_repository.go:54-56` with:

```go
// Delete issues a DELETE and returns the deleted account snapshot as echoed
// by Organizze.
func (r *AccountRepository) Delete(ctx context.Context, id int64) (*domain.Account, error) {
	var out domain.Account
	if err := r.exec.Delete(ctx, fmt.Sprintf("/accounts/%d", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
```

- [ ] **Step 4: Update usecase interface + service**

In `internal/usecase/account.go`, change `AccountWriter.Delete` and `AccountService.Delete` signatures to return `(*domain.Account, error)`. Update the matching test repo fake in `account_test.go` (return `&domain.Account{ID: id}` on the happy path).

- [ ] **Step 5: Update MCP layer**

Edit `internal/adapter/mcp/tools_accounts.go`:

```go
type AccountService interface {
	List(ctx context.Context) ([]domain.Account, error)
	Get(ctx context.Context, id int64) (*domain.Account, error)
	Create(ctx context.Context, params domain.CreateAccountParams) (*domain.Account, error)
	Update(ctx context.Context, id int64, params domain.UpdateAccountParams) (*domain.Account, error)
	Delete(ctx context.Context, id int64) (*domain.Account, error)
}

type DeleteAccountOutput struct {
	Deleted bool            `json:"deleted"`
	ID      int64           `json:"id"`
	Account *domain.Account `json:"account,omitempty"`
}
```

In `deleteAccountHandler`:

```go
func deleteAccountHandler(svc AccountService) mcpsdk.ToolHandlerFor[DeleteAccountInput, DeleteAccountOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in DeleteAccountInput) (*mcpsdk.CallToolResult, DeleteAccountOutput, error) {
		a, err := svc.Delete(ctx, in.ID)
		if err != nil {
			return nil, DeleteAccountOutput{}, err
		}
		return nil, DeleteAccountOutput{Deleted: true, ID: in.ID, Account: a}, nil
	}
}
```

- [ ] **Step 6: Update the MCP test for delete (fakeAccountSvc returns a stub)**

Mirror the Category test pattern from Task 3 Step 4.

- [ ] **Step 7: Run full suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/adapter/organizze/account_repository.go internal/adapter/organizze/account_repository_test.go internal/usecase/account.go internal/usecase/account_test.go internal/adapter/mcp/tools_accounts.go internal/adapter/mcp/tools_accounts_test.go
git commit -m "feat(accounts): surface deleted account snapshot in delete_account output"
```

---

# Task 6: Credit cards — expose missing fields on `update_credit_card`

**Files:**
- Modify: `internal/domain/credit_card.go:36-42`
- Modify: `internal/adapter/mcp/tools_credit_cards.go:45-52, 101-112`
- Modify: `internal/adapter/organizze/credit_card_repository_test.go` (extend update test)

Add to `UpdateCreditCardParams`: `LimitCents *int64`, `CardNetwork *string`, `Archived *bool`, `Default *bool`.

- [ ] **Step 1: Write failing wire test**

Append to `internal/adapter/organizze/credit_card_repository_test.go`:

```go
func TestCreditCardRepository_Update_SendsAllOptionalFields(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":3,"name":"Visa","limit_cents":2000000,"archived":false,"default":true}`)
	}))
	defer ts.Close()
	repo := organizze.NewCreditCardRepository(newTestExecutor(t, ts.URL))
	limit := int64(2000000)
	network := "mastercard"
	archived := false
	defaultCard := true
	if _, err := repo.Update(context.Background(), 3, domain.UpdateCreditCardParams{
		LimitCents:  &limit,
		CardNetwork: &network,
		Archived:    &archived,
		Default:     &defaultCard,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	// Body must contain all four fields exactly.
	got := string(gotBody)
	for _, want := range []string{`"limit_cents":2000000`, `"card_network":"mastercard"`, `"archived":false`, `"default":true`} {
		if !strings.Contains(got, want) {
			t.Errorf("body = %q, missing %s", got, want)
		}
	}
}
```

(If `strings` not imported in the test file, add it.)

- [ ] **Step 2: Run — verify failure**

Run: `go test ./internal/adapter/organizze -run TestCreditCardRepository_Update_SendsAllOptional -v`
Expected: compile FAIL.

- [ ] **Step 3: Extend `UpdateCreditCardParams`**

Edit `internal/domain/credit_card.go:36-42`:

```go
type UpdateCreditCardParams struct {
	Name                *string `json:"name,omitempty"`
	DueDay              *int    `json:"due_day,omitempty"`
	ClosingDay          *int    `json:"closing_day,omitempty"`
	Description         *string `json:"description,omitempty"`
	UpdateInvoicesSince *string `json:"update_invoices_since,omitempty"`
	LimitCents          *int64  `json:"limit_cents,omitempty"`
	CardNetwork         *string `json:"card_network,omitempty"`
	Archived            *bool   `json:"archived,omitempty"`
	Default             *bool   `json:"default,omitempty"`
}
```

- [ ] **Step 4: Extend `UpdateCreditCardInput` + handler**

Edit `internal/adapter/mcp/tools_credit_cards.go:45-52`:

```go
type UpdateCreditCardInput struct {
	ID                  int64   `json:"id"                              jsonschema:"The numeric Organizze credit card id to update."`
	Name                *string `json:"name,omitempty"                  jsonschema:"New name."`
	DueDay              *int    `json:"due_day,omitempty"               jsonschema:"New due day (1-31)."`
	ClosingDay          *int    `json:"closing_day,omitempty"           jsonschema:"New closing day (1-31)."`
	Description         *string `json:"description,omitempty"           jsonschema:"New description."`
	UpdateInvoicesSince *string `json:"update_invoices_since,omitempty" jsonschema:"If set (YYYY-MM-DD), Organizze retroactively regenerates invoices from this date."`
	LimitCents          *int64  `json:"limit_cents,omitempty"           jsonschema:"New credit limit in cents."`
	CardNetwork         *string `json:"card_network,omitempty"          jsonschema:"New card network (visa, mastercard, hipercard, etc.)."`
	Archived            *bool   `json:"archived,omitempty"              jsonschema:"Archive (true) or unarchive (false) the card."`
	Default             *bool   `json:"default,omitempty"               jsonschema:"Set as default credit card."`
}
```

And the handler call (`tools_credit_cards.go:101-112`):

```go
cc, err := svc.Update(ctx, in.ID, domain.UpdateCreditCardParams{
	Name: in.Name, DueDay: in.DueDay, ClosingDay: in.ClosingDay,
	Description: in.Description, UpdateInvoicesSince: in.UpdateInvoicesSince,
	LimitCents: in.LimitCents, CardNetwork: in.CardNetwork,
	Archived: in.Archived, Default: in.Default,
})
```

- [ ] **Step 5: Run full suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/credit_card.go internal/adapter/mcp/tools_credit_cards.go internal/adapter/organizze/credit_card_repository_test.go
git commit -m "feat(credit_cards): expose limit_cents, card_network, archived, default on update"
```

---

# Task 7: Credit cards — surface deleted card in delete output

Mirror of Task 5 for credit cards. Mechanics are identical, only types differ.

**Files:**
- Modify: `internal/adapter/organizze/credit_card_repository.go:53-56`
- Modify: `internal/adapter/organizze/credit_card_repository_test.go`
- Modify: `internal/usecase/credit_card.go` (interface + service `Delete` returns `(*domain.CreditCard, error)`)
- Modify: `internal/usecase/credit_card_test.go`
- Modify: `internal/adapter/mcp/tools_credit_cards.go:11-17, 62-65, 114-121`
- Modify: `internal/adapter/mcp/tools_credit_cards_test.go`

- [ ] **Step 1: Repository test (RED)**

Append to `internal/adapter/organizze/credit_card_repository_test.go`:

```go
func TestCreditCardRepository_Delete_ReturnsDeletedCard(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":3,"name":"Visa Exclusive","closing_day":4,"due_day":17,"limit_cents":1200000,"archived":true}`)
	}))
	defer ts.Close()
	repo := organizze.NewCreditCardRepository(newTestExecutor(t, ts.URL))
	cc, err := repo.Delete(context.Background(), 3)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if cc == nil || cc.ID != 3 || !cc.Archived {
		t.Errorf("returned = %+v", cc)
	}
}
```

Run: `go test ./internal/adapter/organizze -run TestCreditCardRepository_Delete_Returns -v` — expect compile FAIL.

- [ ] **Step 2: Update repository**

Replace `internal/adapter/organizze/credit_card_repository.go:53-56`:

```go
// Delete issues a DELETE and returns the deleted credit card snapshot.
func (r *CreditCardRepository) Delete(ctx context.Context, id int64) (*domain.CreditCard, error) {
	var out domain.CreditCard
	if err := r.exec.Delete(ctx, fmt.Sprintf("/credit_cards/%d", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
```

- [ ] **Step 3: Propagate signature through usecase + MCP**

Edit `internal/usecase/credit_card.go`:

```go
type CreditCardWriter interface {
	Create(ctx context.Context, params domain.CreateCreditCardParams) (*domain.CreditCard, error)
	Update(ctx context.Context, id int64, params domain.UpdateCreditCardParams) (*domain.CreditCard, error)
	Delete(ctx context.Context, id int64) (*domain.CreditCard, error)
}
```

```go
func (s *CreditCardService) Delete(ctx context.Context, id int64) (*domain.CreditCard, error) {
	return s.repo.Delete(ctx, id)
}
```

Edit `internal/adapter/mcp/tools_credit_cards.go`:

```go
type CreditCardService interface {
	List(ctx context.Context) ([]domain.CreditCard, error)
	Get(ctx context.Context, id int64) (*domain.CreditCard, error)
	Create(ctx context.Context, params domain.CreateCreditCardParams) (*domain.CreditCard, error)
	Update(ctx context.Context, id int64, params domain.UpdateCreditCardParams) (*domain.CreditCard, error)
	Delete(ctx context.Context, id int64) (*domain.CreditCard, error)
}

type DeleteCreditCardOutput struct {
	Deleted    bool               `json:"deleted"`
	ID         int64              `json:"id"`
	CreditCard *domain.CreditCard `json:"credit_card,omitempty"`
}
```

And `deleteCreditCardHandler` (`tools_credit_cards.go:114-121`):

```go
func deleteCreditCardHandler(svc CreditCardService) mcpsdk.ToolHandlerFor[DeleteCreditCardInput, DeleteCreditCardOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in DeleteCreditCardInput) (*mcpsdk.CallToolResult, DeleteCreditCardOutput, error) {
		cc, err := svc.Delete(ctx, in.ID)
		if err != nil {
			return nil, DeleteCreditCardOutput{}, err
		}
		return nil, DeleteCreditCardOutput{Deleted: true, ID: in.ID, CreditCard: cc}, nil
	}
}
```

- [ ] **Step 4: Update the usecase test fake + MCP test fake**

In `internal/usecase/credit_card_test.go` and `internal/adapter/mcp/tools_credit_cards_test.go`, update the fake `Delete` to return `(&domain.CreditCard{ID: id}, nil)`. Extend at least one assertion to confirm the deleted snapshot bubbles up.

- [ ] **Step 5: Run full suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/organizze/credit_card_repository.go internal/adapter/organizze/credit_card_repository_test.go internal/usecase/credit_card.go internal/usecase/credit_card_test.go internal/adapter/mcp/tools_credit_cards.go internal/adapter/mcp/tools_credit_cards_test.go
git commit -m "feat(credit_cards): surface deleted card snapshot in delete_credit_card output"
```

---

# Task 8: Invoices — add `start_date`/`end_date` filters to `list_credit_card_invoices`

**Why:** `ORGANIZZE_API.md:134`:
> "...faturas de cartão de crédito são paginadas por período. Para informar qual período utilize os parâmetros `&start_date=2015-09-01&end_date=2015-09-30`. Se você não informar o período o Organizze vai limitar os registros para... Ano atual para faturas de cartão de crédito."

Without these filters, the tool cannot retrieve invoices from any past year. High-impact gap.

**Files:**
- Modify: `internal/domain/filters.go` (new `ListInvoicesFilter`)
- Modify: `internal/adapter/organizze/invoice_repository.go:19-25` (List takes filter)
- Modify: `internal/adapter/organizze/invoice_repository_test.go`
- Modify: `internal/usecase/invoice.go` (interface + service)
- Modify: `internal/usecase/invoice_test.go`
- Modify: `internal/adapter/mcp/tools_invoices.go:11-22, 33-41, 53-57`
- Modify: `internal/adapter/mcp/tools_invoices_test.go`

- [ ] **Step 1: Failing wire test**

Append to `internal/adapter/organizze/invoice_repository_test.go`:

```go
func TestInvoiceRepository_List_AppendsDateFilters(t *testing.T) {
	var gotURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	}))
	defer ts.Close()
	repo := organizze.NewInvoiceRepository(newTestExecutor(t, ts.URL))
	if _, err := repo.List(context.Background(), 7, domain.ListInvoicesFilter{
		StartDate: "2024-01-01", EndDate: "2024-12-31",
	}); err != nil {
		t.Fatalf("List: %v", err)
	}
	want := "/credit_cards/7/invoices?end_date=2024-12-31&start_date=2024-01-01"
	if gotURL != want {
		t.Errorf("URL = %q, want %q", gotURL, want)
	}
}

func TestInvoiceRepository_List_NoFilter_OmitsQuery(t *testing.T) {
	var gotURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	}))
	defer ts.Close()
	repo := organizze.NewInvoiceRepository(newTestExecutor(t, ts.URL))
	if _, err := repo.List(context.Background(), 7, domain.ListInvoicesFilter{}); err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotURL != "/credit_cards/7/invoices" {
		t.Errorf("URL = %q, want /credit_cards/7/invoices", gotURL)
	}
}
```

Run: `go test ./internal/adapter/organizze -run TestInvoiceRepository_List_ -v` — expect compile FAIL.

- [ ] **Step 2: Add `ListInvoicesFilter`**

Append to `internal/domain/filters.go`:

```go
// ListInvoicesFilter is the filter for InvoiceService.List. Empty fields are
// omitted. Without a date range, Organizze restricts results to the current
// calendar year (per ORGANIZZE_API.md "Paginação").
type ListInvoicesFilter struct {
	StartDate string // YYYY-MM-DD
	EndDate   string // YYYY-MM-DD
}
```

- [ ] **Step 3: Update repository signature**

Edit `internal/adapter/organizze/invoice_repository.go`:

```go
func (r *InvoiceRepository) List(ctx context.Context, cardID int64, f domain.ListInvoicesFilter) ([]domain.Invoice, error) {
	q := url.Values{}
	if f.StartDate != "" {
		q.Set("start_date", f.StartDate)
	}
	if f.EndDate != "" {
		q.Set("end_date", f.EndDate)
	}
	path := fmt.Sprintf("/credit_cards/%d/invoices", cardID)
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var out []domain.Invoice
	if err := r.exec.Get(ctx, path, &out); err != nil {
		return nil, err
	}
	return out, nil
}
```

Add `"net/url"` to imports.

- [ ] **Step 4: Update usecase**

Edit `internal/usecase/invoice.go`:

```go
type InvoiceRepository interface {
	List(ctx context.Context, creditCardID int64, filter domain.ListInvoicesFilter) ([]domain.Invoice, error)
	Get(ctx context.Context, creditCardID, invoiceID int64) (*domain.Invoice, error)
}

func (s *InvoiceService) List(ctx context.Context, creditCardID int64, filter domain.ListInvoicesFilter) ([]domain.Invoice, error) {
	return s.repo.List(ctx, creditCardID, filter)
}
```

- [ ] **Step 5: Update MCP layer**

Edit `internal/adapter/mcp/tools_invoices.go`:

```go
type InvoiceService interface {
	List(ctx context.Context, creditCardID int64, filter domain.ListInvoicesFilter) ([]domain.Invoice, error)
	Get(ctx context.Context, creditCardID, invoiceID int64) (*domain.Invoice, error)
}

type ListInvoicesInput struct {
	CreditCardID int64  `json:"credit_card_id" jsonschema:"The numeric credit card id whose invoices to list."`
	StartDate    string `json:"start_date,omitempty" jsonschema:"Optional YYYY-MM-DD lower bound. Without a range, Organizze caps results to the current calendar year."`
	EndDate      string `json:"end_date,omitempty"   jsonschema:"Optional YYYY-MM-DD upper bound."`
}
```

And the handler:

```go
func listInvoicesHandler(svc InvoiceService) mcpsdk.ToolHandlerFor[ListInvoicesInput, ListInvoicesOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in ListInvoicesInput) (*mcpsdk.CallToolResult, ListInvoicesOutput, error) {
		invs, err := svc.List(ctx, in.CreditCardID, domain.ListInvoicesFilter{
			StartDate: in.StartDate, EndDate: in.EndDate,
		})
		if err != nil {
			return nil, ListInvoicesOutput{}, err
		}
		return nil, ListInvoicesOutput{Invoices: invs}, nil
	}
}
```

Tool description (`tools_invoices.go:55`):

```go
Description: "List invoices for a given credit card. Optional start_date / end_date (YYYY-MM-DD) widen beyond the default current-year window.",
```

- [ ] **Step 6: Fix usecase tests + MCP handler tests for new signature**

Update existing fakes in `internal/usecase/invoice_test.go` and `internal/adapter/mcp/tools_invoices_test.go`. The fake's `List` now takes a filter; spot-assert the filter is forwarded unchanged.

- [ ] **Step 7: Run full suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/domain/filters.go internal/adapter/organizze/invoice_repository.go internal/adapter/organizze/invoice_repository_test.go internal/usecase/invoice.go internal/usecase/invoice_test.go internal/adapter/mcp/tools_invoices.go internal/adapter/mcp/tools_invoices_test.go
git commit -m "feat(invoices): add start_date/end_date filters to list_credit_card_invoices"
```

---

# Task 9: Invoices — add `get_credit_card_invoice_payment` tool

**Why:** `ORGANIZZE_API.md:884-919` documents `GET /credit_cards/{card_id}/invoices/{invoice_id}/payments` returning a Transaction-shaped object (the consolidated payment record for the invoice). Currently unimplemented.

**Files:**
- Modify: `internal/adapter/organizze/invoice_repository.go` (add `Payment` method)
- Modify: `internal/adapter/organizze/invoice_repository_test.go`
- Modify: `internal/usecase/invoice.go` (interface + service)
- Modify: `internal/usecase/invoice_test.go`
- Modify: `internal/adapter/mcp/tools_invoices.go` (new Input/Output, handler, tool registration)
- Modify: `internal/adapter/mcp/tools_invoices_test.go`

- [ ] **Step 1: Failing wire test**

Append to `internal/adapter/organizze/invoice_repository_test.go`:

```go
func TestInvoiceRepository_Payment_HitsPaymentsURL(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":1033,"description":"Pagamento fatura","date":"2015-09-16","paid":true,"amount_cents":0,"account_id":3,"category_id":21}`)
	}))
	defer ts.Close()
	repo := organizze.NewInvoiceRepository(newTestExecutor(t, ts.URL))
	tx, err := repo.Payment(context.Background(), 3, 186)
	if err != nil {
		t.Fatalf("Payment: %v", err)
	}
	if gotPath != "/credit_cards/3/invoices/186/payments" {
		t.Errorf("path = %q", gotPath)
	}
	if tx == nil || tx.ID != 1033 || tx.Description != "Pagamento fatura" {
		t.Errorf("returned = %+v", tx)
	}
}
```

Run: `go test ./internal/adapter/organizze -run TestInvoiceRepository_Payment -v` — expect compile FAIL.

- [ ] **Step 2: Add repository method**

Append to `internal/adapter/organizze/invoice_repository.go`:

```go
// Payment returns the consolidated payment Transaction for an invoice
// (GET /credit_cards/{cardID}/invoices/{invoiceID}/payments).
func (r *InvoiceRepository) Payment(ctx context.Context, cardID, invoiceID int64) (*domain.Transaction, error) {
	var tx domain.Transaction
	if err := r.exec.Get(ctx, fmt.Sprintf("/credit_cards/%d/invoices/%d/payments", cardID, invoiceID), &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}
```

- [ ] **Step 3: Extend usecase interface + service**

Edit `internal/usecase/invoice.go`:

```go
type InvoiceRepository interface {
	List(ctx context.Context, creditCardID int64, filter domain.ListInvoicesFilter) ([]domain.Invoice, error)
	Get(ctx context.Context, creditCardID, invoiceID int64) (*domain.Invoice, error)
	Payment(ctx context.Context, creditCardID, invoiceID int64) (*domain.Transaction, error)
}

func (s *InvoiceService) Payment(ctx context.Context, creditCardID, invoiceID int64) (*domain.Transaction, error) {
	return s.repo.Payment(ctx, creditCardID, invoiceID)
}
```

- [ ] **Step 4: Add MCP tool**

In `internal/adapter/mcp/tools_invoices.go`:

```go
type InvoiceService interface {
	List(ctx context.Context, creditCardID int64, filter domain.ListInvoicesFilter) ([]domain.Invoice, error)
	Get(ctx context.Context, creditCardID, invoiceID int64) (*domain.Invoice, error)
	Payment(ctx context.Context, creditCardID, invoiceID int64) (*domain.Transaction, error)
}

type GetInvoicePaymentInput struct {
	CreditCardID int64 `json:"credit_card_id" jsonschema:"The numeric credit card id."`
	InvoiceID    int64 `json:"invoice_id"     jsonschema:"The numeric invoice id."`
}

type GetInvoicePaymentOutput struct {
	Payment domain.Transaction `json:"payment"`
}

func getInvoicePaymentHandler(svc InvoiceService) mcpsdk.ToolHandlerFor[GetInvoicePaymentInput, GetInvoicePaymentOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetInvoicePaymentInput) (*mcpsdk.CallToolResult, GetInvoicePaymentOutput, error) {
		tx, err := svc.Payment(ctx, in.CreditCardID, in.InvoiceID)
		if err != nil {
			return nil, GetInvoicePaymentOutput{}, err
		}
		return nil, GetInvoicePaymentOutput{Payment: *tx}, nil
	}
}
```

Register in `registerInvoiceTools` (`tools_invoices.go:53-62`):

```go
mcpsdk.AddTool(s, &mcpsdk.Tool{
	Name:        "get_credit_card_invoice_payment",
	Description: "Fetch the consolidated payment Transaction for a credit-card invoice (GET /credit_cards/{credit_card_id}/invoices/{invoice_id}/payments).",
}, getInvoicePaymentHandler(svc))
```

- [ ] **Step 5: Update usecase + MCP test fakes**

Add `Payment` stubs to `fakeInvoiceRepo` in `invoice_test.go` and `fakeInvoiceSvc` in `tools_invoices_test.go`. Add at least one happy-path handler test.

- [ ] **Step 6: Run full suite**

Run: `go test ./...`
Expected: PASS (except `integration_test.go` which doesn't yet know about the new tool — will be fixed in Task 17).

If integration_test fails on the unexpected new tool count, add `"get_credit_card_invoice_payment"` to `allExpectedTools` (`integration_test.go:156-170`) and add the route to the fake server:

```go
case r.Method == http.MethodGet && r.URL.Path == "/credit_cards/1/invoices/100/payments":
	_, _ = io.WriteString(w, `{"id":1033,"description":"Pagamento fatura","amount_cents":0,"account_id":1,"category_id":10,"date":"2026-05-14"}`)
```

And the roundtrip case:

```go
{"get_credit_card_invoice_payment", "get_credit_card_invoice_payment", map[string]any{"credit_card_id": 1, "invoice_id": 100}},
```

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/organizze/invoice_repository.go internal/adapter/organizze/invoice_repository_test.go internal/usecase/invoice.go internal/usecase/invoice_test.go internal/adapter/mcp/tools_invoices.go internal/adapter/mcp/tools_invoices_test.go internal/adapter/mcp/integration_test.go
git commit -m "feat(invoices): add get_credit_card_invoice_payment MCP tool"
```

---

# Task 10: Transactions — add `installments_attributes` (parcelada)

**Why:** `ORGANIZZE_API.md:1168-1212` documents the parcelada (installment) variant of `POST /transactions`:
> ```json
> { "description": "...", "date": "...", "installments_attributes": {"periodicity": "monthly", "total": 12} }
> ```

This is the sibling of the v0.4.0 `recurrence_attributes`. Missed in that release.

Mutual exclusion: a single create cannot supply both `recurrence_attributes` and `installments_attributes` — Organizze's domain doesn't allow being both fixed and parceled at once. We enforce this in `validateCreate`.

**Files:**
- Modify: `internal/domain/transaction.go:37-79` (new `InstallmentsAttributes` type + field)
- Modify: `internal/usecase/transaction.go:62-79` (extend `validateCreate`)
- Modify: `internal/usecase/transaction_test.go`
- Modify: `internal/adapter/organizze/transaction_repository_test.go` (wire test)
- Modify: `internal/adapter/mcp/tools_transactions.go:40-58, 118-136, 170-173`
- Modify: `internal/adapter/mcp/tools_transactions_test.go`

- [ ] **Step 1: Domain test for the JSON shape**

Add to `internal/domain/transaction_test.go` (create if missing):

```go
package domain_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

func TestCreateTransactionParams_InstallmentsAttributes_Marshals(t *testing.T) {
	p := domain.CreateTransactionParams{
		Description: "Computador",
		Date:        "2026-05-14",
		AmountCents: -100000,
		AccountID:   1,
		CategoryID:  10,
		Installments: &domain.InstallmentsAttributes{
			Periodicity: domain.PeriodicityMonthly,
			Total:       12,
		},
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"installments_attributes":{"periodicity":"monthly","total":12}`) {
		t.Errorf("body = %s", string(b))
	}
}
```

Run: `go test ./internal/domain -run TestCreateTransactionParams_Installments -v` — expect compile FAIL.

- [ ] **Step 2: Add domain type + field**

Edit `internal/domain/transaction.go`. Append `Installments *InstallmentsAttributes` to `CreateTransactionParams`:

```go
type CreateTransactionParams struct {
	Description  string                   `json:"description"`
	Date         string                   `json:"date"`
	AmountCents  int64                    `json:"amount_cents"`
	AccountID    int64                    `json:"account_id"`
	CategoryID   int64                    `json:"category_id"`
	Paid         bool                     `json:"paid"`
	Notes        string                   `json:"notes,omitempty"`
	ContactID    *int64                   `json:"contact_id,omitempty"`
	Tags         []Tag                    `json:"tags,omitempty"`
	Recurrence   *RecurrenceAttributes    `json:"recurrence_attributes,omitempty"`
	Installments *InstallmentsAttributes  `json:"installments_attributes,omitempty"`
}

// InstallmentsAttributes turns POST /transactions into an installment (parcelada)
// create. Total is the number of installments (>=2 is meaningful); the response
// carries total_installments == total.
type InstallmentsAttributes struct {
	Periodicity Periodicity `json:"periodicity"`
	Total       int         `json:"total"`
}
```

- [ ] **Step 3: Usecase validation test**

Append to `internal/usecase/transaction_test.go`:

```go
func TestTransactionService_Create_RejectsBothRecurrenceAndInstallments(t *testing.T) {
	repo := &fakeTransactionRepo{}
	svc := usecase.NewTransactionService(repo)
	_, err := svc.Create(context.Background(), domain.CreateTransactionParams{
		Description: "x", Date: "2026-05-14", AmountCents: 1, AccountID: 1, CategoryID: 1,
		Recurrence:   &domain.RecurrenceAttributes{Periodicity: domain.PeriodicityMonthly},
		Installments: &domain.InstallmentsAttributes{Periodicity: domain.PeriodicityMonthly, Total: 12},
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}

func TestTransactionService_Create_RejectsInstallments_BadPeriodicity(t *testing.T) {
	svc := usecase.NewTransactionService(&fakeTransactionRepo{})
	_, err := svc.Create(context.Background(), domain.CreateTransactionParams{
		Description: "x", Date: "2026-05-14", AmountCents: 1, AccountID: 1, CategoryID: 1,
		Installments: &domain.InstallmentsAttributes{Periodicity: "daily", Total: 12},
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}

func TestTransactionService_Create_RejectsInstallments_NonPositiveTotal(t *testing.T) {
	svc := usecase.NewTransactionService(&fakeTransactionRepo{})
	_, err := svc.Create(context.Background(), domain.CreateTransactionParams{
		Description: "x", Date: "2026-05-14", AmountCents: 1, AccountID: 1, CategoryID: 1,
		Installments: &domain.InstallmentsAttributes{Periodicity: domain.PeriodicityMonthly, Total: 0},
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}
```

- [ ] **Step 4: Extend `validateCreate`**

Edit `internal/usecase/transaction.go:62-79`:

```go
func validateCreate(p domain.CreateTransactionParams) error {
	switch {
	case p.Description == "":
		return fmt.Errorf("%w: description is required", domain.ErrValidation)
	case p.Date == "":
		return fmt.Errorf("%w: date is required", domain.ErrValidation)
	case p.AmountCents == 0:
		return fmt.Errorf("%w: amount_cents must be non-zero", domain.ErrValidation)
	case p.AccountID == 0:
		return fmt.Errorf("%w: account_id is required", domain.ErrValidation)
	case p.CategoryID == 0:
		return fmt.Errorf("%w: category_id is required", domain.ErrValidation)
	}
	if p.Recurrence != nil && p.Installments != nil {
		return fmt.Errorf("%w: recurrence_attributes and installments_attributes are mutually exclusive", domain.ErrValidation)
	}
	if p.Recurrence != nil && !p.Recurrence.Periodicity.Valid() {
		return fmt.Errorf("%w: recurrence.periodicity must be one of %v", domain.ErrValidation, domain.AllPeriodicities)
	}
	if p.Installments != nil {
		if !p.Installments.Periodicity.Valid() {
			return fmt.Errorf("%w: installments.periodicity must be one of %v", domain.ErrValidation, domain.AllPeriodicities)
		}
		if p.Installments.Total <= 0 {
			return fmt.Errorf("%w: installments.total must be > 0", domain.ErrValidation)
		}
	}
	return nil
}
```

Run: `go test ./internal/usecase -run TestTransactionService_Create -v` — expect PASS.

- [ ] **Step 5: Plumb through MCP**

Edit `internal/adapter/mcp/tools_transactions.go`:

```go
type CreateTransactionInput struct {
	Description  string             `json:"description" jsonschema:"Short transaction description."`
	Date         string             `json:"date"        jsonschema:"YYYY-MM-DD."`
	AmountCents  int64              `json:"amount_cents" jsonschema:"Cents; negative=expense, positive=income."`
	AccountID    int64              `json:"account_id"   jsonschema:"Source account id."`
	CategoryID   int64              `json:"category_id"  jsonschema:"Category id."`
	Paid         bool               `json:"paid"         jsonschema:"Whether the transaction is already paid."`
	Notes        string             `json:"notes,omitempty"      jsonschema:"Optional notes."`
	ContactID    *int64             `json:"contact_id,omitempty" jsonschema:"Optional contact id."`
	Tags         []domain.Tag       `json:"tags,omitempty"      jsonschema:"Optional tags."`
	Recurrence   *RecurrenceInput   `json:"recurrence,omitempty"   jsonschema:"Optional. Set to create a fixed recurring transaction (recurrence_attributes). Mutually exclusive with installments."`
	Installments *InstallmentsInput `json:"installments,omitempty" jsonschema:"Optional. Set to create an installment-plan transaction (installments_attributes). Mutually exclusive with recurrence."`
}

// InstallmentsInput selects an installment plan for a parcelada create.
type InstallmentsInput struct {
	Periodicity string `json:"periodicity" jsonschema:"One of: weekly, biweekly, monthly, bimonthly, trimonthly, yearly."`
	Total       int    `json:"total"       jsonschema:"Total number of installments (>=1)."`
}
```

Update the handler (`tools_transactions.go:118-136`):

```go
func createTransactionHandler(svc TransactionService) mcpsdk.ToolHandlerFor[CreateTransactionInput, CreateTransactionOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in CreateTransactionInput) (*mcpsdk.CallToolResult, CreateTransactionOutput, error) {
		params := domain.CreateTransactionParams{
			Description: in.Description, Date: in.Date, AmountCents: in.AmountCents,
			AccountID: in.AccountID, CategoryID: in.CategoryID, Paid: in.Paid,
			Notes: in.Notes, ContactID: in.ContactID, Tags: in.Tags,
		}
		if in.Recurrence != nil {
			params.Recurrence = &domain.RecurrenceAttributes{
				Periodicity: domain.Periodicity(in.Recurrence.Periodicity),
			}
		}
		if in.Installments != nil {
			params.Installments = &domain.InstallmentsAttributes{
				Periodicity: domain.Periodicity(in.Installments.Periodicity),
				Total:       in.Installments.Total,
			}
		}
		tx, err := svc.Create(ctx, params)
		if err != nil {
			return nil, CreateTransactionOutput{}, err
		}
		return nil, CreateTransactionOutput{Transaction: *tx}, nil
	}
}
```

Update `create_transaction` tool description (`tools_transactions.go:170-173`):

```go
Description: "Create a new Organizze transaction. amount_cents is negative for expenses, positive for income. For a fixed recurring transaction, pass `recurrence` with a `periodicity` (weekly, biweekly, monthly, bimonthly, trimonthly, yearly). For a parcelada (installment) transaction, pass `installments` with `periodicity` and `total` (number of installments). `recurrence` and `installments` are mutually exclusive.",
```

- [ ] **Step 6: MCP handler test**

Add to `tools_transactions_test.go`:

```go
func TestCreateTransactionHandler_PassesInstallmentsThrough(t *testing.T) {
	var got domain.CreateTransactionParams
	svc := &fakeTransactionSvc{
		create: func(_ context.Context, p domain.CreateTransactionParams) (*domain.Transaction, error) {
			got = p
			return &domain.Transaction{ID: 1, TotalInstallments: p.Installments.Total}, nil
		},
	}
	_, err := callTool[mcp.CreateTransactionInput, mcp.CreateTransactionOutput](
		t, registerTransactionToolsForTest(svc), "create_transaction",
		mcp.CreateTransactionInput{
			Description: "Computador", Date: "2026-05-14", AmountCents: -100000,
			AccountID: 1, CategoryID: 10, Paid: false,
			Installments: &mcp.InstallmentsInput{Periodicity: "monthly", Total: 12},
		},
	)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if got.Installments == nil || got.Installments.Total != 12 || got.Installments.Periodicity != domain.PeriodicityMonthly {
		t.Errorf("Installments = %+v", got.Installments)
	}
}
```

- [ ] **Step 7: Wire test (optional but recommended)**

In `internal/adapter/organizze/transaction_repository_test.go`, append a test asserting `installments_attributes` round-trips on the body if not already covered by the repo's marshal test.

- [ ] **Step 8: Run full suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/domain/transaction.go internal/domain/transaction_test.go internal/usecase/transaction.go internal/usecase/transaction_test.go internal/adapter/mcp/tools_transactions.go internal/adapter/mcp/tools_transactions_test.go internal/adapter/organizze/transaction_repository_test.go
git commit -m "feat(transactions): support installments_attributes (parcelada) on create"
```

---

# Task 11: Transactions — add `update_future` / `update_all` to PUT

**Files:**
- Modify: `internal/domain/transaction.go:105-115` (extend `UpdateTransactionParams`)
- Modify: `internal/adapter/organizze/transaction_repository_test.go` (extend update wire test)
- Modify: `internal/adapter/mcp/tools_transactions.go:66-77, 138-150, 174-177`
- Modify: `internal/adapter/mcp/tools_transactions_test.go`

- [ ] **Step 1: Domain test for JSON round-trip**

Append to `internal/domain/transaction_test.go`:

```go
func TestUpdateTransactionParams_RecurringFlags_Marshal(t *testing.T) {
	uf := true
	p := domain.UpdateTransactionParams{UpdateFuture: &uf}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != `{"update_future":true}` {
		t.Errorf("body = %s", string(b))
	}
}
```

Run: `go test ./internal/domain -run TestUpdateTransactionParams_RecurringFlags -v` — expect compile FAIL.

- [ ] **Step 2: Extend `UpdateTransactionParams`**

Edit `internal/domain/transaction.go:105-115`:

```go
type UpdateTransactionParams struct {
	Description  *string `json:"description,omitempty"`
	Date         *string `json:"date,omitempty"`
	AmountCents  *int64  `json:"amount_cents,omitempty"`
	AccountID    *int64  `json:"account_id,omitempty"`
	CategoryID   *int64  `json:"category_id,omitempty"`
	Paid         *bool   `json:"paid,omitempty"`
	Notes        *string `json:"notes,omitempty"`
	ContactID    *int64  `json:"contact_id,omitempty"`
	Tags         []Tag   `json:"tags,omitempty"`
	UpdateFuture *bool   `json:"update_future,omitempty"`
	UpdateAll    *bool   `json:"update_all,omitempty"`
}
```

- [ ] **Step 3: Extend `UpdateTransactionInput` + handler**

Edit `internal/adapter/mcp/tools_transactions.go:66-77`:

```go
type UpdateTransactionInput struct {
	ID           int64        `json:"id" jsonschema:"The numeric transaction id to update."`
	Description  *string      `json:"description,omitempty"  jsonschema:"New description."`
	Date         *string      `json:"date,omitempty"         jsonschema:"New date YYYY-MM-DD."`
	AmountCents  *int64       `json:"amount_cents,omitempty" jsonschema:"New amount in cents."`
	AccountID    *int64       `json:"account_id,omitempty"   jsonschema:"New account id."`
	CategoryID   *int64       `json:"category_id,omitempty"  jsonschema:"New category id."`
	Paid         *bool        `json:"paid,omitempty"         jsonschema:"New paid flag."`
	Notes        *string      `json:"notes,omitempty"        jsonschema:"New notes."`
	ContactID    *int64       `json:"contact_id,omitempty"   jsonschema:"New contact id."`
	Tags         []domain.Tag `json:"tags,omitempty"         jsonschema:"Replacement tag list."`
	UpdateFuture *bool        `json:"update_future,omitempty" jsonschema:"For recurring/installment series: also apply this update to the current and all FUTURE occurrences."`
	UpdateAll    *bool        `json:"update_all,omitempty"    jsonschema:"For recurring/installment series: also apply this update to ALL occurrences, including past ones. May alter the account balance if past entries were already paid."`
}
```

Update the handler (`tools_transactions.go:138-150`):

```go
tx, err := svc.Update(ctx, in.ID, domain.UpdateTransactionParams{
	Description: in.Description, Date: in.Date, AmountCents: in.AmountCents,
	AccountID: in.AccountID, CategoryID: in.CategoryID, Paid: in.Paid,
	Notes: in.Notes, ContactID: in.ContactID, Tags: in.Tags,
	UpdateFuture: in.UpdateFuture, UpdateAll: in.UpdateAll,
})
```

Bump the `update_transaction` tool description (`tools_transactions.go:174-177`):

```go
Description: "Update fields on an existing Organizze transaction. Only fields you provide are changed; omitted fields are left unchanged. For recurring (fixa) or installment (parcelada) series, set update_future=true to propagate the change to this and all future occurrences, or update_all=true to propagate to every occurrence (may alter past-paid balances).",
```

- [ ] **Step 4: Wire test**

Append to `internal/adapter/organizze/transaction_repository_test.go`:

```go
func TestTransactionRepository_Update_SendsUpdateFuture(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":101,"description":"x","amount_cents":1,"account_id":1,"category_id":1,"date":"2026-05-14"}`)
	}))
	defer ts.Close()
	repo := organizze.NewTransactionRepository(newTestExecutor(t, ts.URL))
	uf := true
	if _, err := repo.Update(context.Background(), 101, domain.UpdateTransactionParams{UpdateFuture: &uf}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if string(gotBody) != `{"update_future":true}` {
		t.Errorf("body = %q", string(gotBody))
	}
}
```

- [ ] **Step 5: Run full suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/transaction.go internal/domain/transaction_test.go internal/adapter/organizze/transaction_repository_test.go internal/adapter/mcp/tools_transactions.go
git commit -m "feat(transactions): support update_future and update_all on update"
```

---

# Task 12: Transactions — add `update_future` / `update_all` to DELETE

**Why:** `ORGANIZZE_API.md:1266-1281`:
> "No caso de movimentações fixas ou parceladas, para excluir a movimentação e as próximas ocorrências envie o atributo `"update_future": true`; Caso queira excluir todas as ocorrências, inclusive as anteriores, envie o atributo `"update_all": true`."

This requires the Task 1 DELETE-with-body capability plus a new `DeleteTransactionParams` carried through usecase + MCP layers.

**Files:**
- Modify: `internal/domain/transaction.go` (new `DeleteTransactionParams` type)
- Modify: `internal/adapter/organizze/transaction_repository.go:73-76` (Delete signature)
- Modify: `internal/adapter/organizze/transaction_repository_test.go`
- Modify: `internal/usecase/transaction.go:17-21, 58-60`
- Modify: `internal/usecase/transaction_test.go`
- Modify: `internal/adapter/mcp/tools_transactions.go:11-18, 83-92, 152-159, 178-181`
- Modify: `internal/adapter/mcp/tools_transactions_test.go`

- [ ] **Step 1: Wire test for DELETE-with-body**

Append to `internal/adapter/organizze/transaction_repository_test.go`:

```go
func TestTransactionRepository_Delete_SendsBodyWhenFlagsSet(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()
	repo := organizze.NewTransactionRepository(newTestExecutor(t, ts.URL))
	uf := true
	if err := repo.Delete(context.Background(), 101, domain.DeleteTransactionParams{UpdateFuture: &uf}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if string(gotBody) != `{"update_future":true}` {
		t.Errorf("body = %q", string(gotBody))
	}
}

func TestTransactionRepository_Delete_NoFlags_SendsNoBody(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()
	repo := organizze.NewTransactionRepository(newTestExecutor(t, ts.URL))
	if err := repo.Delete(context.Background(), 101, domain.DeleteTransactionParams{}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(gotBody) != 0 {
		t.Errorf("body = %q on empty params, want empty", string(gotBody))
	}
}
```

Run: `go test ./internal/adapter/organizze -run TestTransactionRepository_Delete -v` — expect compile FAIL.

- [ ] **Step 2: Add `DeleteTransactionParams`**

Append to `internal/domain/transaction.go`:

```go
// DeleteTransactionParams scopes a DELETE to a single occurrence (default) or
// the recurring/installment series. UpdateFuture and UpdateAll are mutually
// exclusive; only one may be set. Zero value means "delete this occurrence
// only" and produces an empty request body.
type DeleteTransactionParams struct {
	UpdateFuture *bool `json:"update_future,omitempty"`
	UpdateAll    *bool `json:"update_all,omitempty"`
}

// IsZero reports whether the params would marshal to an empty JSON object.
func (p DeleteTransactionParams) IsZero() bool {
	return p.UpdateFuture == nil && p.UpdateAll == nil
}
```

- [ ] **Step 3: Update repository**

Replace `internal/adapter/organizze/transaction_repository.go:73-76`:

```go
// Delete issues a DELETE. If params is non-zero, its fields ride along as a
// JSON body so Organizze can apply the deletion to a recurring/installment
// series per `ORGANIZZE_API.md` "Excluir movimentação".
func (r *TransactionRepository) Delete(ctx context.Context, id int64, params domain.DeleteTransactionParams) error {
	var body any
	if !params.IsZero() {
		body = params
	}
	return r.exec.Delete(ctx, fmt.Sprintf("/transactions/%d", id), body, nil)
}
```

- [ ] **Step 4: Update usecase interface + service**

Edit `internal/usecase/transaction.go:17-21`:

```go
type TransactionWriter interface {
	Create(ctx context.Context, params domain.CreateTransactionParams) (*domain.Transaction, error)
	Update(ctx context.Context, id int64, params domain.UpdateTransactionParams) (*domain.Transaction, error)
	Delete(ctx context.Context, id int64, params domain.DeleteTransactionParams) error
}
```

And the service method (`internal/usecase/transaction.go:58-60`):

```go
func (s *TransactionService) Delete(ctx context.Context, id int64, p domain.DeleteTransactionParams) error {
	if p.UpdateFuture != nil && p.UpdateAll != nil {
		return fmt.Errorf("%w: update_future and update_all are mutually exclusive", domain.ErrValidation)
	}
	return s.repo.Delete(ctx, id, p)
}
```

Add a test:

```go
func TestTransactionService_Delete_RejectsBothFlags(t *testing.T) {
	svc := usecase.NewTransactionService(&fakeTransactionRepo{})
	tt := true
	err := svc.Delete(context.Background(), 1, domain.DeleteTransactionParams{UpdateFuture: &tt, UpdateAll: &tt})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}
```

- [ ] **Step 5: Update MCP layer**

Edit `internal/adapter/mcp/tools_transactions.go:17`:

```go
Delete(ctx context.Context, id int64, params domain.DeleteTransactionParams) error
```

Edit `internal/adapter/mcp/tools_transactions.go:83-92`:

```go
type DeleteTransactionInput struct {
	ID           int64 `json:"id" jsonschema:"The numeric transaction id to delete."`
	UpdateFuture *bool `json:"update_future,omitempty" jsonschema:"For recurring/installment series: also delete the current and all FUTURE occurrences. Mutually exclusive with update_all."`
	UpdateAll    *bool `json:"update_all,omitempty"    jsonschema:"For recurring/installment series: delete ALL occurrences, including past ones. May alter the account balance if past entries were paid. Mutually exclusive with update_future."`
}
```

Edit `internal/adapter/mcp/tools_transactions.go:152-159`:

```go
func deleteTransactionHandler(svc TransactionService) mcpsdk.ToolHandlerFor[DeleteTransactionInput, DeleteTransactionOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in DeleteTransactionInput) (*mcpsdk.CallToolResult, DeleteTransactionOutput, error) {
		if err := svc.Delete(ctx, in.ID, domain.DeleteTransactionParams{
			UpdateFuture: in.UpdateFuture, UpdateAll: in.UpdateAll,
		}); err != nil {
			return nil, DeleteTransactionOutput{}, err
		}
		return nil, DeleteTransactionOutput{Deleted: true, ID: in.ID}, nil
	}
}
```

Bump tool description (`tools_transactions.go:178-181`):

```go
Description: "Permanently delete an Organizze transaction by id. For recurring (fixa) or installment (parcelada) series, set update_future=true to also delete this and all future occurrences, or update_all=true to delete every occurrence (may alter past-paid balances). The two flags are mutually exclusive.",
```

- [ ] **Step 6: Update all `Delete(ctx, id)` callsites in fakes / handler tests**

The fakes in `usecase/transaction_test.go` and `tools_transactions_test.go` need their `Delete` method signature updated to accept `domain.DeleteTransactionParams`. Existing test cases passing zero params remain unchanged in semantic.

- [ ] **Step 7: Run full suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/domain/transaction.go internal/adapter/organizze/transaction_repository.go internal/adapter/organizze/transaction_repository_test.go internal/usecase/transaction.go internal/usecase/transaction_test.go internal/adapter/mcp/tools_transactions.go internal/adapter/mcp/tools_transactions_test.go
git commit -m "feat(transactions): support update_future and update_all on delete"
```

---

# Task 13: Transactions — surface deleted transaction in delete output

Mirror of Tasks 3 / 5 / 7 for transactions. Same `DeleteXOutput` shape.

**Files:**
- Modify: `internal/adapter/organizze/transaction_repository.go` (Task 12's Delete returns the body)
- Modify: `internal/usecase/transaction.go` (Delete returns `(*domain.Transaction, error)`)
- Modify: `internal/adapter/mcp/tools_transactions.go` (interface + Output + handler)
- Modify: matching test files

- [ ] **Step 1: Repository test**

Replace `TestTransactionRepository_Delete_*` (added in Task 12) with versions that also assert a returned transaction:

```go
func TestTransactionRepository_Delete_ReturnsDeletedTransaction(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":101,"description":"x","amount_cents":1,"account_id":1,"category_id":1,"date":"2026-05-14"}`)
	}))
	defer ts.Close()
	repo := organizze.NewTransactionRepository(newTestExecutor(t, ts.URL))
	tx, err := repo.Delete(context.Background(), 101, domain.DeleteTransactionParams{})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if tx == nil || tx.ID != 101 {
		t.Errorf("returned = %+v", tx)
	}
}
```

- [ ] **Step 2: Repository signature**

Replace the `Delete` method written in Task 12:

```go
func (r *TransactionRepository) Delete(ctx context.Context, id int64, params domain.DeleteTransactionParams) (*domain.Transaction, error) {
	var body any
	if !params.IsZero() {
		body = params
	}
	var out domain.Transaction
	if err := r.exec.Delete(ctx, fmt.Sprintf("/transactions/%d", id), body, &out); err != nil {
		return nil, err
	}
	// Organizze may return 204 No Content for non-series deletes; the executor
	// will leave out as zero. Caller-side branch is safe either way.
	if out.ID == 0 {
		return nil, nil
	}
	return &out, nil
}
```

- [ ] **Step 3: Usecase + MCP signature**

Update `TransactionWriter.Delete`, `TransactionService.Delete`, `TransactionService` interface in MCP, all returning `(*domain.Transaction, error)`. Update the handler:

```go
func deleteTransactionHandler(svc TransactionService) mcpsdk.ToolHandlerFor[DeleteTransactionInput, DeleteTransactionOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in DeleteTransactionInput) (*mcpsdk.CallToolResult, DeleteTransactionOutput, error) {
		tx, err := svc.Delete(ctx, in.ID, domain.DeleteTransactionParams{
			UpdateFuture: in.UpdateFuture, UpdateAll: in.UpdateAll,
		})
		if err != nil {
			return nil, DeleteTransactionOutput{}, err
		}
		return nil, DeleteTransactionOutput{Deleted: true, ID: in.ID, Transaction: tx}, nil
	}
}
```

And `DeleteTransactionOutput`:

```go
type DeleteTransactionOutput struct {
	Deleted     bool                `json:"deleted"`
	ID          int64               `json:"id"`
	Transaction *domain.Transaction `json:"transaction,omitempty"`
}
```

- [ ] **Step 4: Update fakes and tests**

The fakes in `usecase/transaction_test.go` and `tools_transactions_test.go` updated in Task 12 need their `Delete` to return a transaction.

- [ ] **Step 5: Run full suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/organizze/transaction_repository.go internal/adapter/organizze/transaction_repository_test.go internal/usecase/transaction.go internal/usecase/transaction_test.go internal/adapter/mcp/tools_transactions.go internal/adapter/mcp/tools_transactions_test.go
git commit -m "feat(transactions): surface deleted transaction snapshot in delete_transaction output"
```

---

# Task 14: Transfers — expand Transfer domain struct with all documented fields

**Why:** `ORGANIZZE_API.md:1326-1350` documents 12+ response fields that the current `domain.Transfer` (16 lines, 11 fields) drops on the floor. Adding them makes List/Get/Create/Update/Delete responses fully informative without changing wire calls.

**Files:**
- Modify: `internal/domain/transfer.go:4-16` (add fields + new `Attachment` placeholder type)
- Update: any test that pattern-matches on `domain.Transfer{...}` may need new fields (use `cmp.Diff` rather than struct equality where possible — existing repo tests likely do)

- [ ] **Step 1: Failing JSON round-trip test**

Add to `internal/domain/transfer_test.go` (create if missing):

```go
package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

func TestTransfer_UnmarshalsAllDocumentedFields(t *testing.T) {
	in := `{
		"id": 10, "description": "Transferência", "date": "2015-09-01",
		"paid": true, "amount_cents": -10000,
		"total_installments": 1, "installment": 1, "recurring": false,
		"account_id": 3, "category_id": 21, "notes": null,
		"attachments_count": 0,
		"credit_card_id": null, "credit_card_invoice_id": null,
		"paid_credit_card_id": null, "paid_credit_card_invoice_id": null,
		"oposite_transaction_id": 11, "oposite_account_id": 4,
		"created_at": "2015-09-01T23:42:29-03:00",
		"updated_at": "2015-09-01T23:42:29-03:00",
		"tags": [], "attachments": [], "recurrence_id": null,
		"deleted": true
	}`
	var tr domain.Transfer
	if err := json.Unmarshal([]byte(in), &tr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	checks := map[string]bool{
		"ID":                    tr.ID == 10,
		"TotalInstallments":     tr.TotalInstallments == 1,
		"Installment":           tr.Installment == 1,
		"Recurring=false":       tr.Recurring == false,
		"AttachmentsCount=0":    tr.AttachmentsCount == 0,
		"OppositeTransactionID": tr.OppositeTransactionID != nil && *tr.OppositeTransactionID == 11,
		"CreatedAt":             tr.CreatedAt == "2015-09-01T23:42:29-03:00",
		"UpdatedAt":             tr.UpdatedAt == "2015-09-01T23:42:29-03:00",
		"Tags non-nil":          tr.Tags != nil,
		"Attachments non-nil":   tr.Attachments != nil,
		"Deleted":               tr.Deleted,
	}
	for label, ok := range checks {
		if !ok {
			t.Errorf("check failed: %s; transfer = %+v", label, tr)
		}
	}
}
```

Run: `go test ./internal/domain -run TestTransfer_UnmarshalsAllDocumented -v` — expect compile FAIL.

- [ ] **Step 2: Expand `domain.Transfer`**

Replace `internal/domain/transfer.go:4-16` with:

```go
// Transfer is a movement of money between two accounts. The JSON shape mirrors
// `ORGANIZZE_API.md` "Transfers" — see Listar/Detalhar/Criar/Atualizar/Excluir.
type Transfer struct {
	ID                       int64             `json:"id"`
	Description              string            `json:"description"`
	Date                     string            `json:"date"`
	Paid                     bool              `json:"paid"`
	AmountCents              int64             `json:"amount_cents"`
	TotalInstallments        int               `json:"total_installments,omitempty"`
	Installment              int               `json:"installment,omitempty"`
	Recurring                bool              `json:"recurring,omitempty"`
	AccountID                int64             `json:"account_id"`
	CategoryID               int64             `json:"category_id"`
	Notes                    string            `json:"notes,omitempty"`
	AttachmentsCount         int               `json:"attachments_count,omitempty"`
	CreditCardID             *int64            `json:"credit_card_id,omitempty"`
	CreditCardInvoiceID      *int64            `json:"credit_card_invoice_id,omitempty"`
	PaidCreditCardID         *int64            `json:"paid_credit_card_id,omitempty"`
	PaidCreditCardInvoiceID  *int64            `json:"paid_credit_card_invoice_id,omitempty"`
	OppositeTransactionID    *int64            `json:"oposite_transaction_id,omitempty"`
	OppositeAccountID        *int64            `json:"oposite_account_id,omitempty"`
	RecurrenceID             *int64            `json:"recurrence_id,omitempty"`
	Tags                     []Tag             `json:"tags,omitempty"`
	Attachments              []json.RawMessage `json:"attachments,omitempty"`
	CreatedAt                string            `json:"created_at,omitempty"`
	UpdatedAt                string            `json:"updated_at,omitempty"`
	Deleted                  bool              `json:"deleted,omitempty"`
}
```

Add `"encoding/json"` to the file's imports.

Why `[]json.RawMessage` for `Attachments`: the API doc only shows `"attachments": []` and never reveals the element shape. RawMessage round-trips losslessly without committing to an invented schema.

Why `OppositeAccountID`/`OppositeTransactionID` become `*int64`: per the doc they can be `null` (in transactions where there's no opposite). Existing tests passing zero values keep working under JSON omitempty.

- [ ] **Step 3: Repair test/fixture call sites**

Run: `go build ./...`

If any usecase or repo test constructs `domain.Transfer{... OppositeAccountID: 4 ...}` with the old non-pointer types, update those construction sites to use pointer values, e.g.:

```go
oppoAcct := int64(4)
domain.Transfer{ID: 10, OppositeAccountID: &oppoAcct}
```

A grep pass will surface them: `grep -rn "OppositeAccountID\|OppositeTransactionID" internal/`.

- [ ] **Step 4: Run full suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/transfer.go internal/domain/transfer_test.go internal/usecase/transfer_test.go internal/adapter/mcp/tools_transfers_test.go internal/adapter/organizze/transfer_repository_test.go
git commit -m "feat(transfers): model every documented response field on domain.Transfer"
```

Note: only the test files that actually changed need to be staged; the grep in Step 3 dictates the list.

---

# Task 15: Transfers — add `get_transfer` endpoint

**Why:** `ORGANIZZE_API.md:1354-1388` documents `GET /transfers/{id}` — the only resource where the MCP currently lacks a `get_*` tool.

**Files:**
- Modify: `internal/adapter/organizze/transfer_repository.go` (add `Get`)
- Modify: `internal/adapter/organizze/transfer_repository_test.go`
- Modify: `internal/usecase/transfer.go` (add reader interface method + service method)
- Modify: `internal/usecase/transfer_test.go`
- Modify: `internal/adapter/mcp/tools_transfers.go` (interface + Input/Output + handler + tool registration)
- Modify: `internal/adapter/mcp/tools_transfers_test.go`

- [ ] **Step 1: Repository test**

Append to `internal/adapter/organizze/transfer_repository_test.go`:

```go
func TestTransferRepository_Get_HitsTransferURL(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":10,"description":"Transferência","amount_cents":-10000,"account_id":3,"date":"2015-09-01"}`)
	}))
	defer ts.Close()
	repo := organizze.NewTransferRepository(newTestExecutor(t, ts.URL))
	tr, err := repo.Get(context.Background(), 10)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if gotPath != "/transfers/10" {
		t.Errorf("path = %q", gotPath)
	}
	if tr == nil || tr.ID != 10 {
		t.Errorf("returned = %+v", tr)
	}
}
```

Run: `go test ./internal/adapter/organizze -run TestTransferRepository_Get -v` — expect compile FAIL.

- [ ] **Step 2: Add `Get` to repository**

Append to `internal/adapter/organizze/transfer_repository.go`:

```go
// Get returns a single transfer by id.
func (r *TransferRepository) Get(ctx context.Context, id int64) (*domain.Transfer, error) {
	var tr domain.Transfer
	if err := r.exec.Get(ctx, fmt.Sprintf("/transfers/%d", id), &tr); err != nil {
		return nil, err
	}
	return &tr, nil
}
```

- [ ] **Step 3: Extend usecase**

Edit `internal/usecase/transfer.go`:

```go
type TransferReader interface {
	List(ctx context.Context, filter domain.ListTransfersFilter) ([]domain.Transfer, error)
	Get(ctx context.Context, id int64) (*domain.Transfer, error)
}

func (s *TransferService) Get(ctx context.Context, id int64) (*domain.Transfer, error) {
	return s.repo.Get(ctx, id)
}
```

- [ ] **Step 4: Add MCP tool**

Edit `internal/adapter/mcp/tools_transfers.go`:

```go
type TransferService interface {
	List(ctx context.Context, filter domain.ListTransfersFilter) ([]domain.Transfer, error)
	Get(ctx context.Context, id int64) (*domain.Transfer, error)
	Create(ctx context.Context, params domain.CreateTransferParams) (*domain.Transfer, error)
	Update(ctx context.Context, id int64, params domain.UpdateTransferParams) (*domain.Transfer, error)
	Delete(ctx context.Context, id int64) (*domain.Transfer, error)  // see Task 16
}

type GetTransferInput struct {
	ID int64 `json:"id" jsonschema:"The numeric Organizze transfer id."`
}

type GetTransferOutput struct {
	Transfer domain.Transfer `json:"transfer"`
}

func getTransferHandler(svc TransferService) mcpsdk.ToolHandlerFor[GetTransferInput, GetTransferOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetTransferInput) (*mcpsdk.CallToolResult, GetTransferOutput, error) {
		tr, err := svc.Get(ctx, in.ID)
		if err != nil {
			return nil, GetTransferOutput{}, err
		}
		return nil, GetTransferOutput{Transfer: *tr}, nil
	}
}
```

Register the tool in `registerTransferTools` (between `list_transfers` and `create_transfer`):

```go
mcpsdk.AddTool(s, &mcpsdk.Tool{
	Name:        "get_transfer",
	Description: "Fetch a single Organizze transfer by id.",
}, getTransferHandler(svc))
```

Note: the `Delete` return signature widens to `(*domain.Transfer, error)` here in anticipation of Task 16. If you prefer to keep the two tasks atomic, leave it as `error` in this commit and update the signature in Task 16. To keep the suite green between tasks, the recommended order is to land Tasks 15 and 16 together.

- [ ] **Step 5: Update fakes**

In `internal/usecase/transfer_test.go` and `internal/adapter/mcp/tools_transfers_test.go`, add `Get` to the fakes. Add a happy-path handler test for `get_transfer`.

- [ ] **Step 6: Run full suite**

Run: `go test ./...`
Expected: PASS in `usecase` and `organizze`; `integration_test.go` will fail on tool count + missing fake route — fixed in Task 17.

- [ ] **Step 7: Don't commit yet**

Combine with Task 16 (same `Delete` signature change).

---

# Task 16: Transfers — surface deleted transfer and read API's `deleted` field

**Why:** `ORGANIZZE_API.md:1495-1518` shows the DELETE response carries the full transfer object plus `"deleted": true`. The current `deleteTransferHandler` (`tools_transfers.go:95-101`) fabricates `Deleted: true` client-side. We now read the real value.

This mirrors Tasks 5, 7, 13 — same `DeleteXOutput` shape.

**Files:**
- Modify: `internal/adapter/organizze/transfer_repository.go:58-61` (Delete returns transfer)
- Modify: `internal/adapter/organizze/transfer_repository_test.go`
- Modify: `internal/usecase/transfer.go` (interface + service `Delete` returns `(*domain.Transfer, error)`)
- Modify: `internal/usecase/transfer_test.go`
- Modify: `internal/adapter/mcp/tools_transfers.go:51-58, 95-101`
- Modify: `internal/adapter/mcp/tools_transfers_test.go`

- [ ] **Step 1: Repository test**

Append to `internal/adapter/organizze/transfer_repository_test.go`:

```go
func TestTransferRepository_Delete_ReturnsDeletedTransferWithDeletedTrue(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":10,"description":"Transferência","amount_cents":-10000,"account_id":3,"date":"2015-09-01","deleted":true}`)
	}))
	defer ts.Close()
	repo := organizze.NewTransferRepository(newTestExecutor(t, ts.URL))
	tr, err := repo.Delete(context.Background(), 10)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if tr == nil || tr.ID != 10 || !tr.Deleted {
		t.Errorf("returned = %+v", tr)
	}
}
```

(`Deleted` was added to `domain.Transfer` in Task 14.)

- [ ] **Step 2: Update repository**

Replace `internal/adapter/organizze/transfer_repository.go:58-61`:

```go
// Delete issues a DELETE and returns the deleted transfer snapshot (with
// Deleted=true) as echoed by Organizze.
func (r *TransferRepository) Delete(ctx context.Context, id int64) (*domain.Transfer, error) {
	var out domain.Transfer
	if err := r.exec.Delete(ctx, fmt.Sprintf("/transfers/%d", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
```

- [ ] **Step 3: Update usecase**

Edit `internal/usecase/transfer.go`:

```go
type TransferWriter interface {
	Create(ctx context.Context, params domain.CreateTransferParams) (*domain.Transfer, error)
	Update(ctx context.Context, id int64, params domain.UpdateTransferParams) (*domain.Transfer, error)
	Delete(ctx context.Context, id int64) (*domain.Transfer, error)
}

func (s *TransferService) Delete(ctx context.Context, id int64) (*domain.Transfer, error) {
	return s.repo.Delete(ctx, id)
}
```

- [ ] **Step 4: Update MCP layer**

Edit `internal/adapter/mcp/tools_transfers.go:51-58`:

```go
type DeleteTransferOutput struct {
	Deleted  bool             `json:"deleted"`
	ID       int64            `json:"id"`
	Transfer *domain.Transfer `json:"transfer,omitempty"`
}
```

And `deleteTransferHandler` (`tools_transfers.go:95-101`):

```go
func deleteTransferHandler(svc TransferService) mcpsdk.ToolHandlerFor[DeleteTransferInput, DeleteTransferOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in DeleteTransferInput) (*mcpsdk.CallToolResult, DeleteTransferOutput, error) {
		tr, err := svc.Delete(ctx, in.ID)
		if err != nil {
			return nil, DeleteTransferOutput{}, err
		}
		deleted := true
		if tr != nil {
			deleted = tr.Deleted || true // API echoes deleted=true; on 204 we still report deleted=true
		}
		return nil, DeleteTransferOutput{Deleted: deleted, ID: in.ID, Transfer: tr}, nil
	}
}
```

(Simpler form acceptable: always set `Deleted: true` since the only way to reach this branch is a successful 2xx; the optional `Transfer` carries the API's snapshot.)

- [ ] **Step 5: Update fakes**

Update `internal/usecase/transfer_test.go` and `internal/adapter/mcp/tools_transfers_test.go` `Delete` fakes to return `(&domain.Transfer{ID: id, Deleted: true}, nil)`.

- [ ] **Step 6: Run full suite**

Run: `go test ./... 2>&1 | grep -E "FAIL|ok"`
Expected: every package PASS, except `internal/adapter/mcp` integration tests which will be fixed in Task 17.

- [ ] **Step 7: Commit Tasks 15 + 16 together**

```bash
git add internal/adapter/organizze/transfer_repository.go internal/adapter/organizze/transfer_repository_test.go internal/usecase/transfer.go internal/usecase/transfer_test.go internal/adapter/mcp/tools_transfers.go internal/adapter/mcp/tools_transfers_test.go
git commit -m "feat(transfers): add get_transfer; surface deleted transfer in delete_transfer output"
```

---

# Task 17: Integration test + README catalogue update

**Files:**
- Modify: `internal/adapter/mcp/integration_test.go` (fake routes + tool list + roundtrip cases)
- Modify: `README.md` (tool catalogue)

- [ ] **Step 1: Add new fake-server routes**

Edit `internal/adapter/mcp/integration_test.go:22-103` — the `fakeOrganizze` switch. Add:

```go
case r.Method == http.MethodGet && r.URL.Path == "/transfers/123":
	_, _ = io.WriteString(w, `{"id":123,"description":"Transferência","amount_cents":-50000,"account_id":2,"oposite_account_id":1,"date":"2026-05-14"}`)
case r.Method == http.MethodGet && r.URL.Path == "/credit_cards/1/invoices/100/payments":
	_, _ = io.WriteString(w, `{"id":1033,"description":"Pagamento fatura","amount_cents":0,"account_id":1,"category_id":10,"date":"2026-05-14"}`)
```

Also: existing `/transfers` (line 49), `/credit_cards/1/invoices` (line 45), and any others now exercise the new domain fields. The existing responses are minimal-but-valid JSON that the expanded `Transfer`/`Invoice` types will still unmarshal cleanly because every new field is `omitempty`. No edit needed there.

- [ ] **Step 2: Update `allExpectedTools`**

Edit `internal/adapter/mcp/integration_test.go:156-170`:

```go
var allExpectedTools = []string{
	"get_user",
	"list_accounts", "get_account",
	"create_account", "update_account", "delete_account",
	"list_categories", "get_category",
	"create_category", "update_category", "delete_category",
	"list_budgets",
	"list_credit_cards", "get_credit_card",
	"create_credit_card", "update_credit_card", "delete_credit_card",
	"list_credit_card_invoices", "get_credit_card_invoice",
	"get_credit_card_invoice_payment",
	"list_transfers", "get_transfer",
	"create_transfer", "update_transfer", "delete_transfer",
	"list_transactions", "get_transaction",
	"create_transaction", "update_transaction", "delete_transaction",
}
```

(30 tools.)

- [ ] **Step 3: Update the roundtrip table**

Add cases to `TestIntegration_EveryToolRoundtripsThroughProtocol` (`integration_test.go:208+`):

```go
{"get_credit_card_invoice_payment", "get_credit_card_invoice_payment",
	map[string]any{"credit_card_id": 1, "invoice_id": 100}},
{"get_transfer", "get_transfer", map[string]any{"id": 123}},
```

- [ ] **Step 4: Update README catalogue**

Edit `README.md` lines 96-123. The new table:

```markdown
| 1  | `get_user`                          | UserService.Get                       | no        |
| 2  | `list_accounts`                     | AccountService.List                   | no        |
| 3  | `get_account`                       | AccountService.Get                    | no        |
| 4  | `create_account`                    | AccountService.Create                 | **yes**   |
| 5  | `update_account`                    | AccountService.Update                 | **yes**   |
| 6  | `delete_account`                    | AccountService.Delete                 | **yes**   |
| 7  | `list_categories`                   | CategoryService.List                  | no        |
| 8  | `get_category`                      | CategoryService.Get                   | no        |
| 9  | `create_category`                   | CategoryService.Create                | **yes**   |
| 10 | `update_category`                   | CategoryService.Update                | **yes**   |
| 11 | `delete_category`                   | CategoryService.Delete                | **yes**   |
| 12 | `list_budgets`                      | BudgetService.List (period routing)   | no        |
| 13 | `list_credit_cards`                 | CreditCardService.List                | no        |
| 14 | `get_credit_card`                   | CreditCardService.Get                 | no        |
| 15 | `create_credit_card`                | CreditCardService.Create              | **yes**   |
| 16 | `update_credit_card`                | CreditCardService.Update              | **yes**   |
| 17 | `delete_credit_card`                | CreditCardService.Delete              | **yes**   |
| 18 | `list_credit_card_invoices`         | InvoiceService.List                   | no        |
| 19 | `get_credit_card_invoice`           | InvoiceService.Get                    | no        |
| 20 | `get_credit_card_invoice_payment`   | InvoiceService.Payment                | no        |
| 21 | `list_transfers`                    | TransferService.List                  | no        |
| 22 | `get_transfer`                      | TransferService.Get                   | no        |
| 23 | `create_transfer`                   | TransferService.Create                | **yes**   |
| 24 | `update_transfer`                   | TransferService.Update                | **yes**   |
| 25 | `delete_transfer`                   | TransferService.Delete                | **yes**   |
| 26 | `list_transactions`                 | TransactionService.List               | no        |
| 27 | `get_transaction`                   | TransactionService.Get                | no        |
| 28 | `create_transaction`                | TransactionService.Create             | **yes**   |
| 29 | `update_transaction`                | TransactionService.Update             | **yes**   |
| 30 | `delete_transaction`                | TransactionService.Delete             | **yes**   |
```

Also: scan the README for any narrative that references "16 tools" or "28 tools" and update.

- [ ] **Step 5: Run full suite**

Run: `go test ./...`
Expected: every package PASS.

- [ ] **Step 6: Run `go vet` + build**

Run: `go vet ./... && go build ./...`
Expected: clean.

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/mcp/integration_test.go README.md
git commit -m "test+docs: integration coverage and README catalogue for new tools"
```

---

# Self-review checklist

Before declaring this plan done, the implementer should verify:

1. **Spec coverage** — every row in the "Audit summary" table at the top has a corresponding task that landed.

2. **No placeholders** — search the codebase for `TODO`, `FIXME`, or `tbd` introduced by this plan. None should remain.

3. **Type consistency** — every `Delete*Output` shape is `{Deleted bool, ID int64, X *domain.X}`. Every `Delete` repository method follows `(ctx, id, ...) → (*domain.X, error)` (or `error` for the no-body category case before Task 2 lands).

4. **API coverage** — every `POST`, `PUT`, `DELETE`, `GET` listed in `ORGANIZZE_API.md` has at least one MCP tool that exercises it:
   - GET /users/:id → `get_user`
   - GET /accounts, /accounts/:id, POST, PUT, DELETE → `list_accounts`, `get_account`, `create_account`, `update_account`, `delete_account`
   - GET /budgets[/year[/month]] → `list_budgets`
   - GET /categories, /categories/:id, POST, PUT, DELETE → 5 tools
   - GET /credit_cards, /credit_cards/:id, POST, PUT, DELETE → 5 tools
   - GET /credit_cards/:id/invoices, /credit_cards/:id/invoices/:invoice_id → 2 tools
   - GET /credit_cards/:id/invoices/:invoice_id/payments → `get_credit_card_invoice_payment` (Task 9)
   - GET /transfers, /transfers/:id, POST, PUT, DELETE → 5 tools (Task 15 adds the missing Get)
   - GET /transactions, /transactions/:id, POST (basic/recurrence/installments), PUT, DELETE → 5 tools

5. **No regressions on field projection** — every `domain.X` is rendered verbatim through MCP outputs; no MCP tool selects a subset of fields.

6. **Tool descriptions explain non-obvious wire semantics** — recurring/installment flags, replacement_id reassignment, date-range pagination caps, balance-impact warnings.

---

# Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-14-full-api-parity.md`. Two execution options:

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks, fast iteration. Each task is sized for one subagent invocation; Tasks 2+3 and 15+16 are intentionally paired (same signature change) and should be done in a single dispatch each.

2. **Inline Execution** — execute tasks 1 → 17 in this session using executing-plans, with a checkpoint after the foundation (Task 1), after the delete-output convention is rolled out (Tasks 2-7), after invoices (Tasks 8-9), after transactions (Tasks 10-13), after transfers (Tasks 14-16), and before pushing the docs commit (Task 17).

Which approach?
