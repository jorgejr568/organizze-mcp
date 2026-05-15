.PHONY: build test test-integration migrate-up docker docker-push clean

IMAGE_NAME := jorgejr568/organizze-mcp-ingestion-consumer
IMAGE_TAG  ?= latest
BINARY     := consumer
MIGRATION  := migrations/001_init.sql

# Local-only binary build (handy for `go run ./...` testing). Not used by the
# Docker workflow, which builds inside its own multi-stage image.
build:
	go build -trimpath -ldflags='-s -w' -o $(BINARY) ./

test:
	go test -race -count=1 ./...

test-integration:
	@test -n "$$STATS_DATABASE_URL_TEST" || (echo "STATS_DATABASE_URL_TEST must be set" >&2; exit 1)
	go test -tags integration -race -count=1 ./...

migrate-up:
	@test -n "$$STATS_DATABASE_URL" || (echo "STATS_DATABASE_URL must be set (use the target DB's DSN)" >&2; exit 1)
	psql "$$STATS_DATABASE_URL" -v ON_ERROR_STOP=1 -f $(MIGRATION)

# Build locally. Build context is the repo root so the Dockerfile can see
# both `cmd/consumer/` and other top-level config.
docker:
	cd ../.. && docker build -f cmd/consumer/Dockerfile -t $(IMAGE_NAME):$(IMAGE_TAG) .

docker-push: docker
	docker push $(IMAGE_NAME):$(IMAGE_TAG)

clean:
	rm -f $(BINARY)
