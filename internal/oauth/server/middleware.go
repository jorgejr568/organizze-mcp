package server

import (
	"net/http"
	"strings"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/credprovider"
	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

// Bearer wraps next with the OAuth resource-server check: requires a valid
// access token in Authorization, looks up the user, decrypts the Organizze
// API key, and places the resolved credentials on the request context.
func (s *Server) Bearer(next http.Handler) http.Handler {
	challenge := `Bearer resource_metadata="` + s.cfg.PublicURL + `/.well-known/oauth-protected-resource"`
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("Authorization")
		if !strings.HasPrefix(raw, "Bearer ") {
			w.Header().Set("WWW-Authenticate", challenge)
			http.Error(w, "missing bearer", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(raw, "Bearer ")
		tok, err := s.cfg.Store.GetToken(r.Context(), storage.HashToken(token))
		if err != nil || tok.Kind != "access" || tok.RevokedAt != nil || tok.ExpiresAt.Before(s.cfg.Now()) {
			w.Header().Set("WWW-Authenticate", challenge)
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		user, err := s.cfg.Store.GetUser(r.Context(), tok.UserID)
		if err != nil {
			http.Error(w, "server_error", http.StatusInternalServerError)
			return
		}
		apiKey, err := s.cfg.Cipher.Open(user.APIKeyCipher, user.APIKeyNonce)
		if err != nil {
			http.Error(w, "server_error", http.StatusInternalServerError)
			return
		}
		ctx := credprovider.WithCredentials(r.Context(), credprovider.Credentials{
			Email: user.OrganizzeEmail, APIKey: string(apiKey), UserAgent: user.UserAgent,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
