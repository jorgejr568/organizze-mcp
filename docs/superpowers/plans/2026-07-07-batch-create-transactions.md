# create_transactions Batch Tool Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `create_transactions` MCP tool that creates up to 100 Organizze transactions in one call, best-effort, with per-item results.

**Architecture:** Organizze has no batch endpoint, so this is pure orchestration: a new `TransactionService.CreateBatch` fans out over the existing validated single-create path (`s.Create` → `validateCreate` → `repo.Create`) with a small fixed worker pool. Nothing is rolled back; each item's success/failure is reported by index. The MCP handler maps the array input onto domain params (via a shared `toCreateParams` helper extracted from the existing single-create handler) and maps the per-item results into a structured output with `created`/`failed` totals.

**Tech Stack:** Go, `github.com/modelcontextprotocol/go-sdk` v1.6.0, standard `sync` for the worker pool.

## Global Constraints

- Optional wire fields are pointer-with-`,omitempty`; domain owns JSON tags; repositories forward params verbatim. (No new wire fields in this change.)
- Validation lives in the usecase layer and wraps `domain.ErrValidation` via `fmt.Errorf("%w: ...")` so callers can `errors.Is`.
- `mcp` package imports `domain` only — not `usecase`. The batch result type therefore lives in `domain`.
- Max batch size is **100** (`domain.MaxBatchCreateTransactions`); concurrency is a fixed **5** (`batchCreateConcurrency`), not caller-configurable.
- Best-effort semantics: per-item validation/API failures (including `domain.ErrRateLimited`) are reported per item; only an empty or over-cap batch returns a top-level error. No auto-retry, no rollback, no auto-chunking.
- `make test && make lint && make build` must all pass. CI runs `go test -race -count=1 ./...`.
- Every bot-authored commit ends with `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`.

---

### Task 1: Usecase `CreateBatch` (+ domain constant & result type)

**Files:**
- Modify: `internal/domain/transaction.go` (append constant + `BatchCreateResult` type)
- Modify: `internal/usecase/transaction.go` (add `sync` import, `batchCreateConcurrency` const, `CreateBatch` method)
- Test: `internal/usecase/transaction_test.go` (append batch tests + a configurable fake repo)

**Interfaces:**
- Consumes: existing `TransactionService.Create(ctx, domain.CreateTransactionParams) (*domain.Transaction, error)` and `domain.CreateTransactionParams`.
- Produces:
  - `const domain.MaxBatchCreateTransactions = 100`
  - `type domain.BatchCreateResult struct { Index int; Transaction *domain.Transaction; Err error }`
  - `func (s *TransactionService) CreateBatch(ctx context.Context, items []domain.CreateTransactionParams) ([]domain.BatchCreateResult, error)`

- [ ] **Step 1: Add the domain constant and result type**

Append to the end of `internal/domain/transaction.go`:

```go
// MaxBatchCreateTransactions caps a single create_transactions call. Organizze
// has no batch endpoint, so the tool fans out to individual POSTs; the cap
// bounds fan-out and matches the documented tool contract.
const MaxBatchCreateTransactions = 100

// BatchCreateResult is the outcome for one item in a batch create. Exactly one
// of Transaction / Err is non-nil. Index is the item's position in the input
// slice, preserved regardless of the order goroutines finish in. It lives in
// domain (not usecase) so the mcp adapter can consume it without importing
// usecase.
type BatchCreateResult struct {
	Index       int
	Transaction *Transaction
	Err         error
}
```

- [ ] **Step 2: Write the failing usecase tests**

Append to `internal/usecase/transaction_test.go`. First add `"sync"` to the import block (it currently imports `context`, `errors`, `testing`, and the domain package), then append:

