package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/jorgejr568/organizze-mcp/cmd/ingest/internal/handler"
)

func main() {
	queueURL := mustEnv("STATS_QUEUE_URL")
	secretARN := mustEnv("INGEST_SHARED_SECRET_ARN")

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	secret := fetchSharedSecret(ctx, cfg, secretARN)

	h := &handler.Handler{
		QueueURL: queueURL,
		Secret:   secret,
		SQS:      sqs.NewFromConfig(cfg),
		Log:      log.Default(),
	}

	lambda.Start(h.Handle)
}

// fetchSharedSecret materialises the X-Ingest-Token value from AWS Secrets
// Manager at cold start. Failure is fatal: a misconfigured ARN is a deploy-time
// bug, not a per-request condition, and silently 401-ing every request would
// hide it. The 5s deadline keeps cold-starts bounded if Secrets Manager is slow.
func fetchSharedSecret(ctx context.Context, cfg aws.Config, arn string) string {
	fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	sm := secretsmanager.NewFromConfig(cfg)
	out, err := sm.GetSecretValue(fetchCtx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(arn),
	})
	if err != nil {
		log.Fatalf("fetch shared secret: %v", err)
	}
	if out.SecretString == nil || *out.SecretString == "" {
		log.Fatalf("shared secret value is empty (SecretId=%s)", arn)
	}
	return *out.SecretString
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var %s", key)
	}
	return v
}
