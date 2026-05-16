package storage

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// requireDB skips the test if OAUTH_DATABASE_URL is not set.
func requireDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("OAUTH_DATABASE_URL")
	if dsn == "" {
		t.Skip("OAUTH_DATABASE_URL not set; skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestApplyMigrationsIsIdempotent(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	if err := ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("first ApplyMigrations: %v", err)
	}
	if err := ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("second ApplyMigrations: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables WHERE table_name = 'oauth_users'`).Scan(&n); err != nil {
		t.Fatalf("schema check: %v", err)
	}
	if n != 1 {
		t.Errorf("oauth_users table count = %d, want 1", n)
	}
}
