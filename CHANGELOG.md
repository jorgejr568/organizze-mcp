# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- **`get_credit_card_invoice` no longer fails to decode invoices whose transactions carry tags.** `GET /credit_cards/{credit_card_id}/invoices/{invoice_id}` returns `transactions[].tags` as a comma-separated string (e.g. `"coffee,weekday"`), not the documented `[{"name":"..."}, ...]` array shape every other transactions endpoint emits. The whole response decode aborted with `json: cannot unmarshal string into Go struct field Transaction.transactions.tags of type []domain.Tag`, so the tool was effectively broken for any invoice that had tagged transactions. `domain.Tags` is now a defined slice type with a flex `UnmarshalJSON` that accepts both the array shape (every other endpoint) and the comma-separated string shape (this endpoint and any future undocumented offenders), trimming whitespace and dropping empty parts. Marshalling still produces the documented array shape, so outbound request bodies are unchanged. Applied to both `Transaction.Tags` and `Transfer.Tags`.

## [0.9.2] - 2026-05-17

### Changed
- **Consolidated Docker image publishing into `.github/workflows/release.yml`.** All three images (`jorgejr568/organizze-mcp`, `jorgejr568/organizze-mcp-oauth`, `jorgejr568/organizze-mcp-ingestion-consumer`) are now built and pushed only on `v*` tag pushes, with the same semver tag set (`:<version>` / `:<major>.<minor>` / `:<major>` / `:latest`). Drops the previous main-push-triggered `:sha-<short>` tags — single source of truth for "what's published" is now the git tag. GitHub release notes now list pull commands for all three images.
- **Consolidated per-module CI into `.github/workflows/ci.yml`.** The root `ci.yml` now also runs vet + race-test + build for the standalone `cmd/consumer/` and `cmd/ingest/` modules (separate go.mod files that root-level `go test ./...` cannot reach). `.github/workflows/oauth.yml` and `.github/workflows/consumer.yml` are removed; `.github/workflows/ingest.yml` keeps only its Lambda-deploy job (its test job moved to ci.yml). Net effect: every PR exercises every module in a single workflow file, and the Docker/Lambda deploy paths are the only thing the per-target workflows do.

## [0.9.1] - 2026-05-17

### Added
- `.github/workflows/oauth.yml`: builds and pushes the multi-tenant OAuth binary to Docker Hub as `jorgejr568/organizze-mcp-oauth`. On every push to `main` touching `cmd/organizze-mcp-oauth/**`, `internal/**`, or the go module files, ships `:latest` + `:sha-<short>` (multi-arch amd64+arm64); on `v*` tag pushes, additionally emits semver tags (`:0.9.0`, `:0.9`, `:0`). Pull requests still run vet + race tests + the OAuth-binary build, but skip the Docker push. Mirrors the consumer-binary workflow shape. Operators using Easypanel / k8s / plain docker can now pull a published image instead of building locally.

## [0.9.0] - 2026-05-17

