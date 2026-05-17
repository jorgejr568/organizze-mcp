package storage

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage/migrations"
)

// schemaMigrationsBootstrap creates the migration-version ledger. Run once at
// startup before reading other migrations; idempotent.
const schemaMigrationsBootstrap = `CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`

// ApplyMigrations runs every embedded *.sql file in lexical order, skipping
// files already recorded in schema_migrations. Each file is expected to wrap
// its own statements in BEGIN; ... COMMIT; for atomicity. Files with the
// `_down.sql` suffix are reserved for the Makefile rollback target and are
// never executed by the runner.
func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schemaMigrationsBootstrap); err != nil {
		return fmt.Errorf("oauth/storage: bootstrap schema_migrations: %w", err)
	}

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
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`,
			name,
		).Scan(&exists); err != nil {
			return fmt.Errorf("oauth/storage: check applied %s: %w", name, err)
		}
		if exists {
			continue
		}
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("oauth/storage: read %s: %w", name, err)
		}
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("oauth/storage: apply %s: %w", name, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO schema_migrations (version) VALUES ($1)`,
			name,
		); err != nil {
			// The migration succeeded but ledgering it failed. Surface the
			// error — the next startup will re-attempt the migration, but
			// every shipped migration uses IF NOT EXISTS / IF EXISTS guards
			// so the re-run is safe.
			return fmt.Errorf("oauth/storage: record %s: %w", name, err)
		}
	}
	return nil
}
