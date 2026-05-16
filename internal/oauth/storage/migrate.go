package storage

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage/migrations"
)

// ApplyMigrations runs every embedded *.sql file in lexical order. Each
// file is expected to wrap its own statements in BEGIN; ... COMMIT; for
// atomicity — the runner does not start a transaction itself. The init
// migration uses CREATE TABLE IF NOT EXISTS so re-runs are no-ops.
func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("oauth/storage: read embed: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), "_down.sql") {
			// Down files are applied only via the Makefile target, never
			// by the migration runner — running 001_down right after
			// 001_init would wipe every table.
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("oauth/storage: read %s: %w", name, err)
		}
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("oauth/storage: apply %s: %w", name, err)
		}
	}
	return nil
}