### Added
- `cmd/organizze-mcp-oauth/`: multi-tenant variant of the MCP server that hosts an OAuth 2.1 Authorization Server. Each ChatGPT user authenticates with their own Organizze credentials; the operator no longer embeds a single API key in env vars. Backed by Postgres (`internal/oauth/storage/`); API keys stored AES-GCM-encrypted under `OAUTH_ENCRYPTION_KEY`. See `cmd/organizze-mcp-oauth/README.md`. Security hardenings landed during PR review: refresh-token rotation is atomic in SQL (`UPDATE … WHERE revoked_at IS NULL RETURNING`) so concurrent reuse cannot mint two pairs; authorization-code replay revokes every token issued from that code (RFC 6749 §10.5 / Security BCP §4.10) via a new `code_hash` column on `oauth_tokens`; PKCE verifier length is enforced per RFC 7636 §4.1 (43–128) and `code_challenge` is validated against the base64url charset (not just length); the authorize POST is protected by an HMAC-signed consent binding (`OAUTH_COOKIE_SECRET`-keyed, 10-min TTL) so a tampered `client_id`/`redirect_uri`/`code_challenge` from a cross-site form is rejected; DCR (unauthenticated write surface) is per-IP rate-limited (10/min, burst 10); Bearer middleware accepts the scheme case-insensitively per RFC 6750 §2.1; `schema_migrations` ledger table records applied migrations so future `ALTER TABLE` files do not rely on `IF EXISTS` guards for idempotency. Migration `002_indexes_and_code_hash.sql` adds the column plus indexes on `oauth_tokens.refresh_for` / `code_hash` and `oauth_codes.expires_at`. The binary serves the legacy MCP SSE transport at `/sse` (and `/mcp/sse`) alongside the Streamable HTTP handler at `/mcp` — ChatGPT's MCP connector probes the SSE endpoint during action discovery, so an SSE-less deployment fails with `MCP_ACTION_DISCOVERY_FAILED` even when the OAuth flow itself completes. A root-path dispatcher also routes any `GET /` with `Accept: text/event-stream` to the SSE handler so clients that probe the bare origin (the URL most operators paste into ChatGPT's connector dialog) succeed without needing the explicit `/sse` suffix. The authorize form is styled in Organizze's brand language (green CTA, Inter typeface, white card on light-gray background, Portuguese copy) and a returning user is offered a one-click **"Continuar como `<email>`"** shortcut backed by an HMAC-signed browser-session cookie (`organizze_oauth_session`, HttpOnly + Secure + SameSite=Lax) — the stored API key is re-validated against Organizze before the shortcut succeeds, so a key revoked between visits falls back to the full form with an explanatory error. A **"Entrar com outra conta"** link (`?reset=1`) clears the session row + cookie for users who want to authorize as a different Organizze account. The MCP adapter emits a structured zap record per tool call (`tool call` at info on success, `tool call failed` at warn on error) with fields `tool`, `status`, `error_class`, `duration_ms` — same non-sensitive vocabulary as the stats reporter, never tool arguments or return values. The OAuth Bearer middleware logs each request as `bearer accepted` (info, with `user_id`, `client_id`, `path`) or `bearer rejected` (warn, with a fixed `reason` vocabulary — `missing_or_malformed` / `unknown` / `wrong_kind` / `revoked` / `expired`) so operators can correlate tool activity back to a user without scraping logs from multiple sources. The OAuth binary also wires into the same off-box stats pipeline as the single-tenant binary (ingest Lambda → SQS → consumer → Postgres) — purely env-driven (`MCP_STATS_INGEST_URL` + `MCP_STATS_INGEST_TOKEN`; `MCP_STATS_OPTOUT=1` force-disables). Unlike the single-tenant Docker image, the OAuth binary is not stamped with build-time defaults — operators set the env vars explicitly or get NoopReporter. The transport field on emitted events is `http-oauth` so single-tenant vs OAuth traffic is distinguishable downstream.
- **Stats pipeline** (`cmd/ingest/` + `cmd/consumer/` + `internal/stats/`): end-to-end telemetry from the MCP server to a Postgres-backed event store. Every MCP tool call emits a small non-sensitive event (tool name, duration, success/error status, coarse error class — never arguments, return values, or free-text error messages) on a background goroutine to a Function-URL-fronted ingest Lambda; the ingest fetches its X-Ingest-Token from AWS Secrets Manager at cold start and forwards raw JSON to SQS; a long-running consumer container (Docker image `jorgejr568/organizze-mcp-ingestion-consumer` on Docker Hub) polls SQS and persists each message into a `stats_events` JSONB table with idempotent `INSERT ... ON CONFLICT DO NOTHING` semantics. Two new GitHub Actions workflows: `ingest.yml` builds and ships the ingest Lambda via `aws lambda update-function-code`; `consumer.yml` builds and pushes the consumer Docker image to Docker Hub on push to `main` touching the respective subdirectory; the existing `release.yml` Docker workflow is extended to bake the ingest URL (`vars.INGESTION_DEPLOY_URL`) and token into officially-released binaries via `-ldflags`. The token is fetched from AWS Secrets Manager at build time (single source of truth, same secret the ingest Lambda reads at cold start) — no GitHub-secret-side copy to keep in sync. Set `MCP_STATS_OPTOUT=1` to disable.

