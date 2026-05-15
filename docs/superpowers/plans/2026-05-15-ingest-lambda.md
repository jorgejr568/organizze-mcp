# Stats Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Go source + Makefiles + GitHub Actions deploy pipelines for the three components that together form the stats ingestion pipeline:

1. **Ingest Lambda** (`organizze-mcp-stats-ingest`) — HTTPS Function URL endpoint that authenticates an upstream caller via a shared secret and forwards raw JSON onto an SQS queue.
2. **Consumer Lambda** (`organizze-mcp-stats-consumer`) — SQS-triggered worker that inserts each message into a Postgres `stats_events` table with idempotent semantics (deduplicated on SQS message ID), reporting partial batch failures back to Lambda so only failed records get retried.
3. **MCP-side stats reporter** — a background, fire-and-forget reporter inside the existing MCP server (`internal/stats/` + an instrumentation hook on every tool registration) that POSTs one small, non-sensitive event per tool call to the ingest Function URL. Opt-out via `MCP_STATS_OPTOUT=1` (default ON). Tool-call latency is not affected: events go through a buffered channel and are dropped (with a log warning) if the queue is full.

**Architecture:** Each Lambda lives in its own self-contained Go module (`cmd/ingest/` and `cmd/consumer/`) so the AWS Lambda runtime, AWS SDK v2, and `pgx` dependencies do not pollute the root MCP server module. Handler logic in `cmd/{ingest,consumer}/internal/handler/` is decoupled from runtime entry points via narrow interfaces (`SendMessageAPI` for ingest, `StatsStore` for consumer), enabling table-driven unit tests with fakes. The consumer's Postgres implementation lives in `cmd/consumer/internal/store/` and is exercised by an integration test gated behind a `//go:build integration` tag.

The MCP-side reporter lives in `internal/stats/` (sibling of `internal/{domain,usecase,adapter}`). It exposes a small `Reporter` interface; the production `HTTPReporter` owns a buffered channel + a single drain goroutine that POSTs each event with a 3s timeout. `NoopReporter` is used when stats are opted-out or when no ingest URL is configured. The MCP adapter (`internal/adapter/mcp/`) wraps every tool handler with `addInstrumentedTool`, which times the call, classifies any returned error (`domain.ErrValidation` → `"validation"`, `domain.ErrRateLimited` → `"rate_limited"`, etc.), and hands a populated `Event` to the reporter — all on the return path so a slow ingest endpoint cannot stall a tool response.

Two new GitHub Actions workflows (`.github/workflows/ingest.yml`, `.github/workflows/consumer.yml`) deploy the Lambdas. The existing `.github/workflows/release.yml` (Docker image) is extended to inject the ingest URL + token into the binary via `-ldflags` so officially-released artifacts ship with stats enabled by default. Local `go build` and PR CI do not inject these, so dev builds and unofficial forks are silent — `NoopReporter` is selected at runtime.

**Tech Stack:** Go 1.25, `github.com/aws/aws-lambda-go`, `github.com/aws/aws-sdk-go-v2/{config,service/sqs}` (ingest), `github.com/jackc/pgx/v5` + `pgxpool` (consumer), stdlib `net/http`/`log`/`crypto/subtle`/`encoding/json` (MCP reporter). AWS CLI v2 + `aws-actions/configure-aws-credentials@v4` in CI.

**Assumptions about already-provisioned infrastructure** (flag back if any are wrong):

- **Consumer Lambda is Terraform-provisioned out-of-band**, same way the ingest Lambda is. Function name: `organizze-mcp-stats-consumer`. Runtime: `provided.al2023`. Arch: `arm64`. Region: `us-east-1`. Memory: 256 MB (more headroom than ingest because of pgx + TLS). Timeout: 30s.
- **An SQS event source mapping** connects the same queue the ingest writes to (`STATS_QUEUE_URL`) to the consumer Lambda. The mapping has `FunctionResponseTypes = ["ReportBatchItemFailures"]` so the consumer's `BatchItemFailures` response is honored.
- **The consumer's IAM exec role** has `sqs:ReceiveMessage`, `sqs:DeleteMessage`, `sqs:GetQueueAttributes`, `sqs:ChangeMessageVisibility` on the queue ARN, plus VPC + Secrets Manager access if the Postgres DSN comes from there. Network reachability to Postgres (security groups, NAT/VPC endpoint) is assumed working.
- **`STATS_DATABASE_URL`** is injected into the consumer's env vars by Terraform, formatted as a libpq URI (`postgres://user:pass@host:5432/db?sslmode=require`).
- **Deployer IAM user `organizze-mcp-deployer`** has `lambda:UpdateFunctionCode` / `GetFunction` / `UpdateFunctionConfiguration` / `PublishVersion` scoped to **both** function ARNs. Credentials are reused for both workflows via these repo-level GitHub Actions inputs (the naming is misleading — they cover the whole pipeline, not just ingest):
  - `secrets.INGESTION_DEPLOY_AWS_ACCESS_KEY_ID` (secret)
  - `secrets.INGESTION_DEPLOY_AWS_SECRET_ACCESS_KEY` (secret)
  - `vars.INGESTION_DEPLOY_AWS_REGION` (**variable, not secret** — the region is non-sensitive)
- **Database migration is applied out-of-band** via `psql` against the live database **before the first consumer deploy**. The Lambda itself does **not** run migrations on cold start. The single SQL file at `cmd/consumer/migrations/001_init.sql` is the source of truth.
- **Ingest Function URL** is recorded as `vars.INGESTION_DEPLOY_URL` so the `release.yml` workflow can bake it into Docker images. The matching shared secret is stored as `secrets.INGESTION_DEPLOY_TOKEN` and must equal the live function's `INGEST_SHARED_SECRET` (rotate both in lockstep). If either is empty at build time, the released binary falls back to `NoopReporter` and no stats are emitted.

**Repo-flow note:** AGENTS.md requires the worktree → branch → PR → CI-green → squash-merge cycle for every change. Do that for this work. Use branch `feat/stats-pipeline` and worktree `.claude/worktrees/stats-pipeline`. Do **not** push directly to `main`. Both Lambdas ship in a single PR — they're conceptually one feature (ingest is useless without a consumer).

---

## File Structure

**New files — Part A: Ingest Lambda**

- `cmd/ingest/go.mod` — independent module `github.com/jorgejr568/organizze-mcp/cmd/ingest`, Go 1.25.
- `cmd/ingest/go.sum`
- `cmd/ingest/main.go` — reads env, builds SQS client, calls `lambda.Start`.
- `cmd/ingest/internal/handler/handler.go` — `Handler` struct, `SendMessageAPI` interface, `Handle` method.
- `cmd/ingest/internal/handler/handler_test.go` — table-driven tests with fake SQS.
- `cmd/ingest/Makefile` — `build`, `zip`, `deploy`, `test`, `clean`.
- `cmd/ingest/README.md`.
- `.github/workflows/ingest.yml` — test (PR + push) + deploy (push-to-main).

**New files — Part B: Consumer Lambda**

- `cmd/consumer/go.mod` — independent module `github.com/jorgejr568/organizze-mcp/cmd/consumer`, Go 1.25.
- `cmd/consumer/go.sum`
- `cmd/consumer/main.go` — reads `STATS_DATABASE_URL`, builds `pgxpool`, pings, calls `lambda.Start`.
- `cmd/consumer/internal/handler/handler.go` — `Handler` struct, `StatsStore` interface, `Handle(ctx, events.SQSEvent)` method.
- `cmd/consumer/internal/handler/handler_test.go` — table-driven tests with a fake store.
- `cmd/consumer/internal/store/pgstore.go` — `PGStore` implementing `StatsStore` over `pgxpool.Pool`.
- `cmd/consumer/internal/store/pgstore_integration_test.go` — gated by `//go:build integration`; skipped unless `STATS_DATABASE_URL_TEST` is set.
- `cmd/consumer/migrations/001_init.sql` — `stats_events` table + indexes. Source of truth, applied via `psql` (or the `make migrate-up` shortcut).
- `cmd/consumer/Makefile` — `build`, `zip`, `deploy`, `test`, `test-integration`, `migrate-up`, `clean`.
- `cmd/consumer/README.md`.
- `.github/workflows/consumer.yml`.

**New files — Part C: MCP-side stats reporter**

- `internal/stats/stats.go` — `Event` struct (JSON-tagged for the wire), `Reporter` interface, `NoopReporter`.
- `internal/stats/stats_test.go` — tests for `Event` JSON shape + `NoopReporter`.
- `internal/stats/http_reporter.go` — `HTTPReporter` with a buffered channel + single drain goroutine; non-blocking `Record`, fire-and-forget POST with timeout, warn-on-error.
- `internal/stats/http_reporter_test.go` — tests using `httptest.Server`: body shape, auth header, drop-when-full, error logging.
- `internal/adapter/mcp/instrument.go` — `addInstrumentedTool` helper wrapping `mcpsdk.AddTool` + `classifyError(err)` mapping domain sentinels to `error_class` strings.
- `internal/adapter/mcp/instrument_test.go` — verifies the wrapped handler still invokes the original AND that `Reporter.Record` is called with the expected fields on success + each error branch.

**Modified files — Part C**

- `internal/adapter/mcp/server.go` — `Dependencies` gains a `Reporter stats.Reporter` field; each `register*Tools` call passes it down.
- `internal/adapter/mcp/tools_{user,account,category,budget,credit_cards,invoices,transfer,transactions}.go` — each `register*Tools(s, svc)` signature becomes `register*Tools(s, r, svc)`; each `mcpsdk.AddTool(s, ...)` call site becomes `addInstrumentedTool(s, r, ...)`.
- `cmd/organizze-mcp/main.go` — composition root reads stats config from env (`MCP_STATS_OPTOUT`, `MCP_STATS_INGEST_URL`, `MCP_STATS_INGEST_TOKEN`) with build-time defaults, builds the appropriate `Reporter`, and stores it on `Dependencies`.
- `Makefile` — extend `LDFLAGS` with `-X` flags injecting `stats.DefaultIngestURL` and `stats.DefaultIngestToken` from `INGEST_URL` / `INGEST_TOKEN` make-variables (both default to empty).
- `Dockerfile` — accept `ARG INGEST_URL=` and `ARG INGEST_TOKEN=` and add matching `-X` ldflags to the `go build`.
- `.github/workflows/release.yml` — pass `vars.INGESTION_DEPLOY_URL` and `secrets.INGESTION_DEPLOY_TOKEN` as `build-args` to the Docker build action.
- `README.md` (root) — short section documenting `MCP_STATS_OPTOUT` + the event shape.

**Modified files — overall**

- `AGENTS.md` — append a "Stats Pipeline" section covering all three components, the deploy workflows, the shared secrets, the migration responsibility, and the MCP-side opt-out env var.
- `CHANGELOG.md` — add an entry under `## [Unreleased]` describing the full pipeline (ingest + SQS + consumer + Postgres + MCP reporter) and the workflows.
- `.gitignore` — ignore `cmd/{ingest,consumer}/bootstrap` and `cmd/{ingest,consumer}/function.zip`.

**Untouched**

- `internal/domain/**`, `internal/usecase/**`, `internal/adapter/organizze/**`. Stats is a cross-cutting concern at the MCP boundary; it does not reach into the domain or the Organizze adapter.

---

## Task 1: Worktree + branch

**Files:** none (environment setup).

- [ ] **Step 1: Create isolated worktree**

Use the harness `EnterWorktree({name: "stats-pipeline"})` if available, otherwise:

```bash
git worktree add -b feat/stats-pipeline .claude/worktrees/stats-pipeline main
cd .claude/worktrees/stats-pipeline
```

- [ ] **Step 2: Confirm clean state**

Run: `git status`
Expected: `On branch feat/stats-pipeline` / `nothing to commit, working tree clean`.

---

## Task 2: Scaffold `cmd/ingest` module

**Files:**
- Create: `cmd/ingest/go.mod`
- Create: `cmd/ingest/main.go` (stub, just enough to compile)
- Modify: `.gitignore`

- [ ] **Step 1: Make the directory tree**

```bash
mkdir -p cmd/ingest/internal/handler
```

- [ ] **Step 2: Initialize the Go module**

```bash
cd cmd/ingest
go mod init github.com/jorgejr568/organizze-mcp/cmd/ingest
```

Open `cmd/ingest/go.mod` and confirm the first line is `module github.com/jorgejr568/organizze-mcp/cmd/ingest` and `go 1.25` is present. If `go mod init` defaulted to a lower version, set `go 1.25` manually.

- [ ] **Step 3: Add the three required dependencies**

```bash
cd cmd/ingest
go get github.com/aws/aws-lambda-go@latest
go get github.com/aws/aws-sdk-go-v2/config@latest
go get github.com/aws/aws-sdk-go-v2/service/sqs@latest
```

- [ ] **Step 4: Write a compiling stub `cmd/ingest/main.go`**

Create `cmd/ingest/main.go`:

```go
package main

func main() {}
```

(We will replace this in Task 8 once the handler exists.)

- [ ] **Step 5: Verify the module builds**

Run: `cd cmd/ingest && go build ./...`
Expected: exit 0, no output.

- [ ] **Step 6: Update root `.gitignore`**

Append these two lines to `/.gitignore`:

```
cmd/ingest/bootstrap
cmd/ingest/function.zip
```