```go
var errBatchBoom = errors.New("boom")

// batchRepo echoes each created transaction so tests can map results back to
// inputs by Description, counts how many Creates reached the repo, and fails
// Create for any params whose Description == failOn.
type batchRepo struct {
	mu       sync.Mutex
	createdN int
	failOn   string
}

func (r *batchRepo) List(context.Context, domain.ListTransactionsFilter) ([]domain.Transaction, error) {
	return nil, nil
}
func (r *batchRepo) Get(_ context.Context, id int64) (*domain.Transaction, error) {
	return &domain.Transaction{ID: id}, nil
}
func (r *batchRepo) Create(_ context.Context, p domain.CreateTransactionParams) (*domain.Transaction, error) {
	r.mu.Lock()
	r.createdN++
	r.mu.Unlock()
	if r.failOn != "" && p.Description == r.failOn {
		return nil, errBatchBoom
	}
	return &domain.Transaction{Description: p.Description, AmountCents: p.AmountCents}, nil
}
func (r *batchRepo) Update(context.Context, int64, domain.UpdateTransactionParams) (*domain.Transaction, error) {
	return nil, nil
}
func (r *batchRepo) Delete(context.Context, int64, domain.DeleteTransactionParams) (*domain.Transaction, error) {
	return nil, nil
}

func validBatchItem(desc string) domain.CreateTransactionParams {
	return domain.CreateTransactionParams{
		Description: desc, Date: "2026-05-14", AmountCents: -1500, AccountID: 1, CategoryID: 10,
	}
}

func TestCreateBatch_AllSucceed_PreservesOrder(t *testing.T) {
	repo := &batchRepo{}
	svc := NewTransactionService(repo)
	items := []domain.CreateTransactionParams{
		validBatchItem("a"), validBatchItem("b"), validBatchItem("c"),
	}
	results, err := svc.CreateBatch(context.Background(), items)
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}
	for i, r := range results {
		if r.Index != i {
			t.Errorf("results[%d].Index = %d", i, r.Index)
		}
		if r.Err != nil {
			t.Errorf("results[%d].Err = %v, want nil", i, r.Err)
		}
		if r.Transaction == nil || r.Transaction.Description != items[i].Description {
			t.Errorf("results[%d].Transaction = %+v, want Description %q", i, r.Transaction, items[i].Description)
		}
	}
	if repo.createdN != 3 {
		t.Errorf("repo.createdN = %d, want 3", repo.createdN)
	}
}

func TestCreateBatch_ValidationErrorIsolated(t *testing.T) {
	repo := &batchRepo{}
	svc := NewTransactionService(repo)
	bad := validBatchItem("b")
	bad.CategoryID = 0 // fails validateCreate before reaching the repo
	items := []domain.CreateTransactionParams{validBatchItem("a"), bad, validBatchItem("c")}
	results, err := svc.CreateBatch(context.Background(), items)
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}
	if results[0].Err != nil || results[2].Err != nil {
		t.Errorf("neighbours failed: [0]=%v [2]=%v", results[0].Err, results[2].Err)
	}
	if !errors.Is(results[1].Err, domain.ErrValidation) {
		t.Errorf("results[1].Err = %v, want ErrValidation", results[1].Err)
	}
	if repo.createdN != 2 {
		t.Errorf("repo.createdN = %d, want 2 (invalid item must not hit the repo)", repo.createdN)
	}
}

func TestCreateBatch_APIErrorIsolated(t *testing.T) {
	repo := &batchRepo{failOn: "boom"}
	svc := NewTransactionService(repo)
	items := []domain.CreateTransactionParams{
		validBatchItem("a"), validBatchItem("boom"), validBatchItem("c"),
	}
	results, err := svc.CreateBatch(context.Background(), items)
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}
	if !errors.Is(results[1].Err, errBatchBoom) {
		t.Errorf("results[1].Err = %v, want errBatchBoom", results[1].Err)
	}
	if results[0].Err != nil || results[2].Err != nil {
		t.Errorf("neighbours failed: [0]=%v [2]=%v", results[0].Err, results[2].Err)
	}
}

func TestCreateBatch_RejectsEmpty(t *testing.T) {
	svc := NewTransactionService(&batchRepo{})
	if _, err := svc.CreateBatch(context.Background(), nil); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
}

func TestCreateBatch_RejectsOverCap(t *testing.T) {
	repo := &batchRepo{}
	svc := NewTransactionService(repo)
	items := make([]domain.CreateTransactionParams, domain.MaxBatchCreateTransactions+1)
	for i := range items {
		items[i] = validBatchItem("x")
	}
	if _, err := svc.CreateBatch(context.Background(), items); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
	if repo.createdN != 0 {
		t.Errorf("repo.createdN = %d, want 0 (guard must short-circuit before fan-out)", repo.createdN)
	}
}

func TestCreateBatch_PreservesOrderUnderConcurrency(t *testing.T) {
	repo := &batchRepo{}
	svc := NewTransactionService(repo)
	items := make([]domain.CreateTransactionParams, 25)
	for i := range items {
		items[i] = validBatchItem(fmt.Sprintf("item-%02d", i))
	}
	results, err := svc.CreateBatch(context.Background(), items)
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}
	for i := range items {
		if results[i].Transaction == nil || results[i].Transaction.Description != items[i].Description {
			t.Fatalf("results[%d] = %+v, want Description %q", i, results[i].Transaction, items[i].Description)
		}
	}
}
```

