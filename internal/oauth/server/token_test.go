package server

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

func issueCode(t *testing.T, fs *fakeStore, clientID string, codeVerifier string) (code string, userID int64) {
	t.Helper()
	user, _ := fs.UpsertUserByEmail(context.Background(), storage.User{
		OrganizzeEmail: "u@x.com", APIKeyCipher: []byte{1}, APIKeyNonce: []byte{2}, UserAgent: "UA",
	})
	sum := sha256.Sum256([]byte(codeVerifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	code = newRandomToken()
	_ = fs.CreateAuthCode(context.Background(), storage.AuthCode{
		CodeHash:            storage.HashToken(code),
		ClientID:            clientID,
		UserID:              user.ID,
		RedirectURI:         "https://chat.example.com/cb",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
		ExpiresAt:           time.Now().Add(5 * time.Minute).UTC(),
	})
	return code, user.ID
}

func postForm(srv *Server, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestToken_AuthorizationCode_Success(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))
	verifier := "verifier-1234567890123456789012345678901234567890"
	code, _ := issueCode(t, fs, c.ID, verifier)

	rec := postForm(srv, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.URIs[0]},
		"client_id":     {c.ID},
		"code_verifier": {verifier},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["access_token"] == "" || got["refresh_token"] == "" {
		t.Errorf("missing tokens: %v", got)
	}
	if got["token_type"] != "Bearer" {
		t.Errorf("token_type = %v", got["token_type"])
	}
}

func TestToken_AuthorizationCode_RejectsWrongVerifier(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))
	code, _ := issueCode(t, fs, c.ID, "right-verifier-that-is-long-enough-123456789")
	rec := postForm(srv, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.URIs[0]},
		"client_id":     {c.ID},
		"code_verifier": {"wrong-verifier-that-is-long-enough-123456789"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestToken_AuthorizationCode_SingleUse(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))
	verifier := "verifier-that-is-long-enough-12345678901234567"
	code, _ := issueCode(t, fs, c.ID, verifier)
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.URIs[0]},
		"client_id":     {c.ID},
		"code_verifier": {verifier},
	}
	if rec := postForm(srv, form); rec.Code != http.StatusOK {
		t.Fatalf("first = %d", rec.Code)
	}
	if rec := postForm(srv, form); rec.Code != http.StatusBadRequest {
		t.Errorf("second use status = %d", rec.Code)
	}
}

func TestToken_RefreshGrant_Rotates(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))
	verifier := "verifier-that-is-long-enough-12345678901234567"
	code, _ := issueCode(t, fs, c.ID, verifier)
	rec := postForm(srv, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.URIs[0]},
		"client_id":     {c.ID},
		"code_verifier": {verifier},
	})
	var first map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &first)
	oldRefresh := first["refresh_token"].(string)

	rec = postForm(srv, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {oldRefresh},
		"client_id":     {c.ID},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh status = %d body=%s", rec.Code, rec.Body.String())
	}
	var second map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &second)
	if second["refresh_token"] == oldRefresh {
		t.Error("refresh_token did not rotate")
	}

	rec = postForm(srv, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {oldRefresh},
		"client_id":     {c.ID},
	})
	if rec.Code == http.StatusOK {
		t.Error("old refresh should be revoked after rotation")
	}
}

func TestToken_RefreshGrant_ConcurrentRotation_OnlyOneSucceeds(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))
	verifier := "verifier-that-is-long-enough-12345678901234567"
	code, _ := issueCode(t, fs, c.ID, verifier)
	rec := postForm(srv, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.URIs[0]},
		"client_id":     {c.ID},
		"code_verifier": {verifier},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("seed exchange status = %d", rec.Code)
	}
	var pair map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &pair)
	refresh := pair["refresh_token"].(string)

	var success, failure atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			r := postForm(srv, url.Values{
				"grant_type":    {"refresh_token"},
				"refresh_token": {refresh},
				"client_id":     {c.ID},
			})
			if r.Code == http.StatusOK {
				success.Add(1)
			} else {
				failure.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if success.Load() != 1 {
		t.Fatalf("expected exactly 1 success, got %d (failures: %d)", success.Load(), failure.Load())
	}
}

