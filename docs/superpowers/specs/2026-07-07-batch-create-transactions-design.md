# Design: `create_transactions` batch tool

**Date:** 2026-07-07
**Status:** Approved (pending implementation plan)
**Type:** New MCP tool (additive) → minor version bump

## Problem

Callers can only create Organizze transactions one at a time via `create_transaction`.
Bulk entry (e.g. importing a month of expenses) means one MCP tool call per row, which
is slow and noisy for LLM clients. We want a single tool that accepts many transactions
at once.

## Constraint: Organizze has no batch endpoint

The Organizze REST API v2 `POST /transactions` creates exactly one transaction. There is
no bulk/batch endpoint. Therefore this tool is **pure orchestration on our side** — it
loops over N individual POSTs. Consequences:

- **No new wire types.** The batch reuses `domain.CreateTransactionParams`.
- **No repository changes.** It reuses `TransactionRepository.Create` per item.
- **Partial failure is intrinsic** — some POSTs can succeed while others fail.

## Decisions (from brainstorming)

| Question | Decision |
| --- | --- |
| Partial-failure semantics | **Best-effort + per-item report.** Attempt all items; return a per-item result array (success + created transaction, or failure + error). Nothing is rolled back. |
| Concurrency | **Small bounded concurrency = 5** in flight (worker pool). |
| Per-item validation error | **Report per-item, keep going.** Item is marked failed with its validation error; no network call made for it; other items still created. |
| Batch size limit | **Reject `> 100` (and `0`) up-front** with `ErrValidation`. No auto-chunking. |
| Rate-limit mid-batch | Surfaced as per-item failures (`domain.ErrRateLimited`). **No auto-retry** (YAGNI; caller retries the failed indices). |
| Concurrency configurable? | **No** — fixed constant. Easy to expose later. |

## Architecture (four layers)

### Domain — `internal/domain/transaction.go`

Add one constant and one orchestration result type. The result type lives in `domain`
(alongside `ListTransactionsFilter`, which is likewise orchestration rather than a raw
wire shape) so that both `usecase` and the `mcp` adapter reference it without the `mcp`
package having to import `usecase` — preserving the existing "mcp imports domain only"
boundary.

```go
// MaxBatchCreateTransactions caps a single create_transactions call. Organizze
// has no batch endpoint, so the tool fans out to individual POSTs; the cap
// bounds fan-out and matches the documented tool contract.
const MaxBatchCreateTransactions = 100

// BatchCreateResult is the outcome for one item in a batch create. Exactly one of
// Transaction / Err is non-nil. Index is the item's position in the input slice,
// preserved regardless of completion order under concurrency.
type BatchCreateResult struct {
    Index       int
    Transaction *Transaction
    Err         error
}
```

### Usecase — `internal/usecase/transaction.go`

New method on `TransactionService` (returns the domain result type):

```go
func (s *TransactionService) CreateBatch(
    ctx context.Context,
    items []domain.CreateTransactionParams,
) ([]domain.BatchCreateResult, error)
```

Behaviour:

1. **Size guard** (the only path returning a top-level error):
   - `len(items) == 0` → `fmt.Errorf("%w: at least one transaction is required", domain.ErrValidation)`
   - `len(items) > domain.MaxBatchCreateTransactions` → `fmt.Errorf("%w: at most %d transactions per batch", domain.ErrValidation, domain.MaxBatchCreateTransactions)`
2. **Fan out** with a bounded worker pool, concurrency = `batchCreateConcurrency` (const = 5):
   - Semaphore channel `sem := make(chan struct{}, batchCreateConcurrency)`.
   - One goroutine per item; each acquires the semaphore, calls `s.Create(ctx, items[i])`
     (which runs `validateCreate` then `repo.Create`), and writes `results[i]`.
   - Each goroutine writes **only its own index** → no shared-state mutation, no mutex,
     order preserved.
   - `sync.WaitGroup` to await completion.
3. Return `(results, nil)`. Per-item validation errors and API errors (incl.
   `domain.ErrRateLimited`) live in `results[i].Err`; the batch itself succeeds.

