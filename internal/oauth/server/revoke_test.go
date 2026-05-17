package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

func TestRevoke_RemovesRefreshFamily(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)

	refreshHash := storage.HashToken("rt")
	accessHash := storage.HashToken("at")
	_ = fs.CreateToken(context.Background(), storage.Token{TokenHash: refreshHash, Kind: "refresh", ExpiresAt: time.Now().Add(time.Hour)})
	_ = fs.CreateToken(context.Background(), storage.Token{TokenHash: accessHash, Kind: "access", RefreshFor: refreshHash, ExpiresAt: time.Now().Add(time.Hour)})

	body := url.Values{"token": {"rt"}}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	r, _ := fs.GetToken(context.Background(), refreshHash)
	a, _ := fs.GetToken(context.Background(), accessHash)
	if r.RevokedAt == nil || a.RevokedAt == nil {
		t.Error("expected both tokens revoked")
	}
}