func TestToken_AuthorizationCode_Replay_RevokesFamily(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))
	verifier := "verifier-that-is-long-enough-12345678901234567"
	code, _ := issueCode(t, fs, c.ID, verifier)
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.URIs[0]},
		"client_id":     {c.ID},
		"code_verifier": {verifier},
	}
	rec := postForm(srv, form)
	if rec.Code != http.StatusOK {
		t.Fatalf("first exchange status = %d", rec.Code)
	}
	var pair map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &pair)
	accessHash := storage.HashToken(pair["access_token"].(string))
	refreshHash := storage.HashToken(pair["refresh_token"].(string))

	// Replay: must fail AND must revoke both tokens issued from the code.
	rec2 := postForm(srv, form)
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("replay status = %d, want 400", rec2.Code)
	}
	access, _ := fs.GetToken(context.Background(), accessHash)
	if access.RevokedAt == nil {
		t.Errorf("access token not revoked after code replay")
	}
	refresh, _ := fs.GetToken(context.Background(), refreshHash)
	if refresh.RevokedAt == nil {
		t.Errorf("refresh token not revoked after code replay")
	}
}

func TestVerifyPKCE_RejectsOutOfRangeVerifier(t *testing.T) {
	short := strings.Repeat("a", 42)
	okLen := strings.Repeat("a", 43)
	long := strings.Repeat("a", 129)
	sum := sha256.Sum256([]byte(okLen))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	if verifyPKCE(short, challenge) {
		t.Error("expected reject on 42-char verifier")
	}
	if verifyPKCE(long, challenge) {
		t.Error("expected reject on 129-char verifier")
	}
	if !verifyPKCE(okLen, challenge) {
		t.Error("expected accept on 43-char verifier")
	}
}

func TestToken_AuthorizationCode_ConcurrentConsume_OnlyOneSucceeds(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))
	verifier := "verifier-that-is-long-enough-12345678901234567"
	code, _ := issueCode(t, fs, c.ID, verifier)
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.URIs[0]},
		"client_id":     {c.ID},
		"code_verifier": {verifier},
	}
	var success, failure atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if postForm(srv, form).Code == http.StatusOK {
				success.Add(1)
			} else {
				failure.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if success.Load() != 1 {
		t.Fatalf("expected exactly 1 success, got %d (failures: %d)", success.Load(), failure.Load())
	}
}

func TestToken_RefreshGrant_ExpiredReturnsInvalidGrantNoFamilyRevoke(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))

	// Seed an expired refresh token + an unrelated live refresh in the same family.
	otherRefreshHash := storage.HashToken("other-refresh")
	_ = fs.CreateToken(context.Background(), storage.Token{
		TokenHash: otherRefreshHash, Kind: "refresh", ClientID: c.ID, UserID: 1,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	expiredHash := storage.HashToken("expired-refresh")
	_ = fs.CreateToken(context.Background(), storage.Token{
		TokenHash: expiredHash, Kind: "refresh", ClientID: c.ID, UserID: 1,
		ExpiresAt: time.Now().Add(-time.Hour),
	})

	rec := postForm(srv, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {"expired-refresh"},
		"client_id":     {c.ID},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	// The unrelated live refresh must still be usable — expiry MUST NOT
	// trigger family revocation, only genuine reuse-of-revoked does.
	other, _ := fs.GetToken(context.Background(), otherRefreshHash)
	if other.RevokedAt != nil {
		t.Error("unrelated live refresh was revoked when expired refresh was replayed")
	}
}

func TestToken_RefreshGrant_GarbageDoesNotKillFamily(t *testing.T) {
	srv, fs := newServerWithFakeStore(t)
	srv.cfg.Cipher = mustTestCipher(t)
	c := seedClientRecord()
	_ = fs.CreateClient(context.Background(), storageClient(c))
	verifier := "verifier-that-is-long-enough-12345678901234567"
	code, _ := issueCode(t, fs, c.ID, verifier)

	// Mint a valid token pair.
	rec := postForm(srv, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.URIs[0]},
		"client_id":     {c.ID},
		"code_verifier": {verifier},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("initial exchange status = %d", rec.Code)
	}
	var pair map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &pair)
	goodRefresh := pair["refresh_token"].(string)

	// Replay a garbage refresh token. Must NOT kill the legitimate family.
	rec = postForm(srv, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {"never-issued-by-this-server"},
		"client_id":     {c.ID},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("garbage refresh status = %d", rec.Code)
	}

	// The legitimate refresh should still rotate successfully.
	rec = postForm(srv, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {goodRefresh},
		"client_id":     {c.ID},
	})
	if rec.Code != http.StatusOK {
		t.Errorf("legitimate refresh after garbage replay status = %d body=%s", rec.Code, rec.Body.String())
	}
}
