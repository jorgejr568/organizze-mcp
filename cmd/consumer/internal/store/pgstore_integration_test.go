//go:build integration

package store

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// connect skips the test when STATS_DATABASE_URL_TEST is not configured.
// CI does NOT run integration tests — they're for local verification
// against a disposable Postgres (e.g. a docker run --rm postgres:16).
func connect(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("STATS_DATABASE_URL_TEST")
	if dsn == "" {
		t.Skip("STATS_DATABASE_URL_TEST not set; skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestPGStore_InsertIsIdempotent(t *testing.T) {
	pool := connect(t)
	ctx := context.Background()
	store := New(pool)

	msgID := "test-" + t.Name()
	payload := []byte(`{"test":true}`)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM stats_events WHERE message_id = $1`, msgID)
	})

	for i := 0; i < 3; i++ {
		if err := store.Insert(ctx, msgID, payload); err != nil {
			t.Fatalf("insert #%d: %v", i, err)
		}
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM stats_events WHERE message_id = $1`, msgID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 row after 3 inserts; got %d", count)
	}
}

func TestPGStore_StoresJSONBPayload(t *testing.T) {
	pool := connect(t)
	ctx := context.Background()
	store := New(pool)

	msgID := "test-" + t.Name()
	payload := []byte(`{"nested":{"k":[1,2,3]}}`)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM stats_events WHERE message_id = $1`, msgID)
	})

	if err := store.Insert(ctx, msgID, payload); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// jsonb_path_query confirms the row is queryable as structured JSON,
	// not just stored as a string.
	var got int
	if err := pool.QueryRow(ctx, `
		SELECT (payload #>> '{nested,k,1}')::int
		FROM stats_events WHERE message_id = $1
	`, msgID).Scan(&got); err != nil {
		t.Fatalf("jsonb query: %v", err)
	}
	if got != 2 {
		t.Fatalf("expected nested.k[1] = 2, got %d", got)
	}
}
