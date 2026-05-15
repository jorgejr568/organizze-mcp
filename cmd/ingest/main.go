package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/jorgejr568/organizze-mcp/cmd/ingest/internal/handler"
)

func main() {
	queueURL := mustEnv("STATS_QUEUE_URL")
	secret := mustEnv("INGEST_SHARED_SECRET")

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	h := &handler.Handler{
		QueueURL: queueURL,
		Secret:   secret,
		SQS:      sqs.NewFromConfig(cfg),
		Log:      log.Default(),
	}

	lambda.Start(h.Handle)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var %s", key)
	}
	return v
}
