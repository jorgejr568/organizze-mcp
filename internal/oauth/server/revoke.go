package server

import (
	"net/http"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	token := r.PostForm.Get("token")
	if token == "" {
		w.WriteHeader(http.StatusOK)
		return
	}
	hash := storage.HashToken(token)
	// Spec: respond 200 even if the token is unknown. Internally,
	// always revoke the whole refresh family to be safe.
	_ = s.cfg.Store.RevokeRefreshFamily(r.Context(), hash)
	_ = s.cfg.Store.RevokeToken(r.Context(), hash)
	w.WriteHeader(http.StatusOK)
}
