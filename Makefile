.PHONY: build test test-cover lint run-stdio run-http docker docker-run clean

BINARY := organizze-mcp
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
INGEST_URL ?=
INGEST_TOKEN ?=
LDFLAGS := -s -w \
	-X 'github.com/jorgejr568/organizze-mcp/internal/adapter/mcp.Version=$(VERSION)' \
	-X 'github.com/jorgejr568/organizze-mcp/internal/stats.DefaultIngestURL=$(INGEST_URL)' \
	-X 'github.com/jorgejr568/organizze-mcp/internal/stats.DefaultIngestToken=$(INGEST_TOKEN)'

build:
	go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) ./cmd/organizze-mcp

test:
	go test ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	go vet ./...

run-stdio: build
	MCP_TRANSPORT=stdio ./bin/$(BINARY)

run-http: build
	MCP_TRANSPORT=http MCP_HTTP_ADDR=:8080 ./bin/$(BINARY)

docker:
	docker build -t organizze-mcp:latest .

docker-run:
	docker run --rm -i \
		-e ORGANIZZE_API_KEY -e ORGANIZZE_EMAIL -e ORGANIZZE_USER_AGENT \
		organizze-mcp:latest

clean:
	rm -rf bin/ coverage.out coverage.html

.PHONY: oauth-build oauth-migrate-up oauth-migrate-down

oauth-build:
	go build -o bin/organizze-mcp-oauth ./cmd/organizze-mcp-oauth

oauth-migrate-up:
	@test -n "$$OAUTH_DATABASE_URL" || (echo "OAUTH_DATABASE_URL must be set" && exit 1)
	psql "$$OAUTH_DATABASE_URL" -v ON_ERROR_STOP=1 -f internal/oauth/storage/migrations/001_init.sql

oauth-migrate-down:
	@test -n "$$OAUTH_DATABASE_URL" || (echo "OAUTH_DATABASE_URL must be set" && exit 1)
	psql "$$OAUTH_DATABASE_URL" -v ON_ERROR_STOP=1 -f internal/oauth/storage/migrations/001_down.sql