Notes:
- Reuses `s.Create`, so every existing gotcha (account routing, installments-total,
  recurrence/installments exclusivity) is enforced per item with zero duplication.
- Context cancellation: if `ctx` is done, in-flight `s.Create` calls return the ctx
  error, which is recorded per item like any other failure.

### Adapter (HTTP) — `internal/adapter/organizze/`

**No changes.** `CreateBatch` calls the existing `Create` per item.

### Adapter (MCP) — `internal/adapter/mcp/tools_transactions.go`

1. **Extract mapping helper** (removes duplication between single and batch create):

   ```go
   func toCreateParams(in CreateTransactionInput) domain.CreateTransactionParams
   ```

   Move the existing `createTransactionHandler` body's input→params mapping (including
   the `Recurrence`/`Installments` sub-struct handling) into this helper; call it from
   both handlers.

2. **New input/output types:**

   ```go
   type CreateTransactionsInput struct {
       Transactions []CreateTransactionInput `json:"transactions" jsonschema:"..."`
   }

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

3. **Handler** `createTransactionsHandler`:
   - Map each `CreateTransactionInput` → params via `toCreateParams`.
   - Call `svc.CreateBatch`. If it returns a top-level error (size guard), return it.
   - Map each `domain.BatchCreateResult` → `CreateTransactionResult`
     (`Success = Err == nil`; `Error = err.Error()` when failed; increment `Created`/`Failed`).

4. **Interface:** add `CreateBatch(ctx, []domain.CreateTransactionParams) ([]domain.BatchCreateResult, error)`
   to the local `TransactionService` interface. Because the result type lives in `domain`
   (see Domain layer above), the `mcp` package needs no new import — it already imports
   `domain` — and the "mcp imports domain only" boundary is preserved.

5. **Register tool** in `registerTransactionTools`:
   - Name: `create_transactions`.
   - Description: creates up to 100 transactions in one call; **best-effort — some may
     succeed while others fail**; inspect `results[]` (each has `index`, `success`,
     `transaction` or `error`) plus `created`/`failed` counts; same per-item rules as
     `create_transaction` (account routing, installments-total-is-total, recurrence vs
     installments exclusivity).

### Composition root — `cmd/organizze-mcp/main.go`

No change expected: the tool is registered inside `registerTransactionTools`, which
already receives the transaction service.

## Testing (layers that apply, per AGENTS.md)

No wire-shape (`*_repository_test.go`) or jsonshape (`jsonshape_test.go`) test — there is
no new wire field.

- **Usecase** `internal/usecase/transaction_test.go`:
  - all items succeed → all results have `Transaction`, no `Err`.
  - one item invalid (e.g. missing `category_id`) → that index has `Err` (via
    `errors.Is(..., domain.ErrValidation)`); the others succeed.
  - one item's repo `Create` returns an API error → isolated to that index.
  - `len == 0` → top-level `ErrValidation`.
  - `len > 100` → top-level `ErrValidation`.
  - order preserved: results[i] corresponds to items[i] even though a fake repo
    completes them out of order.
- **MCP** `internal/adapter/mcp/tools_transactions_test.go`:
  - input→params plumbing for a batch (fields, including installments/recurrence, reach
    the params correctly via `toCreateParams`).
  - result/summary mapping: mixed success/failure batch yields correct `Success`,
    `Error`, `Created`, `Failed`.
- **Integration** `internal/adapter/mcp/integration_test.go`:
  - end-to-end `create_transactions` roundtrip against the fake Organizze server,
    including a mixed batch where one item is rejected by the fake server.

## CHANGELOG

Add under `## [Unreleased]` → `### Added`: the `create_transactions` tool — create up to
100 transactions per call, best-effort with per-item results, bounded concurrency;
note that Organizze has no batch endpoint so this fans out to individual POSTs and
partial success is possible.

## Versioning

New tool, additive → **minor bump** (`X.Y+1.0`) in the follow-up release PR.

## Out of scope (YAGNI)

- Configurable concurrency.
- Auto-retry on rate-limit.
- Auto-chunking of `> 100`.
- All-or-nothing rollback.
- Batch update / batch delete (separate future work if wanted).
