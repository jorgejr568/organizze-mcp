package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

func writeOAuthError(w http.ResponseWriter, status int, code, description string) {
	body := map[string]string{"error": code}
	if description != "" {
		body["error_description"] = description
	}
	writeJSON(w, status, body)
}

type registerRequest struct {
	ClientName   string   `json:"client_name"`
	RedirectURIs []string `json:"redirect_uris"`
}

type registerResponse struct {
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
	ClientSecretExpiresAt   int64    `json:"client_secret_expires_at"` // 0 = never expires (RFC 7591 §3.2.1)
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOAuthError(w, http.StatusMethodNotAllowed, "invalid_request", "POST required")
		return
	}
	if !s.dcrLimiter.allow(clientIP(r)) {
		writeOAuthError(w, http.StatusTooManyRequests, "rate_limited", "too many client registrations from this IP")
		return
	}
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "malformed JSON")
		return
	}
	if len(req.RedirectURIs) == 0 {
		writeOAuthError(w, http.StatusBadRequest, "invalid_redirect_uri", "at least one redirect_uri required")
		return
	}
	for _, u := range req.RedirectURIs {
		if !strings.HasPrefix(u, "https://") {
			writeOAuthError(w, http.StatusBadRequest, "invalid_redirect_uri", "must be https")
			return
		}
	}
	id := newPublicID()
	secret := newClientSecret()
	if err := s.cfg.Store.CreateClient(r.Context(), storage.Client{
		ID:               id,
		ClientSecretHash: storage.HashToken(secret),
		ClientName:       req.ClientName,
		RedirectURIs:     req.RedirectURIs,
	}); err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "")
		return
	}
	writeJSON(w, http.StatusCreated, registerResponse{
		ClientID:                id,
		ClientSecret:            secret,
		ClientIDIssuedAt:        s.cfg.Now().Unix(),
		ClientSecretExpiresAt:   0,
		ClientName:              req.ClientName,
		RedirectURIs:            req.RedirectURIs,
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		// Perplexity's MCP DCR client uses the Python SDK, whose
		// OAuthClientMetadata pydantic schema limits this field to a
		// Literal["none", "client_secret_post"] — "client_secret_basic"
		// triggers CLIENT_REGISTRATION_FAILED / "Input should be 'none'
		// or 'client_secret_post'". The token endpoint accepts both
		// basic and post regardless of what's advertised here.
		TokenEndpointAuthMethod: "client_secret_post",
	})
}

// newPublicID returns a 192-bit URL-safe random identifier for OAuth clients.
func newPublicID() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// newClientSecret returns a 256-bit URL-safe random secret suitable for
// client_secret_basic. The hash (sha256) is what we persist; the plaintext
// is shown to the registering caller exactly once.
func newClientSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
