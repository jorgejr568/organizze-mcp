// Package organizze is the HTTP/REST adapter for the Organizze API.
//
// Layering:
//   - HTTPClient is the smallest abstraction repositories depend on for HTTP
//     transport. Tests substitute fakes via this interface.
//   - Client is the default HTTPClient implementation — a thin wrapper around
//     stdlib *http.Client. It exists so cross-cutting concerns (timeout today;
//     retries/logging tomorrow) live in one place.
//   - RequestExecutor (see executor.go) sits above HTTPClient and owns auth,
//     User-Agent, JSON marshaling, and error mapping. Repositories never see
//     *http.Request, *http.Response, or io.Reader.
package organizze

import (
	"net/http"
	"time"
)

const defaultTimeout = 30 * time.Second

// HTTPClient is the abstraction every repository depends on for HTTP transport.
// A single method keeps it ISP-minimal: any net/http-shaped client (or fake)
// satisfies it.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// ClientOptions configures the default HTTPClient implementation.
type ClientOptions struct {
	// Timeout is the per-request deadline. If zero, 30s is used.
	Timeout time.Duration
}

// Client is the default HTTPClient: a thin wrapper around stdlib *http.Client.
// Exported so callers can pass it where *http.Client itself would go, and so
// the constructor can return a concrete *Client per the "return structs" idiom.
type Client struct {
	inner *http.Client
}

// NewClient builds a default Client with the given options. The returned value
// satisfies HTTPClient.
func NewClient(opts ClientOptions) *Client {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	return &Client{inner: &http.Client{Timeout: timeout}}
}

// Do implements HTTPClient.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.inner.Do(req)
}

// Timeout returns the per-request deadline this Client was configured with.
// Callers that need custom transports (proxies, TLS, retries) should construct
// their own *http.Client and pass it as the HTTPClient argument to
// NewRequestExecutor — Client is only the default-settings convenience.
func (c *Client) Timeout() time.Duration {
	return c.inner.Timeout
}

// Compile-time check.
var _ HTTPClient = (*Client)(nil)
