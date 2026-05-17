package server

import (
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/credprovider"
	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

// Bearer wraps next with the OAuth resource-server check: requires a valid
// access token in Authorization, looks up the user, decrypts the Organizze
// API key, and places the resolved credentials on the request context.
// Each request emits a structured zap record on accept (info) or reject
// (warn) so operators can correlate tool calls back to a user without
// scraping logs from multiple sources.
func (s *Server) Bearer(next http.Handler) http.Handler {
	challenge := `Bearer resource_metadata="` + s.cfg.PublicURL + `/.well-known/oauth-protected-resource"`
	log := s.cfg.Logger.Named("bearer")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("Authorization")
		// RFC 6750 §2.1 — the scheme is case-insensitive.
		const schemeLen = len("bearer ")
		if len(raw) <= schemeLen || !strings.EqualFold(raw[:schemeLen], "Bearer ") {
			log.Warn("bearer rejected",
				zap.String("reason", "missing_or_malformed"),
				zap.String("path", r.URL.Path),
			)
			w.Header().Set("WWW-Authenticate", challenge)
			http.Error(w, "missing bearer", http.StatusUnauthorized)
			return
		}
		token := raw[schemeLen:]
		tok, err := s.cfg.Store.GetToken(r.Context(), storage.HashToken(token))
		if err != nil || tok.Kind != "access" || tok.RevokedAt != nil || tok.ExpiresAt.Before(s.cfg.Now()) {
			// Reason vocabulary is fixed (no free-text) so a leaked log
			// never contains tokens or user-controlled strings.
			reason := "invalid"
			switch {
			case err != nil:
				reason = "unknown"
			case tok.Kind != "access":
				reason = "wrong_kind"
			case tok.RevokedAt != nil:
				reason = "revoked"
			case tok.ExpiresAt.Before(s.cfg.Now()):
				reason = "expired"
			}
			log.Warn("bearer rejected",
				zap.String("reason", reason),
				zap.String("path", r.URL.Path),
			)
			w.Header().Set("WWW-Authenticate", challenge)
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		user, err := s.cfg.Store.GetUser(r.Context(), tok.UserID)
		if err != nil {
			log.Error("bearer user lookup failed",
				zap.Int64("user_id", tok.UserID),
				zap.Error(err),
			)
			http.Error(w, "server_error", http.StatusInternalServerError)
			return
		}
		apiKey, err := s.cfg.Cipher.Open(user.APIKeyCipher, user.APIKeyNonce)
		if err != nil {
			log.Error("bearer cipher open failed",
				zap.Int64("user_id", user.ID),
				zap.Error(err),
			)
			http.Error(w, "server_error", http.StatusInternalServerError)
			return
		}
		log.Info("bearer accepted",
			zap.Int64("user_id", user.ID),
			zap.String("client_id", tok.ClientID),
			zap.String("path", r.URL.Path),
		)
		ctx := credprovider.WithCredentials(r.Context(), credprovider.Credentials{
			Email: user.OrganizzeEmail, APIKey: string(apiKey), UserAgent: user.UserAgent,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
