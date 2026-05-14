# Organizze MCP — Accounts/Categories/CreditCards/Transfers Write & Delete

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the existing Organizze MCP server with write (POST + PUT) and delete (DELETE) operations for four resources that are currently read-only — **accounts**, **categories**, **credit cards**, **transfers** — so every mutating endpoint documented at https://github.com/organizze/api-doc is exposed as an MCP tool. Transactions already cover the full CRUD surface; this plan brings the remaining four resources to parity.

**Architecture:** No new layers, files, or interfaces beyond what already exists for transactions. For each of the four resources we replicate the transaction-shaped pattern exactly:

1. `internal/domain/<resource>.go` — add `Create<Resource>Params` + `Update<Resource>Params` value objects.
2. `internal/adapter/organizze/<resource>_repository.go` — add `Create`, `Update`, `Delete` methods on the existing repository struct. (Tests live alongside.)
3. `internal/usecase/<resource>.go` — split the existing repository interface into `<Resource>Reader` + `<Resource>Writer` + a composed `<Resource>Repository`, add `Create`/`Update`/`Delete` to the service with required-field validation, mirror the transaction style.
4. `internal/adapter/mcp/tools_<resource>.go` — extend the consumer-side `<Resource>Service` interface, add `Create<Resource>Input/Output`, `Update<Resource>Input/Output`, `Delete<Resource>Input/Output`, three new handlers, and three `mcpsdk.AddTool` calls in `register<Resource>Tools`.
5. `internal/adapter/mcp/integration_test.go` — extend `fakeOrganizze`, `allExpectedTools`, and the roundtrip table with the new 12 tools.
6. `README.md` — update the tool catalogue.

**Reference patterns** to mirror line-for-line (replace `Transaction`/`transaction` with the target resource name):
- Domain types: `internal/domain/transaction.go:39-62`
- Repository: `internal/adapter/organizze/transaction_repository.go:55-76`
- Repository tests: `internal/adapter/organizze/transaction_repository_test.go:51-121`
- Reader/Writer/Repository interfaces + service: `internal/usecase/transaction.go:10-77`
- Service tests: `internal/usecase/transaction_test.go:11-107`
- MCP tool inputs/outputs/handlers: `internal/adapter/mcp/tools_transactions.go:40-170`
- MCP handler tests: `internal/adapter/mcp/tools_transactions_test.go:31-144`

**Tech Stack:** Unchanged. Go ≥ 1.23, stdlib `net/http`/`encoding/json`/`testing`/`net/http/httptest`, `github.com/modelcontextprotocol/go-sdk`.

**New tool catalogue (12 tools added; 16 → 28):**

| Tool | Service.Method | Mutating? |
|---|---|---|
| `create_account`, `update_account`, `delete_account` | AccountService.{Create, Update, Delete} | **yes** |
| `create_category`, `update_category`, `delete_category` | CategoryService.{Create, Update, Delete} | **yes** |
| `create_credit_card`, `update_credit_card`, `delete_credit_card` | CreditCardService.{Create, Update, Delete} | **yes** |
| `create_transfer`, `update_transfer`, `delete_transfer` | TransferService.{Create, Update, Delete} | **yes** |

`delete_category` accepts an optional `replacement_id` query parameter that the Organizze API uses to reassign affected transactions when a category is removed (documented as *"Ao excluir uma categoria você pode informar uma categoria para substitui-la"*).

**Delete response convention:** Match the existing `delete_transaction` pattern — the repository discards the response body (uses `exec.Delete`) and the MCP handler returns `{deleted: true, id: <id>}`. The Organizze API does return the deleted object for these endpoints, but the consistent shape with `delete_transaction` is preferable to a per-resource bespoke output type. Do not change this without a reason.

---

## Design principles applied

These are the same principles that drove the original plan (see `docs/superpowers/plans/2026-05-14-organizze-mcp.md:46-71`). The short version:

- **DRY at one boundary:** all auth, JSON, and HTTP-error mapping stays in `adapter/organizze/executor.go`. Repository write methods are 3 lines apiece: marshal nothing, delegate, unmarshal.
- **Interface Segregation:** every repository that grows a writer side gets split into `<Resource>Reader` + `<Resource>Writer` + composed `<Resource>Repository`, mirroring `usecase/transaction.go:10-27`.
- **Open/Closed:** no edits to the executor, no edits to the composition root other than the resource imports already wired. The composition root already calls `usecase.New<Resource>Service(organizze.New<Resource>Repository(exec))` — the same constructors keep working because we only add methods, never rename.
- **TDD:** every step writes the test first, runs red, then green, then commits.
- **One commit per resource per layer** keeps the diff reviewable.

---

## Required-field validation matrix

Validation lives in the service layer (mirrors `validateCreate` in `internal/usecase/transaction.go:62-76`). Required fields are what the Organizze API rejects with 422 when absent:

| Resource | Create requires | Update requires | Delete requires |
|---|---|---|---|
| Account | `name`, `type` | `id` | `id` |
| Category | `name` | `id` | `id` |
| CreditCard | `name`, `due_day` (1-31), `closing_day` (1-31) | `id` | `id` |
| Transfer | `credit_account_id`, `debit_account_id`, `amount_cents` (non-zero), `date` | `id` | `id` |

`id` checks are not implemented as `ErrValidation` returns — `id` is a path parameter; if the caller passes 0, the upstream API will return 404 and the existing error-mapping pipeline will surface `ErrNotFound`. Don't duplicate that as a validation rule.

---

## File structure (delta only)

Files touched (all already exist):
```
internal/domain/
  account.go          # + CreateAccountParams, UpdateAccountParams
  category.go         # + CreateCategoryParams, UpdateCategoryParams, DeleteCategoryParams
  credit_card.go      # + CreateCreditCardParams, UpdateCreditCardParams
  transfer.go         # + CreateTransferParams, UpdateTransferParams
internal/adapter/organizze/
  account_repository.go        # + Create, Update, Delete
  account_repository_test.go   # + 3 tests
  category_repository.go       # + Create, Update, Delete (Delete builds ?replacement_id=)
  category_repository_test.go  # + 3 tests
  credit_card_repository.go    # + Create, Update, Delete
  credit_card_repository_test.go # + 3 tests
  transfer_repository.go       # + Create, Update, Delete
  transfer_repository_test.go  # + 3 tests
internal/usecase/
  account.go        # split AccountRepository into Reader/Writer; add Create/Update/Delete on AccountService
  account_test.go   # + validation + create/update/delete tests
  category.go       # same shape
  category_test.go  # same shape
  credit_card.go    # same shape
  credit_card_test.go # same shape
  transfer.go       # same shape
  transfer_test.go  # same shape
internal/adapter/mcp/
  tools_accounts.go        # + service interface methods, 3 Input/Output types, 3 handlers, 3 AddTool calls
  tools_accounts_test.go   # + 3 handler tests
  tools_categories.go      # same
  tools_categories_test.go # same
  tools_credit_cards.go    # same
  tools_credit_cards_test.go # same
  tools_transfers.go       # same
  tools_transfers_test.go  # same
  integration_test.go      # + fakeOrganizze branches, + allExpectedTools entries, + roundtrip cases
README.md                  # + tool catalogue rows
```

No new files. No interface renames. No changes to `cmd/organizze-mcp/main.go` (the composition root) — it already wires `New<Resource>Service(New<Resource>Repository(exec))` for every resource; the existing wiring stays valid because we only **add** methods to the concrete repository structs, and Go's structural interfaces pick them up automatically.

---

## Task 1: Accounts — domain types

**Files:**
- Modify: `internal/domain/account.go`

- [ ] **Step 1: Add Create/Update param types**

Append to `internal/domain/account.go` (after the existing `Account` struct):

```go
// CreateAccountParams are the inputs to AccountService.Create. The Organizze
// API requires name and type; description and default are optional.
type CreateAccountParams struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Default     bool   `json:"default,omitempty"`
}

// UpdateAccountParams describes a partial update; nil pointers are omitted.
type UpdateAccountParams struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Default     *bool   `json:"default,omitempty"`
	Type        *string `json:"type,omitempty"`
}
```

- [ ] **Step 2: Verify compile**

Run: `go build ./internal/domain/...`
Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/domain/account.go
git commit -m "feat(domain): add CreateAccountParams + UpdateAccountParams"
```

---

## Task 2: Accounts — repository writes (TDD)

**Files:**
- Modify: `internal/adapter/organizze/account_repository.go`
- Test: `internal/adapter/organizze/account_repository_test.go`

- [ ] **Step 1: Write failing repository tests**

Append to `internal/adapter/organizze/account_repository_test.go`:

```go
func TestAccountRepository_Create(t *testing.T) {
	var gotBody domain.CreateAccountParams
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/accounts" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":18,"name":"Itaú CC","type":"checking","default":true}`)
	})
	repo := NewAccountRepository(exec)
	a, err := repo.Create(context.Background(), domain.CreateAccountParams{
		Name: "Itaú CC", Type: "checking", Default: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.ID != 18 || a.Name != "Itaú CC" {
		t.Errorf("got %+v", a)
	}
	if gotBody.Name != "Itaú CC" || gotBody.Type != "checking" {
		t.Errorf("server received %+v", gotBody)
	}
}

func TestAccountRepository_Update_SendsOnlySetFields(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/accounts/18" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&raw)
		_, _ = io.WriteString(w, `{"id":18,"name":"Renamed","type":"checking"}`)
	})
	repo := NewAccountRepository(exec)
	name := "Renamed"
	a, err := repo.Update(context.Background(), 18, domain.UpdateAccountParams{Name: &name})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if a.Name != "Renamed" {
		t.Errorf("got %+v", a)
	}
	if _, has := raw["type"]; has {
		t.Errorf("absent fields must be omitted; body=%v", raw)
	}
	if raw["name"] != "Renamed" {
		t.Errorf("body=%v", raw)
	}
}

func TestAccountRepository_Delete(t *testing.T) {
	called := false
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodDelete || r.URL.Path != "/accounts/18" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	repo := NewAccountRepository(exec)
	if err := repo.Delete(context.Background(), 18); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !called {
		t.Error("handler not invoked")
	}
}
```

