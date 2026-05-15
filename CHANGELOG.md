# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.6.2] - 2026-05-15

### Fixed
- **`update_transaction` credit-card routing silently dropped `credit_card_id` on PUT.** Live audit against the Organizze sandbox confirmed `PUT /transactions/{id}` exhibits the same silent-drop trap as the POST endpoint fixed in v0.6.1: when both `account_id` and `credit_card_id` are in the body, Organizze nulls `credit_card_id` (and `credit_card_invoice_id`) and the transaction stays on / moves to the bank account. Three changes together close the trap on update:
  - Service-layer validation on `TransactionService.Update` rejects `account_id` + `credit_card_id` simultaneously with `domain.ErrValidation` and names the silent-drop trap; also rejects `credit_card_invoice_id` without `credit_card_id`.
  - The `update_transaction` tool description and `account_id` / `credit_card_id` / `credit_card_invoice_id` jsonschema hints spell out the mutual-exclusion rule (mirrors the v0.6.1 `create_transaction` wording).
  - `UpdateTransactionParams` and `UpdateTransactionInput` gain `credit_card_invoice_id` so callers can pin a moved transaction to a specific invoice (live-verified: PUT with `{credit_card_id, credit_card_invoice_id}` and no `account_id` persists both).

## [0.6.1] - 2026-05-15

### Fixed
- **`create_transaction` credit-card billing was silently broken.** Organizze drops `credit_card_id` when `account_id` is also present in the request body, so transactions intended for a credit card were landing on the user's default bank account with no visible error. Three changes together close the trap:
  - `domain.CreateTransactionParams.AccountID` now JSON-marshals with `omitempty`; passing `AccountID: 0` no longer leaks `"account_id":0` onto the wire.
  - Service-layer validation requires exactly one of `account_id` or `credit_card_id`, rejects the both-set combination with `domain.ErrValidation` and a message that names the silent-drop trap, and rejects `credit_card_invoice_id` without `credit_card_id`.
  - The `create_transaction` tool description and `account_id` / `credit_card_id` jsonschema hints spell out the mutual-exclusion rule.
- API behaviour verified against the live Organizze sandbox: `{credit_card_id: 386176}` (no `account_id`) returns `account_id == 386176`, `credit_card_id == 386176`, `account_type == "CreditCard"`; adding `account_id` to the same body returns `credit_card_id: null` and routes to the default bank account.

## [0.6.0] - 2026-05-15

### Added
- `create_transaction` and `update_transaction` now accept `credit_card_id`; `create_transaction` additionally accepts `credit_card_invoice_id`. Closes the gap with `openapi.yaml`'s `TransactionInput` / `UpdateTransaction` schemas — clients can create or move transactions on a specific credit card / invoice.
- `Transaction` response now includes `attachments []string`, mirroring `openapi.yaml`'s `Transaction.attachments`.

### Changed
- `create_transaction` tool description explicitly documents the Organizze installment-amount rule: when `installments` is set, `amount_cents` is the TOTAL across all installments — Organizze divides evenly. To get per-installment value X with N installments, send `amount_cents = X * N`. (Caused a real user surprise: sending R$165.80 with `total=2` yielded two R$82.90 installments.) The `amount_cents` field's jsonschema description on `CreateTransactionInput` carries the same note.

## [0.5.0] - 2026-05-14

### Added
- **Two new MCP tools** (catalogue grows 28 → 30):
  - `get_credit_card_invoice_payment` — fetches the consolidated payment `Transaction` for an invoice via `GET /credit_cards/{credit_card_id}/invoices/{invoice_id}/payments`. Was the only documented Organizze endpoint with no MCP counterpart.
  - `get_transfer` — fetches a single transfer by id via `GET /transfers/{id}`. Brings transfers into symmetry with every other resource that has both `list_*` and `get_*` tools.
