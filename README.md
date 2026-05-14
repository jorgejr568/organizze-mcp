# organizze-mcp

A Model Context Protocol (MCP) server exposing the [Organizze](https://www.organizze.com.br/) REST API to LLM clients (Claude Desktop, Claude Code, etc.), built in Go with a layered Clean Architecture.

## Status

[![CI](https://github.com/jorgejr568/organizze-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/jorgejr568/organizze-mcp/actions/workflows/ci.yml)
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

## Tool catalogue (28 tools)

`amount_cents` is **negative for expenses, positive for income**.

| #  | Tool                         | Operation                           | Mutating? |
|----|------------------------------|-------------------------------------|-----------|
| 1  | `get_user`                   | UserService.Get                     | no        |
| 2  | `list_accounts`              | AccountService.List                 | no        |
| 3  | `get_account`                | AccountService.Get                  | no        |
| 4  | `create_account`             | AccountService.Create               | **yes**   |
| 5  | `update_account`             | AccountService.Update               | **yes**   |
| 6  | `delete_account`             | AccountService.Delete               | **yes**   |
| 7  | `list_categories`            | CategoryService.List                | no        |
| 8  | `get_category`               | CategoryService.Get                 | no        |
| 9  | `create_category`            | CategoryService.Create              | **yes**   |
| 10 | `update_category`            | CategoryService.Update              | **yes**   |
| 11 | `delete_category`            | CategoryService.Delete              | **yes**   |
| 12 | `list_budgets`               | BudgetService.List (period routing) | no        |
| 13 | `list_credit_cards`          | CreditCardService.List              | no        |
| 14 | `get_credit_card`            | CreditCardService.Get               | no        |
| 15 | `create_credit_card`         | CreditCardService.Create            | **yes**   |
| 16 | `update_credit_card`         | CreditCardService.Update            | **yes**   |
| 17 | `delete_credit_card`         | CreditCardService.Delete            | **yes**   |
| 18 | `list_credit_card_invoices`  | InvoiceService.List                 | no        |
| 19 | `get_credit_card_invoice`    | InvoiceService.Get                  | no        |
| 20 | `list_transfers`             | TransferService.List                | no        |
| 21 | `create_transfer`            | TransferService.Create              | **yes**   |
| 22 | `update_transfer`            | TransferService.Update              | **yes**   |
| 23 | `delete_transfer`            | TransferService.Delete              | **yes**   |
| 24 | `list_transactions`          | TransactionService.List             | no        |
| 25 | `get_transaction`            | TransactionService.Get              | no        |
| 26 | `create_transaction`         | TransactionService.Create           | **yes**   |
| 27 | `update_transaction`         | TransactionService.Update           | **yes**   |
| 28 | `delete_transaction`         | TransactionService.Delete           | **yes**   |

## Development

```bash
make test        # full suite
make test-cover  # with coverage report
make lint        # go vet
make build       # binary at bin/organizze-mcp
make docker      # container image
```

## License

MIT — see [`LICENSE`](LICENSE).