Add `"fmt"` to the test file's import block (used by the concurrency test).

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/usecase/... -run TestCreateBatch -v`
Expected: compile error / FAIL — `svc.CreateBatch` undefined.

- [ ] **Step 4: Implement `CreateBatch`**

In `internal/usecase/transaction.go`, add `"sync"` to the import block (currently `context`, `fmt`, and the domain package), then add near the other methods:

```go
// batchCreateConcurrency bounds how many item POSTs are in flight at once
// during CreateBatch. Organizze has no batch endpoint and rate-limits, so we
// fan out with a small fixed worker pool rather than all-at-once.
const batchCreateConcurrency = 5

// CreateBatch creates up to domain.MaxBatchCreateTransactions transactions,
// each via the same validated single-create path as Create. It is best-effort:
// every item is attempted, and per-item validation or API failures (including
// domain.ErrRateLimited) land in that item's BatchCreateResult.Err rather than
// aborting the batch. The returned slice is index-aligned with items regardless
// of the order goroutines finish in. A non-nil top-level error is returned only
// when the batch itself is invalid (empty or over the cap); in that case no
// item is attempted.
func (s *TransactionService) CreateBatch(ctx context.Context, items []domain.CreateTransactionParams) ([]domain.BatchCreateResult, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: at least one transaction is required", domain.ErrValidation)
	}
	if len(items) > domain.MaxBatchCreateTransactions {
		return nil, fmt.Errorf("%w: at most %d transactions per batch, got %d", domain.ErrValidation, domain.MaxBatchCreateTransactions, len(items))
	}

	results := make([]domain.BatchCreateResult, len(items))
	sem := make(chan struct{}, batchCreateConcurrency)
	var wg sync.WaitGroup
	for i := range items {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			// Each goroutine writes only results[i]; no shared mutation, no mutex.
			tx, err := s.Create(ctx, items[i])
			results[i] = domain.BatchCreateResult{Index: i, Transaction: tx, Err: err}
		}(i)
	}
	wg.Wait()
	return results, nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/usecase/... -run TestCreateBatch -race -v`
Expected: PASS (all six tests, no race).

- [ ] **Step 6: Commit**

```bash
git add internal/domain/transaction.go internal/usecase/transaction.go internal/usecase/transaction_test.go
git commit -m "$(cat <<'EOF'
feat(usecase): add TransactionService.CreateBatch

Best-effort batch create over the existing validated single-create path,
bounded to 5 concurrent POSTs and capped at 100 items. Organizze has no
batch endpoint, so per-item failures are reported by index instead of
aborting the batch.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: MCP `create_transactions` tool

**Files:**
- Modify: `internal/adapter/mcp/tools_transactions.go` (extract `toCreateParams`; add batch input/output types, handler, interface method, registration)
- Modify: `internal/adapter/mcp/tools_transactions_test.go` (add `CreateBatch` to both fakes; add handler tests)
- Modify: `internal/adapter/mcp/integration_test.go` (add `"create_transactions"` to `allExpectedTools`)