You may need to add imports if missing: `encoding/json`, `io`, `net/http`. Check the existing imports in `account_repository_test.go` first.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/organizze/ -run TestAccountRepository -v`
Expected: FAIL with `repo.Create undefined`, `repo.Update undefined`, `repo.Delete undefined`.

- [ ] **Step 3: Implement Create, Update, Delete**

Append to `internal/adapter/organizze/account_repository.go`:

```go
// Create issues a POST and returns the persisted account.
func (r *AccountRepository) Create(ctx context.Context, params domain.CreateAccountParams) (*domain.Account, error) {
	var a domain.Account
	if err := r.exec.Post(ctx, "/accounts", params, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// Update issues a PUT with only the non-nil fields from params.
func (r *AccountRepository) Update(ctx context.Context, id int64, params domain.UpdateAccountParams) (*domain.Account, error) {
	var a domain.Account
	if err := r.exec.Put(ctx, fmt.Sprintf("/accounts/%d", id), params, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// Delete issues a DELETE.
func (r *AccountRepository) Delete(ctx context.Context, id int64) error {
	return r.exec.Delete(ctx, fmt.Sprintf("/accounts/%d", id))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/organizze/ -run TestAccountRepository -v`
Expected: PASS (5 tests including the existing `List`/`Get`).

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/organizze/account_repository.go internal/adapter/organizze/account_repository_test.go
git commit -m "feat(organizze): AccountRepository Create/Update/Delete"
```

---

## Task 3: Accounts — service writes (TDD)

**Files:**
- Modify: `internal/usecase/account.go`
- Test: `internal/usecase/account_test.go`

- [ ] **Step 1: Write failing service tests**

Append to `internal/usecase/account_test.go` (read the file first to see existing fake structure; the existing fake is read-only and needs to be extended).

Replace the existing fake declaration in `account_test.go` with this extended version (use Read first to find the exact lines; the existing `fakeAccountRepo` only has `List` and `Get`):

```go
// extend fakeAccountRepo
type fakeAccountRepo struct {
	listed    bool
	gotID     int64
	created   domain.CreateAccountParams
	updatedID int64
	deletedID int64
}

func (f *fakeAccountRepo) List(_ context.Context) ([]domain.Account, error) {
	f.listed = true
	return nil, nil
}
func (f *fakeAccountRepo) Get(_ context.Context, id int64) (*domain.Account, error) {
	f.gotID = id
	return &domain.Account{ID: id}, nil
}
func (f *fakeAccountRepo) Create(_ context.Context, p domain.CreateAccountParams) (*domain.Account, error) {
	f.created = p
	return &domain.Account{ID: 18, Name: p.Name, Type: p.Type}, nil
}
func (f *fakeAccountRepo) Update(_ context.Context, id int64, _ domain.UpdateAccountParams) (*domain.Account, error) {
	f.updatedID = id
	return &domain.Account{ID: id}, nil
}
func (f *fakeAccountRepo) Delete(_ context.Context, id int64) error {
	f.deletedID = id
	return nil
}
```

If the file already has a `fakeAccountRepo` with `List`/`Get`, **replace** that block (don't duplicate). Keep all existing tests untouched.

Then append the new tests:

```go
func TestAccountService_Create_ValidatesRequiredFields(t *testing.T) {
	svc := NewAccountService(&fakeAccountRepo{})
	cases := []struct {
		name string
		in   domain.CreateAccountParams
	}{
		{"name missing", domain.CreateAccountParams{Type: "checking"}},
		{"type missing", domain.CreateAccountParams{Name: "Checking"}},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if _, err := svc.Create(context.Background(), c.in); !errors.Is(err, domain.ErrValidation) {
				t.Errorf("err=%v, want ErrValidation", err)
			}
		})
	}
}

func TestAccountService_Create_Succeeds(t *testing.T) {
	repo := &fakeAccountRepo{}
	svc := NewAccountService(repo)
	a, err := svc.Create(context.Background(), domain.CreateAccountParams{
		Name: "Itaú CC", Type: "checking",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.ID != 18 || repo.created.Type != "checking" {
		t.Errorf("a=%+v repo.created=%+v", a, repo.created)
	}
}

func TestAccountService_UpdateDelete(t *testing.T) {
	repo := &fakeAccountRepo{}
	svc := NewAccountService(repo)
	if _, err := svc.Update(context.Background(), 18, domain.UpdateAccountParams{}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if repo.updatedID != 18 {
		t.Errorf("repo.updatedID = %d", repo.updatedID)
	}
	if err := svc.Delete(context.Background(), 18); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.deletedID != 18 {
		t.Errorf("repo.deletedID = %d", repo.deletedID)
	}
}
```

Add the `errors` import if it isn't already present.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/usecase/ -run TestAccountService -v`
Expected: FAIL — interface assertion (the fake doesn't satisfy the current read-only `AccountRepository` because we haven't extended it yet) **and** missing `svc.Create/Update/Delete`. May fail to compile; that's fine — compile errors are TDD red.

- [ ] **Step 3: Extend interfaces and service**

Replace the contents of `internal/usecase/account.go` with:

```go
package usecase

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// AccountReader is the read-only slice of AccountRepository.
type AccountReader interface {
	List(ctx context.Context) ([]domain.Account, error)
	Get(ctx context.Context, id int64) (*domain.Account, error)
}

// AccountWriter is the mutating slice of AccountRepository.
type AccountWriter interface {
	Create(ctx context.Context, params domain.CreateAccountParams) (*domain.Account, error)
	Update(ctx context.Context, id int64, params domain.UpdateAccountParams) (*domain.Account, error)
	Delete(ctx context.Context, id int64) error
}

// AccountRepository composes reader and writer for callers that need both.
type AccountRepository interface {
	AccountReader
	AccountWriter
}

// AccountService orchestrates account operations.
type AccountService struct {
	repo AccountRepository
}

func NewAccountService(repo AccountRepository) *AccountService {
	return &AccountService{repo: repo}
}

func (s *AccountService) List(ctx context.Context) ([]domain.Account, error) {
	return s.repo.List(ctx)
}

func (s *AccountService) Get(ctx context.Context, id int64) (*domain.Account, error) {
	return s.repo.Get(ctx, id)
}

func (s *AccountService) Create(ctx context.Context, p domain.CreateAccountParams) (*domain.Account, error) {
	switch {
	case p.Name == "":
		return nil, fmt.Errorf("%w: name is required", domain.ErrValidation)
	case p.Type == "":
		return nil, fmt.Errorf("%w: type is required", domain.ErrValidation)
	}
	return s.repo.Create(ctx, p)
}

func (s *AccountService) Update(ctx context.Context, id int64, p domain.UpdateAccountParams) (*domain.Account, error) {
	return s.repo.Update(ctx, id, p)
}

func (s *AccountService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/usecase/ -run TestAccountService -v`
Expected: PASS for all account service tests.

Then run the full usecase package: `go test ./internal/usecase/...`
Expected: PASS (no regressions).

- [ ] **Step 5: Commit**

```bash
git add internal/usecase/account.go internal/usecase/account_test.go
git commit -m "feat(usecase): AccountService Create/Update/Delete with validation"
```

---

## Task 4: Accounts — MCP tools (TDD)

**Files:**
- Modify: `internal/adapter/mcp/tools_accounts.go`
- Test: `internal/adapter/mcp/tools_accounts_test.go`

- [ ] **Step 1: Write failing handler tests**

Read `internal/adapter/mcp/tools_accounts_test.go` to see the existing fake. Replace the existing `fakeAccountService` (or whatever it's named) with the extended one — keep the existing list/get tests, just extend the fake and add new tests:

```go
type fakeAccountSvc struct {
	listed    bool
	gotID     int64
	created   domain.CreateAccountParams
	updated   struct {
		id     int64
		params domain.UpdateAccountParams
	}
	deletedID int64
	createErr error
}

func (f *fakeAccountSvc) List(_ context.Context) ([]domain.Account, error) {
	f.listed = true
	return []domain.Account{{ID: 1}}, nil
}
func (f *fakeAccountSvc) Get(_ context.Context, id int64) (*domain.Account, error) {
	f.gotID = id
	return &domain.Account{ID: id}, nil
}
func (f *fakeAccountSvc) Create(_ context.Context, p domain.CreateAccountParams) (*domain.Account, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.created = p
	return &domain.Account{ID: 18, Name: p.Name, Type: p.Type}, nil
}
func (f *fakeAccountSvc) Update(_ context.Context, id int64, p domain.UpdateAccountParams) (*domain.Account, error) {
	f.updated.id, f.updated.params = id, p
	return &domain.Account{ID: id}, nil
}
func (f *fakeAccountSvc) Delete(_ context.Context, id int64) error {
	f.deletedID = id
	return nil
}
```

If the existing fake has a different name, keep it; just extend it with the new methods + fields and re-point existing tests if needed. Verify the existing tests still pass after editing.

Then append the new tests:

```go
func TestCreateAccountHandler(t *testing.T) {
	svc := &fakeAccountSvc{}
	h := createAccountHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateAccountInput{
		Name: "Itaú CC", Type: "checking",
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Account.ID != 18 || svc.created.Type != "checking" {
		t.Errorf("out=%+v svc.created=%+v", out, svc.created)
	}
}

func TestCreateAccountHandler_PropagatesValidationError(t *testing.T) {
	svc := &fakeAccountSvc{createErr: domain.ErrValidation}
	h := createAccountHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateAccountInput{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
}

func TestUpdateAccountHandler(t *testing.T) {
	svc := &fakeAccountSvc{}
	h := updateAccountHandler(svc)
	name := "Renamed"
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, UpdateAccountInput{
		ID: 18, Name: &name,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Account.ID != 18 {
		t.Errorf("out = %+v", out)
	}
	if svc.updated.id != 18 || svc.updated.params.Name == nil || *svc.updated.params.Name != "Renamed" {
		t.Errorf("svc.updated = %+v", svc.updated)
	}
}

func TestDeleteAccountHandler(t *testing.T) {
	svc := &fakeAccountSvc{}
	h := deleteAccountHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, DeleteAccountInput{ID: 18})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !out.Deleted || out.ID != 18 || svc.deletedID != 18 {
		t.Errorf("out=%+v svc.deletedID=%d", out, svc.deletedID)
	}
}
```

Imports needed: `errors`, `github.com/jorgejr568/organizze-mcp/internal/domain` (probably already there).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/mcp/ -run 'TestCreateAccountHandler|TestUpdateAccountHandler|TestDeleteAccountHandler' -v`
Expected: FAIL — types and handlers undefined.

- [ ] **Step 3: Implement Input/Output types, handlers, register**

Replace the contents of `internal/adapter/mcp/tools_accounts.go` with:

```go
package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type AccountService interface {
	List(ctx context.Context) ([]domain.Account, error)
	Get(ctx context.Context, id int64) (*domain.Account, error)
	Create(ctx context.Context, params domain.CreateAccountParams) (*domain.Account, error)
	Update(ctx context.Context, id int64, params domain.UpdateAccountParams) (*domain.Account, error)
	Delete(ctx context.Context, id int64) error
}

// ---------- list / get ----------

type ListAccountsOutput struct {
	Accounts []domain.Account `json:"accounts"`
}

type GetAccountInput struct {
	ID int64 `json:"id" jsonschema:"The numeric Organizze account id."`
}

type GetAccountOutput struct {
	Account domain.Account `json:"account"`
}

// ---------- create ----------

type CreateAccountInput struct {
	Name        string `json:"name"                  jsonschema:"Account name."`
	Type        string `json:"type"                  jsonschema:"Account type: checking, savings, or other."`
	Description string `json:"description,omitempty" jsonschema:"Optional description."`
	Default     bool   `json:"default,omitempty"     jsonschema:"Whether to mark as the default account."`
}

type CreateAccountOutput struct {
	Account domain.Account `json:"account"`
}

// ---------- update ----------

type UpdateAccountInput struct {
	ID          int64   `json:"id"                    jsonschema:"The numeric Organizze account id to update."`
	Name        *string `json:"name,omitempty"        jsonschema:"New account name."`
	Description *string `json:"description,omitempty" jsonschema:"New description."`
	Default     *bool   `json:"default,omitempty"     jsonschema:"New default flag."`
	Type        *string `json:"type,omitempty"        jsonschema:"New type."`
}

type UpdateAccountOutput struct {
	Account domain.Account `json:"account"`
}

// ---------- delete ----------

type DeleteAccountInput struct {
	ID int64 `json:"id" jsonschema:"The numeric Organizze account id to delete."`
}

type DeleteAccountOutput struct {
	Deleted bool  `json:"deleted"`
	ID      int64 `json:"id"`
}

// ---------- handlers ----------

func listAccountsHandler(svc AccountService) mcpsdk.ToolHandlerFor[struct{}, ListAccountsOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, ListAccountsOutput, error) {
		accounts, err := svc.List(ctx)
		if err != nil {
			return nil, ListAccountsOutput{}, err
		}
		return nil, ListAccountsOutput{Accounts: accounts}, nil
	}
}

func getAccountHandler(svc AccountService) mcpsdk.ToolHandlerFor[GetAccountInput, GetAccountOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetAccountInput) (*mcpsdk.CallToolResult, GetAccountOutput, error) {
		a, err := svc.Get(ctx, in.ID)
		if err != nil {
			return nil, GetAccountOutput{}, err
		}
		return nil, GetAccountOutput{Account: *a}, nil
	}
}

func createAccountHandler(svc AccountService) mcpsdk.ToolHandlerFor[CreateAccountInput, CreateAccountOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in CreateAccountInput) (*mcpsdk.CallToolResult, CreateAccountOutput, error) {
		a, err := svc.Create(ctx, domain.CreateAccountParams{
			Name: in.Name, Type: in.Type, Description: in.Description, Default: in.Default,
		})
		if err != nil {
			return nil, CreateAccountOutput{}, err
		}
		return nil, CreateAccountOutput{Account: *a}, nil
	}
}

func updateAccountHandler(svc AccountService) mcpsdk.ToolHandlerFor[UpdateAccountInput, UpdateAccountOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in UpdateAccountInput) (*mcpsdk.CallToolResult, UpdateAccountOutput, error) {
		a, err := svc.Update(ctx, in.ID, domain.UpdateAccountParams{
			Name: in.Name, Description: in.Description, Default: in.Default, Type: in.Type,
		})
		if err != nil {
			return nil, UpdateAccountOutput{}, err
		}
		return nil, UpdateAccountOutput{Account: *a}, nil
	}
}

func deleteAccountHandler(svc AccountService) mcpsdk.ToolHandlerFor[DeleteAccountInput, DeleteAccountOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in DeleteAccountInput) (*mcpsdk.CallToolResult, DeleteAccountOutput, error) {
		if err := svc.Delete(ctx, in.ID); err != nil {
			return nil, DeleteAccountOutput{}, err
		}
		return nil, DeleteAccountOutput{Deleted: true, ID: in.ID}, nil
	}
}

func registerAccountTools(s *mcpsdk.Server, svc AccountService) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_accounts",
		Description: "List all bank/cash accounts in Organizze.",
	}, listAccountsHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_account",
		Description: "Fetch a single Organizze account by id.",
	}, getAccountHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "create_account",
		Description: "Create a new Organizze bank/cash account. Required: name, type (checking|savings|other).",
	}, createAccountHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "update_account",
		Description: "Update fields on an existing Organizze account. Only fields you provide are changed.",
	}, updateAccountHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "delete_account",
		Description: "Permanently delete an Organizze account by id.",
	}, deleteAccountHandler(svc))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/mcp/ -run Account -v`
Expected: PASS for all account handler tests, including pre-existing list/get.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/mcp/tools_accounts.go internal/adapter/mcp/tools_accounts_test.go
git commit -m "feat(mcp): create_account/update_account/delete_account tools"
```

