# organizze-mcp-oauth

Multi-tenant variant of `organizze-mcp`. Hosts an OAuth 2.1 Authorization
Server alongside the MCP endpoint so each ChatGPT user authenticates with
their own Organizze credentials instead of the operator embedding a single
set in env vars.

## Prerequisites

- **Postgres** reachable from the container (any version Postgres 13+ is fine). Create an empty database; the binary applies migrations automatically at startup, so no manual `oauth-migrate-up` is needed for the happy path.
- **Public HTTPS endpoint** that resolves to the container's listen port (`8080` by default). `OAUTH_PUBLIC_URL` must match exactly what ChatGPT will hit — including scheme, host, and port if non-default.
- **Two random secrets** generated up-front (see Run example): one 32-byte hex string for `OAUTH_ENCRYPTION_KEY`, one 32+ byte string for `OAUTH_COOKIE_SECRET`. Back up both. Losing the encryption key bricks every stored API key.

## Run

```bash
docker run --rm -p 8080:8080 \
  -e OAUTH_DATABASE_URL=postgres://user:pass@host/organizze_oauth \
  -e OAUTH_PUBLIC_URL=https://your-host.example.com \
  -e OAUTH_ENCRYPTION_KEY=$(openssl rand -hex 32) \
  -e OAUTH_COOKIE_SECRET=$(openssl rand -hex 32) \
  jorgejr568/organizze-mcp-oauth:latest
```

| Env var                | Required | Purpose                                                  |
| ---------------------- | -------- | -------------------------------------------------------- |
| `OAUTH_DATABASE_URL`   | yes      | libpq URI for the OAuth Postgres                          |
| `OAUTH_PUBLIC_URL`     | yes      | Externally reachable origin (no trailing slash, HTTPS)    |
| `OAUTH_ENCRYPTION_KEY` | yes      | Hex-encoded 32 bytes; AES-GCM key for the Organizze API key column |
| `OAUTH_COOKIE_SECRET`  | yes      | HMAC secret for the authorize-flow consent binding and (future) browser session cookie. Length is measured in raw bytes — minimum 32; the `openssl rand -hex 32` example produces 64 ASCII chars and is sufficient. Rotating invalidates in-flight authorize forms ("bad signature" on POST) but does not affect already-issued tokens. |
| `MCP_HTTP_ADDR`        | no       | Listen address, default `:8080`                          |
| `ORGANIZZE_BASE_URL`   | no       | Override Organizze API base                              |
| `ORGANIZZE_LOG_REQUESTS` | no     | Set to `1` to log every outbound Organizze request/response to stderr. Authorization header is redacted. |
| `MCP_STATS_INGEST_URL` | no       | Function URL of the stats-ingest Lambda. Combined with `MCP_STATS_INGEST_TOKEN`, enables off-box telemetry (tool name, status, error_class, duration — never arguments or return values). Missing either env var → NoopReporter (no telemetry). |
| `MCP_STATS_INGEST_TOKEN` | no     | Matching `X-Ingest-Token` for the ingest Lambda. Pair with `MCP_STATS_INGEST_URL`. |
| `MCP_STATS_OPTOUT`     | no       | Set to `1` to force-disable stats even when the ingest env vars are present. |
| `ORGANIZZE_API_KEY`    | **must NOT be set** | Single-tenant env; the binary refuses to start with it set |

## Connect from ChatGPT (Developer Mode)

In ChatGPT -> Settings -> Connectors -> Add custom MCP server:
- URL: `https://<your-host>/mcp`
- Auth: OAuth (ChatGPT will auto-discover via the `.well-known` docs)

On first authorize, ChatGPT will open `<your-host>/oauth/authorize` in a
browser tab. Enter the Organizze email + API key + user-agent string and
approve. The server validates the credentials against the live Organizze
API before storing them.

> **Known limitation:** the consent form re-prompts for credentials on every authorize flow today — session cookies and one-click re-authorize are scaffolded (`internal/oauth/server/session.go`) but not yet wired into the handler. The CSRF and consent-tamper concerns this would otherwise raise are addressed via an HMAC-signed consent binding (`OAUTH_COOKIE_SECRET`-keyed, 10-minute TTL); see `internal/oauth/server/consent.go`. Track [issue / future task] to enable session-backed silent re-authorize.

## Operations

- **Encryption key rotation.** Not yet supported in-binary. To rotate, write
  a one-off Go program that opens each `oauth_users` row with the old key
  and re-seals with the new. Bumping `OAUTH_ENCRYPTION_KEY` alone will make
  every stored `api_key` undecryptable — back up the key.
- **Migrations.** Applied automatically at startup; each applied file is
  recorded in the `schema_migrations` ledger table so re-runs are no-ops
  even for non-idempotent statements. To run manually:
  `OAUTH_DATABASE_URL=... make oauth-migrate-up`.
- **Revoking a user.** `DELETE FROM oauth_users WHERE organizze_email = '…';`
  cascades to sessions, codes, and tokens.
- **Audit.** Bearer-token denials and Organizze validation failures are
  logged at stderr.
