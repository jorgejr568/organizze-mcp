-- 001_init.sql
-- Initial schema for the stats_events ingestion table.
--
-- Idempotency: message_id is the SQS message ID and is UNIQUE so the
-- consumer's `INSERT ... ON CONFLICT (message_id) DO NOTHING` is a safe
-- no-op on SQS redelivery (which is guaranteed at-least-once).

BEGIN;

CREATE TABLE IF NOT EXISTS stats_events (
    id          BIGSERIAL PRIMARY KEY,
    message_id  TEXT NOT NULL UNIQUE,
    payload     JSONB NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS stats_events_received_at_idx
    ON stats_events (received_at);

COMMIT;
