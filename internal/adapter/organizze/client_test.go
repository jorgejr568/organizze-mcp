package organizze

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_DoForwardsToInner(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	}))
	defer ts.Close()

	c := NewClient(ClientOptions{Timeout: 5 * time.Second})
	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()
	if !called {
		t.Error("inner client not invoked")
	}
	if resp.StatusCode != http.StatusTeapot {
		t.Errorf("StatusCode = %d", resp.StatusCode)
	}
}

func TestClient_SatisfiesHTTPClient(t *testing.T) {
	// Compile-time guarantee: *Client implements HTTPClient.
	var _ HTTPClient = NewClient(ClientOptions{})
}

func TestNewClient_AppliesDefaultTimeout(t *testing.T) {
	c := NewClient(ClientOptions{})
	if c.Inner().Timeout == 0 {
		t.Error("default timeout should be non-zero")
	}
}