**Interfaces:**
- Consumes: `domain.BatchCreateResult`, `domain.CreateTransactionParams`, `(*TransactionService).CreateBatch` from Task 1; existing `CreateTransactionInput`, `RecurrenceInput`, `InstallmentsInput`.
- Produces:
  - `func toCreateParams(in CreateTransactionInput) domain.CreateTransactionParams`
  - `type CreateTransactionsInput struct { Transactions []CreateTransactionInput }`
  - `type CreateTransactionResult struct { Index int; Success bool; Transaction *domain.Transaction; Error string }`
  - `type CreateTransactionsOutput struct { Results []CreateTransactionResult; Created int; Failed int }`
  - `func createTransactionsHandler(svc TransactionService) mcpsdk.ToolHandlerFor[CreateTransactionsInput, CreateTransactionsOutput]`
  - new interface method `CreateBatch(ctx, []domain.CreateTransactionParams) ([]domain.BatchCreateResult, error)` on the local `TransactionService` interface
  - registered tool `create_transactions`

- [ ] **Step 1: Extract `toCreateParams` and rewrite the single-create handler to use it**

In `internal/adapter/mcp/tools_transactions.go`, add this helper (place it just above `createTransactionHandler`):

```go
// toCreateParams maps a create input onto the domain params, including the
// recurrence/installments sub-structs. Shared by create_transaction and
// create_transactions so the mapping lives in exactly one place.
func toCreateParams(in CreateTransactionInput) domain.CreateTransactionParams {
	params := domain.CreateTransactionParams{
		Description: in.Description, Date: in.Date, AmountCents: in.AmountCents,
		AccountID: in.AccountID, CategoryID: in.CategoryID, Paid: in.Paid,
		Notes: in.Notes, ContactID: in.ContactID, Tags: in.Tags,
		CreditCardID:        in.CreditCardID,
		CreditCardInvoiceID: in.CreditCardInvoiceID,
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
	return params
}
```

Replace the body of `createTransactionHandler` so it reuses the helper:

```go
func createTransactionHandler(svc TransactionService) mcpsdk.ToolHandlerFor[CreateTransactionInput, CreateTransactionOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in CreateTransactionInput) (*mcpsdk.CallToolResult, CreateTransactionOutput, error) {
		tx, err := svc.Create(ctx, toCreateParams(in))
		if err != nil {
			return nil, CreateTransactionOutput{}, err
		}
		return nil, CreateTransactionOutput{Transaction: *tx}, nil
	}
}
```

- [ ] **Step 2: Verify the extraction is behavior-preserving**

Run: `go test ./internal/adapter/mcp/... -run TestCreateTransactionHandler -v`
Expected: PASS — the existing single-create handler tests (including `_PlumbsRecurrence`, `_PlumbsInstallments`, `_PlumbsCreditCardFields`) still pass, proving `toCreateParams` preserves behavior.

- [ ] **Step 3: Add `CreateBatch` to the `TransactionService` interface and both test fakes**

In `internal/adapter/mcp/tools_transactions.go`, add the method to the `TransactionService` interface:

```go
type TransactionService interface {
	List(ctx context.Context, filter domain.ListTransactionsFilter) ([]domain.Transaction, error)
	Get(ctx context.Context, id int64) (*domain.Transaction, error)
	Create(ctx context.Context, params domain.CreateTransactionParams) (*domain.Transaction, error)
	CreateBatch(ctx context.Context, params []domain.CreateTransactionParams) ([]domain.BatchCreateResult, error)
	Update(ctx context.Context, id int64, params domain.UpdateTransactionParams) (*domain.Transaction, error)
	Delete(ctx context.Context, id int64, params domain.DeleteTransactionParams) (*domain.Transaction, error)
}
```

In `internal/adapter/mcp/tools_transactions_test.go`, add a field to `fakeTransactionSvc` (add `batchParams []domain.CreateTransactionParams` and `batchErr error` to the struct) and implement `CreateBatch` for both fakes:

```go
func (f *fakeTransactionSvc) CreateBatch(_ context.Context, params []domain.CreateTransactionParams) ([]domain.BatchCreateResult, error) {
	if f.batchErr != nil {
		return nil, f.batchErr
	}
	f.batchParams = params
	results := make([]domain.BatchCreateResult, len(params))
	for i, p := range params {
		if p.Description == "fail" {
			results[i] = domain.BatchCreateResult{Index: i, Err: domain.ErrValidation}
			continue
		}
		results[i] = domain.BatchCreateResult{
			Index:       i,
			Transaction: &domain.Transaction{ID: int64(700 + i), Description: p.Description, AmountCents: p.AmountCents},
		}
	}
	return results, nil
}

func (nopTransactionSvc) CreateBatch(context.Context, []domain.CreateTransactionParams) ([]domain.BatchCreateResult, error) {
	return nil, nil
}
```

