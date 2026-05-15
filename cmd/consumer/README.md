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
