# Plan: recurring fixed transactions in create_transaction

## Goal

Support the Organizze "Cria uma movimentação recorrente fixa" endpoint variant of
`POST /transactions` from the MCP `create_transaction` tool. Today the tool
omits the `recurrence_attributes` block, so callers cannot create fixed
recurring transactions.

## API contract (from organizze/api-doc)

`POST /transactions` accepts an optional nested object:

```json
{
  "description": "Despesa fixa",
  "notes": "Pagamento via boleto",
  "date": "2015-09-16",
  "recurrence_attributes": {"periodicity": "monthly"}
}
```

`periodicity` ∈ `{monthly, yearly, weekly, biweekly, bimonthly, trimonthly}`.

When present, the response carries `"recurring": true`.

## Design decisions

1. **Optional pointer** — `Recurrence *RecurrenceAttributes` with `omitempty`,
   so omission keeps the existing one-off-create behaviour byte-identical on
   the wire.
2. **Periodicity allowlist** — validated at the usecase layer, wrapped in
   `domain.ErrValidation`, matching how the rest of the package surfaces input
   errors.
3. **MCP shape** — nested `recurrence: {periodicity: "monthly"}` input. Keeps
   the JSON-Schema flat and obvious; mirrors the Organizze body.
4. **Do not touch update path** — Organizze's update semantics for recurrence
   are out of scope and not covered by the linked endpoint.
5. **Existing required-field validations stand** — the doc's minimal example
   omits `amount_cents`/`account_id`/`category_id`, but in practice all four
   are needed; we keep the current checks rather than relax them for
   recurrence-only callers.

## Tasks

### 1. Domain types

Edit `internal/domain/transaction.go`:

- Add a `RecurrenceAttributes` struct with a single `Periodicity string
  \`json:"periodicity"\`` field.
- Add exported periodicity constants for the six allowed values.
- Add `Recurrence *RecurrenceAttributes
  \`json:"recurrence_attributes,omitempty\"`` field to
  `CreateTransactionParams`.

Verify: `go build ./...`.

### 2. Usecase validation

Edit `internal/usecase/transaction.go` (`validateCreate`):

- When `p.Recurrence != nil`, require `Periodicity` to be one of the six
  allowed values; otherwise return `fmt.Errorf("%w: invalid periodicity ...",
  domain.ErrValidation)`.

Add tests in `internal/usecase/transaction_test.go`:

- `TestTransactionService_Create_ValidatesRecurrencePeriodicity` — covers
  empty/unknown periodicity (rejected) and one valid value (accepted, plumbed
  to fake repo).

Verify: `go test ./internal/usecase/...`.

### 3. Repository wire shape

Add a test to `internal/adapter/organizze/transaction_repository_test.go`:

- `TestTransactionRepository_Create_IncludesRecurrenceAttributes` — POSTs a
  param with `Recurrence: &RecurrenceAttributes{Periodicity: "monthly"}`,
  decodes the request body into `map[string]any`, asserts `recurrence_attributes.periodicity == "monthly"`.
- `TestTransactionRepository_Create_OmitsRecurrenceWhenNil` — verifies the
  body has no `recurrence_attributes` key when the field is nil.

No production change needed in `transaction_repository.go` — the existing
`Post` call already forwards `params` verbatim.

Verify: `go test ./internal/adapter/organizze/...`.

### 4. MCP tool surface

Edit `internal/adapter/mcp/tools_transactions.go`:

- Add a `RecurrenceInput` (or reuse `domain.RecurrenceAttributes`) struct
  carrying `Periodicity string`.
- Add `Recurrence *RecurrenceInput` field to `CreateTransactionInput`,
  jsonschema-described as "Optional. Set to create a fixed recurring
  transaction. periodicity ∈ {monthly,yearly,weekly,biweekly,bimonthly,trimonthly}."
- Plumb it into `domain.CreateTransactionParams` in
  `createTransactionHandler`.
- Update the `create_transaction` tool description to mention recurring
  support.

Add a test in `internal/adapter/mcp/tools_transactions_test.go`:

- `TestCreateTransactionHandler_PlumbsRecurrence` — asserts the fake service
  receives a non-nil `Recurrence` with the matching periodicity.

Verify: `go test ./internal/adapter/mcp/...`.

### 5. JSON-shape fixture (optional but consistent)

If `internal/adapter/organizze/testdata/transaction*.json` exists for
deserialization roundtrips, extend it with a `recurring: true` fixture and
assert it decodes. Skip if no obvious place exists.

Verify: `go test ./internal/adapter/organizze/...`.

### 6. Docs & changelog

- README: no row change (still 28 tools), but add a one-liner under the
  `create_transaction` row footnote area if there's an existing notes area —
  otherwise skip. Keep the table count unchanged.
- CHANGELOG `[Unreleased]`: add a one-line "Added" entry noting `create_transaction`
  now accepts `recurrence_attributes` for fixed recurring transactions.

### 7. Full verification

- `make test` — full suite green.
- `make lint` — clean.
- `make build` — binary builds.

### 8. Finish branch

Use superpowers:finishing-a-development-branch to commit, push, and open a PR.