The `fakeTransactionSvc` struct definition becomes:

```go
type fakeTransactionSvc struct {
	listFilter domain.ListTransactionsFilter
	created    domain.CreateTransactionParams
	updated    struct {
		id     int64
		params domain.UpdateTransactionParams
	}
	deletedID   int64
	createErr   error
	batchParams []domain.CreateTransactionParams
	batchErr    error
}
```

- [ ] **Step 4: Write the failing handler tests**

Append to `internal/adapter/mcp/tools_transactions_test.go`:

```go
func TestCreateTransactionsHandler_MapsResultsAndCounts(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := createTransactionsHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateTransactionsInput{
		Transactions: []CreateTransactionInput{
			{Description: "a", Date: "2026-05-14", AmountCents: -1500, AccountID: 1, CategoryID: 10},
			{Description: "fail", Date: "2026-05-14", AmountCents: -1500, AccountID: 1, CategoryID: 10},
			{Description: "c", Date: "2026-05-14", AmountCents: -2500, AccountID: 1, CategoryID: 10},
		},
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Created != 2 || out.Failed != 1 {
		t.Errorf("created=%d failed=%d, want 2/1", out.Created, out.Failed)
	}
	if len(out.Results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(out.Results))
	}
	if !out.Results[0].Success || out.Results[0].Transaction == nil {
		t.Errorf("results[0] = %+v, want success", out.Results[0])
	}
	if out.Results[1].Success || out.Results[1].Error == "" {
		t.Errorf("results[1] = %+v, want failure with error text", out.Results[1])
	}
	if out.Results[2].Index != 2 {
		t.Errorf("results[2].Index = %d, want 2", out.Results[2].Index)
	}
	// toCreateParams was applied to each item before dispatch.
	if len(svc.batchParams) != 3 || svc.batchParams[2].AmountCents != -2500 {
		t.Errorf("batchParams = %+v", svc.batchParams)
	}
}

func TestCreateTransactionsHandler_PlumbsInstallments(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := createTransactionsHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateTransactionsInput{
		Transactions: []CreateTransactionInput{
			{
				Description: "Computador", Date: "2026-05-14", AmountCents: -100000,
				AccountID: 1, CategoryID: 10,
				Installments: &InstallmentsInput{Periodicity: "monthly", Total: 12},
			},
		},
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(svc.batchParams) != 1 || svc.batchParams[0].Installments == nil {
		t.Fatalf("installments not forwarded: %+v", svc.batchParams)
	}
	if svc.batchParams[0].Installments.Total != 12 {
		t.Errorf("total = %d, want 12", svc.batchParams[0].Installments.Total)
	}
}

func TestCreateTransactionsHandler_PropagatesTopLevelError(t *testing.T) {
	svc := &fakeTransactionSvc{batchErr: domain.ErrValidation}
	h := createTransactionsHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateTransactionsInput{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
}
```

- [ ] **Step 5: Run the tests to verify they fail**

Run: `go test ./internal/adapter/mcp/... -run TestCreateTransactionsHandler -v`
Expected: compile error / FAIL — `createTransactionsHandler`, `CreateTransactionsInput`, `CreateTransactionsOutput` undefined.

- [ ] **Step 6: Implement the batch types, handler, and registration**

In `internal/adapter/mcp/tools_transactions.go`, add the batch input/output types (place after the `CreateTransactionOutput` block):

```go
// ---------- batch create ----------

type CreateTransactionsInput struct {
	Transactions []CreateTransactionInput `json:"transactions" jsonschema:"The transactions to create, 1 to 100 items. Each item follows the exact same rules as create_transaction (account routing, installments-total, recurrence/installments exclusivity)."`
}

// CreateTransactionResult is the per-item outcome. Success reports whether the
// item was created; on success Transaction is populated, on failure Error
// carries the reason. Index is the item's position in the request array.
type CreateTransactionResult struct {
	Index       int                 `json:"index"`
	Success     bool                `json:"success"`
	Transaction *domain.Transaction `json:"transaction,omitempty"`
	Error       string              `json:"error,omitempty"`
}

type CreateTransactionsOutput struct {
	Results []CreateTransactionResult `json:"results"`
	Created int                       `json:"created"`
	Failed  int                       `json:"failed"`
}
```

