package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres is a pgx-backed implementation of Store.
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres wraps an existing connection pool. The caller owns the pool.
func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

// --- users ---

func (s *Postgres) UpsertUserByEmail(ctx context.Context, u User) (User, error) {
	const q = `
INSERT INTO oauth_users (organizze_email, organizze_api_key_cipher, organizze_api_key_nonce, user_agent)
VALUES ($1, $2, $3, $4)
ON CONFLICT (organizze_email) DO UPDATE
SET organizze_api_key_cipher = EXCLUDED.organizze_api_key_cipher,
    organizze_api_key_nonce  = EXCLUDED.organizze_api_key_nonce,
    user_agent               = EXCLUDED.user_agent,
    updated_at               = NOW()
RETURNING id, organizze_email, organizze_api_key_cipher, organizze_api_key_nonce, user_agent, created_at, updated_at
`
	row := s.pool.QueryRow(ctx, q, u.OrganizzeEmail, u.APIKeyCipher, u.APIKeyNonce, u.UserAgent)
	var got User
	if err := row.Scan(&got.ID, &got.OrganizzeEmail, &got.APIKeyCipher, &got.APIKeyNonce, &got.UserAgent, &got.CreatedAt, &got.UpdatedAt); err != nil {
		return User{}, fmt.Errorf("oauth/storage: upsert user: %w", err)
	}
	return got, nil
}

func (s *Postgres) GetUser(ctx context.Context, id int64) (User, error) {
	const q = `SELECT id, organizze_email, organizze_api_key_cipher, organizze_api_key_nonce, user_agent, created_at, updated_at FROM oauth_users WHERE id = $1`
	row := s.pool.QueryRow(ctx, q, id)
	var u User
	if err := row.Scan(&u.ID, &u.OrganizzeEmail, &u.APIKeyCipher, &u.APIKeyNonce, &u.UserAgent, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return u, nil
}

func (s *Postgres) GetUserByEmail(ctx context.Context, email string) (User, error) {
	const q = `SELECT id, organizze_email, organizze_api_key_cipher, organizze_api_key_nonce, user_agent, created_at, updated_at FROM oauth_users WHERE organizze_email = $1`
	row := s.pool.QueryRow(ctx, q, email)
	var u User
	if err := row.Scan(&u.ID, &u.OrganizzeEmail, &u.APIKeyCipher, &u.APIKeyNonce, &u.UserAgent, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return u, nil
}

// --- clients ---

func (s *Postgres) CreateClient(ctx context.Context, c Client) error {
	uris, err := json.Marshal(c.RedirectURIs)
	if err != nil {
		return fmt.Errorf("oauth/storage: marshal redirect uris: %w", err)
	}
	const q = `INSERT INTO oauth_clients (id, client_secret_hash, client_name, redirect_uris) VALUES ($1, $2, $3, $4)`
	if _, err := s.pool.Exec(ctx, q, c.ID, c.ClientSecretHash, c.ClientName, uris); err != nil {
		return fmt.Errorf("oauth/storage: create client: %w", err)
	}
	return nil
}

func (s *Postgres) GetClient(ctx context.Context, id string) (Client, error) {
	const q = `SELECT id, client_secret_hash, client_name, redirect_uris, created_at FROM oauth_clients WHERE id = $1`
	row := s.pool.QueryRow(ctx, q, id)
	var c Client
	var uris []byte
	if err := row.Scan(&c.ID, &c.ClientSecretHash, &c.ClientName, &uris, &c.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Client{}, ErrNotFound
		}
		return Client{}, err
	}
	if err := json.Unmarshal(uris, &c.RedirectURIs); err != nil {
		return Client{}, fmt.Errorf("oauth/storage: unmarshal redirect uris: %w", err)
	}
	return c, nil
}

// --- sessions ---

func (s *Postgres) CreateSession(ctx context.Context, sess Session) error {
	const q = `INSERT INTO oauth_sessions (id, user_id, expires_at) VALUES ($1, $2, $3)`
	if _, err := s.pool.Exec(ctx, q, sess.ID, sess.UserID, sess.ExpiresAt); err != nil {
		return fmt.Errorf("oauth/storage: create session: %w", err)
	}
	return nil
}

func (s *Postgres) GetSession(ctx context.Context, id string) (Session, error) {
	const q = `SELECT id, user_id, expires_at, created_at FROM oauth_sessions WHERE id = $1 AND expires_at > NOW()`
	row := s.pool.QueryRow(ctx, q, id)
	var sess Session
	if err := row.Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt, &sess.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrNotFound
		}
		return Session{}, err
	}
	return sess, nil
}

