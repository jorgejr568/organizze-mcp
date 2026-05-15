// Package store contains the Postgres-backed implementation of the
// handler's StatsStore interface. Idempotency is enforced by the
// UNIQUE constraint on message_id plus ON CONFLICT DO NOTHING, so
// repeated SQS deliveries collapse to a single row.
package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

const insertSQL = `
INSERT INTO stats_events (message_id, payload)
VALUES ($1, $2)
ON CONFLICT (message_id) DO NOTHING
`

type PGStore struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *PGStore {
	return &PGStore{pool: pool}
}

func (s *PGStore) Insert(ctx context.Context, messageID string, payload []byte) error {
	_, err := s.pool.Exec(ctx, insertSQL, messageID, payload)
	return err
}
