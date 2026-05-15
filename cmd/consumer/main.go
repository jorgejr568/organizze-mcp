package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jorgejr568/organizze-mcp/cmd/consumer/internal/handler"
	"github.com/jorgejr568/organizze-mcp/cmd/consumer/internal/store"
)

func main() {
	dsn := mustEnv("STATS_DATABASE_URL")

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("pgxpool.New: %v", err)
	}

	// Fail fast on a bad DSN: ping with a short deadline at cold start
	// instead of crashing on the first invocation.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		log.Fatalf("postgres ping: %v", err)
	}

	h := &handler.Handler{
		Store: store.New(pool),
		Log:   log.Default(),
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
