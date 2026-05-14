# Security policy

## Supported versions

| Version | Supported |
|---------|-----------|
| 0.2.x   | ✅        |
| 0.1.x   | ❌        |

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

Email `jorgejuniordev@gmail.com` with:
- A description of the issue.
- Reproduction steps or a proof-of-concept if available.
- The version (commit SHA or container digest) you observed it on.

Expect an acknowledgement within 72 hours and a fix or mitigation within 14 days for high-severity issues.

## Threat model in scope

organizze-mcp holds a user's Organizze API token in an environment variable and forwards it via HTTP Basic Auth on every request. Specifically in scope:

- Token leakage via logs, error messages, or panics. The server logs to `stderr` only on stdio; HTTP transport must not echo the token. Verify with `docker logs`.
- Request smuggling, path traversal, or other tool-name injection — MCP tool names and arguments are validated by the SDK schema; report any bypass.
- Supply-chain integrity of the published Docker image. The image is built reproducibly from a tagged commit on `main`; the commit SHA is embedded as the `org.opencontainers.image.revision` label. Compare against this repo before trusting the image.

Out of scope: general Organizze API security (report to Organizze directly), denial of service against your own running instance, social engineering.
