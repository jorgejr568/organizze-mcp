// Package migrations embeds the OAuth-server SQL files so the binary
// can ship them and ApplyMigrations can run them. SQL files are the
// source of truth; this file is just the embed bridge.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
