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
	if err := s.cfg.Store.CreateClient(r.Context(), storage.Client{
		ID:           id,
		ClientName:   req.ClientName,
		RedirectURIs: req.RedirectURIs,
	}); err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "")
		return
	}
	writeJSON(w, http.StatusCreated, registerResponse{
		ClientID:                id,
		ClientName:              req.ClientName,
		RedirectURIs:            req.RedirectURIs,
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "none",
	})
}

// newPublicID returns a 192-bit URL-safe random identifier for public OAuth clients.
func newPublicID() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