func (s *Postgres) DeleteSession(ctx context.Context, id string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM oauth_sessions WHERE id = $1`, id); err != nil {
		return fmt.Errorf("oauth/storage: delete session: %w", err)
	}
	return nil
}

// --- auth codes ---

func (s *Postgres) CreateAuthCode(ctx context.Context, ac AuthCode) error {
	const q = `
INSERT INTO oauth_codes (code_hash, client_id, user_id, redirect_uri, code_challenge, code_challenge_method, scope, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`
	if _, err := s.pool.Exec(ctx, q, ac.CodeHash, ac.ClientID, ac.UserID, ac.RedirectURI, ac.CodeChallenge, ac.CodeChallengeMethod, ac.Scope, ac.ExpiresAt); err != nil {
		return fmt.Errorf("oauth/storage: create auth code: %w", err)
	}
	return nil
}

func (s *Postgres) ConsumeAuthCode(ctx context.Context, codeHash []byte) (AuthCode, error) {
	const q = `
UPDATE oauth_codes
SET consumed_at = NOW()
WHERE code_hash = $1 AND consumed_at IS NULL AND expires_at > NOW()
RETURNING code_hash, client_id, user_id, redirect_uri, code_challenge, code_challenge_method, scope, expires_at, consumed_at
`
	row := s.pool.QueryRow(ctx, q, codeHash)
	var ac AuthCode
	if err := row.Scan(&ac.CodeHash, &ac.ClientID, &ac.UserID, &ac.RedirectURI, &ac.CodeChallenge, &ac.CodeChallengeMethod, &ac.Scope, &ac.ExpiresAt, &ac.ConsumedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AuthCode{}, ErrNotFound
		}
		return AuthCode{}, err
	}
	return ac, nil
}

// --- tokens ---

func (s *Postgres) CreateToken(ctx context.Context, tok Token) error {
	const q = `
INSERT INTO oauth_tokens (token_hash, kind, client_id, user_id, refresh_for, code_hash, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`
	if _, err := s.pool.Exec(ctx, q, tok.TokenHash, tok.Kind, tok.ClientID, tok.UserID, tok.RefreshFor, tok.CodeHash, tok.ExpiresAt); err != nil {
		return fmt.Errorf("oauth/storage: create token: %w", err)
	}
	return nil
}

func (s *Postgres) GetToken(ctx context.Context, tokenHash []byte) (Token, error) {
	const q = `SELECT token_hash, kind, client_id, user_id, refresh_for, code_hash, expires_at, revoked_at, created_at FROM oauth_tokens WHERE token_hash = $1`
	row := s.pool.QueryRow(ctx, q, tokenHash)
	var t Token
	if err := row.Scan(&t.TokenHash, &t.Kind, &t.ClientID, &t.UserID, &t.RefreshFor, &t.CodeHash, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Token{}, ErrNotFound
		}
		return Token{}, err
	}
	return t, nil
}

// RotateRefreshToken atomically marks the refresh row revoked when it is
// currently un-revoked and un-expired. Concurrent callers race on the same
// UPDATE — the loser sees ErrNotFound.
func (s *Postgres) RotateRefreshToken(ctx context.Context, refreshHash []byte) (Token, error) {
	const q = `
UPDATE oauth_tokens
   SET revoked_at = NOW()
 WHERE token_hash = $1
   AND kind = 'refresh'
   AND revoked_at IS NULL
   AND expires_at > NOW()
RETURNING token_hash, kind, client_id, user_id, refresh_for, code_hash, expires_at, revoked_at, created_at
`
	row := s.pool.QueryRow(ctx, q, refreshHash)
	var t Token
	if err := row.Scan(&t.TokenHash, &t.Kind, &t.ClientID, &t.UserID, &t.RefreshFor, &t.CodeHash, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Token{}, ErrNotFound
		}
		return Token{}, err
	}
	return t, nil
}

func (s *Postgres) RevokeToken(ctx context.Context, tokenHash []byte) error {
	if _, err := s.pool.Exec(ctx, `UPDATE oauth_tokens SET revoked_at = NOW() WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash); err != nil {
		return fmt.Errorf("oauth/storage: revoke token: %w", err)
	}
	return nil
}

// RevokeRefreshFamily revokes the refresh token and every access token issued
// from it. Called when a refresh-token reuse is detected, or when the user
// explicitly logs out.
func (s *Postgres) RevokeRefreshFamily(ctx context.Context, refreshHash []byte) error {
	const q = `
UPDATE oauth_tokens
SET revoked_at = NOW()
WHERE revoked_at IS NULL
  AND (token_hash = $1 OR refresh_for = $1)
`
	if _, err := s.pool.Exec(ctx, q, refreshHash); err != nil {
		return fmt.Errorf("oauth/storage: revoke refresh family: %w", err)
	}
	return nil
}

// RevokeFamilyByCode revokes every still-live token issued from the given
// authorization code. Called on code replay.
func (s *Postgres) RevokeFamilyByCode(ctx context.Context, codeHash []byte) error {
	const q = `UPDATE oauth_tokens SET revoked_at = NOW() WHERE code_hash = $1 AND revoked_at IS NULL`
	if _, err := s.pool.Exec(ctx, q, codeHash); err != nil {
		return fmt.Errorf("oauth/storage: revoke family by code: %w", err)
	}
	return nil
}

// Compile-time check that *Postgres satisfies Store.
var _ Store = (*Postgres)(nil)
