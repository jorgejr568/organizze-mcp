# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