Add the handler (place after `createTransactionHandler`):

```go
func createTransactionsHandler(svc TransactionService) mcpsdk.ToolHandlerFor[CreateTransactionsInput, CreateTransactionsOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in CreateTransactionsInput) (*mcpsdk.CallToolResult, CreateTransactionsOutput, error) {
		params := make([]domain.CreateTransactionParams, len(in.Transactions))
		for i, item := range in.Transactions {
			params[i] = toCreateParams(item)
		}
		results, err := svc.CreateBatch(ctx, params)
		if err != nil {
			return nil, CreateTransactionsOutput{}, err
		}
		out := CreateTransactionsOutput{Results: make([]CreateTransactionResult, len(results))}
		for i, r := range results {
			item := CreateTransactionResult{Index: r.Index, Success: r.Err == nil, Transaction: r.Transaction}
			if r.Err != nil {
				item.Error = r.Err.Error()
				out.Failed++
			} else {
				out.Created++
			}
			out.Results[i] = item
		}
		return nil, out, nil
	}
}
```

Register the tool inside `registerTransactionTools`, after the `create_transaction` registration:

```go
addInstrumentedTool(s, inst, &mcpsdk.Tool{
	Name: "create_transactions",
	Description: "Create up to 100 Organizze transactions in a single call. Each item follows the EXACT same rules as create_transaction (amount_cents in cents; exactly one of account_id or credit_card_id; when installments is set amount_cents is the TOTAL across installments; recurrence and installments are mutually exclusive). " +
		"BEST-EFFORT: items are created independently and a failure on one does NOT stop the others. Inspect the response: `results` has one entry per input item with `index`, `success`, and either `transaction` (on success) or `error` (on failure); `created` and `failed` are the totals. Retry only the failed indices. The whole call is rejected up-front only if the batch is empty or has more than 100 items.",
}, createTransactionsHandler(svc))
```

- [ ] **Step 7: Keep the tool-count integration test green**

In `internal/adapter/mcp/integration_test.go`, add `"create_transactions"` to the `allExpectedTools` slice, next to the other transaction tools:

```go
	"list_transactions", "get_transaction",
	"create_transaction", "create_transactions", "update_transaction", "delete_transaction",
```

- [ ] **Step 8: Run the MCP package tests**

Run: `go test ./internal/adapter/mcp/... -race -v`
Expected: PASS — new handler tests pass; `TestIntegration_AllToolsRegisteredWithSchemas` passes with the updated expected-tools list; existing create/update/delete tests unaffected.

- [ ] **Step 9: Commit**

```bash
git add internal/adapter/mcp/tools_transactions.go internal/adapter/mcp/tools_transactions_test.go internal/adapter/mcp/integration_test.go
git commit -m "$(cat <<'EOF'
feat(mcp): add create_transactions batch tool

One tool call creates up to 100 transactions best-effort, returning a
per-item results array with created/failed totals. Extracts the shared
toCreateParams mapping so single- and batch-create stay in lockstep.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Integration roundtrip + CHANGELOG

**Files:**
- Modify: `internal/adapter/mcp/integration_test.go` (add a roundtrip case + a best-effort partial-failure test)
- Modify: `CHANGELOG.md` (add an `## [Unreleased]` → `### Added` entry)

**Interfaces:**
- Consumes: the registered `create_transactions` tool and the existing `fakeOrganizze` server (POST `/transactions` already returns `777`). No fake-server change is needed: valid items POST to the fake; an invalid item is rejected client-side by the service and never reaches the server.

- [ ] **Step 1: Write the failing integration tests**

In `internal/adapter/mcp/integration_test.go`, add a roundtrip case to the `cases` slice in `TestIntegration_EveryToolRoundtripsThroughProtocol` (after the `create_transaction` case):