---

## Task 5: Categories — domain types

**Files:**
- Modify: `internal/domain/category.go`

- [ ] **Step 1: Add Create/Update param types**

Append to `internal/domain/category.go`:

```go
// CreateCategoryParams are the inputs to CategoryService.Create. The Organizze
// API requires name; color and parent_id are optional.
type CreateCategoryParams struct {
	Name     string `json:"name"`
	Color    string `json:"color,omitempty"`
	ParentID *int64 `json:"parent_id,omitempty"`
}

// UpdateCategoryParams describes a partial update; nil pointers are omitted.
type UpdateCategoryParams struct {
	Name     *string `json:"name,omitempty"`
	Color    *string `json:"color,omitempty"`
	ParentID *int64  `json:"parent_id,omitempty"`
}
```

- [ ] **Step 2: Verify compile**

Run: `go build ./internal/domain/...`
Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/domain/category.go
git commit -m "feat(domain): add CreateCategoryParams + UpdateCategoryParams"
```

---

## Task 6: Categories — repository writes (TDD)

**Files:**
- Modify: `internal/adapter/organizze/category_repository.go`
- Test: `internal/adapter/organizze/category_repository_test.go`

The category `Delete` is special: it accepts an optional `replacement_id` to reassign transactions. The repository signature is `Delete(ctx, id int64, replacementID *int64) error`. When `replacementID` is non-nil, the path becomes `/categories/{id}?replacement_id={replacementID}`.

- [ ] **Step 1: Write failing repository tests**

Append to `internal/adapter/organizze/category_repository_test.go`:

```go
func TestCategoryRepository_Create(t *testing.T) {
	var gotBody domain.CreateCategoryParams
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/categories" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":42,"name":"Groceries","color":"#abcdef"}`)
	})
	repo := NewCategoryRepository(exec)
	c, err := repo.Create(context.Background(), domain.CreateCategoryParams{Name: "Groceries"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.ID != 42 || c.Name != "Groceries" {
		t.Errorf("got %+v", c)
	}
	if gotBody.Name != "Groceries" {
		t.Errorf("server received %+v", gotBody)
	}
}

func TestCategoryRepository_Update_SendsOnlySetFields(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/categories/42" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&raw)
		_, _ = io.WriteString(w, `{"id":42,"name":"Food"}`)
	})
	repo := NewCategoryRepository(exec)
	name := "Food"
	c, err := repo.Update(context.Background(), 42, domain.UpdateCategoryParams{Name: &name})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if c.Name != "Food" {
		t.Errorf("got %+v", c)
	}
	if _, has := raw["color"]; has {
		t.Errorf("absent fields must be omitted; body=%v", raw)
	}
}

func TestCategoryRepository_Delete_NoReplacement(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/categories/42" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		if r.URL.RawQuery != "" {
			t.Errorf("expected no query, got %q", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	repo := NewCategoryRepository(exec)
	if err := repo.Delete(context.Background(), 42, nil); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestCategoryRepository_Delete_WithReplacement(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/categories/42" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("replacement_id"); got != "99" {
			t.Errorf("replacement_id = %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	repo := NewCategoryRepository(exec)
	rep := int64(99)
	if err := repo.Delete(context.Background(), 42, &rep); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/organizze/ -run TestCategoryRepository -v`
Expected: FAIL — `repo.Create`, `repo.Update`, `repo.Delete` undefined.

- [ ] **Step 3: Implement Create, Update, Delete**

Append to `internal/adapter/organizze/category_repository.go` (add imports `fmt`, `net/url`, `strconv` if missing):

```go
// Create issues a POST and returns the persisted category.
func (r *CategoryRepository) Create(ctx context.Context, params domain.CreateCategoryParams) (*domain.Category, error) {
	var c domain.Category
	if err := r.exec.Post(ctx, "/categories", params, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Update issues a PUT with only the non-nil fields from params.
func (r *CategoryRepository) Update(ctx context.Context, id int64, params domain.UpdateCategoryParams) (*domain.Category, error) {
	var c domain.Category
	if err := r.exec.Put(ctx, fmt.Sprintf("/categories/%d", id), params, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Delete issues a DELETE. If replacementID is non-nil, the affected transactions
// are reassigned to that category (Organizze "replacement_id" query parameter).
func (r *CategoryRepository) Delete(ctx context.Context, id int64, replacementID *int64) error {
	path := fmt.Sprintf("/categories/%d", id)
	if replacementID != nil {
		q := url.Values{}
		q.Set("replacement_id", strconv.FormatInt(*replacementID, 10))
		path += "?" + q.Encode()
	}
	return r.exec.Delete(ctx, path)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/organizze/ -run TestCategoryRepository -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/organizze/category_repository.go internal/adapter/organizze/category_repository_test.go
git commit -m "feat(organizze): CategoryRepository Create/Update/Delete with replacement_id"
```

---

## Task 7: Categories — service writes (TDD)

**Files:**
- Modify: `internal/usecase/category.go`
- Test: `internal/usecase/category_test.go`

- [ ] **Step 1: Write failing service tests**

Read the existing `internal/usecase/category_test.go` first to find the `fakeCategoryRepo`. Replace it with this extended version (preserve fields needed by existing tests; remove only the old `fakeCategoryRepo` block):

```go
type fakeCategoryRepo struct {
	listed       bool
	gotID        int64
	created      domain.CreateCategoryParams
	updatedID    int64
	deletedID    int64
	deletedRepID *int64
}

func (f *fakeCategoryRepo) List(_ context.Context) ([]domain.Category, error) {
	f.listed = true
	return nil, nil
}
func (f *fakeCategoryRepo) Get(_ context.Context, id int64) (*domain.Category, error) {
	f.gotID = id
	return &domain.Category{ID: id}, nil
}
func (f *fakeCategoryRepo) Create(_ context.Context, p domain.CreateCategoryParams) (*domain.Category, error) {
	f.created = p
	return &domain.Category{ID: 42, Name: p.Name}, nil
}
func (f *fakeCategoryRepo) Update(_ context.Context, id int64, _ domain.UpdateCategoryParams) (*domain.Category, error) {
	f.updatedID = id
	return &domain.Category{ID: id}, nil
}
func (f *fakeCategoryRepo) Delete(_ context.Context, id int64, replacementID *int64) error {
	f.deletedID, f.deletedRepID = id, replacementID
	return nil
}
```

Append new tests:

```go
func TestCategoryService_Create_RequiresName(t *testing.T) {
	svc := NewCategoryService(&fakeCategoryRepo{})
	if _, err := svc.Create(context.Background(), domain.CreateCategoryParams{}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err=%v, want ErrValidation", err)
	}
}

func TestCategoryService_Create_Succeeds(t *testing.T) {
	repo := &fakeCategoryRepo{}
	svc := NewCategoryService(repo)
	c, err := svc.Create(context.Background(), domain.CreateCategoryParams{Name: "Groceries"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.ID != 42 || repo.created.Name != "Groceries" {
		t.Errorf("c=%+v repo.created=%+v", c, repo.created)
	}
}

func TestCategoryService_UpdateDelete(t *testing.T) {
	repo := &fakeCategoryRepo{}
	svc := NewCategoryService(repo)
	if _, err := svc.Update(context.Background(), 42, domain.UpdateCategoryParams{}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if repo.updatedID != 42 {
		t.Errorf("repo.updatedID = %d", repo.updatedID)
	}
	rep := int64(99)
	if err := svc.Delete(context.Background(), 42, &rep); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.deletedID != 42 || repo.deletedRepID == nil || *repo.deletedRepID != 99 {
		t.Errorf("repo.deletedID=%d repo.deletedRepID=%v", repo.deletedID, repo.deletedRepID)
	}
}

func TestCategoryService_Delete_NoReplacement(t *testing.T) {
	repo := &fakeCategoryRepo{}
	svc := NewCategoryService(repo)
	if err := svc.Delete(context.Background(), 42, nil); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.deletedRepID != nil {
		t.Errorf("repo.deletedRepID = %v, want nil", repo.deletedRepID)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/usecase/ -run TestCategoryService -v`
Expected: FAIL — compile errors from missing methods/interfaces.

- [ ] **Step 3: Extend interfaces and service**

Replace contents of `internal/usecase/category.go` with:

```go
package usecase

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type CategoryReader interface {
	List(ctx context.Context) ([]domain.Category, error)
	Get(ctx context.Context, id int64) (*domain.Category, error)
}

type CategoryWriter interface {
	Create(ctx context.Context, params domain.CreateCategoryParams) (*domain.Category, error)
	Update(ctx context.Context, id int64, params domain.UpdateCategoryParams) (*domain.Category, error)
	Delete(ctx context.Context, id int64, replacementID *int64) error
}

type CategoryRepository interface {
	CategoryReader
	CategoryWriter
}

type CategoryService struct {
	repo CategoryRepository
}

func NewCategoryService(repo CategoryRepository) *CategoryService {
	return &CategoryService{repo: repo}
}

func (s *CategoryService) List(ctx context.Context) ([]domain.Category, error) {
	return s.repo.List(ctx)
}

func (s *CategoryService) Get(ctx context.Context, id int64) (*domain.Category, error) {
	return s.repo.Get(ctx, id)
}

func (s *CategoryService) Create(ctx context.Context, p domain.CreateCategoryParams) (*domain.Category, error) {
	if p.Name == "" {
		return nil, fmt.Errorf("%w: name is required", domain.ErrValidation)
	}
	return s.repo.Create(ctx, p)
}

func (s *CategoryService) Update(ctx context.Context, id int64, p domain.UpdateCategoryParams) (*domain.Category, error) {
	return s.repo.Update(ctx, id, p)
}

func (s *CategoryService) Delete(ctx context.Context, id int64, replacementID *int64) error {
	return s.repo.Delete(ctx, id, replacementID)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/usecase/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/usecase/category.go internal/usecase/category_test.go
git commit -m "feat(usecase): CategoryService Create/Update/Delete with replacement_id"
```

