-- 001_init.sql
-- Schema for the multi-tenant OAuth Authorization Server.
--
-- All token columns store SHA-256 hashes, not raw tokens — a DB compromise
-- must not equal active-session takeover. The Organizze API key is stored
-- AES-GCM-encrypted; the encryption key is held only in the operator's env
-- (OAUTH_ENCRYPTION_KEY), so a DB-only leak cannot recover plaintext keys.

BEGIN;

CREATE TABLE IF NOT EXISTS oauth_users (
    id                       BIGSERIAL PRIMARY KEY,
    organizze_email          TEXT NOT NULL UNIQUE,
    organizze_api_key_cipher BYTEA NOT NULL,
    organizze_api_key_nonce  BYTEA NOT NULL,
    user_agent               TEXT NOT NULL,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS oauth_clients (
    id                   TEXT PRIMARY KEY,             -- public client_id
    client_secret_hash   BYTEA,                        -- NULL for public clients (PKCE-only)
    client_name          TEXT NOT NULL,
    redirect_uris        JSONB NOT NULL,               -- ["https://...", ...]
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS oauth_sessions (
    id           TEXT PRIMARY KEY,                     -- random; the cookie value
    user_id      BIGINT NOT NULL REFERENCES oauth_users(id) ON DELETE CASCADE,
    expires_at   TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS oauth_sessions_expires_idx ON oauth_sessions (expires_at);

CREATE TABLE IF NOT EXISTS oauth_codes (
    code_hash             BYTEA PRIMARY KEY,
    client_id             TEXT NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    user_id               BIGINT NOT NULL REFERENCES oauth_users(id) ON DELETE CASCADE,
    redirect_uri          TEXT NOT NULL,
    code_challenge        TEXT NOT NULL,
    code_challenge_method TEXT NOT NULL CHECK (code_challenge_method = 'S256'),
    scope                 TEXT NOT NULL DEFAULT '',
    expires_at            TIMESTAMPTZ NOT NULL,
    consumed_at           TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS oauth_tokens (
    token_hash   BYTEA PRIMARY KEY,
    kind         TEXT NOT NULL CHECK (kind IN ('access', 'refresh')),
    client_id    TEXT NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    user_id      BIGINT NOT NULL REFERENCES oauth_users(id) ON DELETE CASCADE,
    refresh_for  BYTEA REFERENCES oauth_tokens(token_hash) ON DELETE SET NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS oauth_tokens_expires_idx ON oauth_tokens (expires_at);
CREATE INDEX IF NOT EXISTS oauth_tokens_user_idx    ON oauth_tokens (user_id);

COMMIT;
