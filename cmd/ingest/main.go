package main

import (
	"context"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/jorgejr568/organizze-mcp/cmd/ingest/internal/handler"
)

func main() {
	logger := mustLogger()
	defer func() { _ = logger.Sync() }()

	queueURL := mustEnv(logger, "STATS_QUEUE_URL")
	secretARN := mustEnv(logger, "INGEST_SHARED_SECRET_ARN")

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Fatal("aws config", zap.Error(err))
	}

	secret := fetchSharedSecret(ctx, logger, cfg, secretARN)

	h := &handler.Handler{
		QueueURL: queueURL,
		Secret:   secret,
		SQS:      sqs.NewFromConfig(cfg),
		Log:      logger,
	}

	lambda.Start(h.Handle)
}

// mustLogger returns the production JSON logger used across the Lambda. CloudWatch
// Logs receives stderr verbatim, so JSON structured output is parsed directly by
// the log group's discovered fields.
func mustLogger() *zap.Logger {
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{"stderr"}
	cfg.ErrorOutputPaths = []string{"stderr"}
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	l, err := cfg.Build()
	if err != nil {
		panic("build logger: " + err.Error())
	}
	return l
}

// fetchSharedSecret materialises the X-Ingest-Token value from AWS Secrets
// Manager at cold start. Failure is fatal: a misconfigured ARN is a deploy-time
// bug, not a per-request condition, and silently 401-ing every request would
// hide it. The 5s deadline keeps cold-starts bounded if Secrets Manager is slow.
func fetchSharedSecret(ctx context.Context, logger *zap.Logger, cfg aws.Config, arn string) string {
	fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	sm := secretsmanager.NewFromConfig(cfg)
	out, err := sm.GetSecretValue(fetchCtx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(arn),
	})
	if err != nil {
		logger.Fatal("fetch shared secret", zap.Error(err))
	}
	if out.SecretString == nil || *out.SecretString == "" {
		logger.Fatal("shared secret value is empty", zap.String("secret_id", arn))
	}
	return *out.SecretString
}

func mustEnv(logger *zap.Logger, key string) string {
	v := os.Getenv(key)
	if v == "" {
		logger.Fatal("missing required env var", zap.String("key", key))
	}
	return v
}