---

## Task 8: Categories — MCP tools (TDD)

**Files:**
- Modify: `internal/adapter/mcp/tools_categories.go`
- Test: `internal/adapter/mcp/tools_categories_test.go`

- [ ] **Step 1: Write failing handler tests**

Read `internal/adapter/mcp/tools_categories_test.go` first. Extend the existing `fakeCategorySvc` to satisfy the new interface (same shape as the account fake, but with the `replacement_id`-aware delete):

```go
type fakeCategorySvc struct {
	listed    bool
	gotID     int64
	created   domain.CreateCategoryParams
	updated   struct {
		id     int64
		params domain.UpdateCategoryParams
	}
	deletedID    int64
	deletedRepID *int64
	createErr    error
}

func (f *fakeCategorySvc) List(_ context.Context) ([]domain.Category, error) {
	f.listed = true
	return []domain.Category{{ID: 1}}, nil
}
func (f *fakeCategorySvc) Get(_ context.Context, id int64) (*domain.Category, error) {
	f.gotID = id
	return &domain.Category{ID: id}, nil
}
func (f *fakeCategorySvc) Create(_ context.Context, p domain.CreateCategoryParams) (*domain.Category, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.created = p
	return &domain.Category{ID: 42, Name: p.Name}, nil
}
func (f *fakeCategorySvc) Update(_ context.Context, id int64, p domain.UpdateCategoryParams) (*domain.Category, error) {
	f.updated.id, f.updated.params = id, p
	return &domain.Category{ID: id}, nil
}
func (f *fakeCategorySvc) Delete(_ context.Context, id int64, replacementID *int64) error {
	f.deletedID, f.deletedRepID = id, replacementID
	return nil
}
```

Append new tests:

```go
func TestCreateCategoryHandler(t *testing.T) {
	svc := &fakeCategorySvc{}
	h := createCategoryHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateCategoryInput{Name: "Groceries"})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Category.ID != 42 || svc.created.Name != "Groceries" {
		t.Errorf("out=%+v svc.created=%+v", out, svc.created)
	}
}

func TestCreateCategoryHandler_PropagatesValidationError(t *testing.T) {
	svc := &fakeCategorySvc{createErr: domain.ErrValidation}
	h := createCategoryHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateCategoryInput{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
}

func TestUpdateCategoryHandler(t *testing.T) {
	svc := &fakeCategorySvc{}
	h := updateCategoryHandler(svc)
	name := "Food"
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, UpdateCategoryInput{
		ID: 42, Name: &name,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Category.ID != 42 || svc.updated.id != 42 || *svc.updated.params.Name != "Food" {
		t.Errorf("out=%+v svc.updated=%+v", out, svc.updated)
	}
}

func TestDeleteCategoryHandler_NoReplacement(t *testing.T) {
	svc := &fakeCategorySvc{}
	h := deleteCategoryHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, DeleteCategoryInput{ID: 42})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !out.Deleted || out.ID != 42 || svc.deletedID != 42 || svc.deletedRepID != nil {
		t.Errorf("out=%+v svc.deletedID=%d svc.deletedRepID=%v", out, svc.deletedID, svc.deletedRepID)
	}
}

func TestDeleteCategoryHandler_WithReplacement(t *testing.T) {
	svc := &fakeCategorySvc{}
	h := deleteCategoryHandler(svc)
	rep := int64(99)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, DeleteCategoryInput{ID: 42, ReplacementID: &rep})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if svc.deletedRepID == nil || *svc.deletedRepID != 99 {
		t.Errorf("svc.deletedRepID = %v", svc.deletedRepID)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/mcp/ -run Category -v`
Expected: FAIL — types and handlers undefined.

- [ ] **Step 3: Implement Input/Output types, handlers, register**

Replace `internal/adapter/mcp/tools_categories.go` with the same shape as accounts. Read the current file first; preserve its package, imports, and the existing `CategoryService` interface signature lines (just extend them). Final state should be:

```go
package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type CategoryService interface {
	List(ctx context.Context) ([]domain.Category, error)
	Get(ctx context.Context, id int64) (*domain.Category, error)
	Create(ctx context.Context, params domain.CreateCategoryParams) (*domain.Category, error)
	Update(ctx context.Context, id int64, params domain.UpdateCategoryParams) (*domain.Category, error)
	Delete(ctx context.Context, id int64, replacementID *int64) error
}

type ListCategoriesOutput struct {
	Categories []domain.Category `json:"categories"`
}

type GetCategoryInput struct {
	ID int64 `json:"id" jsonschema:"The numeric Organizze category id."`
}

type GetCategoryOutput struct {
	Category domain.Category `json:"category"`
}

type CreateCategoryInput struct {
	Name     string `json:"name"                jsonschema:"Category name."`
	Color    string `json:"color,omitempty"     jsonschema:"Optional hex color (e.g. #abcdef)."`
	ParentID *int64 `json:"parent_id,omitempty" jsonschema:"Optional parent category id."`
}

type CreateCategoryOutput struct {
	Category domain.Category `json:"category"`
}

type UpdateCategoryInput struct {
	ID       int64   `json:"id"                 jsonschema:"The numeric Organizze category id to update."`
	Name     *string `json:"name,omitempty"     jsonschema:"New category name."`
	Color    *string `json:"color,omitempty"    jsonschema:"New hex color."`
	ParentID *int64  `json:"parent_id,omitempty" jsonschema:"New parent category id."`
}

type UpdateCategoryOutput struct {
	Category domain.Category `json:"category"`
}

type DeleteCategoryInput struct {
	ID            int64  `json:"id" jsonschema:"The numeric Organizze category id to delete."`
	ReplacementID *int64 `json:"replacement_id,omitempty" jsonschema:"Optional category id to reassign affected transactions to."`
}

type DeleteCategoryOutput struct {
	Deleted bool  `json:"deleted"`
	ID      int64 `json:"id"`
}

func listCategoriesHandler(svc CategoryService) mcpsdk.ToolHandlerFor[struct{}, ListCategoriesOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, ListCategoriesOutput, error) {
		cs, err := svc.List(ctx)
		if err != nil {
			return nil, ListCategoriesOutput{}, err
		}
		return nil, ListCategoriesOutput{Categories: cs}, nil
	}
}

func getCategoryHandler(svc CategoryService) mcpsdk.ToolHandlerFor[GetCategoryInput, GetCategoryOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetCategoryInput) (*mcpsdk.CallToolResult, GetCategoryOutput, error) {
		c, err := svc.Get(ctx, in.ID)
		if err != nil {
			return nil, GetCategoryOutput{}, err
		}
		return nil, GetCategoryOutput{Category: *c}, nil
	}
}

func createCategoryHandler(svc CategoryService) mcpsdk.ToolHandlerFor[CreateCategoryInput, CreateCategoryOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in CreateCategoryInput) (*mcpsdk.CallToolResult, CreateCategoryOutput, error) {
		c, err := svc.Create(ctx, domain.CreateCategoryParams{Name: in.Name, Color: in.Color, ParentID: in.ParentID})
		if err != nil {
			return nil, CreateCategoryOutput{}, err
		}
		return nil, CreateCategoryOutput{Category: *c}, nil
	}
}

func updateCategoryHandler(svc CategoryService) mcpsdk.ToolHandlerFor[UpdateCategoryInput, UpdateCategoryOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in UpdateCategoryInput) (*mcpsdk.CallToolResult, UpdateCategoryOutput, error) {
		c, err := svc.Update(ctx, in.ID, domain.UpdateCategoryParams{Name: in.Name, Color: in.Color, ParentID: in.ParentID})
		if err != nil {
			return nil, UpdateCategoryOutput{}, err
		}
		return nil, UpdateCategoryOutput{Category: *c}, nil
	}
}

func deleteCategoryHandler(svc CategoryService) mcpsdk.ToolHandlerFor[DeleteCategoryInput, DeleteCategoryOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in DeleteCategoryInput) (*mcpsdk.CallToolResult, DeleteCategoryOutput, error) {
		if err := svc.Delete(ctx, in.ID, in.ReplacementID); err != nil {
			return nil, DeleteCategoryOutput{}, err
		}
		return nil, DeleteCategoryOutput{Deleted: true, ID: in.ID}, nil
	}
}

func registerCategoryTools(s *mcpsdk.Server, svc CategoryService) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_categories",
		Description: "List all Organizze categories.",
	}, listCategoriesHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_category",
		Description: "Fetch a single Organizze category by id.",
	}, getCategoryHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "create_category",
		Description: "Create a new Organizze category. Required: name. Color and parent_id are optional.",
	}, createCategoryHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "update_category",
		Description: "Update fields on an existing Organizze category. Only fields you provide are changed.",
	}, updateCategoryHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "delete_category",
		Description: "Permanently delete an Organizze category by id. Optionally pass replacement_id to reassign affected transactions to that category.",
	}, deleteCategoryHandler(svc))
}
```

If the existing file already exports `listCategoriesHandler` / `getCategoryHandler` / `ListCategoriesOutput` / `GetCategoryInput` / `GetCategoryOutput` with the same signatures, this replacement is safe. Otherwise reconcile by hand using the existing file as the source of truth for read-only types.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/mcp/ -run Category -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/mcp/tools_categories.go internal/adapter/mcp/tools_categories_test.go
git commit -m "feat(mcp): create_category/update_category/delete_category tools"
```

---

## Task 9: Credit Cards — domain types

**Files:**
- Modify: `internal/domain/credit_card.go`

- [ ] **Step 1: Add Create/Update param types**

Append to `internal/domain/credit_card.go`:

```go
// CreateCreditCardParams are the inputs to CreditCardService.Create. The
// Organizze API requires name, due_day (1-31), and closing_day (1-31).
type CreateCreditCardParams struct {
	Name        string `json:"name"`
	DueDay      int    `json:"due_day"`
	ClosingDay  int    `json:"closing_day"`
	CardNetwork string `json:"card_network,omitempty"`
	LimitCents  int64  `json:"limit_cents,omitempty"`
	Description string `json:"description,omitempty"`
	Default     bool   `json:"default,omitempty"`
}

// UpdateCreditCardParams describes a partial update; nil pointers are omitted.
// update_invoices_since is a YYYY-MM-DD date string: when set, Organizze
// retroactively regenerates invoices since that date.
type UpdateCreditCardParams struct {
	Name                *string `json:"name,omitempty"`
	DueDay              *int    `json:"due_day,omitempty"`
	ClosingDay          *int    `json:"closing_day,omitempty"`
	Description         *string `json:"description,omitempty"`
	UpdateInvoicesSince *string `json:"update_invoices_since,omitempty"`
}
```

- [ ] **Step 2: Verify compile**

Run: `go build ./internal/domain/...`
Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/domain/credit_card.go
git commit -m "feat(domain): add CreateCreditCardParams + UpdateCreditCardParams"
```

---

## Task 10: Credit Cards — repository writes (TDD)

**Files:**
- Modify: `internal/adapter/organizze/credit_card_repository.go`
- Test: `internal/adapter/organizze/credit_card_repository_test.go`

- [ ] **Step 1: Write failing repository tests**

Append:

