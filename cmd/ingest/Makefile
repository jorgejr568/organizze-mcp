.PHONY: build zip deploy test clean

FUNCTION_NAME := organizze-mcp-stats-ingest
BINARY := bootstrap
ZIP := function.zip

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

clean:
	rm -f $(BINARY) $(ZIP)
