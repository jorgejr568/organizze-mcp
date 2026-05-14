# organizze-mcp

A Model Context Protocol (MCP) server exposing the [Organizze](https://www.organizze.com.br/) REST API to LLM clients (Claude Desktop, Claude Code, etc.), built in Go with a layered Clean Architecture.

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

## Tool catalogue

16 tools. `amount_cents` is **negative for expenses, positive for income**.

| Tool | Service.Method |
|---|---|
| `get_user` | UserService.Get |
| `list_accounts` / `get_account` | AccountService.{List, Get} |
| `list_categories` / `get_category` | CategoryService.{List, Get} |
| `list_budgets` | BudgetService.List (year/month routing) |
| `list_credit_cards` / `get_credit_card` | CreditCardService.{List, Get} |
| `list_credit_card_invoices` / `get_credit_card_invoice` | InvoiceService.{List, Get} |
| `list_transfers` | TransferService.List |
| `list_transactions` / `get_transaction` | TransactionService.{List, Get} |
| `create_transaction` / `update_transaction` / `delete_transaction` | TransactionService.{Create, Update, Delete} |

## Development

```bash
make test        # full suite
make test-cover  # with coverage report
make lint        # go vet
make build       # binary at bin/organizze-mcp
make docker      # container image
```

## License

MIT (or your preferred license).
