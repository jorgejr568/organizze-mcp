# AGENTS.md

Operating instructions for agents working in this repository. Read top-to-bottom before making any change; the workflow checklist near the end is load-bearing.

## What this project is

A Model Context Protocol (MCP) server that wraps the Organizze REST API (`https://api.organizze.com.br/rest/v2`). Written in Go. Single binary; stdio or Streamable-HTTP transport selected by the `MCP_TRANSPORT` env var. Single composition root at `cmd/organizze-mcp/main.go` — fork-friendly. MIT-licensed.

Upstream API reference: <https://github.com/organizze/api-doc>. A local OpenAPI mirror lives at `openapi.yaml` — treat it as the source of truth for wire shapes.

## Architecture (Clean Architecture, four layers)

```
domain    →  internal/domain/*.go           Types + JSON tags that mirror the Organizze wire shape. Owns validation sentinels (domain.ErrValidation, domain.ErrRateLimited).
usecase   →  internal/usecase/*.go          Service orchestration + validation. Expose Reader/Writer/Repository interfaces; concrete adapters satisfy them.
adapter   →  internal/adapter/organizze/    HTTP layer. RequestExecutor + per-resource repositories. Forwards params verbatim — domain owns JSON tags.
          →  internal/adapter/mcp/          MCP tool registration. Inputs declare jsonschema descriptions inline. Handlers plumb MCP input → domain params → service.
cmd       →  cmd/organizze-mcp/main.go      Composition root. Wires config → executor → repositories → services → MCP server.
```

Tests live next to source: `_test.go` files for unit tests, `internal/adapter/organizze/testdata/*.json` for JSON-shape fixtures, `internal/adapter/mcp/integration_test.go` for end-to-end MCP-protocol roundtrips against a fake Organizze server.

## How to ship a change

This repo enforces a strict PR + tag + release flow. The auto-mode classifier blocks direct pushes to `main` — every change goes through a PR even if it's a one-line CHANGELOG bump.

### Feature / fix workflow

1. **Worktree.** Use a git worktree under `.claude/worktrees/<name>` to keep `main` clean. From inside the harness this is `EnterWorktree({name: "..."}`); from a shell it's `git worktree add .claude/worktrees/<name>`.
2. **Branch.** `feat/<short-name>` for features, `fix/<short-name>` for bug fixes, `chore/release-vX.Y.Z` for changelog header bumps. No other prefixes are in active use.
3. **TDD at three layers** when adding wire fields:
   - **Wire-shape**: `internal/adapter/organizze/<resource>_repository_test.go`. Use `newTestExecutor` + decode the request body into `map[string]any` to assert keys are present when set and absent when nil/zero. Always add a paired "omits-when-nil" test for new optional fields.
   - **JSON-shape**: `internal/adapter/organizze/jsonshape_test.go` + extend the relevant fixture under `testdata/`. Assert the new field round-trips out of a realistic response payload.
   - **Service**: `internal/usecase/<resource>_test.go`. One test per accept branch and one per reject branch when validation changes. Use `errors.Is(err, domain.ErrValidation)` — don't string-match messages.
   - **MCP handler**: `internal/adapter/mcp/tools_<resource>_test.go`. Confirm the input field is plumbed into the domain params struct.
