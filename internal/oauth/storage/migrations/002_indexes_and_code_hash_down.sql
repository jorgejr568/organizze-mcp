BEGIN;
DROP INDEX IF EXISTS oauth_codes_expires_idx;
DROP INDEX IF EXISTS oauth_tokens_code_hash_idx;
DROP INDEX IF EXISTS oauth_tokens_refresh_for_idx;
ALTER TABLE oauth_tokens DROP COLUMN IF EXISTS code_hash;
COMMIT;
