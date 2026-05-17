package organizze

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/credprovider"
)

// newTestExecutor spins up an httptest.Server backed by handler and returns
// a fully-wired RequestExecutor pointing at it.
func newTestExecutor(t *testing.T, handler http.HandlerFunc) (*RequestExecutor, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	exec, err := NewRequestExecutor(RequestExecutorOptions{
		HTTPClient:  NewClient(ClientOptions{}),
		BaseURL:     ts.URL,
		Credentials: credprovider.Static("test@example.com", "test-key", "Test (test@example.com)"),
	})
	if err != nil {
		t.Fatalf("NewRequestExecutor: %v", err)
	}
	return exec, ts
}
