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