- **Transactions — installment plans (parcelada)**: `create_transaction` accepts an optional `installments` object with `periodicity` and `total`, forwarded to Organizze as `installments_attributes`. Mutually exclusive with `recurrence` (the v0.4.0 fixed-recurring variant); the usecase layer rejects both being set with `domain.ErrValidation`.
- **Transactions — recurring/installment series propagation**: `update_transaction` and `delete_transaction` both accept optional `update_future` and `update_all` flags. `update_future=true` propagates the operation to the current and all future occurrences; `update_all=true` includes past occurrences too (may alter the account balance if past entries were already paid). On `delete_transaction` the two flags are mutually exclusive and validated.
- **Invoices — date filters**: `list_credit_card_invoices` accepts optional `start_date` / `end_date` (YYYY-MM-DD). Without a range, Organizze caps results to the current calendar year — clients can now reach historical invoices.
- **Accounts — archive flag**: `update_account` exposes `archived` (`*bool`). Clients can archive or unarchive accounts through MCP.
- **Credit cards — expanded update**: `update_credit_card` exposes `limit_cents`, `card_network`, `archived`, and `default`. Previously only `name`, `due_day`, `closing_day`, `description`, and `update_invoices_since` were mutable.
- **Transfers — fully modeled response**: `domain.Transfer` grew from 11 to 24 fields — `total_installments`, `installment`, `recurring`, `attachments_count`, `credit_card_id`, `credit_card_invoice_id`, `paid_credit_card_id`, `paid_credit_card_invoice_id`, `created_at`, `updated_at`, `tags`, `attachments` (as `[]json.RawMessage`), and the `deleted` discriminator. Opposite ids are now nullable (`*int64`).
- **Delete-output snapshots**: every mutating delete tool (`delete_account`, `delete_category`, `delete_credit_card`, `delete_transaction`, `delete_transfer`) now returns the deleted resource snapshot in an optional `{Deleted, ID, X *domain.X}` shape. The previous `{Deleted, ID}` contract still holds; the new field is opaque (`omitempty`) when the API echoes nothing (e.g. 204 responses).

### Fixed
- **`delete_category` replacement was silently ignored.** `replacement_id` was previously sent as a query string parameter (`?replacement_id=18`) — Organizze documents it as a JSON request body and ignored the query form, silently falling back to the default category. Now sent as `{"replacement_id":<id>}` body with `Content-Type: application/json`. Affected transactions are correctly reassigned per `ORGANIZZE_API.md` "Excluir uma categoria".

### Changed
- `RequestExecutor.Delete` broadened from `(ctx, path) error` to `(ctx, path, body, out any) error` to support DELETE-with-body endpoints and snapshot decoding. The five no-body callsites pass `(nil, nil)` and behave identically on the wire.
- `usecase/{account,category,credit_card,transaction,transfer}.go`: `Delete` methods on the `*Writer` interfaces and the service structs now return `(*domain.X, error)` to surface the deleted snapshot. Adapter conformance is unchanged for callers that ignore the return value.
- README tool catalogue extended from 28 to 30 rows; numbering reflows accordingly.

## [0.4.0] - 2026-05-14

### Added
- `create_transaction` now accepts an optional `recurrence` object with `periodicity` (weekly, biweekly, monthly, bimonthly, trimonthly, yearly), forwarded to the Organizze API as `recurrence_attributes` to create a fixed recurring transaction. Periodicity is validated in the usecase layer (wrapped in `domain.ErrValidation`); omitting `recurrence` leaves the on-the-wire body byte-identical to the prior one-off behaviour.

## [0.3.0] - 2026-05-14

### Added
- 12 new MCP tools bringing four resources to full CRUD parity with the Organizze REST API. Tool catalogue grows from 16 to 28.
  - **Accounts**: `create_account`, `update_account`, `delete_account`.
  - **Categories**: `create_category`, `update_category`, `delete_category`. `delete_category` accepts an optional `replacement_id` to reassign affected transactions (Organizze's `replacement_id` query parameter).
  - **Credit cards**: `create_credit_card`, `update_credit_card`, `delete_credit_card`. `update_credit_card` exposes `update_invoices_since` (YYYY-MM-DD) so Organizze retroactively regenerates invoices from that date.
  - **Transfers**: `create_transfer`, `update_transfer`, `delete_transfer`. Tool descriptions document that credit cards are not accepted as source or destination, and that updates can only modify description/notes/tags (Organizze API constraint).
- `domain.Create*Params` / `Update*Params` value objects for accounts, categories, credit_cards, transfers. Update params use pointer fields so unset fields are omitted on PUT.
- Service-layer validation wraps `domain.ErrValidation` for required fields per resource (e.g. credit card `due_day` / `closing_day` in [1, 31], transfer non-zero `amount_cents`).
- Integration test coverage extended to roundtrip every one of the 28 tools through the MCP protocol against a fake Organizze server.

### Changed
- `internal/usecase/{account,category,credit_card,transfer}.go` now expose `*Reader` + `*Writer` + composed `*Repository` interfaces. The concrete adapter structs satisfy them automatically; no composition-root changes were required.
- README tool catalogue extended from 16 to 28 rows, with a `Mutating?` column.

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
