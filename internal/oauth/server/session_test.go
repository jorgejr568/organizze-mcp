package server

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestSession_RoundTrip(t *testing.T) {
	sm := newSessionManager([]byte("k"), time.Hour)
	w := httptest.NewRecorder()
	sm.write(w, "sess-1")
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName {
		t.Fatalf("cookies = %+v", cookies)
	}
	if cookies[0].MaxAge != int(time.Hour.Seconds()) {
		t.Errorf("MaxAge = %d, want %d", cookies[0].MaxAge, int(time.Hour.Seconds()))
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(cookies[0])
	got, ok := sm.read(r)
	if !ok || got != "sess-1" {
		t.Errorf("read = %q, ok=%v", got, ok)
	}
}

func TestSession_TamperedFails(t *testing.T) {
	sm := newSessionManager([]byte("k"), time.Hour)
	w := httptest.NewRecorder()
	sm.write(w, "sess-1")
	cookies := w.Result().Cookies()

	cookies[0].Value = cookies[0].Value[:len(cookies[0].Value)-2] + "xx"
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(cookies[0])
	if _, ok := sm.read(r); ok {
		t.Error("expected tampered cookie to be rejected")
	}
}
