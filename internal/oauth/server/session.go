package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

const sessionCookieName = "organizze_oauth_session"

// sessionManager signs an opaque session ID into a cookie. The ID points at
// an oauth_sessions row; expiry is enforced server-side via that row's
// expires_at column, not by the HMAC payload — the cookie itself carries
// no timestamp, so a leaked cookie is replayable until the DB row expires
// or is deleted.
type sessionManager struct {
	secret []byte
	ttl    time.Duration
}

func newSessionManager(secret []byte, ttl time.Duration) *sessionManager {
	return &sessionManager{secret: secret, ttl: ttl}
}

// write sets a signed cookie carrying sessionID.
func (m *sessionManager) write(w http.ResponseWriter, sessionID string) {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(sessionID))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	value := sessionID + "." + sig
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(m.ttl.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// read returns the sessionID after verifying the HMAC.
func (m *sessionManager) read(r *http.Request) (string, bool) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}
	parts := strings.SplitN(c.Value, ".", 2)
	if len(parts) != 2 {
		return "", false
	}
	sessionID, sig := parts[0], parts[1]
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(sessionID))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return "", false
	}
	return sessionID, true
}

// clear deletes the cookie at the client.
func (m *sessionManager) clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/",
		MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})
}
