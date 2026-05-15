.PHONY: build zip deploy test test-integration migrate-up clean

FUNCTION_NAME := organizze-mcp-stats-consumer
BINARY := bootstrap
ZIP := function.zip
MIGRATION := migrations/001_init.sql

build:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
		go build -tags lambda.norpc -ldflags='-s -w' -o $(BINARY) ./

zip: build
	rm -f $(ZIP)
	zip -j $(ZIP) $(BINARY)

deploy: zip
	aws lambda update-function-code \
		--function-name $(FUNCTION_NAME) \
		--zip-file fileb://$(ZIP) \
		--publish

test:
	go test -race -count=1 ./...

test-integration:
	@test -n "$$STATS_DATABASE_URL_TEST" || (echo "STATS_DATABASE_URL_TEST must be set" >&2; exit 1)
	go test -tags integration -race -count=1 ./...

migrate-up:
	@test -n "$$STATS_DATABASE_URL" || (echo "STATS_DATABASE_URL must be set (use the target DB's DSN)" >&2; exit 1)
	psql "$$STATS_DATABASE_URL" -v ON_ERROR_STOP=1 -f $(MIGRATION)

clean:
	rm -f $(BINARY) $(ZIP)
