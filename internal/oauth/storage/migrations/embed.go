// Package migrations embeds the OAuth-server SQL files so the binary
// can ship them and ApplyMigrations can run them. SQL files are the
// source of truth; this file is just the embed bridge.
//
// Naming convention:
//   - NNN_<slug>.sql       — applied in lexical order by ApplyMigrations.
//                            Each file owns its own BEGIN/COMMIT.
//   - NNN_<slug>_down.sql  — rollback counterpart; reserved for the
//                            Makefile target, never executed by the runner.
//
// ApplyMigrations skips files matching the `_down.sql` suffix and records
// each applied file in the schema_migrations ledger table.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
