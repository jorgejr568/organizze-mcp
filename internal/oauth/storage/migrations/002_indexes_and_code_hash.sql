-- 002_indexes_and_code_hash.sql
-- Add code_hash on oauth_tokens so an auth-code replay can revoke every
-- token issued from that code (RFC 6749 §10.5 / Security BCP §4.10).
-- Index oauth_tokens.refresh_for so RevokeRefreshFamily is O(family size),
-- not a table scan. Index oauth_codes.expires_at for periodic GC.

BEGIN;

ALTER TABLE oauth_tokens ADD COLUMN IF NOT EXISTS code_hash BYTEA;

CREATE INDEX IF NOT EXISTS oauth_tokens_refresh_for_idx
  ON oauth_tokens (refresh_for) WHERE refresh_for IS NOT NULL;

CREATE INDEX IF NOT EXISTS oauth_tokens_code_hash_idx
  ON oauth_tokens (code_hash) WHERE code_hash IS NOT NULL;

CREATE INDEX IF NOT EXISTS oauth_codes_expires_idx
  ON oauth_codes (expires_at);

COMMIT;