### Changed
- `internal/adapter/organizze.RequestExecutor` now resolves per-request credentials via a `credprovider.CredentialsProvider` callback instead of capturing fixed values at construction. Single-tenant `cmd/organizze-mcp/` callers are unaffected (env values wrapped in `credprovider.Static`).

## [0.8.1] - 2026-05-16

### Changed
- **Structured JSON logging via `go.uber.org/zap`** across all three binaries (MCP server, ingest Lambda, consumer container). Replaces the stdlib `log` package; every log line is now a single JSON record with `level`, `ts` (RFC3339Nano), `caller`, `msg`, and contextual fields (e.g. `request_id`, `message_id`, `tool`, `status`, `bytes`, `error`). All output still goes to stderr — stdout remains reserved for the MCP stdio JSON-RPC channel. Per-request handler logs in the ingest Lambda and per-message handler logs in the consumer now use child loggers (`logger.With(...)`) for the correlation key, replacing manual `[id]` prefixes. CloudWatch Logs / ECS / Fargate / k8s log aggregators parse the JSON directly into structured fields. The Organizze HTTP executor's `ORGANIZZE_LOG_REQUESTS=1` verbose mode (added in v0.7.0) also moves to the same zap logger: `RequestExecutorOptions.LogWriter io.Writer` is replaced with `Logger *zap.Logger`, and the request/response lines become structured records (`organizze request` / `organizze response`) with `method`, `path`, `body`, `status`, `error` fields. The redaction guarantee (Authorization header value and API key never logged) is unchanged and still test-asserted. The ingest and consumer modules add `go.uber.org/zap` to their `go.mod`s; tests use `zap.NewNop()` / `zaptest/observer` instead of `bytes.Buffer`.

## [0.7.0] - 2026-05-15

### Added
- **Verbose request logging.** Set `ORGANIZZE_LOG_REQUESTS=1` to make the Organizze HTTP executor emit one stderr line per outbound request (method, path, JSON body) and one per response (status code, body truncated to 2KB). Off by default; cost when disabled is a single boolean test. The Authorization header is never written to the log — a redaction test guards this. Intended for diagnosing the silent-drop class of bugs documented in `AGENTS.md` (e.g. `account_id` + `credit_card_id` mutual-exclusion, `replacement_id` as query param vs body).

### Changed
- Release workflow now creates the GitHub release automatically when a `v*` tag is pushed. The new `release` job in `.github/workflows/release.yml` extracts the CHANGELOG section for the tag, appends a `## Docker` pull block and a `compare/vPREV...vNEW` URL, and publishes via `softprops/action-gh-release@v2`. Eliminates the final manual step (`gh release create …`) from the release flow documented in `AGENTS.md`. Fails the workflow if the CHANGELOG does not already contain a section for the pushed tag — keeps the "CHANGELOG-bump PR before tag" ordering load-bearing.
- **`domain.Transfer.Attachments` is now `[]string`** (was `[]json.RawMessage` in v0.5.0–v0.6.2), matching `domain.Transaction.Attachments` and `openapi.yaml`'s `Transaction.attachments` schema (`array of string`). The v0.5.0 escape hatch was chosen defensively; a 2026-05-14 live-API audit found zero non-empty `attachments` payloads in either resource across six years of history, so the documented OpenAPI shape stands as authoritative. **Breaking** for any external fork that imported `internal/domain` and read `Transfer.Attachments` as raw bytes — no in-tree caller does. If Organizze ever serializes a non-string element, decode will now fail loudly at the boundary, exposing the divergence rather than smuggling it through opaque bytes.
- Internal cleanup: `domain.Periodicity.Valid` now uses `slices.Contains` (clears a long-standing lint hint), and `usecase.validateCreate` was decomposed from a mix of switches and loose `if` blocks into a list-of-checks form with one shared `validatePeriodicity` helper. No observable behaviour change; every existing `TestTransactionService_Create_*` test passes unmodified.

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