```go
func TestCreditCardRepository_Create(t *testing.T) {
	var gotBody domain.CreateCreditCardParams
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/credit_cards" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":7,"name":"Nubank","closing_day":20,"due_day":27,"limit_cents":500000}`)
	})
	repo := NewCreditCardRepository(exec)
	cc, err := repo.Create(context.Background(), domain.CreateCreditCardParams{
		Name: "Nubank", DueDay: 27, ClosingDay: 20, LimitCents: 500000,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if cc.ID != 7 || cc.LimitCents != 500000 {
		t.Errorf("got %+v", cc)
	}
	if gotBody.DueDay != 27 || gotBody.ClosingDay != 20 {
		t.Errorf("server received %+v", gotBody)
	}
}

func TestCreditCardRepository_Update_SendsOnlySetFields(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/credit_cards/7" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&raw)
		_, _ = io.WriteString(w, `{"id":7,"name":"Renamed","closing_day":20,"due_day":27}`)
	})
	repo := NewCreditCardRepository(exec)
	name := "Renamed"
	cc, err := repo.Update(context.Background(), 7, domain.UpdateCreditCardParams{Name: &name})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if cc.Name != "Renamed" {
		t.Errorf("got %+v", cc)
	}
	if _, has := raw["due_day"]; has {
		t.Errorf("absent fields must be omitted; body=%v", raw)
	}
}

