# Stats Consumer

Long-running Go process that drains the stats SQS queue into the
`stats_events` Postgres table. Ships as a Docker image at
[`jorgejr568/organizze-mcp-ingestion-consumer`](https://hub.docker.com/r/jorgejr568/organizze-mcp-ingestion-consumer)
on Docker Hub. Idempotent on the SQS message ID — duplicate deliveries are
silently de-duplicated by `ON CONFLICT DO NOTHING` against the `UNIQUE`
constraint on `stats_events.message_id`.

> **Not a Lambda.** This is a regular long-running container, deployed by
> the operator to ECS / Fargate / a VM / a k8s pod — wherever. The poll
> loop, signal handling, and graceful shutdown are owned by `main.go`,
> not by the Lambda runtime.

## Pipeline shape

```
upstream → ingest Lambda → SQS queue → consumer container → stats_events (Postgres)
```

## Local commands

```bash
make test               # unit tests with race detector
make build              # native binary at ./consumer (handy for go run testing)
make docker             # build image as jorgejr568/organizze-mcp-ingestion-consumer:latest
                        # (build context is the repo root)
make docker-push        # push to Docker Hub (requires docker login)
make migrate-up         # apply migrations/001_init.sql via psql
                        #   requires STATS_DATABASE_URL
make test-integration   # run pgstore tests against a real Postgres
                        #   requires STATS_DATABASE_URL_TEST
make clean              # remove the local binary
```

## Schema (`stats_events`)

| Column        | Type          | Notes                                            |
| ------------- | ------------- | ------------------------------------------------ |
| `id`          | `BIGSERIAL`   | PK.                                              |
| `message_id`  | `TEXT UNIQUE` | SQS message ID. Drives idempotency.              |
| `payload`     | `JSONB`       | Raw body forwarded by the ingest Lambda.         |
| `received_at` | `TIMESTAMPTZ` | `DEFAULT NOW()`. Indexed for time-range queries. |

The schema lives in `migrations/001_init.sql`. Apply it **before** running
the consumer:

```bash
export STATS_DATABASE_URL='postgres://...?sslmode=require'
make migrate-up
```

The consumer does **not** run migrations on startup; the operator owns
schema changes.

## Deploying the image

CI handles the build + push on every merge to `main` touching
`cmd/consumer/**` (see `.github/workflows/consumer.yml`). The image is
published to Docker Hub with tags `:latest` and `:sha-<short>`. Where you
run it is up to you (ECS / Fargate / a VM / a k8s pod / Compose). The
container must have:

- Network reach to the SQS queue (`STATS_QUEUE_URL`).
- Network reach to Postgres (`STATS_DATABASE_URL`).
- AWS credentials with `sqs:ReceiveMessage` and `sqs:DeleteMessage` on the
  queue ARN — supplied via an IAM role on the host (preferred) or
  `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` env vars.

Minimal example with environment variables:

```bash
docker run --rm \
  -e STATS_QUEUE_URL='https://sqs.us-east-1.amazonaws.com/0000/stats' \
  -e STATS_DATABASE_URL='postgres://...?sslmode=require' \
  -e AWS_REGION=us-east-1 \
  -e AWS_ACCESS_KEY_ID=... \
  -e AWS_SECRET_ACCESS_KEY=... \
  jorgejr568/organizze-mcp-ingestion-consumer:latest
```

## Runtime environment

| Var                  | Required | Purpose                                                  |
| -------------------- | -------- | -------------------------------------------------------- |
| `STATS_DATABASE_URL` | yes      | libpq URI; pgxpool dial string.                          |
| `STATS_QUEUE_URL`    | yes      | SQS queue URL the consumer polls.                        |
| `AWS_REGION`         | yes      | Used by the AWS SDK default credential/region chain.     |

AWS credentials come from the SDK's default credential chain (env vars or
an IAM role on the host).

## Failure model

The poll loop receives up to 10 messages per `ReceiveMessage` call (long
polling, `WaitTimeSeconds=20`). It hands the batch to `Handler.Process`,
which calls `StatsStore.Insert` for each record:

- **Success** → message is `DeleteMessage`d → removed from the queue.
- **Failure** → message is left in the queue → SQS redelivers it after the
  visibility timeout expires.

The handler never short-circuits on a record failure — every record in a
batch is attempted. Idempotency at the DB layer (`ON CONFLICT DO NOTHING`)
makes redelivery safe.

Transient SQS receive errors are logged and the loop backs off briefly
(2s) before retrying. SIGTERM and SIGINT both trigger graceful shutdown:
the in-flight batch (if any) completes, then the process exits.
