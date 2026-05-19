package server

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope,omitempty"`
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOAuthError(w, http.StatusMethodNotAllowed, "invalid_request", "POST required")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		s.grantAuthorizationCode(w, r)
	case "refresh_token":
		s.grantRefreshToken(w, r)
	default:
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "")
	}
}

func (s *Server) grantAuthorizationCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clientID, err := authenticateClient(ctx, s.cfg.Store, r)
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client", "")
		return
	}
	code := r.PostForm.Get("code")
	redirectURI := r.PostForm.Get("redirect_uri")
	verifier := r.PostForm.Get("code_verifier")
	if code == "" || verifier == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "missing required field")
		return
	}
	codeHash := storage.HashToken(code)
	ac, err := s.cfg.Store.ConsumeAuthCode(ctx, codeHash)
	if err != nil {
		// Code unknown, used, or expired. If the code was previously consumed
		// (re-presentation = reuse signal per RFC 6749 §10.5 / Security BCP
		// §4.10), every token issued from it must be revoked. The store call
		// is a no-op when no tokens reference this code, so we can fire it
		// unconditionally on ErrNotFound without distinguishing replay from
		// garbage.
		if errors.Is(err, storage.ErrNotFound) {
			_ = s.cfg.Store.RevokeFamilyByCode(ctx, codeHash)
		}
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "code unknown, used, or expired")
		return
	}
	if ac.ClientID != clientID || ac.RedirectURI != redirectURI {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "client_id or redirect_uri mismatch")
		return
	}
	if !verifyPKCE(verifier, ac.CodeChallenge) {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "PKCE verifier mismatch")
		return
	}
	access, refresh, err := s.issueTokenPair(ctx, clientID, ac.UserID, codeHash)
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "")
		return
	}
	writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.cfg.AccessTokenTTL.Seconds()),
		RefreshToken: refresh,
	})
}

func (s *Server) grantRefreshToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clientID, err := authenticateClient(ctx, s.cfg.Store, r)
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client", "")
		return
	}
	rt := r.PostForm.Get("refresh_token")
	if rt == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "")
		return
	}
	hash := storage.HashToken(rt)

	// Pre-read to detect prior-revocation as the reuse-attack signal. The
	// pre-read + RotateRefreshToken pair is safe under concurrency: rotation
	// is what's atomic; the pre-read only adds the family-revoke branch for
	// already-revoked rows. Garbage / expired tokens take the same flat
	// invalid_grant exit as a lost rotation race.
	if pre, err := s.cfg.Store.GetToken(ctx, hash); err == nil && pre.Kind == "refresh" && pre.RevokedAt != nil {
		_ = s.cfg.Store.RevokeRefreshFamily(ctx, hash)
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "")
		return
	}

	tok, err := s.cfg.Store.RotateRefreshToken(ctx, hash)
	if err != nil {
		// Lost the race, or garbage, or expired. RFC 6749-compliant
		// invalid_grant — do NOT revoke the family because we can't
		// distinguish unknown-token from rotated-by-someone-else.
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "")
		return
	}
	if tok.ClientID != clientID {
		// Mismatched client — revoke the family defensively since a leaked
		// refresh token + bogus client_id is a strong attack signal.
		_ = s.cfg.Store.RevokeRefreshFamily(ctx, hash)
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "client_id mismatch")
		return
	}
	access, refresh, err := s.issueTokenPair(ctx, clientID, tok.UserID, nil)
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "")
		return
	}
	writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.cfg.AccessTokenTTL.Seconds()),
		RefreshToken: refresh,
	})
}

// issueTokenPair creates a refresh+access token pair. codeHash is the
// SHA-256 of the authorization code that minted this pair (used by
// RevokeFamilyByCode on code replay); pass nil from the refresh-grant path.
func (s *Server) issueTokenPair(ctx context.Context, clientID string, userID int64, codeHash []byte) (string, string, error) {
	access := newRandomToken()
	refresh := newRandomToken()
	now := s.cfg.Now()
	refreshHash := storage.HashToken(refresh)
	if err := s.cfg.Store.CreateToken(ctx, storage.Token{
		TokenHash: refreshHash,
		Kind:      "refresh",
		ClientID:  clientID,
		UserID:    userID,
		CodeHash:  codeHash,
		ExpiresAt: now.Add(s.cfg.RefreshTokenTTL),
	}); err != nil {
		return "", "", err
	}
	if err := s.cfg.Store.CreateToken(ctx, storage.Token{
		TokenHash:  storage.HashToken(access),
		Kind:       "access",
		ClientID:   clientID,
		UserID:     userID,
		RefreshFor: refreshHash,
		CodeHash:   codeHash,
		ExpiresAt:  now.Add(s.cfg.AccessTokenTTL),
	}); err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

// verifyPKCE checks the code_verifier against the stored code_challenge per
// RFC 7636 §4.6 (S256). The 43–128 length bound is RFC 7636 §4.1.
func verifyPKCE(verifier, challenge string) bool {
	if n := len(verifier); n < 43 || n > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}
