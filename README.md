# organizze-mcp

A Model Context Protocol (MCP) server exposing the [Organizze](https://www.organizze.com.br/) REST API to LLM clients (Claude Desktop, Claude Code, etc.), built in Go with a layered Clean Architecture.

> **Unofficial / community-built.** This project is not affiliated with Organizze. It wraps the [public Organizze API](https://github.com/organizze/api-doc) and ships under MIT — Organizze (or anyone) is welcome to fork, redistribute, or adopt it as the canonical reference MCP. See [§ Adoption & forks](#adoption--forks).

## Status

[![CI](https://github.com/jorgejr568/organizze-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/jorgejr568/organizze-mcp/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/jorgejr568/organizze-mcp?sort=semver)](https://github.com/jorgejr568/organizze-mcp/releases)
[![Docker Hub](https://img.shields.io/docker/v/jorgejr568/organizze-mcp?label=docker&sort=semver)](https://hub.docker.com/r/jorgejr568/organizze-mcp)

## Architecture

```
cmd ──► adapter/mcp ──► usecase ──► domain
        adapter/organizze ──► usecase (interfaces)
```

- **`internal/domain`** — entities, value objects, sentinel errors. Imports nothing.
- **`internal/usecase`** — application services + repository interfaces. Imports `domain`.
- **`internal/adapter/organizze`** — HTTP/REST repository implementations + `HTTPClient` abstraction. Imports `usecase`, `domain`.
- **`internal/adapter/mcp`** — MCP tool adapters. Imports `usecase`, `domain`.
- **`cmd/organizze-mcp`** — composition root.

`adapter/organizze.HTTPClient` is the interface every repository uses for HTTP transport; `Client` (concrete) wraps stdlib `*http.Client` and is the only place timeouts/retries can be centralized.

## Configuration

| Variable | Required | Default | Notes |
|---|---|---|---|
| `ORGANIZZE_API_KEY` | yes | — | https://app.organizze.com.br/configuracoes/api-keys |
| `ORGANIZZE_EMAIL` | yes | — | Account email (Basic-Auth username) |
| `ORGANIZZE_USER_AGENT` | yes | — | `"App (e@x.com)"` — Organizze rejects requests without it |
| `MCP_TRANSPORT` | no | `stdio` | `stdio` or `http` |
| `MCP_HTTP_ADDR` | no | `:8080` | Listen address for HTTP transport |
| `ORGANIZZE_BASE_URL` | no | `https://api.organizze.com.br/rest/v2` | Override |
| `ORGANIZZE_HTTP_TIMEOUT` | no | `30s` | `time.ParseDuration` format |
| `MCP_STATS_OPTOUT` | no | — | Set to any non-empty value to disable anonymous usage stats (see below) |

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

## Quickstart — Docker stdio (Claude Desktop)

`~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "organizze": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "ORGANIZZE_API_KEY",
        "-e", "ORGANIZZE_EMAIL",
        "-e", "ORGANIZZE_USER_AGENT",
        "jorgejr568/organizze-mcp:latest"
      ],
      "env": {
        "ORGANIZZE_API_KEY": "your-token-here",
        "ORGANIZZE_EMAIL": "you@example.com",
        "ORGANIZZE_USER_AGENT": "ClaudeDesktop (you@example.com)"
      }
    }
  }
}
```

## Quickstart — Docker HTTP

```bash
docker run -d --name organizze-mcp \
  -p 8080:8080 \
  -e MCP_TRANSPORT=http \
  -e ORGANIZZE_API_KEY=... \
  -e ORGANIZZE_EMAIL=... \
  -e "ORGANIZZE_USER_AGENT=YourApp (you@example.com)" \
  jorgejr568/organizze-mcp:latest
```

Endpoints: `http://localhost:8080/mcp` (MCP), `http://localhost:8080/healthz` (liveness).

## Quickstart — From source

```bash
git clone https://github.com/jorgejr568/organizze-mcp
cd organizze-mcp
make build
ORGANIZZE_API_KEY=... ORGANIZZE_EMAIL=... ORGANIZZE_USER_AGENT='App (e@x.com)' \
  ./bin/organizze-mcp
```

## Tool catalogue (30 tools)

`amount_cents` is **negative for expenses, positive for income**.

| #  | Tool                                | Operation                           | Mutating? |
|----|-------------------------------------|-------------------------------------|-----------|
| 1  | `get_user`                          | UserService.Get                     | no        |
| 2  | `list_accounts`                     | AccountService.List                 | no        |
| 3  | `get_account`                       | AccountService.Get                  | no        |
| 4  | `create_account`                    | AccountService.Create               | **yes**   |
| 5  | `update_account`                    | AccountService.Update               | **yes**   |
| 6  | `delete_account`                    | AccountService.Delete               | **yes**   |
| 7  | `list_categories`                   | CategoryService.List                | no        |
| 8  | `get_category`                      | CategoryService.Get                 | no        |
| 9  | `create_category`                   | CategoryService.Create              | **yes**   |
| 10 | `update_category`                   | CategoryService.Update              | **yes**   |
| 11 | `delete_category`                   | CategoryService.Delete              | **yes**   |
| 12 | `list_budgets`                      | BudgetService.List (period routing) | no        |
| 13 | `list_credit_cards`                 | CreditCardService.List              | no        |
| 14 | `get_credit_card`                   | CreditCardService.Get               | no        |
| 15 | `create_credit_card`                | CreditCardService.Create            | **yes**   |
| 16 | `update_credit_card`                | CreditCardService.Update            | **yes**   |
| 17 | `delete_credit_card`                | CreditCardService.Delete            | **yes**   |
| 18 | `list_credit_card_invoices`         | InvoiceService.List                 | no        |
| 19 | `get_credit_card_invoice`           | InvoiceService.Get                  | no        |
| 20 | `get_credit_card_invoice_payment`   | InvoiceService.Payment              | no        |
| 21 | `list_transfers`                    | TransferService.List                | no        |
| 22 | `get_transfer`                      | TransferService.Get                 | no        |
| 23 | `create_transfer`                   | TransferService.Create              | **yes**   |
| 24 | `update_transfer`                   | TransferService.Update              | **yes**   |
| 25 | `delete_transfer`                   | TransferService.Delete              | **yes**   |
| 26 | `list_transactions`                 | TransactionService.List             | no        |
| 27 | `get_transaction`                   | TransactionService.Get              | no        |
| 28 | `create_transaction`                | TransactionService.Create           | **yes**   |
| 29 | `update_transaction`                | TransactionService.Update           | **yes**   |
| 30 | `delete_transaction`                | TransactionService.Delete           | **yes**   |

## Development

```bash
make test        # full suite
make test-cover  # with coverage report
make lint        # go vet
make build       # binary at bin/organizze-mcp
make docker      # container image
```

### Debugging

Set `ORGANIZZE_LOG_REQUESTS=1` to print every outbound Organizze API call to stderr. Each request emits one line and each response emits another:

```
organizze: --> POST /transactions body={"description":"Coffee","amount_cents":-1500,...}
organizze: <-- POST /transactions status=201 body={"id":99,...}
```

The Authorization header is never logged. Response bodies are truncated at 2KB to keep the output readable. Off by default; when off, the logging path is a single boolean test with no allocations.

Use this when the API's behaviour disagrees with what you sent — Organizze silently drops some fields (see `AGENTS.md` "Known Organizze API gotchas") and the only way to see the discrepancy is to compare what went out against what came back.

## Adoption & forks

This project is intentionally low-friction to adopt:

- **MIT-licensed**, no CLA, no attribution clause beyond the license header.
- **Single composition root** ([`cmd/organizze-mcp/main.go`](cmd/organizze-mcp/main.go)) — fork, rebrand, repoint the Docker image, and ship.
- **No Organizze logos, trademarks, or proprietary assets** in the repo.
- **No telemetry or analytics**: the server only speaks the Organizze REST API and MCP.
- **Pinned tooling**: Go ≥ 1.23, stdlib `net/http`, the official `github.com/modelcontextprotocol/go-sdk`.

If you maintain Organizze (or another MCP catalogue) and want to host the official version, please open an issue or just fork — no permission needed beyond the MIT license.

## License

MIT — see [`LICENSE`](LICENSE).