```go
{"create_transactions", "create_transactions", map[string]any{
	"transactions": []any{
		map[string]any{
			"description": "Coffee", "date": "2026-05-14", "amount_cents": -1500,
			"account_id": 1, "category_id": 10, "paid": true,
		},
		map[string]any{
			"description": "Lunch", "date": "2026-05-14", "amount_cents": -3200,
			"account_id": 1, "category_id": 10, "paid": true,
		},
	},
}},
```

Then add a dedicated best-effort test at the end of the file:

```go
// A batch mixing a valid item with one that fails service-layer validation
// (missing category_id) must NOT surface as a tool error — best-effort means
// the call succeeds and the failure is reported per item. A fail-fast or
// all-or-nothing design would make this call error instead.
func TestIntegration_CreateTransactions_PartialFailureIsNotToolError(t *testing.T) {
	sess := newConnectedSession(t)
	res, err := sess.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "create_transactions", Arguments: map[string]any{
			"transactions": []any{
				map[string]any{
					"description": "Coffee", "date": "2026-05-14", "amount_cents": -1500,
					"account_id": 1, "category_id": 10, "paid": true,
				},
				map[string]any{
					// no category_id → rejected in the service layer, never hits the server
					"description": "Broken", "date": "2026-05-14", "amount_cents": -1500,
					"account_id": 1,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected best-effort success (IsError=false); content=%v", res.Content)
	}
	if len(res.Content) == 0 {
		t.Errorf("no content")
	}
}
```

- [ ] **Step 2: Run the integration tests to verify they pass**

Run: `go test ./internal/adapter/mcp/... -run TestIntegration -race -v`
Expected: PASS — the roundtrip case issues two POSTs to the fake server; the partial-failure case returns `IsError=false` with content. (These are new tests exercising newly-wired behavior; they pass because Task 2 registered the tool.)

- [ ] **Step 3: Add the CHANGELOG entry**

In `CHANGELOG.md`, under `## [Unreleased]`, add (create the `### Added` subsection if absent):

```markdown
### Added

- `create_transactions` tool: create up to 100 transactions in a single call.
  Organizze has no batch endpoint, so the server fans out to individual POSTs
  (bounded concurrency) — the call is **best-effort**: each item succeeds or
  fails independently and the response returns a per-item `results` array plus
  `created`/`failed` totals. Nothing is rolled back; retry only the failed
  indices. Batches that are empty or exceed 100 items are rejected up-front.
  Each item follows the same rules as `create_transaction` (account routing,
  installments-total-is-total, recurrence/installments exclusivity).
```

- [ ] **Step 4: Run the full suite, lint, and build**

Run: `make test && make lint && make build`
Expected: all succeed. Then mirror CI:
Run: `go test ./... -race -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/mcp/integration_test.go CHANGELOG.md
git commit -m "$(cat <<'EOF'
test(mcp): integration roundtrip for create_transactions + CHANGELOG

Covers the happy-path protocol roundtrip (two POSTs) and asserts that a
mixed batch with one client-rejected item is best-effort — the tool call
succeeds rather than erroring.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Post-implementation (per AGENTS.md)

This lands via the standard PR flow (feature branch in a worktree → PR → CI green → squash-merge). New tool + additive → **minor version bump** in a separate follow-up release PR (promote `## [Unreleased]`, tag `vX.Y.0`, manual GitHub release).

## Self-Review notes

- **Spec coverage:** domain constant/type (Task 1 Step 1); usecase `CreateBatch` with size guard + bounded concurrency + best-effort per-item (Task 1); `toCreateParams` extraction, batch types, handler, interface, registration (Task 2); usecase tests (Task 1 Step 2), MCP handler tests (Task 2 Step 4), integration roundtrip + partial-failure (Task 3); CHANGELOG (Task 3 Step 3). No wire-shape/jsonshape tests — no new wire field, matching the spec.
- **Best-effort at the stats layer (noted, intentional):** per-item failures live inside the output, so `instrument` records the batch tool call as `status=ok`. That is correct — the tool call itself succeeded. No change to `instrument.go` is needed.
- **Type consistency:** `domain.BatchCreateResult{Index, Transaction, Err}` is produced in Task 1 and consumed unchanged in Task 2's interface/handler; `toCreateParams`, `CreateTransactionsInput/Output`, `CreateTransactionResult`, and `createTransactionsHandler` names are used identically across tasks.