4. **Implementation.** Add fields as optional pointers with `,omitempty` JSON tags. Domain types own the wire shape; the adapter forwards `params` verbatim via `executor.Post/Put/Delete`. Service-layer validations wrap `domain.ErrValidation` with `fmt.Errorf("%w: ...")`.
5. **CHANGELOG.** Add an entry under `## [Unreleased]` describing user-visible impact and the *why*. Follow [Keep a Changelog](https://keepachangelog.com/) sections: Added / Changed / Fixed / Removed. Be specific about wire-shape changes and known API surprises.
6. **Verify.** `make test && make lint && make build` must all succeed. CI re-runs `go test -race -count=1 ./...`.
7. **Commit.** Conventional commits (`feat:`, `fix:`, `docs(...)`, `chore:`). Multi-line body via HEREDOC. Every bot-authored commit ends with `Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>`.
8. **Push + PR.** `git push -u origin <branch>` then `gh pr create` with `## Summary`, optional `## Why`, and `## Test Plan` (checkbox list). The footer `🤖 Generated with [Claude Code](https://claude.com/claude-code)` is conventional.
9. **CI.** Wait for the `Test` check (workflow `CI`) to reach `COMPLETED / SUCCESS`. Tight poll loop:
   ```bash
   until [ "$(gh pr view N --json statusCheckRollup --jq '.statusCheckRollup[0].status')" = "COMPLETED" ]; do sleep 5; done
   ```
10. **Merge.** `gh pr merge N --squash --delete-branch`. The local `--delete-branch` step may fail with `fatal: 'main' is already used by worktree` — that's the harness, not GitHub. Confirm with `gh pr view N --json state` (should be `MERGED`).
11. **Exit worktree.** `ExitWorktree({action: "remove", discard_changes: true})`. The local feature commit was rewritten on `main` by the squash, so its SHA is stale; discarding is safe.
12. **Pull main.** Back in the main checkout: `git checkout main && git pull`. If a plan file was copied into the worktree and merged in, the loose copy in `main`'s working tree blocks the pull — `rm` it first.

### Release workflow

Releases are a **separate PR** that promotes `## [Unreleased]` to a versioned section, then a **tag push** that triggers the Docker workflow, then a **manual GitHub release**.

1. After your feature PR is merged on `main`, branch `chore/release-vX.Y.Z`.
2. Edit `CHANGELOG.md`: insert `## [X.Y.Z] - YYYY-MM-DD` between `## [Unreleased]` and the next existing version section. Leave `## [Unreleased]` empty above it.
3. Commit (`docs(changelog): release vX.Y.Z`), push, open PR (title `release: vX.Y.Z (changelog header)`), wait for CI, squash-merge.
4. From `main` after pulling:
   ```bash
   git tag -a vX.Y.Z -m "vX.Y.Z"
   git push origin vX.Y.Z
   ```
   `.github/workflows/release.yml` triggers on `v*` tag push: it re-runs tests with `-race` and builds + pushes multi-arch Docker images (`jorgejr568/organizze-mcp:X.Y.Z`, `:X.Y`, `:X`, `:latest`).
5. **Manually create the GitHub release**: the workflow does NOT do this. Use `gh release create vX.Y.Z --title "vX.Y.Z" --notes "..."`. Release notes should restate the CHANGELOG entry, list Docker tags, and link the compare URL `https://github.com/jorgejr568/organizze-mcp/compare/vPREV...vX.Y.Z`.

### Versioning (semver)

- **Patch (`X.Y.Z+1`)** — bug fix, doc clarification, additive optional field with safe defaults. Most fixes-after-release are patches (e.g. v0.6.0 → v0.6.1 for the silent-drop trap).
- **Minor (`X.Y+1.0`)** — new features, new tools, additive fields that change validation. Default for feature PRs.
- **Major (`X+1.0.0`)** — would break callers. Not yet exercised; we're still 0.x.

## Conventions

### Commits
- Conventional prefixes: `feat:`, `fix:`, `docs(scope):`, `chore:`, `refactor:`, `test:`.
- Subject under ~70 chars; details in body.
- Body explains the *why* (constraint, gotcha, root cause), not the *what* — the diff already shows the what.
- HEREDOC for multi-paragraph messages so quoting stays sane.

### Pull requests
- Title matches the commit subject of the squash result.
- Body sections: `## Summary` (2–4 bullets), `## Why` (optional, for non-obvious decisions), `## Test Plan` (checkbox list — what you ran, what was verified).
- Reference relevant PRs (`See #N`) when continuing prior work.
- Footer: `🤖 Generated with [Claude Code](https://claude.com/claude-code)`.

### Code
- Pointer-with-`omitempty` for optional wire fields. `int64` zero is a sentinel meaning "not set" for ID-like fields, paired with `omitempty` if Organizze treats absent-and-zero differently.
- Domain types own JSON tags. Repositories pass `params` verbatim — no per-field re-marshalling.
- Validation in the usecase layer, never in handlers. Wrap `domain.ErrValidation` with `fmt.Errorf("%w: <field> must ...")` so callers can `errors.Is`.
- jsonschema descriptions on MCP inputs document gotchas inline so LLM clients see them at tool-call time.
- Tool descriptions on `mcpsdk.AddTool` document semantic surprises (installment-amount-is-total, credit-card-vs-account mutual exclusion, etc.). LLM-facing prose is part of the contract.
- No comments that restate the code. Comments earn their place by explaining *why* a non-obvious choice was made (e.g. why `UpdateTransactionParams` uses pointer fields). Don't comment removed code.

## Known Organizze API gotchas (load-bearing)

These came from production-burning surprises. The MCP encodes the workarounds; future contributors must keep them encoded.

1. **Installment `amount_cents` is the TOTAL, not per-installment.** When `installments_attributes` is set, Organizze divides `amount_cents` evenly across `total` installments. Sending R$165.80 with `total: 2` yields two R$82.90 installments — *not* two R$165.80. Tool description and the `amount_cents` jsonschema warn callers; do not change this behaviour without also updating both texts.
2. **`account_id` and `credit_card_id` are mutually exclusive on POST AND PUT `/transactions`.** If both are present in the body, Organizze silently drops `credit_card_id` (and `credit_card_invoice_id`) and routes / keeps the transaction on the bank account. The MCP's service-layer validation rejects the both-set combination up-front on both Create and Update. On POST, `AccountID` is marshalled with `omitempty` so passing `0` doesn't leak `"account_id":0` and re-trigger the silent drop. On PUT, `AccountID` is a `*int64` and stays absent from the body when nil.
3. **`credit_card_invoice_id` requires `credit_card_id`.** No invoice without a card; the service rejects orphan invoice references.
4. **DELETE /categories takes `replacement_id` in the request BODY**, not as a query parameter. Sending it as `?replacement_id=N` is silently ignored and Organizze falls back to the default category. The repository uses a DELETE-with-body, which is why `RequestExecutor.Delete` takes `(ctx, path, body, out)` and not just `(ctx, path)`.
5. **PUT /transactions: absent fields mean "leave unchanged", not "clear to null".** `UpdateTransactionParams` uses pointer fields with `,omitempty` to preserve that semantic. Replacing this with non-pointer fields would silently clear data on partial updates.
6. **`recurrence` and `installments` are mutually exclusive on create.** Service-layer validation enforces it.
7. **User-Agent header is required.** Format: `ApplicationName (email@example.com)`. Omitting it returns 400.

## Tooling quirks

- **Auto-mode classifier blocks `git push origin main`.** Even a one-line CHANGELOG bump goes through a PR. Don't waste tokens trying to bypass.
- **Worktree cleanup race after `gh pr merge --delete-branch`.** The local `git fetch ... --prune` step inside `gh` can fail with `fatal: 'main' is already used by worktree at ...` because the main checkout is open in another working tree. The GitHub-side merge still completes successfully. Confirm with `gh pr view N --json state,mergedAt` before assuming it failed.
- **Plan files in `docs/superpowers/plans/`.** When a plan is written from the main checkout *before* entering a worktree, the file lives on disk but isn't on the worktree branch. Either copy it into the worktree (`cp ../../docs/... docs/...`) before staging, or accept that the plan ships in a follow-up. After merge, the loose untracked copy in main's working tree will block `git pull` — `rm` it.
- **Semgrep PostToolUse hook is configured as blocking but no-ops without `SEMGREP_APP_TOKEN`.** Writes/edits still succeed; the warning is noise. Either authenticate (`semgrep login`) or downgrade the hook to non-blocking in `settings.json`.

## Live API testing (use sparingly)

When investigating production behaviour the user can expose three env vars in the shell:

- `ORGANIZZE_EMAIL`
- `ORGANIZZE_API_KEY`
- `ORGANIZZE_USER_AGENT`

Curl pattern:

```bash
curl -s -u "$ORGANIZZE_EMAIL:$ORGANIZZE_API_KEY" \
  -H "Content-Type: application/json" \
  -H "User-Agent: $ORGANIZZE_USER_AGENT" \
  -X POST "https://api.organizze.com.br/rest/v2/transactions" \
  -d '{...}'
```

**Rules:**
- Always ask before creating non-reversible state.
- Create-and-delete (cleanup in the same call chain) is acceptable for verifying wire behaviour, but use minimal amounts (`amount_cents: -1`) and clear `description` markers (`"MCP test - auto-delete"`).
- Don't leave test data in real invoices. Always DELETE.
- Don't list this as an external service or share its keys.

## Useful commands

```bash
make test            # full suite
make test-cover      # with coverage
make lint            # go vet
make build           # binary at bin/organizze-mcp (link-time version injected)
make docker          # local container image build

go test ./internal/adapter/organizze/... -run <pattern> -v   # focus a single package
go test ./... -race -count=1                                  # mirror CI

gh pr view N --json mergeStateStatus,statusCheckRollup        # PR readiness
gh run list --workflow=release.yml --limit 1                  # release-time Docker workflow status
```

## Stats Pipeline (`cmd/ingest/` + `cmd/consumer/` + `internal/stats/`)

Three components form the stats ingestion pipeline:

```
MCP server (internal/stats HTTPReporter, fire-and-forget)
   ↓ HTTPS POST with X-Ingest-Token
cmd/ingest (Function URL, shared-secret auth)
   ↓ SQS SendMessage (raw body)
SQS queue
   ↓ ReceiveMessage / DeleteMessage long-poll
cmd/consumer (long-running container, idempotent INSERT)
   ↓ INSERT ... ON CONFLICT DO NOTHING
stats_events (Postgres)
```

Each is an **independent Go module** (`cmd/ingest/go.mod`,
`cmd/consumer/go.mod`) so the AWS Lambda runtime, AWS SDK v2, and pgx
dependencies do not pollute the MCP server module. Root-level
`go test ./...` does **not** descend into either module; use
`cd cmd/<name> && make test` or rely on the per-component workflows.

The MCP-side reporter lives in `internal/stats/` (a sibling of
`internal/{domain,usecase,adapter}`). Tool registrations go through
`addInstrumentedTool` in `internal/adapter/mcp/instrument.go`, which
times the call, classifies the error via a small fixed vocabulary,
and hands a populated `Event` to the reporter on the return path.

- **Targets:** ingest is the AWS Lambda `organizze-mcp-stats-ingest` (`provided.al2023`, `arm64`, `us-east-1`). Consumer is a Docker image at `jorgejr568/organizze-mcp-ingestion-consumer` on Docker Hub — operator-deployed to ECS / Fargate / k8s / a VM, not a Lambda.
- **Infra owner:** the ingest function, the SQS queue, the Secrets Manager secret for the ingest token, and the Postgres are Terraform-provisioned in a separate repo. The consumer's deployment target is operator-owned and out of scope here.
- **Deploys:** push to `main` touching `cmd/ingest/**` triggers `.github/workflows/ingest.yml` which calls `aws lambda update-function-code` using `secrets.INGESTION_DEPLOY_AWS_ACCESS_KEY_ID` / `secrets.INGESTION_DEPLOY_AWS_SECRET_ACCESS_KEY` / `vars.INGESTION_DEPLOY_AWS_REGION` (region is a **variable, not secret**). The deployer IAM user is NOT scoped to a consumer Lambda — there isn't one. All three Docker images (`organizze-mcp`, `organizze-mcp-oauth`, `organizze-mcp-ingestion-consumer`) are built and pushed to Docker Hub by `.github/workflows/release.yml` on every `v*` tag push, using `secrets.DOCKERHUB_USERNAME` / `secrets.DOCKERHUB_TOKEN`. Vet+test for the standalone `cmd/consumer/` and `cmd/ingest/` modules runs from `.github/workflows/ci.yml` on every PR (root-level `go test ./...` does not descend into separate go.mod subdirs, so ci.yml has explicit `cd cmd/<x> && go test` steps); `ingest.yml` is deploy-only and `consumer.yml` no longer exists.
- **MCP-side build-time defaults** (officially-released Docker images only):
  - `vars.INGESTION_DEPLOY_URL` — Function URL of the ingest Lambda
  - The ingest token is **not** stored as a GitHub secret. The `release.yml` workflow uses the deployer IAM user's `ReadIngestSharedSecret` permission to fetch the value from AWS Secrets Manager at build time (secret id supplied via `vars.INGESTION_DEPLOY_SECRET_ID`, currently `organizze-mcp/ingest-shared-secret`) and passes it as a docker build-arg masked with `::add-mask::`. Single source of truth is the Secrets Manager secret — the same one the ingest Lambda reads at cold start. Rotation is therefore a one-step `aws secretsmanager update-secret` followed by re-tagging a release; nothing to keep in lockstep.
- **MCP runtime overrides / opt-out:** `MCP_STATS_OPTOUT=1` disables stats entirely; `MCP_STATS_INGEST_URL` / `MCP_STATS_INGEST_TOKEN` override the build-time defaults.
- **Lambda runtime env vars:**
  - Ingest: `STATS_QUEUE_URL` (Terraform-injected), `INGEST_SHARED_SECRET_ARN` (Secrets Manager ARN — `cmd/ingest/main.go` calls `secretsmanager:GetSecretValue` at cold start; rotate the secret value via `aws secretsmanager update-secret` and re-tag a release so the next Docker image picks up the new value).
  - Consumer: `STATS_DATABASE_URL` (libpq URI), `STATS_QUEUE_URL` (SQS queue URL), `AWS_REGION`; AWS credentials via the host's IAM role or env-var creds with sqs:ReceiveMessage / DeleteMessage / GetQueueAttributes on the queue ARN.
- **Database schema:** `cmd/consumer/migrations/001_init.sql` is the source of truth. Apply out-of-band via `cd cmd/consumer && make migrate-up` before schema-affecting changes.
- **Idempotency:** consumer uses `INSERT ... ON CONFLICT (message_id) DO NOTHING`, so SQS at-least-once redelivery is a safe no-op.
- **Non-sensitivity invariant:** the MCP `Event` must contain only non-sensitive fields (tool name, status, error_class from a fixed vocabulary, timing, version, transport). No tool arguments, no return values, no account IDs, no free-text error messages. New fields require explicit review against this rule.

See `cmd/ingest/README.md`, `cmd/consumer/README.md`, and `internal/stats/` for details.

## When in doubt

Read the closest existing test before writing a new one — the patterns are tight and consistent, and reusing them is faster than reinventing. Read `openapi.yaml` before adding wire fields — it's the source of truth, and gaps against it are what motivate most of the change history.
