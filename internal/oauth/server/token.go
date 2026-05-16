package server

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
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
	code := r.PostForm.Get("code")
	clientID := r.PostForm.Get("client_id")
	redirectURI := r.PostForm.Get("redirect_uri")
	verifier := r.PostForm.Get("code_verifier")
	if code == "" || clientID == "" || verifier == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "missing required field")
		return
	}
	ac, err := s.cfg.Store.ConsumeAuthCode(ctx, storage.HashToken(code))
	if err != nil {
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
	access, refresh, err := s.issueTokenPair(ctx, clientID, ac.UserID)
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
	rt := r.PostForm.Get("refresh_token")
	clientID := r.PostForm.Get("client_id")
	if rt == "" || clientID == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "")
		return
	}
	hash := storage.HashToken(rt)
	tok, err := s.cfg.Store.GetToken(ctx, hash)
	if err != nil || tok.Kind != "refresh" || tok.ExpiresAt.Before(s.cfg.Now()) {
		// Unknown / wrong-kind / expired: flat invalid_grant, NO family revoke.
		// (A garbage or expired token replay must not let an attacker kill a
		// legitimate user's session.)
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "")
		return
	}
	if tok.RevokedAt != nil {
		// Genuine reuse signal: this refresh token was valid at some point
		// but has been rotated/revoked. RFC 6749 §10.4 / OAuth security BCP
		// recommends revoking the whole descendant family in this case to
		// invalidate whatever the attacker may be holding.
		_ = s.cfg.Store.RevokeRefreshFamily(ctx, hash)
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "")
		return
	}
	if tok.ClientID != clientID {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "client_id mismatch")
		return
	}
	if err := s.cfg.Store.RevokeRefreshFamily(ctx, hash); err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "")
		return
	}
	access, refresh, err := s.issueTokenPair(ctx, clientID, tok.UserID)
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

func (s *Server) issueTokenPair(ctx context.Context, clientID string, userID int64) (string, string, error) {
	access := newRandomToken()
	refresh := newRandomToken()
	now := s.cfg.Now()
	refreshHash := storage.HashToken(refresh)
	if err := s.cfg.Store.CreateToken(ctx, storage.Token{
		TokenHash: refreshHash,
		Kind:      "refresh",
		ClientID:  clientID,
		UserID:    userID,
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
		ExpiresAt:  now.Add(s.cfg.AccessTokenTTL),
	}); err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

func verifyPKCE(verifier, challenge string) bool {
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}