func TestCreditCardRepository_Delete(t *testing.T) {
	called := false
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodDelete || r.URL.Path != "/credit_cards/7" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	repo := NewCreditCardRepository(exec)
	if err := repo.Delete(context.Background(), 7); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !called {
		t.Error("handler not invoked")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/organizze/ -run TestCreditCardRepository -v`
Expected: FAIL — methods undefined.

- [ ] **Step 3: Implement Create, Update, Delete**

Append to `internal/adapter/organizze/credit_card_repository.go`:

```go
// Create issues a POST and returns the persisted credit card.
func (r *CreditCardRepository) Create(ctx context.Context, params domain.CreateCreditCardParams) (*domain.CreditCard, error) {
	var cc domain.CreditCard
	if err := r.exec.Post(ctx, "/credit_cards", params, &cc); err != nil {
		return nil, err
	}
	return &cc, nil
}

// Update issues a PUT with only the non-nil fields from params.
func (r *CreditCardRepository) Update(ctx context.Context, id int64, params domain.UpdateCreditCardParams) (*domain.CreditCard, error) {
	var cc domain.CreditCard
	if err := r.exec.Put(ctx, fmt.Sprintf("/credit_cards/%d", id), params, &cc); err != nil {
		return nil, err
	}
	return &cc, nil
}

// Delete issues a DELETE.
func (r *CreditCardRepository) Delete(ctx context.Context, id int64) error {
	return r.exec.Delete(ctx, fmt.Sprintf("/credit_cards/%d", id))
}
```

Add `fmt` import if not already present.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/organizze/ -run TestCreditCardRepository -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/organizze/credit_card_repository.go internal/adapter/organizze/credit_card_repository_test.go
git commit -m "feat(organizze): CreditCardRepository Create/Update/Delete"
```

---

## Task 11: Credit Cards — service writes (TDD)

**Files:**
- Modify: `internal/usecase/credit_card.go`
- Test: `internal/usecase/credit_card_test.go`

- [ ] **Step 1: Write failing service tests**

Read `internal/usecase/credit_card_test.go` first. Extend the existing fake to:

```go
type fakeCreditCardRepo struct {
	listed    bool
	gotID     int64
	created   domain.CreateCreditCardParams
	updatedID int64
	deletedID int64
}

func (f *fakeCreditCardRepo) List(_ context.Context) ([]domain.CreditCard, error) {
	f.listed = true
	return nil, nil
}
func (f *fakeCreditCardRepo) Get(_ context.Context, id int64) (*domain.CreditCard, error) {
	f.gotID = id
	return &domain.CreditCard{ID: id}, nil
}
func (f *fakeCreditCardRepo) Create(_ context.Context, p domain.CreateCreditCardParams) (*domain.CreditCard, error) {
	f.created = p
	return &domain.CreditCard{ID: 7, Name: p.Name}, nil
}
func (f *fakeCreditCardRepo) Update(_ context.Context, id int64, _ domain.UpdateCreditCardParams) (*domain.CreditCard, error) {
	f.updatedID = id
	return &domain.CreditCard{ID: id}, nil
}
func (f *fakeCreditCardRepo) Delete(_ context.Context, id int64) error {
	f.deletedID = id
	return nil
}
```

Append:

```go
func TestCreditCardService_Create_ValidatesRequiredFields(t *testing.T) {
	svc := NewCreditCardService(&fakeCreditCardRepo{})
	cases := []struct {
		name string
		in   domain.CreateCreditCardParams
	}{
		{"name missing", domain.CreateCreditCardParams{DueDay: 27, ClosingDay: 20}},
		{"due_day zero", domain.CreateCreditCardParams{Name: "x", ClosingDay: 20}},
		{"due_day > 31", domain.CreateCreditCardParams{Name: "x", DueDay: 32, ClosingDay: 20}},
		{"closing_day zero", domain.CreateCreditCardParams{Name: "x", DueDay: 27}},
		{"closing_day > 31", domain.CreateCreditCardParams{Name: "x", DueDay: 27, ClosingDay: 99}},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if _, err := svc.Create(context.Background(), c.in); !errors.Is(err, domain.ErrValidation) {
				t.Errorf("err=%v, want ErrValidation", err)
			}
		})
	}
}

func TestCreditCardService_Create_Succeeds(t *testing.T) {
	repo := &fakeCreditCardRepo{}
	svc := NewCreditCardService(repo)
	cc, err := svc.Create(context.Background(), domain.CreateCreditCardParams{
		Name: "Nubank", DueDay: 27, ClosingDay: 20,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if cc.ID != 7 || repo.created.Name != "Nubank" {
		t.Errorf("cc=%+v repo.created=%+v", cc, repo.created)
	}
}

func TestCreditCardService_UpdateDelete(t *testing.T) {
	repo := &fakeCreditCardRepo{}
	svc := NewCreditCardService(repo)
	if _, err := svc.Update(context.Background(), 7, domain.UpdateCreditCardParams{}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if repo.updatedID != 7 {
		t.Errorf("repo.updatedID = %d", repo.updatedID)
	}
	if err := svc.Delete(context.Background(), 7); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.deletedID != 7 {
		t.Errorf("repo.deletedID = %d", repo.deletedID)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/usecase/ -run TestCreditCardService -v`
Expected: FAIL — compile errors.

- [ ] **Step 3: Extend interfaces and service**

Replace `internal/usecase/credit_card.go` with:

```go
package usecase

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type CreditCardReader interface {
	List(ctx context.Context) ([]domain.CreditCard, error)
	Get(ctx context.Context, id int64) (*domain.CreditCard, error)
}

type CreditCardWriter interface {
	Create(ctx context.Context, params domain.CreateCreditCardParams) (*domain.CreditCard, error)
	Update(ctx context.Context, id int64, params domain.UpdateCreditCardParams) (*domain.CreditCard, error)
	Delete(ctx context.Context, id int64) error
}

type CreditCardRepository interface {
	CreditCardReader
	CreditCardWriter
}

type CreditCardService struct {
	repo CreditCardRepository
}

func NewCreditCardService(repo CreditCardRepository) *CreditCardService {
	return &CreditCardService{repo: repo}
}

func (s *CreditCardService) List(ctx context.Context) ([]domain.CreditCard, error) {
	return s.repo.List(ctx)
}

func (s *CreditCardService) Get(ctx context.Context, id int64) (*domain.CreditCard, error) {
	return s.repo.Get(ctx, id)
}

func (s *CreditCardService) Create(ctx context.Context, p domain.CreateCreditCardParams) (*domain.CreditCard, error) {
	switch {
	case p.Name == "":
		return nil, fmt.Errorf("%w: name is required", domain.ErrValidation)
	case p.DueDay < 1 || p.DueDay > 31:
		return nil, fmt.Errorf("%w: due_day must be between 1 and 31", domain.ErrValidation)
	case p.ClosingDay < 1 || p.ClosingDay > 31:
		return nil, fmt.Errorf("%w: closing_day must be between 1 and 31", domain.ErrValidation)
	}
	return s.repo.Create(ctx, p)
}

func (s *CreditCardService) Update(ctx context.Context, id int64, p domain.UpdateCreditCardParams) (*domain.CreditCard, error) {
	return s.repo.Update(ctx, id, p)
}

func (s *CreditCardService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/usecase/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/usecase/credit_card.go internal/usecase/credit_card_test.go
git commit -m "feat(usecase): CreditCardService Create/Update/Delete with validation"
```

---

## Task 12: Credit Cards — MCP tools (TDD)

**Files:**
- Modify: `internal/adapter/mcp/tools_credit_cards.go`
- Test: `internal/adapter/mcp/tools_credit_cards_test.go`

- [ ] **Step 1: Write failing handler tests**

Read existing test file first. Extend the fake to include write methods:

```go
type fakeCreditCardSvc struct {
	listed    bool
	gotID     int64
	created   domain.CreateCreditCardParams
	updated   struct {
		id     int64
		params domain.UpdateCreditCardParams
	}
	deletedID int64
	createErr error
}

func (f *fakeCreditCardSvc) List(_ context.Context) ([]domain.CreditCard, error) {
	f.listed = true
	return []domain.CreditCard{{ID: 1}}, nil
}
func (f *fakeCreditCardSvc) Get(_ context.Context, id int64) (*domain.CreditCard, error) {
	f.gotID = id
	return &domain.CreditCard{ID: id}, nil
}
func (f *fakeCreditCardSvc) Create(_ context.Context, p domain.CreateCreditCardParams) (*domain.CreditCard, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.created = p
	return &domain.CreditCard{ID: 7, Name: p.Name}, nil
}
func (f *fakeCreditCardSvc) Update(_ context.Context, id int64, p domain.UpdateCreditCardParams) (*domain.CreditCard, error) {
	f.updated.id, f.updated.params = id, p
	return &domain.CreditCard{ID: id}, nil
}
func (f *fakeCreditCardSvc) Delete(_ context.Context, id int64) error {
	f.deletedID = id
	return nil
}
```

Append new tests (same pattern as accounts):

```go
func TestCreateCreditCardHandler(t *testing.T) {
	svc := &fakeCreditCardSvc{}
	h := createCreditCardHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateCreditCardInput{
		Name: "Nubank", DueDay: 27, ClosingDay: 20,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.CreditCard.ID != 7 || svc.created.DueDay != 27 {
		t.Errorf("out=%+v svc.created=%+v", out, svc.created)
	}
}

func TestCreateCreditCardHandler_PropagatesValidationError(t *testing.T) {
	svc := &fakeCreditCardSvc{createErr: domain.ErrValidation}
	h := createCreditCardHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateCreditCardInput{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
}

func TestUpdateCreditCardHandler(t *testing.T) {
	svc := &fakeCreditCardSvc{}
	h := updateCreditCardHandler(svc)
	name := "Renamed"
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, UpdateCreditCardInput{
		ID: 7, Name: &name,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.CreditCard.ID != 7 || svc.updated.id != 7 || *svc.updated.params.Name != "Renamed" {
		t.Errorf("out=%+v svc.updated=%+v", out, svc.updated)
	}
}

func TestDeleteCreditCardHandler(t *testing.T) {
	svc := &fakeCreditCardSvc{}
	h := deleteCreditCardHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, DeleteCreditCardInput{ID: 7})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !out.Deleted || out.ID != 7 || svc.deletedID != 7 {
		t.Errorf("out=%+v svc.deletedID=%d", out, svc.deletedID)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/mcp/ -run CreditCard -v`
Expected: FAIL — types and handlers undefined.

- [ ] **Step 3: Implement Input/Output types, handlers, register**

Replace `internal/adapter/mcp/tools_credit_cards.go` with (read first to preserve existing list/get types & imports):

```go
package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type CreditCardService interface {
	List(ctx context.Context) ([]domain.CreditCard, error)
	Get(ctx context.Context, id int64) (*domain.CreditCard, error)
	Create(ctx context.Context, params domain.CreateCreditCardParams) (*domain.CreditCard, error)
	Update(ctx context.Context, id int64, params domain.UpdateCreditCardParams) (*domain.CreditCard, error)
	Delete(ctx context.Context, id int64) error
}

type ListCreditCardsOutput struct {
	CreditCards []domain.CreditCard `json:"credit_cards"`
}

type GetCreditCardInput struct {
	ID int64 `json:"id" jsonschema:"The numeric Organizze credit card id."`
}

type GetCreditCardOutput struct {
	CreditCard domain.CreditCard `json:"credit_card"`
}

type CreateCreditCardInput struct {
	Name        string `json:"name"                  jsonschema:"Credit card name."`
	DueDay      int    `json:"due_day"               jsonschema:"Bill due day (1-31)."`
	ClosingDay  int    `json:"closing_day"           jsonschema:"Statement closing day (1-31)."`
	CardNetwork string `json:"card_network,omitempty" jsonschema:"Optional card network (visa, mastercard, etc.)."`
	LimitCents  int64  `json:"limit_cents,omitempty"  jsonschema:"Optional credit limit in cents."`
	Description string `json:"description,omitempty"  jsonschema:"Optional description."`
	Default     bool   `json:"default,omitempty"      jsonschema:"Mark as the default credit card."`
}

type CreateCreditCardOutput struct {
	CreditCard domain.CreditCard `json:"credit_card"`
}

type UpdateCreditCardInput struct {
	ID                  int64   `json:"id"                              jsonschema:"The numeric Organizze credit card id to update."`
	Name                *string `json:"name,omitempty"                  jsonschema:"New name."`
	DueDay              *int    `json:"due_day,omitempty"               jsonschema:"New due day (1-31)."`
	ClosingDay          *int    `json:"closing_day,omitempty"           jsonschema:"New closing day (1-31)."`
	Description         *string `json:"description,omitempty"           jsonschema:"New description."`
	UpdateInvoicesSince *string `json:"update_invoices_since,omitempty" jsonschema:"If set (YYYY-MM-DD), Organizze retroactively regenerates invoices from this date."`
}

type UpdateCreditCardOutput struct {
	CreditCard domain.CreditCard `json:"credit_card"`
}

type DeleteCreditCardInput struct {
	ID int64 `json:"id" jsonschema:"The numeric Organizze credit card id to delete."`
}

type DeleteCreditCardOutput struct {
	Deleted bool  `json:"deleted"`
	ID      int64 `json:"id"`
}

func listCreditCardsHandler(svc CreditCardService) mcpsdk.ToolHandlerFor[struct{}, ListCreditCardsOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, ListCreditCardsOutput, error) {
		cc, err := svc.List(ctx)
		if err != nil {
			return nil, ListCreditCardsOutput{}, err
		}
		return nil, ListCreditCardsOutput{CreditCards: cc}, nil
	}
}

func getCreditCardHandler(svc CreditCardService) mcpsdk.ToolHandlerFor[GetCreditCardInput, GetCreditCardOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetCreditCardInput) (*mcpsdk.CallToolResult, GetCreditCardOutput, error) {
		cc, err := svc.Get(ctx, in.ID)
		if err != nil {
			return nil, GetCreditCardOutput{}, err
		}
		return nil, GetCreditCardOutput{CreditCard: *cc}, nil
	}
}

func createCreditCardHandler(svc CreditCardService) mcpsdk.ToolHandlerFor[CreateCreditCardInput, CreateCreditCardOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in CreateCreditCardInput) (*mcpsdk.CallToolResult, CreateCreditCardOutput, error) {
		cc, err := svc.Create(ctx, domain.CreateCreditCardParams{
			Name: in.Name, DueDay: in.DueDay, ClosingDay: in.ClosingDay,
			CardNetwork: in.CardNetwork, LimitCents: in.LimitCents,
			Description: in.Description, Default: in.Default,
		})
		if err != nil {
			return nil, CreateCreditCardOutput{}, err
		}
		return nil, CreateCreditCardOutput{CreditCard: *cc}, nil
	}
}

func updateCreditCardHandler(svc CreditCardService) mcpsdk.ToolHandlerFor[UpdateCreditCardInput, UpdateCreditCardOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in UpdateCreditCardInput) (*mcpsdk.CallToolResult, UpdateCreditCardOutput, error) {
		cc, err := svc.Update(ctx, in.ID, domain.UpdateCreditCardParams{
			Name: in.Name, DueDay: in.DueDay, ClosingDay: in.ClosingDay,
			Description: in.Description, UpdateInvoicesSince: in.UpdateInvoicesSince,
		})
		if err != nil {
			return nil, UpdateCreditCardOutput{}, err
		}
		return nil, UpdateCreditCardOutput{CreditCard: *cc}, nil
	}
}

func deleteCreditCardHandler(svc CreditCardService) mcpsdk.ToolHandlerFor[DeleteCreditCardInput, DeleteCreditCardOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in DeleteCreditCardInput) (*mcpsdk.CallToolResult, DeleteCreditCardOutput, error) {
		if err := svc.Delete(ctx, in.ID); err != nil {
			return nil, DeleteCreditCardOutput{}, err
		}
		return nil, DeleteCreditCardOutput{Deleted: true, ID: in.ID}, nil
	}
}

func registerCreditCardTools(s *mcpsdk.Server, svc CreditCardService) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_credit_cards",
		Description: "List all Organizze credit cards.",
	}, listCreditCardsHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_credit_card",
		Description: "Fetch a single Organizze credit card by id.",
	}, getCreditCardHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "create_credit_card",
		Description: "Create a new Organizze credit card. Required: name, due_day (1-31), closing_day (1-31).",
	}, createCreditCardHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "update_credit_card",
		Description: "Update fields on an existing Organizze credit card. Only fields you provide are changed.",
	}, updateCreditCardHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "delete_credit_card",
		Description: "Permanently delete an Organizze credit card by id.",
	}, deleteCreditCardHandler(svc))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/mcp/ -run CreditCard -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/mcp/tools_credit_cards.go internal/adapter/mcp/tools_credit_cards_test.go
git commit -m "feat(mcp): create_credit_card/update_credit_card/delete_credit_card tools"
```

---

## Task 13: Transfers — domain types

**Files:**
- Modify: `internal/domain/transfer.go`

- [ ] **Step 1: Add Create/Update param types**

Append to `internal/domain/transfer.go`:

```go
// CreateTransferParams are the inputs to TransferService.Create. The Organizze
// API requires credit_account_id, debit_account_id, amount_cents, date.
// "credit" is the receiving account; "debit" is the sending account.
// Only bank accounts are allowed (credit cards are rejected upstream).
type CreateTransferParams struct {
	CreditAccountID int64  `json:"credit_account_id"`
	DebitAccountID  int64  `json:"debit_account_id"`
	AmountCents     int64  `json:"amount_cents"`
	Date            string `json:"date"`
	Paid            bool   `json:"paid"`
	Tags            []Tag  `json:"tags,omitempty"`
}

// UpdateTransferParams describes a partial update; nil pointers are omitted.
// Organizze's PUT /transfers/{id} only accepts description, notes, and tags.
type UpdateTransferParams struct {
	Description *string `json:"description,omitempty"`
	Notes       *string `json:"notes,omitempty"`
	Tags        []Tag   `json:"tags,omitempty"`
}
```

- [ ] **Step 2: Verify compile**

Run: `go build ./internal/domain/...`
Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/domain/transfer.go
git commit -m "feat(domain): add CreateTransferParams + UpdateTransferParams"
```

---

## Task 14: Transfers — repository writes (TDD)

**Files:**
- Modify: `internal/adapter/organizze/transfer_repository.go`
- Test: `internal/adapter/organizze/transfer_repository_test.go`

- [ ] **Step 1: Write failing repository tests**

Append:

```go
func TestTransferRepository_Create(t *testing.T) {
	var gotBody domain.CreateTransferParams
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/transfers" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":123,"description":"Transferência","amount_cents":50000,"account_id":2,"oposite_account_id":1,"date":"2026-05-14"}`)
	})
	repo := NewTransferRepository(exec)
	tr, err := repo.Create(context.Background(), domain.CreateTransferParams{
		CreditAccountID: 2, DebitAccountID: 1, AmountCents: 50000, Date: "2026-05-14", Paid: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tr.ID != 123 {
		t.Errorf("got %+v", tr)
	}
	if gotBody.CreditAccountID != 2 || gotBody.DebitAccountID != 1 || gotBody.AmountCents != 50000 {
		t.Errorf("server received %+v", gotBody)
	}
}

func TestTransferRepository_Update_SendsOnlySetFields(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/transfers/123" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&raw)
		_, _ = io.WriteString(w, `{"id":123,"description":"Reimbursement","amount_cents":50000}`)
	})
	repo := NewTransferRepository(exec)
	desc := "Reimbursement"
	tr, err := repo.Update(context.Background(), 123, domain.UpdateTransferParams{Description: &desc})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if tr.Description != "Reimbursement" {
		t.Errorf("got %+v", tr)
	}
	if _, has := raw["notes"]; has {
		t.Errorf("absent fields must be omitted; body=%v", raw)
	}
	if raw["description"] != "Reimbursement" {
		t.Errorf("body=%v", raw)
	}
}

func TestTransferRepository_Delete(t *testing.T) {
	called := false
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodDelete || r.URL.Path != "/transfers/123" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	repo := NewTransferRepository(exec)
	if err := repo.Delete(context.Background(), 123); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !called {
		t.Error("handler not invoked")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/organizze/ -run TestTransferRepository -v`
Expected: FAIL — methods undefined.

- [ ] **Step 3: Implement Create, Update, Delete**

Append to `internal/adapter/organizze/transfer_repository.go` (add `fmt` import if not present):

```go
// Create issues a POST and returns the persisted transfer.
func (r *TransferRepository) Create(ctx context.Context, params domain.CreateTransferParams) (*domain.Transfer, error) {
	var tr domain.Transfer
	if err := r.exec.Post(ctx, "/transfers", params, &tr); err != nil {
		return nil, err
	}
	return &tr, nil
}

// Update issues a PUT with only the non-nil fields from params.
func (r *TransferRepository) Update(ctx context.Context, id int64, params domain.UpdateTransferParams) (*domain.Transfer, error) {
	var tr domain.Transfer
	if err := r.exec.Put(ctx, fmt.Sprintf("/transfers/%d", id), params, &tr); err != nil {
		return nil, err
	}
	return &tr, nil
}

// Delete issues a DELETE.
func (r *TransferRepository) Delete(ctx context.Context, id int64) error {
	return r.exec.Delete(ctx, fmt.Sprintf("/transfers/%d", id))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/organizze/ -run TestTransferRepository -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/organizze/transfer_repository.go internal/adapter/organizze/transfer_repository_test.go
git commit -m "feat(organizze): TransferRepository Create/Update/Delete"
```

---

## Task 15: Transfers — service writes (TDD)

**Files:**
- Modify: `internal/usecase/transfer.go`
- Test: `internal/usecase/transfer_test.go`

- [ ] **Step 1: Write failing service tests**

Read `internal/usecase/transfer_test.go` and extend the fake:

```go
type fakeTransferRepo struct {
	listFilter domain.ListTransfersFilter
	created    domain.CreateTransferParams
	updatedID  int64
	deletedID  int64
}

func (f *fakeTransferRepo) List(_ context.Context, fl domain.ListTransfersFilter) ([]domain.Transfer, error) {
	f.listFilter = fl
	return nil, nil
}
func (f *fakeTransferRepo) Create(_ context.Context, p domain.CreateTransferParams) (*domain.Transfer, error) {
	f.created = p
	return &domain.Transfer{ID: 123, AmountCents: p.AmountCents}, nil
}
func (f *fakeTransferRepo) Update(_ context.Context, id int64, _ domain.UpdateTransferParams) (*domain.Transfer, error) {
	f.updatedID = id
	return &domain.Transfer{ID: id}, nil
}
func (f *fakeTransferRepo) Delete(_ context.Context, id int64) error {
	f.deletedID = id
	return nil
}
```

Append:

```go
func TestTransferService_Create_ValidatesRequiredFields(t *testing.T) {
	svc := NewTransferService(&fakeTransferRepo{})
	cases := []struct {
		name string
		in   domain.CreateTransferParams
	}{
		{"credit_account_id zero", domain.CreateTransferParams{DebitAccountID: 1, AmountCents: 100, Date: "2026-05-14"}},
		{"debit_account_id zero", domain.CreateTransferParams{CreditAccountID: 2, AmountCents: 100, Date: "2026-05-14"}},
		{"amount_cents zero", domain.CreateTransferParams{CreditAccountID: 2, DebitAccountID: 1, Date: "2026-05-14"}},
		{"date missing", domain.CreateTransferParams{CreditAccountID: 2, DebitAccountID: 1, AmountCents: 100}},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if _, err := svc.Create(context.Background(), c.in); !errors.Is(err, domain.ErrValidation) {
				t.Errorf("err=%v, want ErrValidation", err)
			}
		})
	}
}

func TestTransferService_Create_Succeeds(t *testing.T) {
	repo := &fakeTransferRepo{}
	svc := NewTransferService(repo)
	tr, err := svc.Create(context.Background(), domain.CreateTransferParams{
		CreditAccountID: 2, DebitAccountID: 1, AmountCents: 50000, Date: "2026-05-14", Paid: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tr.ID != 123 || repo.created.AmountCents != 50000 {
		t.Errorf("tr=%+v repo.created=%+v", tr, repo.created)
	}
}

func TestTransferService_UpdateDelete(t *testing.T) {
	repo := &fakeTransferRepo{}
	svc := NewTransferService(repo)
	if _, err := svc.Update(context.Background(), 123, domain.UpdateTransferParams{}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if repo.updatedID != 123 {
		t.Errorf("repo.updatedID = %d", repo.updatedID)
	}
	if err := svc.Delete(context.Background(), 123); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.deletedID != 123 {
		t.Errorf("repo.deletedID = %d", repo.deletedID)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/usecase/ -run TestTransferService -v`
Expected: FAIL — compile errors.

- [ ] **Step 3: Extend interfaces and service**

Replace `internal/usecase/transfer.go` with:

```go
package usecase

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type TransferReader interface {
	List(ctx context.Context, filter domain.ListTransfersFilter) ([]domain.Transfer, error)
}

type TransferWriter interface {
	Create(ctx context.Context, params domain.CreateTransferParams) (*domain.Transfer, error)
	Update(ctx context.Context, id int64, params domain.UpdateTransferParams) (*domain.Transfer, error)
	Delete(ctx context.Context, id int64) error
}

type TransferRepository interface {
	TransferReader
	TransferWriter
}

type TransferService struct {
	repo TransferRepository
}

func NewTransferService(repo TransferRepository) *TransferService {
	return &TransferService{repo: repo}
}

func (s *TransferService) List(ctx context.Context, filter domain.ListTransfersFilter) ([]domain.Transfer, error) {
	return s.repo.List(ctx, filter)
}

func (s *TransferService) Create(ctx context.Context, p domain.CreateTransferParams) (*domain.Transfer, error) {
	switch {
	case p.CreditAccountID == 0:
		return nil, fmt.Errorf("%w: credit_account_id is required", domain.ErrValidation)
	case p.DebitAccountID == 0:
		return nil, fmt.Errorf("%w: debit_account_id is required", domain.ErrValidation)
	case p.AmountCents == 0:
		return nil, fmt.Errorf("%w: amount_cents must be non-zero", domain.ErrValidation)
	case p.Date == "":
		return nil, fmt.Errorf("%w: date is required", domain.ErrValidation)
	}
	return s.repo.Create(ctx, p)
}

func (s *TransferService) Update(ctx context.Context, id int64, p domain.UpdateTransferParams) (*domain.Transfer, error) {
	return s.repo.Update(ctx, id, p)
}

func (s *TransferService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/usecase/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/usecase/transfer.go internal/usecase/transfer_test.go
git commit -m "feat(usecase): TransferService Create/Update/Delete with validation"
```

---

## Task 16: Transfers — MCP tools (TDD)

**Files:**
- Modify: `internal/adapter/mcp/tools_transfers.go`
- Test: `internal/adapter/mcp/tools_transfers_test.go`

- [ ] **Step 1: Write failing handler tests**

Read existing fake. Extend it:

```go
type fakeTransferSvc struct {
	listFilter domain.ListTransfersFilter
	created    domain.CreateTransferParams
	updated    struct {
		id     int64
		params domain.UpdateTransferParams
	}
	deletedID int64
	createErr error
}

func (f *fakeTransferSvc) List(_ context.Context, fl domain.ListTransfersFilter) ([]domain.Transfer, error) {
	f.listFilter = fl
	return []domain.Transfer{{ID: 1}}, nil
}
func (f *fakeTransferSvc) Create(_ context.Context, p domain.CreateTransferParams) (*domain.Transfer, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.created = p
	return &domain.Transfer{ID: 123, AmountCents: p.AmountCents}, nil
}
func (f *fakeTransferSvc) Update(_ context.Context, id int64, p domain.UpdateTransferParams) (*domain.Transfer, error) {
	f.updated.id, f.updated.params = id, p
	return &domain.Transfer{ID: id}, nil
}
func (f *fakeTransferSvc) Delete(_ context.Context, id int64) error {
	f.deletedID = id
	return nil
}
```

Append new tests (mirror transactions):

```go
func TestCreateTransferHandler(t *testing.T) {
	svc := &fakeTransferSvc{}
	h := createTransferHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateTransferInput{
		CreditAccountID: 2, DebitAccountID: 1, AmountCents: 50000, Date: "2026-05-14", Paid: true,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Transfer.ID != 123 || svc.created.AmountCents != 50000 {
		t.Errorf("out=%+v svc.created=%+v", out, svc.created)
	}
}

func TestCreateTransferHandler_PropagatesValidationError(t *testing.T) {
	svc := &fakeTransferSvc{createErr: domain.ErrValidation}
	h := createTransferHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateTransferInput{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
}

func TestUpdateTransferHandler(t *testing.T) {
	svc := &fakeTransferSvc{}
	h := updateTransferHandler(svc)
	desc := "Reimbursement"
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, UpdateTransferInput{
		ID: 123, Description: &desc,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Transfer.ID != 123 || svc.updated.id != 123 || *svc.updated.params.Description != "Reimbursement" {
		t.Errorf("out=%+v svc.updated=%+v", out, svc.updated)
	}
}

func TestDeleteTransferHandler(t *testing.T) {
	svc := &fakeTransferSvc{}
	h := deleteTransferHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, DeleteTransferInput{ID: 123})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !out.Deleted || out.ID != 123 || svc.deletedID != 123 {
		t.Errorf("out=%+v svc.deletedID=%d", out, svc.deletedID)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/mcp/ -run Transfer -v`
Expected: FAIL — types and handlers undefined.

- [ ] **Step 3: Implement Input/Output types, handlers, register**

Replace `internal/adapter/mcp/tools_transfers.go` with:

```go
package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type TransferService interface {
	List(ctx context.Context, filter domain.ListTransfersFilter) ([]domain.Transfer, error)
	Create(ctx context.Context, params domain.CreateTransferParams) (*domain.Transfer, error)
	Update(ctx context.Context, id int64, params domain.UpdateTransferParams) (*domain.Transfer, error)
	Delete(ctx context.Context, id int64) error
}

type ListTransfersInput struct {
	StartDate string `json:"start_date,omitempty" jsonschema:"Optional YYYY-MM-DD lower bound."`
	EndDate   string `json:"end_date,omitempty"   jsonschema:"Optional YYYY-MM-DD upper bound."`
}

type ListTransfersOutput struct {
	Transfers []domain.Transfer `json:"transfers"`
}

type CreateTransferInput struct {
	CreditAccountID int64        `json:"credit_account_id" jsonschema:"Receiving account id (bank account; not a credit card)."`
	DebitAccountID  int64        `json:"debit_account_id"  jsonschema:"Sending account id (bank account; not a credit card)."`
	AmountCents     int64        `json:"amount_cents"      jsonschema:"Transfer amount in cents (non-zero)."`
	Date            string       `json:"date"              jsonschema:"YYYY-MM-DD."`
	Paid            bool         `json:"paid"              jsonschema:"Whether the transfer is already settled."`
	Tags            []domain.Tag `json:"tags,omitempty"    jsonschema:"Optional tags."`
}

type CreateTransferOutput struct {
	Transfer domain.Transfer `json:"transfer"`
}

type UpdateTransferInput struct {
	ID          int64        `json:"id"                  jsonschema:"The numeric Organizze transfer id to update."`
	Description *string      `json:"description,omitempty" jsonschema:"New description."`
	Notes       *string      `json:"notes,omitempty"       jsonschema:"New notes."`
	Tags        []domain.Tag `json:"tags,omitempty"        jsonschema:"Replacement tag list."`
}

type UpdateTransferOutput struct {
	Transfer domain.Transfer `json:"transfer"`
}

type DeleteTransferInput struct {
	ID int64 `json:"id" jsonschema:"The numeric Organizze transfer id to delete."`
}

type DeleteTransferOutput struct {
	Deleted bool  `json:"deleted"`
	ID      int64 `json:"id"`
}

func listTransfersHandler(svc TransferService) mcpsdk.ToolHandlerFor[ListTransfersInput, ListTransfersOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in ListTransfersInput) (*mcpsdk.CallToolResult, ListTransfersOutput, error) {
		ts, err := svc.List(ctx, domain.ListTransfersFilter{StartDate: in.StartDate, EndDate: in.EndDate})
		if err != nil {
			return nil, ListTransfersOutput{}, err
		}
		return nil, ListTransfersOutput{Transfers: ts}, nil
	}
}

func createTransferHandler(svc TransferService) mcpsdk.ToolHandlerFor[CreateTransferInput, CreateTransferOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in CreateTransferInput) (*mcpsdk.CallToolResult, CreateTransferOutput, error) {
		tr, err := svc.Create(ctx, domain.CreateTransferParams{
			CreditAccountID: in.CreditAccountID, DebitAccountID: in.DebitAccountID,
			AmountCents: in.AmountCents, Date: in.Date, Paid: in.Paid, Tags: in.Tags,
		})
		if err != nil {
			return nil, CreateTransferOutput{}, err
		}
		return nil, CreateTransferOutput{Transfer: *tr}, nil
	}
}

func updateTransferHandler(svc TransferService) mcpsdk.ToolHandlerFor[UpdateTransferInput, UpdateTransferOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in UpdateTransferInput) (*mcpsdk.CallToolResult, UpdateTransferOutput, error) {
		tr, err := svc.Update(ctx, in.ID, domain.UpdateTransferParams{
			Description: in.Description, Notes: in.Notes, Tags: in.Tags,
		})
		if err != nil {
			return nil, UpdateTransferOutput{}, err
		}
		return nil, UpdateTransferOutput{Transfer: *tr}, nil
	}
}

func deleteTransferHandler(svc TransferService) mcpsdk.ToolHandlerFor[DeleteTransferInput, DeleteTransferOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in DeleteTransferInput) (*mcpsdk.CallToolResult, DeleteTransferOutput, error) {
		if err := svc.Delete(ctx, in.ID); err != nil {
			return nil, DeleteTransferOutput{}, err
		}
		return nil, DeleteTransferOutput{Deleted: true, ID: in.ID}, nil
	}
}

func registerTransferTools(s *mcpsdk.Server, svc TransferService) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_transfers",
		Description: "List Organizze transfers, optionally filtered by date range.",
	}, listTransfersHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "create_transfer",
		Description: "Create a new Organizze transfer between two bank accounts. Required: credit_account_id (receiving), debit_account_id (sending), amount_cents, date. Credit cards are NOT accepted as source or destination.",
	}, createTransferHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "update_transfer",
		Description: "Update fields on an existing Organizze transfer. Only description, notes, and tags can be modified.",
	}, updateTransferHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "delete_transfer",
		Description: "Permanently delete an Organizze transfer by id.",
	}, deleteTransferHandler(svc))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/mcp/ -run Transfer -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/mcp/tools_transfers.go internal/adapter/mcp/tools_transfers_test.go
git commit -m "feat(mcp): create_transfer/update_transfer/delete_transfer tools"
```

---

## Task 17: Integration tests — extend fake server and roundtrip table

**Files:**
- Modify: `internal/adapter/mcp/integration_test.go`

This task wires the 12 new tools into the full-stack integration test. The fake Organizze server gains 12 new request handlers; `allExpectedTools` grows by 12 names; the roundtrip table grows by 12 cases.

- [ ] **Step 1: Extend `fakeOrganizze` with write/delete branches**

In `internal/adapter/mcp/integration_test.go`, locate the `switch` statement inside `fakeOrganizze` (`internal/adapter/mcp/integration_test.go:25-66`). Add these branches **before** the `default:` clause. Each branch returns a minimal but valid JSON body matching the domain struct.

```go
// accounts write
case r.Method == http.MethodPost && r.URL.Path == "/accounts":
	w.WriteHeader(http.StatusCreated)
	_, _ = io.WriteString(w, `{"id":18,"name":"Itaú CC","type":"checking","default":true}`)
case r.Method == http.MethodPut && r.URL.Path == "/accounts/1":
	_, _ = io.WriteString(w, `{"id":1,"name":"Checking-renamed","type":"checking"}`)
case r.Method == http.MethodDelete && r.URL.Path == "/accounts/1":
	w.WriteHeader(http.StatusNoContent)

// categories write
case r.Method == http.MethodPost && r.URL.Path == "/categories":
	w.WriteHeader(http.StatusCreated)
	_, _ = io.WriteString(w, `{"id":42,"name":"Groceries"}`)
case r.Method == http.MethodPut && r.URL.Path == "/categories/10":
	_, _ = io.WriteString(w, `{"id":10,"name":"Food-updated"}`)
case r.Method == http.MethodDelete && r.URL.Path == "/categories/10":
	w.WriteHeader(http.StatusNoContent)

// credit cards write
case r.Method == http.MethodPost && r.URL.Path == "/credit_cards":
	w.WriteHeader(http.StatusCreated)
	_, _ = io.WriteString(w, `{"id":7,"name":"Nubank","closing_day":20,"due_day":27,"limit_cents":500000}`)
case r.Method == http.MethodPut && r.URL.Path == "/credit_cards/1":
	_, _ = io.WriteString(w, `{"id":1,"name":"Nubank-renamed","closing_day":20,"due_day":27,"limit_cents":500000}`)
case r.Method == http.MethodDelete && r.URL.Path == "/credit_cards/1":
	w.WriteHeader(http.StatusNoContent)

// transfers write
case r.Method == http.MethodPost && r.URL.Path == "/transfers":
	w.WriteHeader(http.StatusCreated)
	_, _ = io.WriteString(w, `{"id":123,"description":"Transferência","amount_cents":50000,"account_id":2,"oposite_account_id":1,"date":"2026-05-14"}`)
case r.Method == http.MethodPut && r.URL.Path == "/transfers/123":
	_, _ = io.WriteString(w, `{"id":123,"description":"Reimbursement","amount_cents":50000,"account_id":2,"oposite_account_id":1,"date":"2026-05-14"}`)
case r.Method == http.MethodDelete && r.URL.Path == "/transfers/123":
	w.WriteHeader(http.StatusNoContent)
```

- [ ] **Step 2: Extend `allExpectedTools`**

Locate `allExpectedTools` (`internal/adapter/mcp/integration_test.go:119-129`). Replace it with the full 28-name list:

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
	"list_transfers",
	"create_transfer", "update_transfer", "delete_transfer",
	"list_transactions", "get_transaction",
	"create_transaction", "update_transaction", "delete_transaction",
}
```

- [ ] **Step 3: Extend the roundtrip case table**

Locate the `cases` slice in `TestIntegration_EveryToolRoundtripsThroughProtocol` (`internal/adapter/mcp/integration_test.go:169-202`). Add these entries (insert each near its existing read-only siblings, or append at the end — order doesn't matter):

```go
{"create_account", "create_account", map[string]any{
	"name": "Itaú CC", "type": "checking",
}},
{"update_account", "update_account", map[string]any{
	"id": 1, "name": "Checking-renamed",
}},
{"delete_account", "delete_account", map[string]any{"id": 1}},

{"create_category", "create_category", map[string]any{"name": "Groceries"}},
{"update_category", "update_category", map[string]any{
	"id": 10, "name": "Food-updated",
}},
{"delete_category", "delete_category", map[string]any{"id": 10}},

{"create_credit_card", "create_credit_card", map[string]any{
	"name": "Nubank", "due_day": 27, "closing_day": 20,
}},
{"update_credit_card", "update_credit_card", map[string]any{
	"id": 1, "name": "Nubank-renamed",
}},
{"delete_credit_card", "delete_credit_card", map[string]any{"id": 1}},

{"create_transfer", "create_transfer", map[string]any{
	"credit_account_id": 2, "debit_account_id": 1,
	"amount_cents": 50000, "date": "2026-05-14", "paid": true,
}},
{"update_transfer", "update_transfer", map[string]any{
	"id": 123, "description": "Reimbursement",
}},
{"delete_transfer", "delete_transfer", map[string]any{"id": 123}},
```

- [ ] **Step 4: Run the integration tests**

Run: `go test ./internal/adapter/mcp/ -run TestIntegration -v`
Expected: PASS for `TestIntegration_AllToolsRegisteredWithSchemas` (now 28 tools), `TestIntegration_EveryToolRoundtripsThroughProtocol` (now 28 cases plus the 3 budget variants), and the two error-case tests (unchanged).

- [ ] **Step 5: Run the full test suite**

Run: `go test ./...`
Expected: PASS across all packages. If any package fails, fix the root cause before continuing (the only files that should fail are ones this plan changed).

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/mcp/integration_test.go
git commit -m "test(mcp): integration coverage for accounts/categories/credit_cards/transfers write+delete"
```

---

## Task 18: README — update tool catalogue

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Locate the tool catalogue section**

Run: `grep -n "Tool catalogue\|| Tool " README.md`
Identify the markdown table that lists the 16 existing tools. (The table lives under a `## Tool catalogue` heading — see the original plan at `docs/superpowers/plans/2026-05-14-organizze-mcp.md:5059-5075`.)

- [ ] **Step 2: Replace the catalogue table with the 28-tool version**

Replace the table body. The final table should be:

```markdown
| Tool | Method | Mutating? |
|---|---|---|
| `get_user` | `UserService.Get` | no |
| `list_accounts`, `get_account` | `AccountService.{List, Get}` | no |
| `create_account`, `update_account`, `delete_account` | `AccountService.{Create, Update, Delete}` | **yes** |
| `list_categories`, `get_category` | `CategoryService.{List, Get}` | no |
| `create_category`, `update_category`, `delete_category` | `CategoryService.{Create, Update, Delete}` | **yes** |
| `list_budgets` | `BudgetService.List` | no |
| `list_credit_cards`, `get_credit_card` | `CreditCardService.{List, Get}` | no |
| `create_credit_card`, `update_credit_card`, `delete_credit_card` | `CreditCardService.{Create, Update, Delete}` | **yes** |
| `list_credit_card_invoices`, `get_credit_card_invoice` | `InvoiceService.{List, Get}` | no |
| `list_transfers` | `TransferService.List` | no |
| `create_transfer`, `update_transfer`, `delete_transfer` | `TransferService.{Create, Update, Delete}` | **yes** |
| `list_transactions`, `get_transaction` | `TransactionService.{List, Get}` | no |
| `create_transaction`, `update_transaction`, `delete_transaction` | `TransactionService.{Create, Update, Delete}` | **yes** |
```

Match the column headers and exact formatting of whatever the README currently uses — if the existing table is using `Service.Method` without backticks, drop the backticks; consistency with the existing style wins over the example above.

- [ ] **Step 3: Verify markdown renders**

Run: `grep -c "^|" README.md` to confirm the row count grew, and visually inspect the file (open it in your editor) for table alignment.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs(readme): document accounts/categories/credit_cards/transfers write+delete tools"
```

---

## Final verification

- [ ] **Step 1: Run the full test suite**

Run: `go test ./...`
Expected: PASS across every package.

- [ ] **Step 2: Verify the binary builds**

Run: `go build ./cmd/organizze-mcp`
Expected: no output, exit 0. Delete the resulting `organizze-mcp` binary or `.gitignore`-it (the repo's existing `.gitignore` should already cover it).

- [ ] **Step 3: Verify the composition root still wires correctly**

The composition root at `cmd/organizze-mcp/main.go:66-75` should NOT need any changes — every `usecase.New<Resource>Service(organizze.New<Resource>Repository(exec))` call keeps working because we only **added** methods to the existing repository concretes and split the existing repository interfaces into reader/writer halves (the concrete struct satisfies both halves automatically).

Sanity-check by reading `cmd/organizze-mcp/main.go` and confirming no edits are needed.

- [ ] **Step 4: Verify tool count via `gh` or local run (optional)**

If desired, smoke-test the binary:
```bash
ORGANIZZE_API_KEY=fake ORGANIZZE_EMAIL=test@example.com ORGANIZZE_USER_AGENT="Test (test@x.com)" \
  ./organizze-mcp < /dev/null
```
Expected: it should fail to do real work (no real API), but should boot and log `organizze-mcp v0.1.0 starting on stdio`. The 28 tools are registered at startup time, not on first request.

---

## Self-review checklist

Run these against the plan as a final pass before execution. Fix anything you find inline.

**1. Spec coverage:** Every endpoint from the user's request and the Organizze API doc is wired.

| Endpoint | Task # |
|---|---|
| POST /accounts | 1–4 |
| PUT /accounts/{id} | 1–4 |
| DELETE /accounts/{id} | 1–4 |
| POST /categories | 5–8 |
| PUT /categories/{id} | 5–8 |
| DELETE /categories/{id} (with replacement_id) | 5–8 |
| POST /credit_cards | 9–12 |
| PUT /credit_cards/{id} | 9–12 |
| DELETE /credit_cards/{id} | 9–12 |
| POST /transfers | 13–16 |
| PUT /transfers/{id} | 13–16 |
| DELETE /transfers/{id} | 13–16 |
| Integration test coverage | 17 |
| README update | 18 |

**2. Type consistency check:**
- `Create<X>Params` / `Update<X>Params` names match across domain, repo, service, MCP layers.
- Category Delete signature: `Delete(ctx, id int64, replacementID *int64)` is used in repo, service, MCP `DeleteCategoryInput.ReplacementID`. **Other** resources use the simpler `Delete(ctx, id int64)` signature.
- `update_invoices_since` lives only on credit card `UpdateCreditCardParams` and `UpdateCreditCardInput`.
- `CreditCardOutput` field name is `CreditCard` (with backing JSON `"credit_card"`), matching the existing read-only `GetCreditCardOutput`.
- Transfer body uses `credit_account_id` / `debit_account_id` not `from_account_id` / `to_account_id` (the Organizze API's naming).

**3. Placeholder scan:** No "TBD", "implement later", "similar to X", "add validation", "handle errors appropriately" — every step has concrete code or a concrete command.

**4. Test coverage per resource:**
Every resource has: repo Create + Update (omits unset fields) + Delete tests; service Create-validates + Create-succeeds + Update + Delete tests; MCP Create + Create-validation-passthrough + Update + Delete handler tests. Categories also has Delete-with-replacement and Delete-without-replacement.

**5. No master interfaces or unused symbols:** Every interface added is named per its consumer; nothing exports types that no caller uses.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-14-write-delete-endpoints.md`. Two execution options:

**1. Subagent-Driven (recommended)** — A fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.
