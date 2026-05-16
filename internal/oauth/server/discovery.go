package server

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleProtectedResource(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]any{
		"resource":                 s.cfg.PublicURL + "/mcp",
		"authorization_servers":    []string{s.cfg.PublicURL},
		"bearer_methods_supported": []string{"header"},
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAuthorizationServer(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]any{
		"issuer":                                s.cfg.PublicURL,
		"authorization_endpoint":                s.cfg.PublicURL + "/oauth/authorize",
		"token_endpoint":                        s.cfg.PublicURL + "/oauth/token",
		"registration_endpoint":                 s.cfg.PublicURL + "/oauth/register",
		"revocation_endpoint":                   s.cfg.PublicURL + "/oauth/revoke",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