- [ ] **Step 7: Sanity-check that the root module is unaffected**

From repo root: `go build ./...`
Expected: exit 0. The new `cmd/ingest` directory is a separate module and is silently skipped by root-level `./...`.

- [ ] **Step 8: Commit**

```bash
git add cmd/ingest/go.mod cmd/ingest/go.sum cmd/ingest/main.go .gitignore
git commit -m "$(cat <<'EOF'
chore(ingest): scaffold standalone module for stats ingest lambda

Carve cmd/ingest/ into its own Go module so the AWS Lambda runtime and
SDK v2 deps do not bleed into the MCP server module. Stub main.go will
be replaced once the handler lands.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Handler skeleton + `SendMessageAPI` interface

**Files:**
- Create: `cmd/ingest/internal/handler/handler.go`

This task only introduces types so subsequent TDD tasks compile. No logic, no tests yet.

- [ ] **Step 1: Write `cmd/ingest/internal/handler/handler.go`**

```go
// Package handler implements the AWS Lambda Function URL handler for the
// stats ingest endpoint. It authenticates the caller with a shared secret,
// validates the JSON body, and forwards the raw bytes onto SQS.
package handler

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// SendMessageAPI is the subset of the SQS client used by the handler.
// Defined as an interface so tests can substitute a fake.
type SendMessageAPI interface {
	SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

// Handler holds the runtime dependencies that survive across Lambda
// invocations (cold-start initialization).
type Handler struct {
	QueueURL string
	Secret   string
	SQS      SendMessageAPI
	Log      *log.Logger
}

// Handle is the Lambda Function URL entrypoint (payload format v2.0).
// Filled in across Tasks 4–7.
func (h *Handler) Handle(ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	return events.LambdaFunctionURLResponse{StatusCode: 500}, nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd cmd/ingest && go build ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add cmd/ingest/internal/handler/handler.go
git commit -m "$(cat <<'EOF'
feat(ingest): introduce handler skeleton and SendMessageAPI seam

Carve out the SQS dependency behind a narrow interface so the handler
can be unit-tested without touching AWS. Logic lands in the following
TDD commits.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: TDD — auth (missing + wrong token → 401)

**Files:**
- Create: `cmd/ingest/internal/handler/handler_test.go`
- Modify: `cmd/ingest/internal/handler/handler.go`

- [ ] **Step 1: Write the failing tests**

Create `cmd/ingest/internal/handler/handler_test.go`:

```go
package handler

import (
	"context"
	"io"
	"log"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type fakeSQS struct {
	out *sqs.SendMessageOutput
	err error
	in  *sqs.SendMessageInput
}

func (f *fakeSQS) SendMessage(_ context.Context, params *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	f.in = params
	return f.out, f.err
}

func newHandler(t *testing.T, sender SendMessageAPI) *Handler {
	t.Helper()
	return &Handler{
		QueueURL: "https://sqs.us-east-1.amazonaws.com/000/test",
		Secret:   "super-secret",
		SQS:      sender,
		Log:      log.New(io.Discard, "", 0),
	}
}

func req(method, body string, headers map[string]string) events.LambdaFunctionURLRequest {
	return events.LambdaFunctionURLRequest{
		RequestContext: events.LambdaFunctionURLRequestContext{
			RequestID: "req-test-id",
			HTTP: events.LambdaFunctionURLRequestContextHTTPDescription{
				Method: method,
			},
		},
		Headers: headers,
		Body:    body,
	}
}

func TestHandle_MissingToken_Returns401(t *testing.T) {
	h := newHandler(t, &fakeSQS{})
	resp, err := h.Handle(context.Background(), req("POST", `{"ok":true}`, map[string]string{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("status: got %d want 401", resp.StatusCode)
	}
	if strings.Contains(resp.Body, "super-secret") {
		t.Fatalf("response body leaked the secret: %q", resp.Body)
	}
}

func TestHandle_WrongToken_Returns401(t *testing.T) {
	h := newHandler(t, &fakeSQS{})
	resp, err := h.Handle(context.Background(), req("POST", `{"ok":true}`, map[string]string{
		"x-ingest-token": "nope",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("status: got %d want 401", resp.StatusCode)
	}
}

// Sentinel test to confirm the fake satisfies the interface.
var _ SendMessageAPI = (*fakeSQS)(nil)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd cmd/ingest && go test ./internal/handler/ -run TestHandle_ -v`
Expected: both tests FAIL with `status: got 500 want 401`.

- [ ] **Step 3: Implement auth**

Replace the body of `Handle` in `cmd/ingest/internal/handler/handler.go`:

```go
func (h *Handler) Handle(ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	logger := h.Log
	if logger == nil {
		logger = log.Default()
	}
	prefix := "[" + req.RequestContext.RequestID + "] "

	provided := req.Headers["x-ingest-token"]
	if subtle.ConstantTimeCompare([]byte(provided), []byte(h.Secret)) != 1 {
		logger.Print(prefix + "auth: rejected request (token mismatch or missing)")
		return jsonResponse(401, `{"error":"unauthorized"}`), nil
	}

	return jsonResponse(500, `{"error":"not implemented"}`), nil
}

func jsonResponse(status int, body string) events.LambdaFunctionURLResponse {
	return events.LambdaFunctionURLResponse{
		StatusCode: status,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       body,
	}
}
```

Add the `crypto/subtle` import to the import block at the top of the file:

```go
import (
	"context"
	"crypto/subtle"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)
```

- [ ] **Step 4: Re-run tests**

Run: `cd cmd/ingest && go test ./internal/handler/ -run TestHandle_ -v`
Expected: both auth tests PASS. (Other tests don't exist yet.)

- [ ] **Step 5: Commit**

```bash
git add cmd/ingest/internal/handler/handler.go cmd/ingest/internal/handler/handler_test.go
git commit -m "$(cat <<'EOF'
feat(ingest): authenticate requests with shared-secret header

Use crypto/subtle.ConstantTimeCompare against X-Ingest-Token. Missing
or wrong tokens get a flat 401 with no secret leakage in the body, and
the rejection is logged with the Lambda request ID so we can correlate
in CloudWatch without echoing the token value.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: TDD — only POST is allowed (405 otherwise)

**Files:**
- Modify: `cmd/ingest/internal/handler/handler.go`
- Modify: `cmd/ingest/internal/handler/handler_test.go`

- [ ] **Step 1: Add the failing test**

Append to `handler_test.go`:

```go
func TestHandle_NonPostMethod_Returns405(t *testing.T) {
	h := newHandler(t, &fakeSQS{})
	resp, err := h.Handle(context.Background(), req("GET", "", map[string]string{
		"x-ingest-token": "super-secret",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 405 {
		t.Fatalf("status: got %d want 405", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd cmd/ingest && go test ./internal/handler/ -run TestHandle_NonPostMethod -v`
Expected: FAIL with `status: got 500 want 405`.

- [ ] **Step 3: Implement the method check**

In `handler.go`, immediately after the auth block (before the 500 stub), insert:

```go
	if req.RequestContext.HTTP.Method != "POST" {
		logger.Printf(prefix+"method %s rejected", req.RequestContext.HTTP.Method)
		return jsonResponse(405, `{"error":"method not allowed"}`), nil
	}
```

- [ ] **Step 4: Run all handler tests**

Run: `cd cmd/ingest && go test ./internal/handler/ -v`
Expected: 3 PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/ingest/internal/handler/handler.go cmd/ingest/internal/handler/handler_test.go
git commit -m "$(cat <<'EOF'
feat(ingest): reject non-POST methods with 405

The ingest endpoint exists to receive a single shape of payload. Any
verb other than POST is a client bug — fail loudly so the caller sees
the contract violation immediately instead of silently dropping.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: TDD — body validation (empty + non-JSON → 400)

**Files:**
- Modify: `cmd/ingest/internal/handler/handler.go`
- Modify: `cmd/ingest/internal/handler/handler_test.go`

- [ ] **Step 1: Add failing tests**

Append to `handler_test.go`:

```go
func TestHandle_EmptyBody_Returns400(t *testing.T) {
	h := newHandler(t, &fakeSQS{})
	resp, err := h.Handle(context.Background(), req("POST", "", map[string]string{
		"x-ingest-token": "super-secret",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("status: got %d want 400", resp.StatusCode)
	}
}

func TestHandle_InvalidJSON_Returns400(t *testing.T) {
	h := newHandler(t, &fakeSQS{})
	resp, err := h.Handle(context.Background(), req("POST", "not-json{", map[string]string{
		"x-ingest-token": "super-secret",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("status: got %d want 400", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd cmd/ingest && go test ./internal/handler/ -run 'TestHandle_(EmptyBody|InvalidJSON)' -v`
Expected: both FAIL with `status: got 500 want 400`.

- [ ] **Step 3: Implement body decoding + validation**

Add `encoding/base64` and `encoding/json` to the imports in `handler.go`:

```go
import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)
```

After the method check, before the 500 stub, insert:

```go
	body := []byte(req.Body)
	if req.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(req.Body)
		if err != nil {
			logger.Print(prefix + "body: base64 decode failed")
			return jsonResponse(400, `{"error":"invalid body encoding"}`), nil
		}
		body = decoded
	}
	if len(body) == 0 {
		logger.Print(prefix + "body: empty")
		return jsonResponse(400, `{"error":"empty body"}`), nil
	}
	if !json.Valid(body) {
		logger.Print(prefix + "body: invalid json")
		return jsonResponse(400, `{"error":"invalid json"}`), nil
	}
```

- [ ] **Step 4: Re-run all handler tests**

Run: `cd cmd/ingest && go test ./internal/handler/ -v`
Expected: 5 PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/ingest/internal/handler/handler.go cmd/ingest/internal/handler/handler_test.go
git commit -m "$(cat <<'EOF'
feat(ingest): validate request body shape before enqueue

Reject empty bodies and bodies that aren't valid JSON with 400. The
body is not unmarshalled into a strict schema — the raw bytes are
passed through to SQS so the stats payload can evolve without
redeploying the Lambda. json.Valid is enough to confirm well-formed
JSON without forcing a structure.

Also decode base64 bodies up-front since Function URLs base64-encode
non-text content types and we want a single code path downstream.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: TDD — SQS forward (happy 202, error 500)

**Files:**
- Modify: `cmd/ingest/internal/handler/handler.go`
- Modify: `cmd/ingest/internal/handler/handler_test.go`

- [ ] **Step 1: Add failing tests**

Append to `handler_test.go`:

```go
func TestHandle_HappyPath_Returns202_AndForwardsBody(t *testing.T) {
	fake := &fakeSQS{
		out: &sqs.SendMessageOutput{MessageId: ptr("msg-abc")},
	}
	h := newHandler(t, fake)
	body := `{"stat":"page_view","count":3}`
	resp, err := h.Handle(context.Background(), req("POST", body, map[string]string{
		"x-ingest-token": "super-secret",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 202 {
		t.Fatalf("status: got %d want 202", resp.StatusCode)
	}
	if !strings.Contains(resp.Body, `"queued":true`) {
		t.Fatalf("body missing queued:true: %q", resp.Body)
	}
	if !strings.Contains(resp.Body, `"message_id":"msg-abc"`) {
		t.Fatalf("body missing message_id: %q", resp.Body)
	}

	if fake.in == nil {
		t.Fatalf("SendMessage was not called")
	}
	if got := aws.ToString(fake.in.QueueUrl); got != "https://sqs.us-east-1.amazonaws.com/000/test" {
		t.Fatalf("queue url: got %q", got)
	}
	if got := aws.ToString(fake.in.MessageBody); got != body {
		t.Fatalf("message body: got %q want %q", got, body)
	}
}

func TestHandle_SQSError_Returns500_AndDoesNotLeakError(t *testing.T) {
	fake := &fakeSQS{
		err: errors.New("aws: ThrottlingException: rate exceeded for queue arn:aws:...:secret-queue"),
	}
	h := newHandler(t, fake)
	resp, err := h.Handle(context.Background(), req("POST", `{"ok":true}`, map[string]string{
		"x-ingest-token": "super-secret",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 500 {
		t.Fatalf("status: got %d want 500", resp.StatusCode)
	}
	if strings.Contains(resp.Body, "ThrottlingException") || strings.Contains(resp.Body, "secret-queue") {
		t.Fatalf("response body leaked aws error: %q", resp.Body)
	}
}

func ptr[T any](v T) *T { return &v }
```

Add `"errors"` and `"github.com/aws/aws-sdk-go-v2/aws"` to the test file imports.

- [ ] **Step 2: Run new tests to verify they fail**

Run: `cd cmd/ingest && go test ./internal/handler/ -run 'TestHandle_(HappyPath|SQSError)' -v`
Expected: both FAIL with `status: got 500 want 202` and `SendMessage was not called`.

- [ ] **Step 3: Implement SQS forward + response shaping**

Add to imports in `handler.go`:

```go
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
```

Replace the trailing `return jsonResponse(500, ...)` stub at the bottom of `Handle` with:

```go
	out, err := h.SQS.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(h.QueueURL),
		MessageBody: aws.String(string(body)),
	})
	if err != nil {
		logger.Printf(prefix+"sqs send failed: %v", err)
		return jsonResponse(500, `{"error":"internal error"}`), nil
	}

	msgID := aws.ToString(out.MessageId)
	logger.Printf(prefix+"queued message %s (%d bytes)", msgID, len(body))
	return jsonResponse(202, fmt.Sprintf(`{"queued":true,"message_id":%q}`, msgID)), nil
```

- [ ] **Step 4: Run the full handler test suite with race**

Run: `cd cmd/ingest && go test -race -count=1 ./...`
Expected: all 7 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/ingest/internal/handler/handler.go cmd/ingest/internal/handler/handler_test.go
git commit -m "$(cat <<'EOF'
feat(ingest): forward validated body to SQS and respond 202

Send the raw bytes verbatim as MessageBody so downstream consumers see
exactly what the upstream service sent. Propagate the request context
so Lambda's 10s timeout cancels in-flight SQS calls. AWS errors get a
flat 500 with no detail in the response body — the real error is
logged server-side with the Lambda request ID for correlation.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Wire `main.go`

**Files:**
- Modify: `cmd/ingest/main.go`

- [ ] **Step 1: Replace the stub `main.go`**

Overwrite `cmd/ingest/main.go`:

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/jorgejr568/organizze-mcp/cmd/ingest/internal/handler"
)

func main() {
	queueURL := mustEnv("STATS_QUEUE_URL")
	secret := mustEnv("INGEST_SHARED_SECRET")

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	h := &handler.Handler{
		QueueURL: queueURL,
		Secret:   secret,
		SQS:      sqs.NewFromConfig(cfg),
		Log:      log.Default(),
	}

	lambda.Start(h.Handle)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var %s", key)
	}
	return v
}
```

- [ ] **Step 2: Tidy + build**

```bash
cd cmd/ingest
go mod tidy
go build ./...
```

Expected: exit 0.

- [ ] **Step 3: Cross-compile for the Lambda target**

```bash
cd cmd/ingest
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -ldflags='-s -w' -o bootstrap ./
file bootstrap
```

Expected: `file` reports an ELF 64-bit aarch64 binary. Then clean up: `rm bootstrap`.

- [ ] **Step 4: Commit**

```bash
git add cmd/ingest/main.go cmd/ingest/go.mod cmd/ingest/go.sum
git commit -m "$(cat <<'EOF'
feat(ingest): wire main.go to lambda.Start with cold-start init

Read STATS_QUEUE_URL and INGEST_SHARED_SECRET once at startup and fail
fast if either is missing — better than crashing on every invocation.
The SQS client is built from config.LoadDefaultConfig so Lambda's
ambient STS env vars are picked up automatically.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Makefile

**Files:**
- Create: `cmd/ingest/Makefile`

- [ ] **Step 1: Write `cmd/ingest/Makefile`**

```make
.PHONY: build zip deploy test clean

FUNCTION_NAME := organizze-mcp-stats-ingest
BINARY := bootstrap
ZIP := function.zip

build:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
		go build -tags lambda.norpc -ldflags='-s -w' -o $(BINARY) ./

zip: build
	rm -f $(ZIP)
	zip -j $(ZIP) $(BINARY)

deploy: zip
	aws lambda update-function-code \
		--function-name $(FUNCTION_NAME) \
		--zip-file fileb://$(ZIP) \
		--publish

test:
	go test -race -count=1 ./...

clean:
	rm -f $(BINARY) $(ZIP)
```

- [ ] **Step 2: Verify each target locally**

```bash
cd cmd/ingest
make test     # all 7 handler tests pass
make build    # produces ./bootstrap, arm64 linux
ls -l bootstrap
make zip      # produces ./function.zip
unzip -l function.zip   # lists bootstrap with no subdir prefix
make clean    # removes both
```

Expected each step succeeds. Do **not** run `make deploy` from your workstation unless you have the deployer IAM creds wired up.

- [ ] **Step 3: Commit**

```bash
git add cmd/ingest/Makefile
git commit -m "$(cat <<'EOF'
chore(ingest): add Makefile targets for build, zip, deploy, test, clean

Strip the binary at build time (-s -w) and use -tags lambda.norpc to
drop the unused rpc shim — both shave a few hundred KB off the cold
start. 'zip -j' flattens the bootstrap so it sits at the zip root
where the provided.al2023 runtime expects it.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: README

**Files:**
- Create: `cmd/ingest/README.md`

- [ ] **Step 1: Write `cmd/ingest/README.md`**

```markdown
# Stats Ingest Lambda

Single-purpose AWS Lambda that exposes an HTTPS Function URL, authenticates
the caller with a shared-secret header, validates that the request body is
non-empty JSON, and forwards the raw bytes onto an SQS queue for downstream
consumption. Built for the `organizze-mcp-stats-ingest` function (Go custom
runtime on `provided.al2023`, arm64).

## Local commands

```bash
make test     # go vet + race-tagged unit tests
make build    # cross-compile linux/arm64 bootstrap binary
make zip      # bundle bootstrap into function.zip (flat)
make clean    # remove bootstrap and function.zip
```

## Deploy

CI handles this automatically on push to `main` (see
`.github/workflows/ingest.yml`). For a manual deploy from a workstation
you need the `organizze-mcp-deployer` IAM user's credentials exported
(either via `AWS_PROFILE` or `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` /
`AWS_REGION`):

```bash
make deploy
```

Under the hood this runs:

```bash
aws lambda update-function-code \
  --function-name organizze-mcp-stats-ingest \
  --zip-file fileb://function.zip \
  --publish
```

## Runtime environment

The Lambda reads two env vars at cold start and fails fast if either is
missing:

| Var                    | Source                          | Purpose                                                |
| ---------------------- | ------------------------------- | ------------------------------------------------------ |
| `STATS_QUEUE_URL`      | Terraform (already injected)    | Target SQS queue URL.                                  |
| `INGEST_SHARED_SECRET` | Lambda config (set out-of-band) | Must match the `X-Ingest-Token` header on requests.    |
| `AWS_REGION`           | Lambda runtime                  | Used by the SDK default credential chain.              |

AWS credentials come from the exec role's STS env vars — the function does
not read static creds. Outbound calls are limited to `sqs:SendMessage` on
the configured queue ARN.

### Rotating the shared secret

`INGEST_SHARED_SECRET` is **not** set by Terraform; the deployer must update
it via `update-function-configuration` (requires the deployer user's
`lambda:UpdateFunctionConfiguration` permission, which is already in scope):

```bash
aws lambda update-function-configuration \
  --function-name organizze-mcp-stats-ingest \
  --environment 'Variables={STATS_QUEUE_URL=<queue-url>,INGEST_SHARED_SECRET=<new-secret>}'
```

Pass **every** variable you want to keep — `update-function-configuration`
replaces the entire env block, it does not merge.

## Request contract

```http
POST /
X-Ingest-Token: <shared-secret>
Content-Type: application/json

{"any":"json","you":"want"}
```

| Condition                             | Status |
| ------------------------------------- | ------ |
| Happy path (enqueued)                 | 202    |
| Missing or wrong `X-Ingest-Token`     | 401    |
| Any HTTP method other than `POST`     | 405    |
| Empty body                            | 400    |
| Body is not valid JSON                | 400    |
| SQS `SendMessage` failed              | 500    |

Successful responses include `{"queued":true,"message_id":"<sqs-id>"}` so
the caller can reference the enqueued message in downstream logs.
```

- [ ] **Step 2: Commit**

```bash
git add cmd/ingest/README.md
git commit -m "$(cat <<'EOF'
docs(ingest): document build, deploy, env vars, request contract

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: GitHub Actions deploy workflow

**Files:**
- Create: `.github/workflows/ingest.yml`

- [ ] **Step 1: Write `.github/workflows/ingest.yml`**

```yaml
name: Ingest Lambda

on:
  push:
    branches: [main]
    paths:
      - 'cmd/ingest/**'
      - '.github/workflows/ingest.yml'
  pull_request:
    branches: [main]
    paths:
      - 'cmd/ingest/**'
      - '.github/workflows/ingest.yml'
  workflow_dispatch:

permissions:
  contents: read

concurrency:
  group: ingest-${{ github.ref }}
  cancel-in-progress: false

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: cmd/ingest
    steps:
      - name: Checkout
        uses: actions/checkout@v6

      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version: '1.25'
          cache-dependency-path: cmd/ingest/go.sum

      - name: Vet
        run: go vet ./...

      - name: Test
        run: go test -race -count=1 ./...

      - name: Build (linux/arm64)
        run: make build

  deploy:
    name: Deploy
    needs: test
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: cmd/ingest
    steps:
      - name: Checkout
        uses: actions/checkout@v6

      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version: '1.25'
          cache-dependency-path: cmd/ingest/go.sum

      - name: Build + package
        run: make zip

      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          aws-access-key-id: ${{ secrets.INGESTION_DEPLOY_AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: ${{ secrets.INGESTION_DEPLOY_AWS_SECRET_ACCESS_KEY }}
          aws-region: ${{ vars.INGESTION_DEPLOY_AWS_REGION }}

      - name: Update Lambda function code
        run: |
          aws lambda update-function-code \
            --function-name organizze-mcp-stats-ingest \
            --zip-file fileb://function.zip \
            --publish
```

**Note:** `INGESTION_DEPLOY_AWS_REGION` is a repo **variable** (`vars.*`), not a secret. Access key and secret key are the only sensitive credentials and stay in `secrets.*`. If `vars.INGESTION_DEPLOY_AWS_REGION` is empty at runtime the action will fail with a clear "region not set" message — set the var in the repo settings (Settings → Secrets and variables → Actions → Variables) before merging.

- [ ] **Step 2: Validate the YAML**

```bash
python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/ingest.yml'))" && echo OK
```

Expected: `OK`.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ingest.yml
git commit -m "$(cat <<'EOF'
ci(ingest): add test + deploy workflow for stats ingest lambda

PRs touching cmd/ingest/** get vet + race tests + cross-compile. Push
to main additionally builds the bootstrap zip and ships it to AWS via
aws lambda update-function-code, using the INGESTION_DEPLOY_AWS_*
secrets (scoped to the organizze-mcp-deployer IAM user, which is
limited to this single function).

concurrency.cancel-in-progress is false so a stacked merge can't kill
an in-flight deploy mid-upload.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 12: Scaffold `cmd/consumer` module + migration SQL

**Files:**
- Create: `cmd/consumer/go.mod`
- Create: `cmd/consumer/main.go` (stub)
- Create: `cmd/consumer/migrations/001_init.sql`
- Modify: `.gitignore`

- [ ] **Step 1: Make the directory tree**

```bash
mkdir -p cmd/consumer/internal/handler cmd/consumer/internal/store cmd/consumer/migrations
```

- [ ] **Step 2: Initialize the Go module**

```bash
cd cmd/consumer
go mod init github.com/jorgejr568/organizze-mcp/cmd/consumer
```

Confirm `cmd/consumer/go.mod` shows `module github.com/jorgejr568/organizze-mcp/cmd/consumer` and `go 1.25`.

- [ ] **Step 3: Add dependencies**

```bash
cd cmd/consumer
go get github.com/aws/aws-lambda-go@latest
go get github.com/jackc/pgx/v5@latest
```

`pgxpool` ships inside the `pgx/v5` module so no separate `go get` is needed.

- [ ] **Step 4: Write the migration SQL**

Create `cmd/consumer/migrations/001_init.sql`:

```sql
-- 001_init.sql
-- Initial schema for the stats_events ingestion table.
--
-- Idempotency: message_id is the SQS message ID and is UNIQUE so the
-- consumer's `INSERT ... ON CONFLICT (message_id) DO NOTHING` is a safe
-- no-op on SQS redelivery (which is guaranteed at-least-once).

BEGIN;

CREATE TABLE IF NOT EXISTS stats_events (
    id          BIGSERIAL PRIMARY KEY,
    message_id  TEXT NOT NULL UNIQUE,
    payload     JSONB NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS stats_events_received_at_idx
    ON stats_events (received_at);

COMMIT;
```

- [ ] **Step 5: Write a compiling stub `cmd/consumer/main.go`**

```go
package main

func main() {}
```

(Will be replaced in Task 17.)

- [ ] **Step 6: Verify the module builds**

Run: `cd cmd/consumer && go build ./...`
Expected: exit 0.

- [ ] **Step 7: Append to root `.gitignore`**

Add two lines (alongside the cmd/ingest entries from Task 2):

```
cmd/consumer/bootstrap
cmd/consumer/function.zip
```

- [ ] **Step 8: Sanity-check root module is still unaffected**

From repo root: `go build ./...` and `go test ./...`
Expected: exit 0 for both. The new `cmd/consumer/` directory is a separate module, so root-level `./...` ignores it.

- [ ] **Step 9: Commit**

```bash
git add cmd/consumer/go.mod cmd/consumer/go.sum cmd/consumer/main.go cmd/consumer/migrations/001_init.sql .gitignore
git commit -m "$(cat <<'EOF'
chore(consumer): scaffold standalone module for stats consumer lambda

Mirror cmd/ingest/ — separate Go module so pgx + aws-lambda-go don't
leak into the MCP server module. Migration SQL lives alongside the
code and is applied out-of-band via psql before the first deploy;
the Lambda itself does not run migrations on cold start.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 13: Consumer handler skeleton + `StatsStore` interface

**Files:**
- Create: `cmd/consumer/internal/handler/handler.go`

- [ ] **Step 1: Write `cmd/consumer/internal/handler/handler.go`**

```go
// Package handler implements the AWS Lambda SQS-event handler for the
// stats consumer. Each invocation receives a batch of SQS messages; the
// handler stores each one and reports per-message failures back to Lambda
// so only failed records get redelivered.
package handler

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/events"
)

// StatsStore is the narrow persistence interface the handler depends on.
// PGStore (in cmd/consumer/internal/store) is the production implementation;
// tests substitute a fake.
type StatsStore interface {
	// Insert persists a single SQS message body. Implementations MUST be
	// idempotent — duplicate messageID is not an error, it is a no-op.
	Insert(ctx context.Context, messageID string, payload []byte) error
}

// Handler holds the runtime dependencies that survive across Lambda
// invocations (cold-start initialization).
type Handler struct {
	Store StatsStore
	Log   *log.Logger
}

// Handle is the Lambda SQS-trigger entrypoint. Logic lands in Tasks 14–15.
func (h *Handler) Handle(ctx context.Context, evt events.SQSEvent) (events.SQSEventResponse, error) {
	return events.SQSEventResponse{}, nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd cmd/consumer && go build ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add cmd/consumer/internal/handler/handler.go
git commit -m "$(cat <<'EOF'
feat(consumer): introduce handler skeleton and StatsStore seam

The persistence dependency is behind a narrow interface so handler
tests can use a fake and the production pgx implementation can ship
in a separate file with its own integration test.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 14: TDD — happy path + empty event

**Files:**
- Create: `cmd/consumer/internal/handler/handler_test.go`
- Modify: `cmd/consumer/internal/handler/handler.go`

- [ ] **Step 1: Write the failing tests**

Create `cmd/consumer/internal/handler/handler_test.go`:

```go
package handler

import (
	"context"
	"errors"
	"io"
	"log"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

type call struct {
	messageID string
	payload   []byte
}

type fakeStore struct {
	calls    []call
	errByMsg map[string]error // optional: return err for specific message IDs
}

func (s *fakeStore) Insert(_ context.Context, messageID string, payload []byte) error {
	s.calls = append(s.calls, call{messageID: messageID, payload: append([]byte(nil), payload...)})
	if err, ok := s.errByMsg[messageID]; ok {
		return err
	}
	return nil
}

func newHandler(t *testing.T, store StatsStore) *Handler {
	t.Helper()
	return &Handler{
		Store: store,
		Log:   log.New(io.Discard, "", 0),
	}
}

func sqsRec(id, body string) events.SQSMessage {
	return events.SQSMessage{MessageId: id, Body: body}
}

func TestHandle_EmptyEvent_NoFailures(t *testing.T) {
	store := &fakeStore{}
	h := newHandler(t, store)
	resp, err := h.Handle(context.Background(), events.SQSEvent{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.BatchItemFailures) != 0 {
		t.Fatalf("expected 0 failures, got %d", len(resp.BatchItemFailures))
	}
	if len(store.calls) != 0 {
		t.Fatalf("expected 0 store calls, got %d", len(store.calls))
	}
}

func TestHandle_SingleRecord_PersistsAndReportsNoFailures(t *testing.T) {
	store := &fakeStore{}
	h := newHandler(t, store)

	body := `{"stat":"page_view","count":3}`
	resp, err := h.Handle(context.Background(), events.SQSEvent{
		Records: []events.SQSMessage{sqsRec("msg-1", body)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.BatchItemFailures) != 0 {
		t.Fatalf("expected 0 failures, got %d (%v)", len(resp.BatchItemFailures), resp.BatchItemFailures)
	}
	if len(store.calls) != 1 {
		t.Fatalf("expected 1 store call, got %d", len(store.calls))
	}
	if store.calls[0].messageID != "msg-1" {
		t.Fatalf("messageID: got %q want msg-1", store.calls[0].messageID)
	}
	if string(store.calls[0].payload) != body {
		t.Fatalf("payload: got %q want %q", store.calls[0].payload, body)
	}
}

// Sentinel: fakeStore satisfies the interface at compile time.
var _ StatsStore = (*fakeStore)(nil)
var _ = errors.New // referenced in later tests
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd cmd/consumer && go test ./internal/handler/ -v`
Expected: `TestHandle_EmptyEvent_NoFailures` PASSES (the stub already returns empty failures), `TestHandle_SingleRecord_PersistsAndReportsNoFailures` FAILS with `expected 1 store call, got 0`.

- [ ] **Step 3: Implement the happy path**

Replace `Handle` in `cmd/consumer/internal/handler/handler.go`:

```go
func (h *Handler) Handle(ctx context.Context, evt events.SQSEvent) (events.SQSEventResponse, error) {
	logger := h.Log
	if logger == nil {
		logger = log.Default()
	}

	var failures []events.SQSBatchItemFailure
	for _, rec := range evt.Records {
		if err := h.Store.Insert(ctx, rec.MessageId, []byte(rec.Body)); err != nil {
			logger.Printf("[%s] store insert failed: %v", rec.MessageId, err)
			failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: rec.MessageId})
			continue
		}
		logger.Printf("[%s] persisted (%d bytes)", rec.MessageId, len(rec.Body))
	}
	return events.SQSEventResponse{BatchItemFailures: failures}, nil
}
```

- [ ] **Step 4: Re-run tests**

Run: `cd cmd/consumer && go test ./internal/handler/ -v`
Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/consumer/internal/handler/handler.go cmd/consumer/internal/handler/handler_test.go
git commit -m "$(cat <<'EOF'
feat(consumer): persist each SQS record via the StatsStore seam

Iterate the batch, propagating context, and log each persistence with
the SQS message ID as the correlation key. Empty batches are a valid
input (Lambda may send heartbeat-like empty invocations on event
source mapping startup) and return an empty response.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 15: TDD — partial batch failure

**Files:**
- Modify: `cmd/consumer/internal/handler/handler_test.go`

- [ ] **Step 1: Add the failing test**

Append to `handler_test.go`:

```go
func TestHandle_PartialFailure_ReportsOnlyFailedMessages(t *testing.T) {
	storeErr := errors.New("connection refused")
	store := &fakeStore{
		errByMsg: map[string]error{"msg-bad": storeErr},
	}
	h := newHandler(t, store)

	resp, err := h.Handle(context.Background(), events.SQSEvent{
		Records: []events.SQSMessage{
			sqsRec("msg-1", `{"a":1}`),
			sqsRec("msg-bad", `{"b":2}`),
			sqsRec("msg-3", `{"c":3}`),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.BatchItemFailures) != 1 {
		t.Fatalf("expected 1 failure, got %d (%v)", len(resp.BatchItemFailures), resp.BatchItemFailures)
	}
	if got := resp.BatchItemFailures[0].ItemIdentifier; got != "msg-bad" {
		t.Fatalf("failure ItemIdentifier: got %q want msg-bad", got)
	}

	// The handler must NOT short-circuit on a record failure — every record
	// in the batch should have been attempted.
	if len(store.calls) != 3 {
		t.Fatalf("expected 3 store attempts, got %d", len(store.calls))
	}
}
```

- [ ] **Step 2: Run it**

Run: `cd cmd/consumer && go test ./internal/handler/ -run TestHandle_PartialFailure -v`
Expected: PASS. (Task 14's implementation already handles this — this test pins the contract so it doesn't regress to short-circuiting later.)

If it does **not** pass, the implementation in Task 14 has a bug — fix it before continuing.

- [ ] **Step 3: Run the full consumer test suite with race**

Run: `cd cmd/consumer && go test -race -count=1 ./...`
Expected: all 3 handler tests PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/consumer/internal/handler/handler_test.go
git commit -m "$(cat <<'EOF'
test(consumer): pin partial-batch-failure contract

SQS event source mappings with ReportBatchItemFailures only redeliver
the message IDs the function returns in BatchItemFailures. Anything
missing from that list is treated as 'successfully processed' and
deleted from the queue. This test pins both invariants: one failure
in a batch of three returns exactly one identifier, and the other two
records are still attempted (no early exit).

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 16: pgx-backed `PGStore` + integration test

**Files:**
- Create: `cmd/consumer/internal/store/pgstore.go`
- Create: `cmd/consumer/internal/store/pgstore_integration_test.go`

- [ ] **Step 1: Write `cmd/consumer/internal/store/pgstore.go`**

```go
// Package store contains the Postgres-backed implementation of the
// handler's StatsStore interface. Idempotency is enforced by the
// UNIQUE constraint on message_id plus ON CONFLICT DO NOTHING, so
// repeated SQS deliveries collapse to a single row.
package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

const insertSQL = `
INSERT INTO stats_events (message_id, payload)
VALUES ($1, $2)
ON CONFLICT (message_id) DO NOTHING
`

type PGStore struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *PGStore {
	return &PGStore{pool: pool}
}

func (s *PGStore) Insert(ctx context.Context, messageID string, payload []byte) error {
	_, err := s.pool.Exec(ctx, insertSQL, messageID, payload)
	return err
}
```

- [ ] **Step 2: Write the integration test**

Create `cmd/consumer/internal/store/pgstore_integration_test.go`:

```go
//go:build integration

package store

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// connect skips the test when STATS_DATABASE_URL_TEST is not configured.
// CI does NOT run integration tests — they're for local verification
// against a disposable Postgres (e.g. a docker run --rm postgres:16).
func connect(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("STATS_DATABASE_URL_TEST")
	if dsn == "" {
		t.Skip("STATS_DATABASE_URL_TEST not set; skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestPGStore_InsertIsIdempotent(t *testing.T) {
	pool := connect(t)
	ctx := context.Background()
	store := New(pool)

	msgID := "test-" + t.Name()
	payload := []byte(`{"test":true}`)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM stats_events WHERE message_id = $1`, msgID)
	})

	for i := 0; i < 3; i++ {
		if err := store.Insert(ctx, msgID, payload); err != nil {
			t.Fatalf("insert #%d: %v", i, err)
		}
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM stats_events WHERE message_id = $1`, msgID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 row after 3 inserts; got %d", count)
	}
}

func TestPGStore_StoresJSONBPayload(t *testing.T) {
	pool := connect(t)
	ctx := context.Background()
	store := New(pool)

	msgID := "test-" + t.Name()
	payload := []byte(`{"nested":{"k":[1,2,3]}}`)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM stats_events WHERE message_id = $1`, msgID)
	})

	if err := store.Insert(ctx, msgID, payload); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// jsonb_path_query confirms the row is queryable as structured JSON,
	// not just stored as a string.
	var got int
	if err := pool.QueryRow(ctx, `
		SELECT (payload #>> '{nested,k,1}')::int
		FROM stats_events WHERE message_id = $1
	`, msgID).Scan(&got); err != nil {
		t.Fatalf("jsonb query: %v", err)
	}
	if got != 2 {
		t.Fatalf("expected nested.k[1] = 2, got %d", got)
	}
}
```

- [ ] **Step 3: Verify it compiles under the integration tag**

```bash
cd cmd/consumer
go build ./...
go vet -tags integration ./...
```

Expected: exit 0 for both. (Default `go build ./...` ignores the integration test file because of the build tag.)

- [ ] **Step 4: Optionally run the integration suite locally**

Skip this step if you don't have a disposable Postgres handy.

```bash
docker run --rm -d --name pgtest -p 5433:5432 -e POSTGRES_PASSWORD=test postgres:16
sleep 3
export STATS_DATABASE_URL_TEST="postgres://postgres:test@localhost:5433/postgres?sslmode=disable"
psql "$STATS_DATABASE_URL_TEST" -v ON_ERROR_STOP=1 -f cmd/consumer/migrations/001_init.sql
cd cmd/consumer
go test -tags integration -race -count=1 ./internal/store/ -v
cd ../..
docker stop pgtest
```

Expected: both integration tests PASS.

- [ ] **Step 5: Tidy + commit**

```bash
cd cmd/consumer && go mod tidy && cd ../..
git add cmd/consumer/internal/store/pgstore.go cmd/consumer/internal/store/pgstore_integration_test.go cmd/consumer/go.mod cmd/consumer/go.sum
git commit -m "$(cat <<'EOF'
feat(consumer): pgx-backed StatsStore with idempotent INSERT

ON CONFLICT (message_id) DO NOTHING collapses SQS at-least-once
redeliveries into a single row. Integration tests live behind a
'integration' build tag so CI doesn't need a running Postgres —
they're for local verification only.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 17: Wire consumer `main.go`

**Files:**
- Modify: `cmd/consumer/main.go`

- [ ] **Step 1: Replace the stub `main.go`**

```go
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jorgejr568/organizze-mcp/cmd/consumer/internal/handler"
	"github.com/jorgejr568/organizze-mcp/cmd/consumer/internal/store"
)

func main() {
	dsn := mustEnv("STATS_DATABASE_URL")

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("pgxpool.New: %v", err)
	}

	// Fail fast on a bad DSN: ping with a short deadline at cold start
	// instead of crashing on the first invocation.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		log.Fatalf("postgres ping: %v", err)
	}

	h := &handler.Handler{
		Store: store.New(pool),
		Log:   log.Default(),
	}

	lambda.Start(h.Handle)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var %s", key)
	}
	return v
}
```

- [ ] **Step 2: Tidy + build**

```bash
cd cmd/consumer
go mod tidy
go build ./...
```

Expected: exit 0.

- [ ] **Step 3: Cross-compile for the Lambda target**

```bash
cd cmd/consumer
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -ldflags='-s -w' -o bootstrap ./
file bootstrap
```

Expected: ELF 64-bit aarch64. Clean up: `rm bootstrap`.

- [ ] **Step 4: Commit**

```bash
git add cmd/consumer/main.go cmd/consumer/go.mod cmd/consumer/go.sum
git commit -m "$(cat <<'EOF'
feat(consumer): wire main.go to lambda.Start with pgxpool init

Build the pool once at cold start and ping with a 5s deadline so a
bad DSN or unreachable Postgres surfaces immediately instead of
crashing on the first SQS invocation. Connections are reused across
warm invocations — pgxpool handles re-establishment if the server
drops idle conns.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 18: Consumer Makefile

**Files:**
- Create: `cmd/consumer/Makefile`

- [ ] **Step 1: Write `cmd/consumer/Makefile`**

```make
.PHONY: build zip deploy test test-integration migrate-up clean

FUNCTION_NAME := organizze-mcp-stats-consumer
BINARY := bootstrap
ZIP := function.zip
MIGRATION := migrations/001_init.sql

build:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
		go build -tags lambda.norpc -ldflags='-s -w' -o $(BINARY) ./

zip: build
	rm -f $(ZIP)
	zip -j $(ZIP) $(BINARY)

deploy: zip
	aws lambda update-function-code \
		--function-name $(FUNCTION_NAME) \
		--zip-file fileb://$(ZIP) \
		--publish

test:
	go test -race -count=1 ./...

test-integration:
	@test -n "$$STATS_DATABASE_URL_TEST" || (echo "STATS_DATABASE_URL_TEST must be set" >&2; exit 1)
	go test -tags integration -race -count=1 ./...

migrate-up:
	@test -n "$$STATS_DATABASE_URL" || (echo "STATS_DATABASE_URL must be set (use the target DB's DSN)" >&2; exit 1)
	psql "$$STATS_DATABASE_URL" -v ON_ERROR_STOP=1 -f $(MIGRATION)

clean:
	rm -f $(BINARY) $(ZIP)
```

- [ ] **Step 2: Verify each non-DB target locally**

```bash
cd cmd/consumer
make test     # all 3 handler tests pass
make build    # produces ./bootstrap, arm64 linux
make zip      # produces ./function.zip
unzip -l function.zip   # bootstrap at the root
make clean
```

Expected: each step succeeds. Skip `make migrate-up`, `make deploy`, `make test-integration` unless you have the corresponding credentials.

- [ ] **Step 3: Commit**

```bash
git add cmd/consumer/Makefile
git commit -m "$(cat <<'EOF'
chore(consumer): add Makefile targets

migrate-up shells out to psql so the operator can apply the schema
without installing a Go-native migration tool. Both 'deploy' and
'migrate-up' refuse to run without their required env var so a
misconfigured shell can't accidentally hit the wrong target.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 19: Consumer README

**Files:**
- Create: `cmd/consumer/README.md`

- [ ] **Step 1: Write `cmd/consumer/README.md`**

```markdown
# Stats Consumer Lambda

SQS-triggered AWS Lambda that drains the stats queue and persists each
message into the `stats_events` Postgres table. Built for the
`organizze-mcp-stats-consumer` function (Go custom runtime on
`provided.al2023`, arm64). Idempotent on the SQS message ID — duplicate
deliveries are silently de-duplicated by `ON CONFLICT DO NOTHING` against
the `UNIQUE` constraint on `stats_events.message_id`.

## Pipeline shape

```
upstream → ingest Lambda → SQS queue → consumer Lambda → stats_events (Postgres)
```

## Local commands

```bash
make test               # unit tests with race detector
make build              # cross-compile linux/arm64 bootstrap binary
make zip                # bundle bootstrap into function.zip
make clean              # remove bootstrap and function.zip
make migrate-up         # apply migrations/001_init.sql via psql
                        #   requires STATS_DATABASE_URL
make test-integration   # run pgstore tests against a real Postgres
                        #   requires STATS_DATABASE_URL_TEST
```

## Schema (`stats_events`)

| Column        | Type          | Notes                                        |
| ------------- | ------------- | -------------------------------------------- |
| `id`          | `BIGSERIAL`   | PK.                                          |
| `message_id`  | `TEXT UNIQUE` | SQS message ID. Drives idempotency.          |
| `payload`     | `JSONB`       | Raw body forwarded by the ingest Lambda.     |
| `received_at` | `TIMESTAMPTZ` | `DEFAULT NOW()`. Indexed for time-range queries. |

The schema lives in `migrations/001_init.sql`. Apply it **before** the
first deploy:

```bash
export STATS_DATABASE_URL='postgres://...?sslmode=require'
make migrate-up
```

The Lambda does **not** run migrations on cold start; the operator owns
schema changes.

## Deploy

CI handles this automatically on push to `main` (see
`.github/workflows/consumer.yml`). For a manual deploy from a workstation
you need the `organizze-mcp-deployer` IAM user's credentials exported:

```bash
make deploy
```

## Runtime environment

| Var                   | Source                       | Purpose                                              |
| --------------------- | ---------------------------- | ---------------------------------------------------- |
| `STATS_DATABASE_URL`  | Terraform (already injected) | libpq URI; pgxpool dial string.                      |
| `AWS_REGION`          | Lambda runtime               | Used by the SDK default credential chain (if added). |

The function has no other AWS API calls; its IAM exec role just needs
SQS receive/delete on the source queue (handled by the event source
mapping, not by the function code).

## Failure model

The handler iterates the batch and calls `StatsStore.Insert` for each
record. Each record's outcome is independent:

- **Success** → not included in the response → SQS deletes the message.
- **Failure** → identifier added to `BatchItemFailures` → SQS redelivers
  *only that message*.

The event source mapping must have
`FunctionResponseTypes = ["ReportBatchItemFailures"]` for this contract to
hold; without it, a single failure would re-deliver the entire batch.
```

- [ ] **Step 2: Commit**

```bash
git add cmd/consumer/README.md
git commit -m "$(cat <<'EOF'
docs(consumer): document pipeline shape, schema, env vars, failure model

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 20: GitHub Actions consumer workflow

**Files:**
- Create: `.github/workflows/consumer.yml`

- [ ] **Step 1: Write `.github/workflows/consumer.yml`**

```yaml
name: Consumer Lambda

on:
  push:
    branches: [main]
    paths:
      - 'cmd/consumer/**'
      - '.github/workflows/consumer.yml'
  pull_request:
    branches: [main]
    paths:
      - 'cmd/consumer/**'
      - '.github/workflows/consumer.yml'
  workflow_dispatch:

permissions:
  contents: read

concurrency:
  group: consumer-${{ github.ref }}
  cancel-in-progress: false

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: cmd/consumer
    steps:
      - name: Checkout
        uses: actions/checkout@v6

      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version: '1.25'
          cache-dependency-path: cmd/consumer/go.sum

      - name: Vet
        run: go vet ./...

      - name: Test
        run: go test -race -count=1 ./...

      - name: Build (linux/arm64)
        run: make build

  deploy:
    name: Deploy
    needs: test
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: cmd/consumer
    steps:
      - name: Checkout
        uses: actions/checkout@v6

      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version: '1.25'
          cache-dependency-path: cmd/consumer/go.sum

      - name: Build + package
        run: make zip

      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          aws-access-key-id: ${{ secrets.INGESTION_DEPLOY_AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: ${{ secrets.INGESTION_DEPLOY_AWS_SECRET_ACCESS_KEY }}
          aws-region: ${{ vars.INGESTION_DEPLOY_AWS_REGION }}

      - name: Update Lambda function code
        run: |
          aws lambda update-function-code \
            --function-name organizze-mcp-stats-consumer \
            --zip-file fileb://function.zip \
            --publish
```

The same `INGESTION_DEPLOY_AWS_*` inputs as the ingest workflow — `vars.*` for region, `secrets.*` for the access key + secret key. The deployer IAM user must already have `lambda:UpdateFunctionCode` on the consumer function's ARN; this is an out-of-band Terraform concern.

- [ ] **Step 2: Validate the YAML**

```bash
python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/consumer.yml'))" && echo OK
```

Expected: `OK`.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/consumer.yml
git commit -m "$(cat <<'EOF'
ci(consumer): add test + deploy workflow for stats consumer lambda

Mirror the ingest workflow: vet + race tests + arm64 cross-compile on
PRs touching cmd/consumer/**, deploy on push to main using the shared
INGESTION_DEPLOY_AWS_* inputs (key+secret in secrets.*, region in
vars.*). The deployer IAM user's policy must already include the
consumer function ARN — that's Terraform's job, not this repo's.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 21: Stats package — `Event` + `Reporter` + `NoopReporter`

**Files:**
- Create: `internal/stats/stats.go`
- Create: `internal/stats/stats_test.go`

- [ ] **Step 1: Write `internal/stats/stats.go`**

```go
// Package stats holds the MCP server's tool-call telemetry. The reporter
// is intentionally narrow (one method) so the MCP adapter doesn't depend
// on the wire format. The HTTPReporter in this same package owns the
// wire shape and the background drain.
package stats

// Event is the JSON document POSTed to the ingest endpoint.
//
// Only non-sensitive fields are ever populated — tool name (the public
// MCP tool identifier), timing, status, and a coarse error class. Tool
// arguments, return values, account IDs, and free-text error messages
// are deliberately omitted; new fields must be approved against this
// non-sensitivity rule.
type Event struct {
	V          int    `json:"v"`
	Type       string `json:"type"`
	Timestamp  string `json:"ts"`
	Server     string `json:"server"`
	Version    string `json:"version"`
	Transport  string `json:"transport"`
	Tool       string `json:"tool"`
	DurationMs int64  `json:"duration_ms"`
	Status     string `json:"status"`
	ErrorClass string `json:"error_class,omitempty"`
}

// Reporter is the narrow seam the MCP adapter depends on.
//
// Implementations MUST NOT block — callers (tool-call wrappers) are
// on the request hot path. If the reporter cannot accept the event
// immediately it should drop it, not wait.
type Reporter interface {
	RecordToolCall(toolName, status, errorClass string, durationMs int64)
}

// NoopReporter is the default when stats are opted-out or when no
// ingest URL/token is configured at build time and not overridden
// at runtime.
type NoopReporter struct{}

func (NoopReporter) RecordToolCall(_, _, _ string, _ int64) {}
```

- [ ] **Step 2: Write the tests**

Create `internal/stats/stats_test.go`:

```go
package stats

import (
	"encoding/json"
	"testing"
)

func TestEvent_JSONShape_OmitsEmptyErrorClass(t *testing.T) {
	b, err := json.Marshal(Event{
		V: 1, Type: "tool_call", Timestamp: "2026-05-15T00:00:00Z",
		Server: "organizze-mcp", Version: "0.7.0", Transport: "stdio",
		Tool: "list_transactions", DurationMs: 42, Status: "ok",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `"status":"ok"`; !contains(string(b), want) {
		t.Fatalf("expected %q in %s", want, b)
	}
	if contains(string(b), `"error_class"`) {
		t.Fatalf("error_class should be omitted when empty: %s", b)
	}
}

func TestEvent_JSONShape_IncludesErrorClassWhenSet(t *testing.T) {
	b, err := json.Marshal(Event{Status: "error", ErrorClass: "validation"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !contains(string(b), `"error_class":"validation"`) {
		t.Fatalf("expected error_class in %s", b)
	}
}

func TestNoopReporter_DoesNothing(t *testing.T) {
	// Should not panic, should not block.
	r := NoopReporter{}
	for i := 0; i < 1000; i++ {
		r.RecordToolCall("any_tool", "ok", "", 1)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: Run the tests**

Run: `go test ./internal/stats/ -v`
Expected: 3 tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/stats/stats.go internal/stats/stats_test.go
git commit -m "$(cat <<'EOF'
feat(stats): introduce Event + Reporter seam + NoopReporter

The Reporter interface is intentionally one method (no Close, no
Flush) so the MCP adapter cannot accidentally couple to the wire
shape or to background-goroutine lifetimes. Event includes only
non-sensitive fields by construction; new fields must clear that
bar before being added.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 22: `HTTPReporter` — buffered fire-and-forget POST (TDD)

**Files:**
- Create: `internal/stats/http_reporter.go`
- Create: `internal/stats/http_reporter_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/stats/http_reporter_test.go`:

```go
package stats

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestHTTPReporter_PostsEventToIngestURL(t *testing.T) {
	var got Event
	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Ingest-Token")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(202)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := NewHTTPReporter(ctx, srv.URL, "shh-token", "1.2.3", "stdio", 8, log.New(io.Discard, "", 0))

	r.RecordToolCall("list_transactions", "ok", "", 17)

	waitFor(t, 2*time.Second, func() bool { return got.Tool != "" })

	if got.Tool != "list_transactions" {
		t.Fatalf("tool: got %q", got.Tool)
	}
	if got.Status != "ok" || got.ErrorClass != "" {
		t.Fatalf("status/error_class: got %q/%q", got.Status, got.ErrorClass)
	}
	if got.Version != "1.2.3" || got.Transport != "stdio" {
		t.Fatalf("version/transport: got %q/%q", got.Version, got.Transport)
	}
	if got.DurationMs != 17 {
		t.Fatalf("duration: got %d", got.DurationMs)
	}
	if got.Type != "tool_call" || got.V != 1 {
		t.Fatalf("type/v: got %q/%d", got.Type, got.V)
	}
	if got.Server != "organizze-mcp" {
		t.Fatalf("server: got %q", got.Server)
	}
	if gotToken != "shh-token" {
		t.Fatalf("token header: got %q", gotToken)
	}
}

func TestHTTPReporter_RecordIsNonBlockingWhenServerIsSlow(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
		w.WriteHeader(202)
	}))
	defer srv.Close()
	defer close(block)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := NewHTTPReporter(ctx, srv.URL, "tok", "v", "t", 4, log.New(io.Discard, "", 0))

	// Record should not block even though the server is hanging on the
	// in-flight POST. Time 100 invocations and assert wall-clock budget.
	start := time.Now()
	for i := 0; i < 100; i++ {
		r.RecordToolCall("t", "ok", "", 1)
	}
	if d := time.Since(start); d > 200*time.Millisecond {
		t.Fatalf("Record took too long: %v (expected < 200ms)", d)
	}
}

func TestHTTPReporter_DropsEventsWhenBufferFull(t *testing.T) {
	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)

	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer srv.Close()
	defer close(block)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Buffer size 2 + drain goroutine holding one inflight = 3 absorbed
	// before drops begin.
	r := NewHTTPReporter(ctx, srv.URL, "tok", "v", "t", 2, logger)

	for i := 0; i < 50; i++ {
		r.RecordToolCall("t", "ok", "", 1)
	}

	waitFor(t, 1*time.Second, func() bool {
		return bytes.Contains(logBuf.Bytes(), []byte("dropped tool-call event"))
	})
	if !bytes.Contains(logBuf.Bytes(), []byte("dropped tool-call event")) {
		t.Fatalf("expected drop warning in log; got:\n%s", logBuf.String())
	}
}

func TestHTTPReporter_LogsWhenIngestReturnsError(t *testing.T) {
	var logBuf bytes.Buffer
	var mu sync.Mutex
	logger := log.New(&teeWriter{buf: &logBuf, mu: &mu}, "", 0)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := NewHTTPReporter(ctx, srv.URL, "tok", "v", "t", 4, logger)
	r.RecordToolCall("t", "ok", "", 1)

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return bytes.Contains(logBuf.Bytes(), []byte("ingest responded 500"))
	})
}

// --- helpers ---

type teeWriter struct {
	buf *bytes.Buffer
	mu  *sync.Mutex
}

func (t *teeWriter) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.buf.Write(p)
}

func waitFor(t *testing.T, max time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail to compile (no implementation yet)**

Run: `go test ./internal/stats/ -v`
Expected: compile error — `undefined: NewHTTPReporter`.

- [ ] **Step 3: Write `internal/stats/http_reporter.go`**

```go
package stats

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

// DefaultIngestURL and DefaultIngestToken are injected at link time via
//
//	go build -ldflags="-X 'github.com/jorgejr568/organizze-mcp/internal/stats.DefaultIngestURL=https://...'"
//
// They are intentionally empty for un-ldflagged builds (dev `go build`,
// `go test`, IDE builds). The composition root checks both and falls back
// to NoopReporter if either is missing.
var (
	DefaultIngestURL   = ""
	DefaultIngestToken = ""
)

const (
	defaultHTTPTimeout = 3 * time.Second
	defaultDropLogEvery = 100
)

// HTTPReporter buffers events into a channel and drains them on one
// background goroutine. RecordToolCall never blocks: if the channel is
// full the event is dropped and the drop count is logged every Nth
// drop to keep log noise bounded.
type HTTPReporter struct {
	url       string
	token     string
	version   string
	transport string
	httpc     *http.Client
	events    chan Event
	log       *log.Logger
	dropped   atomic.Uint64
}

// NewHTTPReporter starts the drain goroutine immediately. The goroutine
// exits when ctx is cancelled; callers do not need to call any Close.
func NewHTTPReporter(ctx context.Context, url, token, version, transport string, bufferSize int, logger *log.Logger) *HTTPReporter {
	if logger == nil {
		logger = log.Default()
	}
	r := &HTTPReporter{
		url:       url,
		token:     token,
		version:   version,
		transport: transport,
		httpc:     &http.Client{Timeout: defaultHTTPTimeout},
		events:    make(chan Event, bufferSize),
		log:       logger,
	}
	go r.drain(ctx)
	return r
}

func (r *HTTPReporter) RecordToolCall(toolName, status, errorClass string, durationMs int64) {
	evt := Event{
		V:          1,
		Type:       "tool_call",
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		Server:     "organizze-mcp",
		Version:    r.version,
		Transport:  r.transport,
		Tool:       toolName,
		DurationMs: durationMs,
		Status:     status,
		ErrorClass: errorClass,
	}
	select {
	case r.events <- evt:
	default:
		n := r.dropped.Add(1)
		if n == 1 || n%defaultDropLogEvery == 0 {
			r.log.Printf("stats: dropped tool-call event (%d total)", n)
		}
	}
}

func (r *HTTPReporter) drain(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-r.events:
			r.send(ctx, evt)
		}
	}
}

func (r *HTTPReporter) send(ctx context.Context, evt Event) {
	body, err := json.Marshal(evt)
	if err != nil {
		r.log.Printf("stats: marshal event: %v", err)
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(body))
	if err != nil {
		r.log.Printf("stats: build request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ingest-Token", r.token)
	resp, err := r.httpc.Do(req)
	if err != nil {
		r.log.Printf("stats: post failed: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		r.log.Printf("stats: ingest responded %d for tool=%s", resp.StatusCode, evt.Tool)
	}
}
```

- [ ] **Step 4: Re-run tests**

Run: `go test -race -count=1 ./internal/stats/ -v`
Expected: all 6 tests PASS (3 from Task 21 + 4 here = 7 total; one test is mostly an `atomic.Uint64` reference so it should compile cleanly).

- [ ] **Step 5: Commit**

```bash
git add internal/stats/http_reporter.go internal/stats/http_reporter_test.go
git commit -m "$(cat <<'EOF'
feat(stats): HTTPReporter with non-blocking Record and drain goroutine

Record uses a select+default channel send so a slow ingest endpoint
cannot stall the tool-call hot path. Drops are logged every 100th
event so a sustained outage doesn't flood the logs. The drain
goroutine exits when its ctx is cancelled — the composition root
owns the lifetime.

DefaultIngestURL/DefaultIngestToken are injected via -ldflags in
released artifacts; dev builds leave them empty and the composition
root falls back to NoopReporter so unit tests and local runs stay
silent.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 23: MCP tool-call instrumentation helper

**Files:**
- Create: `internal/adapter/mcp/instrument.go`
- Create: `internal/adapter/mcp/instrument_test.go`

The existing tool handler signature in this repo is
`mcpsdk.ToolHandlerFor[In, Out] = func(ctx, *CallToolRequest, In) (*CallToolResult, Out, error)`
(verified via `internal/adapter/mcp/tools_transactions.go:112` and
`mcp/server.go:503` in the go-sdk). The wrapper must preserve that signature
exactly.

- [ ] **Step 1: Write `internal/adapter/mcp/instrument.go`**

```go
package mcp

import (
	"context"
	"errors"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
	"github.com/jorgejr568/organizze-mcp/internal/stats"
)

// instrument wraps a tool handler with timing + error classification +
// fire-and-forget stats reporting. It is split out from addInstrumentedTool
// so unit tests can exercise the wrapped handler directly without going
// through the MCP server registration.
func instrument[In, Out any](name string, r stats.Reporter, h mcpsdk.ToolHandlerFor[In, Out]) mcpsdk.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, in In) (*mcpsdk.CallToolResult, Out, error) {
		start := time.Now()
		res, out, err := h(ctx, req, in)
		status := "ok"
		if err != nil {
			status = "error"
		}
		r.RecordToolCall(name, status, classifyError(err), time.Since(start).Milliseconds())
		return res, out, err
	}
}

// addInstrumentedTool is a drop-in replacement for mcpsdk.AddTool that
// wires the handler through `instrument`. Tool registration files
// (tools_*.go) call this instead of mcpsdk.AddTool directly.
func addInstrumentedTool[In, Out any](s *mcpsdk.Server, r stats.Reporter, t *mcpsdk.Tool, h mcpsdk.ToolHandlerFor[In, Out]) {
	if r == nil {
		r = stats.NoopReporter{}
	}
	mcpsdk.AddTool(s, t, instrument(t.Name, r, h))
}

// classifyError maps domain sentinels to a small fixed vocabulary so the
// ingest endpoint never sees a free-text error message (which could leak
// account IDs, descriptions, etc.).
func classifyError(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, domain.ErrValidation):
		return "validation"
	case errors.Is(err, domain.ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "context_canceled"
	default:
		return "unknown"
	}
}
```

- [ ] **Step 2: Write `internal/adapter/mcp/instrument_test.go`**

```go
package mcp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type recordedCall struct {
	tool       string
	status     string
	errorClass string
	durationMs int64
}

type recordingReporter struct {
	mu    sync.Mutex
	calls []recordedCall
}

func (r *recordingReporter) RecordToolCall(tool, status, errorClass string, durationMs int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, recordedCall{tool, status, errorClass, durationMs})
}

func (r *recordingReporter) last() recordedCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) == 0 {
		return recordedCall{}
	}
	return r.calls[len(r.calls)-1]
}

type fakeIn struct{}
type fakeOut struct{ N int }

func handlerReturning(err error) mcpsdk.ToolHandlerFor[fakeIn, fakeOut] {
	return func(_ context.Context, _ *mcpsdk.CallToolRequest, _ fakeIn) (*mcpsdk.CallToolResult, fakeOut, error) {
		return &mcpsdk.CallToolResult{}, fakeOut{N: 7}, err
	}
}

func TestInstrument_Success_RecordsOk(t *testing.T) {
	r := &recordingReporter{}
	wrapped := instrument("greet", r, handlerReturning(nil))

	_, out, err := wrapped(context.Background(), &mcpsdk.CallToolRequest{}, fakeIn{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.N != 7 {
		t.Fatalf("output: got %+v", out)
	}

	got := r.last()
	if got.tool != "greet" || got.status != "ok" || got.errorClass != "" {
		t.Fatalf("unexpected recorded call: %+v", got)
	}
	if got.durationMs < 0 {
		t.Fatalf("negative duration: %d", got.durationMs)
	}
}

func TestInstrument_ValidationError_RecordsValidation(t *testing.T) {
	r := &recordingReporter{}
	wrapped := instrument("create_transaction", r, handlerReturning(
		fmt.Errorf("%w: amount_cents must be non-zero", domain.ErrValidation),
	))

	_, _, err := wrapped(context.Background(), &mcpsdk.CallToolRequest{}, fakeIn{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("wrapped handler must propagate the error: got %v", err)
	}
	if got := r.last(); got.status != "error" || got.errorClass != "validation" {
		t.Fatalf("unexpected recorded call: %+v", got)
	}
}

func TestInstrument_RateLimitedError_RecordsRateLimited(t *testing.T) {
	r := &recordingReporter{}
	wrapped := instrument("list_accounts", r, handlerReturning(
		fmt.Errorf("%w: try again", domain.ErrRateLimited),
	))
	_, _, _ = wrapped(context.Background(), &mcpsdk.CallToolRequest{}, fakeIn{})
	if got := r.last(); got.errorClass != "rate_limited" {
		t.Fatalf("error_class: got %q want rate_limited", got.errorClass)
	}
}

func TestInstrument_ContextCanceled_RecordsContextCanceled(t *testing.T) {
	r := &recordingReporter{}
	wrapped := instrument("any", r, handlerReturning(context.Canceled))
	_, _, _ = wrapped(context.Background(), &mcpsdk.CallToolRequest{}, fakeIn{})
	if got := r.last(); got.errorClass != "context_canceled" {
		t.Fatalf("error_class: got %q want context_canceled", got.errorClass)
	}
}

func TestInstrument_UnknownError_RecordsUnknown(t *testing.T) {
	r := &recordingReporter{}
	wrapped := instrument("any", r, handlerReturning(errors.New("boom")))
	_, _, _ = wrapped(context.Background(), &mcpsdk.CallToolRequest{}, fakeIn{})
	if got := r.last(); got.errorClass != "unknown" {
		t.Fatalf("error_class: got %q want unknown", got.errorClass)
	}
}

func TestClassifyError_NilIsEmpty(t *testing.T) {
	if got := classifyError(nil); got != "" {
		t.Fatalf("classifyError(nil): got %q want \"\"", got)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test -race -count=1 ./internal/adapter/mcp/ -run TestInstrument -v`
Expected: 5 instrument tests + classifyError-nil PASS.

Also re-run the full mcp suite to confirm nothing else broke:

Run: `go test -race -count=1 ./internal/adapter/mcp/`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/mcp/instrument.go internal/adapter/mcp/instrument_test.go
git commit -m "$(cat <<'EOF'
feat(mcp): instrument every tool call with the stats reporter

The wrapped handler preserves the mcpsdk.ToolHandlerFor[In, Out]
signature exactly so it slots into AddTool with no other call-site
changes (that mechanical sweep is the next commit). Errors are
classified into a tiny vocabulary (validation/rate_limited/
context_canceled/unknown) before being sent — the ingest endpoint
never sees a free-text error message, which is the boundary that
keeps account IDs and descriptions from leaking into telemetry.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 24: Thread `Reporter` through every `register*Tools`

**Files:**
- Modify: `internal/adapter/mcp/server.go`
- Modify: `internal/adapter/mcp/tools_user.go`
- Modify: `internal/adapter/mcp/tools_accounts.go`
- Modify: `internal/adapter/mcp/tools_categories.go`
- Modify: `internal/adapter/mcp/tools_budgets.go`
- Modify: `internal/adapter/mcp/tools_credit_cards.go`
- Modify: `internal/adapter/mcp/tools_invoices.go`
- Modify: `internal/adapter/mcp/tools_transfers.go`
- Modify: `internal/adapter/mcp/tools_transactions.go`

This is a mechanical sweep: each tools file's `register*Tools(s, svc)` becomes `register*Tools(s, r, svc)`, and each `mcpsdk.AddTool(s, ...)` becomes `addInstrumentedTool(s, r, ...)`.

- [ ] **Step 1: Add `Reporter` to `Dependencies`**

In `internal/adapter/mcp/server.go`, edit the `Dependencies` struct and `New`:

```go
import (
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/stats"
)

type Dependencies struct {
	Reporter stats.Reporter

	User        UserService
	Account     AccountService
	Category    CategoryService
	Budget      BudgetService
	CreditCard  CreditCardService
	Invoice     InvoiceService
	Transfer    TransferService
	Transaction TransactionService
}

func New(deps Dependencies) *mcpsdk.Server {
	r := deps.Reporter
	if r == nil {
		r = stats.NoopReporter{}
	}

	s := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "organizze-mcp",
		Version: Version,
	}, nil)

	registerUserTools(s, r, deps.User)
	registerAccountTools(s, r, deps.Account)
	registerCategoryTools(s, r, deps.Category)
	registerBudgetTools(s, r, deps.Budget)
	registerCreditCardTools(s, r, deps.CreditCard)
	registerInvoiceTools(s, r, deps.Invoice)
	registerTransferTools(s, r, deps.Transfer)
	registerTransactionTools(s, r, deps.Transaction)

	return s
}
```

- [ ] **Step 2: Update each `tools_*.go` register function signature + AddTool calls**

For each of the 8 `tools_*.go` files:

1. Change the function signature, e.g.
   ```go
   // before
   func registerTransactionTools(s *mcpsdk.Server, svc TransactionService) {
   // after
   func registerTransactionTools(s *mcpsdk.Server, r stats.Reporter, svc TransactionService) {
   ```
2. Add `"github.com/jorgejr568/organizze-mcp/internal/stats"` to the import block.
3. In the function body, replace every `mcpsdk.AddTool(s, &mcpsdk.Tool{...}, handler)` with `addInstrumentedTool(s, r, &mcpsdk.Tool{...}, handler)`. The first arg is the same `s`; the new second arg is `r`; the rest is unchanged.

Do them in lexicographic order so progress is auditable: `tools_accounts.go`, `tools_budgets.go`, `tools_categories.go`, `tools_credit_cards.go`, `tools_invoices.go`, `tools_transactions.go`, `tools_transfers.go`, `tools_user.go`.

A useful shell verification after each file:

```bash
go build ./...
```

Expected: exit 0 (the build won't go green until `server.go` matches the new signatures, so do `server.go` last or accept temporary build errors mid-sweep).

- [ ] **Step 3: Update existing per-file tests if they call `register*Tools` directly**

```bash
grep -rn "register[A-Z][a-z]*Tools(" internal/adapter/mcp/ --include='*_test.go'
```

If any test calls `registerXxxTools(s, svc)` directly (not via `mcp.New`), add `stats.NoopReporter{}` as the second arg.

- [ ] **Step 4: Run the full root test suite**

Run: `go test -race -count=1 ./...`
Expected: every test passes. This is the integration check that the sweep didn't drop any AddTool call site.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/mcp/
git commit -m "$(cat <<'EOF'
refactor(mcp): route every AddTool through addInstrumentedTool

Add stats.Reporter to Dependencies and thread it through every
register*Tools call. Each tool registration now goes through the
instrumented wrapper, so every tool call produces a stats event
without per-handler boilerplate. Reporter defaults to NoopReporter
when Dependencies.Reporter is nil so existing call sites that
don't construct one (e.g. integration tests) keep working.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 25: Composition root + build-time injection (Makefile, Dockerfile, release.yml)

**Files:**
- Modify: `cmd/organizze-mcp/main.go`
- Modify: `Makefile`
- Modify: `Dockerfile`
- Modify: `.github/workflows/release.yml`
- Modify: `README.md`

- [ ] **Step 1: Wire the reporter into `cmd/organizze-mcp/main.go`**

The composition root reads the env, picks `HTTPReporter` vs `NoopReporter`, and stores it on `Dependencies`. Locate the section where `mcp.New(mcp.Dependencies{...})` is called and prepend the reporter construction. Approximate pattern:

```go
import (
	// ... existing imports
	"context"
	"os"

	"github.com/jorgejr568/organizze-mcp/internal/adapter/mcp"
	"github.com/jorgejr568/organizze-mcp/internal/stats"
)

func main() {
	// ... existing config / client setup

	transport := os.Getenv("MCP_TRANSPORT")
	reporter := buildStatsReporter(context.Background(), transport)

	server := mcp.New(mcp.Dependencies{
		Reporter:    reporter,
		User:        userSvc,
		Account:     accountSvc,
		Category:    categorySvc,
		Budget:      budgetSvc,
		CreditCard:  creditCardSvc,
		Invoice:     invoiceSvc,
		Transfer:    transferSvc,
		Transaction: transactionSvc,
	})

	// ... existing transport selection (stdio vs http)
}

func buildStatsReporter(ctx context.Context, transport string) stats.Reporter {
	if os.Getenv("MCP_STATS_OPTOUT") != "" {
		return stats.NoopReporter{}
	}
	url := envOr("MCP_STATS_INGEST_URL", stats.DefaultIngestURL)
	token := envOr("MCP_STATS_INGEST_TOKEN", stats.DefaultIngestToken)
	if url == "" || token == "" {
		return stats.NoopReporter{}
	}
	return stats.NewHTTPReporter(ctx, url, token, mcp.Version, transport, 256, nil)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

Read the existing `cmd/organizze-mcp/main.go` first to find the right insertion points and reconcile variable names with the existing composition.

- [ ] **Step 2: Verify the binary builds with the new wiring**

```bash
go build -o /dev/null ./cmd/organizze-mcp
go test -race -count=1 ./...
```

Expected: exit 0 for both.

- [ ] **Step 3: Extend `Makefile` LDFLAGS**

Replace the `LDFLAGS :=` line with:

```make
INGEST_URL ?=
INGEST_TOKEN ?=
LDFLAGS := -s -w \
	-X 'github.com/jorgejr568/organizze-mcp/internal/adapter/mcp.Version=$(VERSION)' \
	-X 'github.com/jorgejr568/organizze-mcp/internal/stats.DefaultIngestURL=$(INGEST_URL)' \
	-X 'github.com/jorgejr568/organizze-mcp/internal/stats.DefaultIngestToken=$(INGEST_TOKEN)'
```

Verify the build target still works without the new vars set:

```bash
make build
strings bin/organizze-mcp | grep -E 'DefaultIngestURL|DefaultIngestToken' || true
```

Expected: build exits 0; binary contains the ldflags but they are empty.

- [ ] **Step 4: Extend `Dockerfile` to accept and apply build args**

Replace the existing `go build` step with:

```dockerfile
ARG TARGETARCH
ARG VERSION=dev
ARG INGEST_URL=
ARG INGEST_TOKEN=
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath \
      -ldflags="-s -w \
        -X 'github.com/jorgejr568/organizze-mcp/internal/adapter/mcp.Version=${VERSION}' \
        -X 'github.com/jorgejr568/organizze-mcp/internal/stats.DefaultIngestURL=${INGEST_URL}' \
        -X 'github.com/jorgejr568/organizze-mcp/internal/stats.DefaultIngestToken=${INGEST_TOKEN}'" \
      -o /out/organizze-mcp ./cmd/organizze-mcp
```

Verify the image still builds without the new args:

```bash
docker build -t organizze-mcp:local .
```

Expected: exit 0.

- [ ] **Step 5: Pass the GitHub Actions inputs through `release.yml`**

In `.github/workflows/release.yml`, locate the `Build and push` step and extend `build-args`:

```yaml
      - name: Build and push
        uses: docker/build-push-action@v7
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          build-args: |
            VERSION=${{ steps.version.outputs.value }}
            INGEST_URL=${{ vars.INGESTION_DEPLOY_URL }}
            INGEST_TOKEN=${{ secrets.INGESTION_DEPLOY_TOKEN }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

Validate the YAML:

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))" && echo OK
```

Expected: `OK`.

- [ ] **Step 6: Update root `README.md` with an opt-out note**

Find an appropriate "Configuration" or "Environment variables" section in the existing `README.md` and add (or extend) a subsection:

```markdown
### Anonymous usage stats

Released binaries (Docker images on Docker Hub and the official builds) emit a
small, non-sensitive event per tool call to a stats ingest endpoint. The
payload includes only the tool name, server version, transport (`stdio` /
`http`), call duration, success/error status, and a coarse error class
(`validation`, `rate_limited`, `context_canceled`, `unknown`). No tool
arguments, return values, account IDs, or error messages are sent.

Reporting is fire-and-forget on a background goroutine: a slow or unreachable
ingest endpoint cannot delay a tool call, and events drop silently (with a
log warning) if the buffer fills.

To opt out:

```bash
export MCP_STATS_OPTOUT=1
```

Unofficial builds (built from source without the `INGEST_URL` / `INGEST_TOKEN`
ldflags) do not emit anything regardless of `MCP_STATS_OPTOUT`.
```

- [ ] **Step 7: Full repo build + race-tagged test sweep**

```bash
go build ./...
go test -race -count=1 ./...
make build
```

Expected: every command exits 0.

- [ ] **Step 8: Commit**

```bash
git add cmd/organizze-mcp/main.go Makefile Dockerfile .github/workflows/release.yml README.md
git commit -m "$(cat <<'EOF'
feat(stats): wire MCP-side reporter with build-time ingest URL injection

cmd/organizze-mcp/main.go picks HTTPReporter or NoopReporter based on
the MCP_STATS_OPTOUT env var and the build-time defaults that come
in via -ldflags. Released Docker images bake the URL from
vars.INGESTION_DEPLOY_URL and the token from
secrets.INGESTION_DEPLOY_TOKEN so the official artifacts ship with
stats enabled by default; unofficial builds leave the ldflags empty
and fall back to NoopReporter unless the user supplies their own
MCP_STATS_INGEST_URL / MCP_STATS_INGEST_TOKEN at runtime.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 26: Top-level glue (AGENTS.md, CHANGELOG, final sweep)

**Files:**
- Modify: `AGENTS.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Append to `AGENTS.md`**

Just before the `## When in doubt` section near the end of the file, insert this new section:

```markdown
## Stats Pipeline (`cmd/ingest/` + `cmd/consumer/` + `internal/stats/`)

Three components form the stats ingestion pipeline:

```
MCP server (internal/stats HTTPReporter, fire-and-forget)
   ↓ HTTPS POST with X-Ingest-Token
cmd/ingest (Function URL, shared-secret auth)
   ↓ SQS SendMessage (raw body)
SQS queue
   ↓ event source mapping with ReportBatchItemFailures
cmd/consumer (SQS trigger, idempotent INSERT)
   ↓ INSERT ... ON CONFLICT DO NOTHING
stats_events (Postgres)
```

Each Lambda is an **independent Go module** (`cmd/ingest/go.mod`,
`cmd/consumer/go.mod`) so the Lambda runtime, AWS SDK v2, and pgx
dependencies do not pollute the MCP server module. Root-level
`go test ./...` does **not** descend into either Lambda module; use
`cd cmd/<name> && make test` or rely on the per-Lambda workflows.

The MCP-side reporter lives in `internal/stats/` (a sibling of
`internal/{domain,usecase,adapter}`). Tool registrations go through
`addInstrumentedTool` in `internal/adapter/mcp/instrument.go`, which
times the call, classifies the error via a small fixed vocabulary,
and hands a populated `Event` to the reporter on the return path.

- **Function names:** `organizze-mcp-stats-ingest`, `organizze-mcp-stats-consumer` (both `provided.al2023`, `arm64`, `us-east-1`).
- **Infra owner:** both functions, the SQS queue, the event source mapping, and the Postgres connectivity are Terraform-provisioned in a separate repo. Do not change function name, runtime, memory, timeout, queue ARN, or DB DSN from here.
- **Lambda deploys:** push to `main` touching `cmd/<name>/**` triggers `.github/workflows/<name>.yml` using:
  - `secrets.INGESTION_DEPLOY_AWS_ACCESS_KEY_ID`
  - `secrets.INGESTION_DEPLOY_AWS_SECRET_ACCESS_KEY`
  - `vars.INGESTION_DEPLOY_AWS_REGION` (**variable, not secret**)
- **MCP-side build-time defaults** (officially-released Docker images only):
  - `vars.INGESTION_DEPLOY_URL` — Function URL of the ingest Lambda
  - `secrets.INGESTION_DEPLOY_TOKEN` — must match the ingest Lambda's `INGEST_SHARED_SECRET`; rotate both together
- **MCP runtime overrides / opt-out:** `MCP_STATS_OPTOUT=1` disables stats entirely; `MCP_STATS_INGEST_URL` / `MCP_STATS_INGEST_TOKEN` override the build-time defaults.
- **Lambda runtime env vars:**
  - Ingest: `STATS_QUEUE_URL` (Terraform-injected), `INGEST_SHARED_SECRET` (set via `aws lambda update-function-configuration`).
  - Consumer: `STATS_DATABASE_URL` (Terraform-injected libpq URI).
- **Database schema:** `cmd/consumer/migrations/001_init.sql` is the source of truth. Apply out-of-band via `cd cmd/consumer && make migrate-up` before schema-affecting changes.
- **Idempotency:** consumer uses `INSERT ... ON CONFLICT (message_id) DO NOTHING`, so SQS at-least-once redelivery is a safe no-op.
- **Non-sensitivity invariant:** the MCP `Event` must contain only non-sensitive fields (tool name, status, error_class from a fixed vocabulary, timing, version, transport). No tool arguments, no return values, no account IDs, no free-text error messages. New fields require explicit review against this rule.

See `cmd/ingest/README.md`, `cmd/consumer/README.md`, and `internal/stats/` for details.
```

- [ ] **Step 2: Add a `CHANGELOG.md` entry**

Under the existing `## [Unreleased]` section (or create one), add:

```markdown
### Added
- **Stats pipeline** (`cmd/ingest/` + `cmd/consumer/` + `internal/stats/`): end-to-end telemetry from the MCP server to a Postgres-backed event store. Every MCP tool call emits a small non-sensitive event (tool name, duration, success/error status, coarse error class — never arguments, return values, or free-text error messages) on a background goroutine to a Function-URL-fronted ingest Lambda; the ingest forwards raw JSON to SQS; an SQS-triggered consumer Lambda persists each message into a `stats_events` JSONB table with idempotent `INSERT ... ON CONFLICT DO NOTHING` semantics. Two new GitHub Actions workflows (`ingest.yml`, `consumer.yml`) build, package, and deploy each Lambda on push to `main` touching the respective subdirectory; the existing `release.yml` Docker workflow is extended to bake the ingest URL (`vars.INGESTION_DEPLOY_URL`) and token (`secrets.INGESTION_DEPLOY_TOKEN`) into officially-released binaries via `-ldflags`. Set `MCP_STATS_OPTOUT=1` to disable.
```

- [ ] **Step 3: Final repo-wide build + test sweep**

```bash
# Root module — now includes internal/stats and the instrumented mcp adapter.
go build ./...
go test -race -count=1 ./...
make build && make clean

# Ingest module.
cd cmd/ingest
go vet ./...
go test -race -count=1 ./...
make build && make clean
cd ../..

# Consumer module.
cd cmd/consumer
go vet ./...
go test -race -count=1 ./...
make build && make clean
cd ../..

# Docker build to verify build-args wire through correctly. No push.
docker build --build-arg INGEST_URL=https://example.invalid --build-arg INGEST_TOKEN=t -t organizze-mcp:local .
```

Expected: every command exits 0.

- [ ] **Step 4: Commit**

```bash
git add AGENTS.md CHANGELOG.md
git commit -m "$(cat <<'EOF'
docs: announce stats pipeline in AGENTS.md and CHANGELOG

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 27: Push, open PR, watch CI

**Files:** none.

- [ ] **Step 1: Push the branch**

```bash
git push -u origin feat/stats-pipeline
```

- [ ] **Step 2: Open the PR**

```bash
gh pr create --title "feat: stats pipeline — ingest + consumer lambdas + MCP-side reporter" --body "$(cat <<'EOF'
## Summary

- **Ingest Lambda** (`cmd/ingest/`): standalone Go module implementing an AWS Lambda HTTP ingest endpoint (`organizze-mcp-stats-ingest`). Authenticates an `X-Ingest-Token` shared-secret header, validates a non-empty JSON body, and forwards the raw bytes to SQS via `SendMessage`. Deploys via `.github/workflows/ingest.yml`.
- **Consumer Lambda** (`cmd/consumer/`): standalone Go module implementing an SQS-triggered AWS Lambda (`organizze-mcp-stats-consumer`) that persists each message into a `stats_events` Postgres table with idempotent `INSERT ... ON CONFLICT (message_id) DO NOTHING` semantics and partial-batch-failure reporting. Schema lives in `cmd/consumer/migrations/001_init.sql`; applied out-of-band via `make migrate-up`. Deploys via `.github/workflows/consumer.yml`.
- **MCP-side reporter** (`internal/stats/` + `internal/adapter/mcp/instrument.go`): fire-and-forget HTTP reporter that emits one small non-sensitive event per MCP tool call to the ingest Function URL. Buffered + drop-on-overflow so a slow ingest endpoint never affects tool-call latency. Opt-out via `MCP_STATS_OPTOUT=1` (default ON for officially-released artifacts). Build-time ingest URL/token injected via `-ldflags` from `vars.INGESTION_DEPLOY_URL` + `secrets.INGESTION_DEPLOY_TOKEN` in `release.yml`; dev builds leave them empty and fall back to `NoopReporter`.

## Why

The three components are conceptually one feature — none of them is useful on its own. They ship in one PR. Each Lambda has its own Go module so AWS SDK v2 and pgx do not bleed into the MCP server module (the existing Docker image and root CI stay completely lean). Auth on ingest uses `crypto/subtle.ConstantTimeCompare` and a generic 401 with no token echo. Body shape is intentionally not strict on the ingest side — `json.Valid` is the only check so the payload can evolve without redeploys. Idempotency on the consumer side is enforced by a UNIQUE constraint + `ON CONFLICT DO NOTHING`. The MCP reporter is built around a hard non-sensitivity invariant: only tool name, duration, status, and a coarse error class from a fixed vocabulary are sent. Tool arguments, return values, and free-text error messages are deliberately excluded.

## Pre-merge checklist (out-of-band, not in this PR)

- [ ] Deployer IAM user (`organizze-mcp-deployer`) has `lambda:UpdateFunctionCode` etc. scoped to **both** function ARNs.
- [ ] Repo secrets exist: `INGESTION_DEPLOY_AWS_ACCESS_KEY_ID`, `INGESTION_DEPLOY_AWS_SECRET_ACCESS_KEY`, `INGESTION_DEPLOY_TOKEN`.
- [ ] Repo variables exist: `INGESTION_DEPLOY_AWS_REGION` (e.g. `us-east-1`), `INGESTION_DEPLOY_URL` (Function URL of the ingest Lambda).
- [ ] `INGESTION_DEPLOY_TOKEN` value matches the live `INGEST_SHARED_SECRET` configured on the ingest function.
- [ ] Postgres schema applied: `cd cmd/consumer && STATS_DATABASE_URL=... make migrate-up`.
- [ ] Consumer's SQS event source mapping has `FunctionResponseTypes = ["ReportBatchItemFailures"]`.

## Test Plan

- [ ] `cd cmd/ingest && go vet ./... && go test -race -count=1 ./...` — 7 tests pass
- [ ] `cd cmd/consumer && go vet ./... && go test -race -count=1 ./...` — 3 handler tests pass
- [ ] Root `go vet ./... && go test -race -count=1 ./...` — existing tests + new stats package (3) + http_reporter (4) + instrument (5+) all pass
- [ ] `make build` produces a binary; `strings bin/organizze-mcp | grep -c 'DefaultIngest'` shows the ldflag symbols (empty values are fine)
- [ ] `make build INGEST_URL=https://example.invalid INGEST_TOKEN=t` injects the values (verify with `strings`)
- [ ] `docker build --build-arg INGEST_URL=https://example.invalid --build-arg INGEST_TOKEN=t -t organizze-mcp:local .` succeeds
- [ ] GitHub Actions `CI / Test`, `Ingest Lambda / Test`, and `Consumer Lambda / Test` jobs are green on this PR
- [ ] After merge, both Lambda deploys + the next Docker release (on tag push) succeed
- [ ] Smoke-test ingest: correct token (expect 202), wrong token (expect 401), GET (expect 405)
- [ ] End-to-end: start the MCP server from a Docker image built with the real `INGESTION_DEPLOY_URL` + `INGESTION_DEPLOY_TOKEN`, invoke a tool, then `SELECT * FROM stats_events ORDER BY id DESC LIMIT 1` and confirm a `{"type":"tool_call",...}` row appears
- [ ] Opt-out: set `MCP_STATS_OPTOUT=1`, invoke a tool, confirm **no** new row appears

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

- [ ] **Step 3: Wait for CI to go green**

```bash
PR=$(gh pr view --json number --jq .number)
until [ "$(gh pr view "$PR" --json statusCheckRollup --jq '[.statusCheckRollup[] | select(.status != "COMPLETED")] | length')" = "0" ]; do sleep 5; done
gh pr view "$PR" --json statusCheckRollup --jq '.statusCheckRollup'
```

Expected: every check (`CI / Test`, `Ingest Lambda / Test`, `Consumer Lambda / Test`) has `"conclusion":"SUCCESS"`. Deploy jobs are push-to-main gated and won't run on the PR.

- [ ] **Step 4: Hand off**

```bash
gh pr view --json url --jq .url
```

Print the URL and stop.

---

## Task 28: Post-merge verification

**Files:** none.

- [ ] **Step 1: Confirm both Lambda deploy workflows ran**

```bash
gh run list --workflow=ingest.yml --limit 3
gh run list --workflow=consumer.yml --limit 3
```

Expected: `Deploy` jobs for the merge commit succeed.

- [ ] **Step 2: Confirm both Lambdas picked up the new code**

```bash
for fn in organizze-mcp-stats-ingest organizze-mcp-stats-consumer; do
  echo "=== $fn ==="
  aws lambda get-function --function-name "$fn" \
    --query 'Configuration.{LastModified:LastModified,Version:Version,Runtime:Runtime,Arch:Architectures}'
done
```

- [ ] **Step 3: Tag a release to exercise the MCP-side build-time injection**

The MCP reporter only kicks in for Docker images built by `release.yml` (which runs on `v*` tag push). Per AGENTS.md release flow: branch `chore/release-vX.Y.Z`, bump CHANGELOG, merge, then:

```bash
git tag -a vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

Watch the workflow:

```bash
gh run list --workflow=release.yml --limit 1
```

Expected: success. Pull the resulting image and confirm the ldflags are populated:

```bash
docker pull jorgejr568/organizze-mcp:X.Y.Z
docker run --rm jorgejr568/organizze-mcp:X.Y.Z --help 2>&1 | head -20  # just to start it
docker run --rm jorgejr568/organizze-mcp:X.Y.Z sh -c 'strings /usr/local/bin/organizze-mcp | grep INGESTION_DEPLOY_URL_VALUE' || true
```

(There is no clean way to read injected ldflag values from a distroless image short of `strings` on the binary outside the container — that's fine for now.)

- [ ] **Step 4: End-to-end smoke test**

Boot the released image against the real Organizze creds, invoke a tool, then query Postgres:

```sql
SELECT id, message_id, payload, received_at
FROM stats_events
WHERE payload->>'type' = 'tool_call'
ORDER BY id DESC
LIMIT 5;
```

Expected: rows appear within a few seconds of invoking tools, each with the tool name, duration, status, and (when applicable) error_class.

- [ ] **Step 5: Opt-out verification**

Run the same image with `MCP_STATS_OPTOUT=1` set, invoke a tool, and confirm **no** new row appears in `stats_events`.

- [ ] **Step 6: Reminder for the user**

If any of these have never been done on the live infrastructure, surface them:

- `INGEST_SHARED_SECRET` set on the ingest function (must equal `secrets.INGESTION_DEPLOY_TOKEN`).
- Migration applied (`cd cmd/consumer && make migrate-up`).
- SQS event source mapping has `FunctionResponseTypes = ["ReportBatchItemFailures"]`.
- Repo variable `INGESTION_DEPLOY_URL` set to the live Function URL.
- Repo secret `INGESTION_DEPLOY_TOKEN` set, matching `INGEST_SHARED_SECRET`.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-15-ingest-lambda.md`. The plan now spans **three sub-projects**:

- **Part A — Ingest Lambda** (Tasks 1–11)
- **Part B — Consumer Lambda** (Tasks 12–20)
- **Part C — MCP-side stats reporter** (Tasks 21–25)
- Plus docs / PR / verification (Tasks 26–28)

A single PR covers everything. If the diff gets unwieldy during execution, a reasonable split is:

- **PR 1:** Parts A + B (Tasks 1–20 + docs/PR scoped to Lambdas only). Ship and deploy.
- **PR 2:** Part C (Tasks 21–25 + docs/PR scoped to the MCP-side reporter). Ship after the ingest Function URL is live and the `vars.INGESTION_DEPLOY_URL` + `secrets.INGESTION_DEPLOY_TOKEN` are populated.

Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration. With 28 tasks this is the better fit; each task is self-contained and reviewable.

**2. Inline Execution** — Execute tasks in this session using `executing-plans`, batch execution with checkpoints.

Which approach?
