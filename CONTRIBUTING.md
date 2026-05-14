# Contributing to organizze-mcp

Thanks for your interest. This is a small, opinionated project — please read this before opening a PR.

## Before you start

For non-trivial changes (a new tool, a new transport, a breaking API change), please open an issue first to discuss the design. The architecture is intentionally layered (`domain` → `usecase` → `adapter` → `cmd`); changes that cut across layers usually need a small spec note.

## Development setup

```bash
git clone https://github.com/jorgejr568/organizze-mcp
cd organizze-mcp
make test       # full suite with race detector
make test-cover # coverage report
make lint       # go vet
make build      # local binary
```

You'll need Go ≥ 1.25 and (for the container path) Docker with buildx.

## Pull-request expectations

- **Tests are part of the change.** Every behaviour change ships with a test that fails on `main` and passes on your branch. Tests verify behaviour, not mocks.
- **One concern per PR.** If you find unrelated polish along the way, split it.
- **Branch protection enforces the CI check.** Push will succeed; merge requires the `Test` job green.
- **Commit messages follow Conventional Commits**: `feat(scope): summary`, `fix(scope): summary`, `chore: summary`, `docs: summary`, `ci: summary`. The PR title becomes the squash-merge subject — make it look like a release-note line.

## Architecture overview

See the README's "Architecture" section. Two rules:

1. **Dependencies point inward.** `domain` imports stdlib only. `usecase` imports `domain`. `adapter/*` imports `usecase` + `domain`. `cmd` imports everyone. Going the other way means refactoring, not adding.
2. **Repository and service interfaces are consumer-owned.** They live in `usecase` (for repos) and in each `adapter/mcp/tools_*.go` (for services); the implementations satisfy them implicitly.

## Adding a new resource (e.g., `Contact`)

The pattern is mechanical. Six small new files, plus two edits:

1. `internal/domain/contact.go` — entity struct
2. `internal/usecase/contact.go` — `ContactRepository` interface + `ContactService` struct
3. `internal/usecase/contact_test.go` — service tests with a fake repo
4. `internal/adapter/organizze/contact_repository.go` — HTTP impl
5. `internal/adapter/organizze/contact_repository_test.go` — `httptest`-backed tests
6. `internal/adapter/mcp/tools_contacts.go` — consumer-side service interface + tool registrations
7. Edit `internal/adapter/mcp/server.go` — add `Contact ContactService` to `Dependencies`; call `registerContactTools` from `New`
8. Edit `cmd/organizze-mcp/main.go` — one line in `buildServer` wiring the service

The integration test in `internal/adapter/mcp/integration_test.go` will fail until you add the resource to `allExpectedTools` and the new endpoints to `fakeOrganizze`.

## Release process

Releases are tag-driven:

```bash
git tag -a vX.Y.Z -m "vX.Y.Z — short summary"
git push origin vX.Y.Z
```

The release workflow runs the suite, builds the multi-arch image, and publishes to Docker Hub.

## License

By contributing, you agree your contributions are licensed under the MIT License (see `LICENSE`).
