package organizze

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

// RequestExecutorOptions configures a RequestExecutor.
type RequestExecutorOptions struct {
	HTTPClient HTTPClient // required
	BaseURL    string     // required (no trailing slash)
	Email      string     // required (Basic-Auth username)
	APIKey     string     // required (Basic-Auth password)
	UserAgent  string     // required (Organizze rejects requests without it)

	// LogRequests, when true, emits one stderr line per outgoing request
	// and one per response (truncated to 2KB). The Authorization header
	// is never written to the log. Off by default; trigger via the
	// ORGANIZZE_LOG_REQUESTS=1 env var (read in internal/config).
	LogRequests bool

	// LogWriter receives the verbose-mode output. Defaults to os.Stderr
	// when nil. Tests inject a *bytes.Buffer.
	LogWriter io.Writer
}

// RequestExecutor encapsulates Basic Auth, User-Agent, JSON marshaling,
// base-URL composition, and HTTP-error mapping. Repositories call its methods;
// they never construct an *http.Request.
type RequestExecutor struct {
	client      HTTPClient
	baseURL     string
	email       string
	apiKey      string
	userAgent   string
	logRequests bool
	logWriter   io.Writer
}

// NewRequestExecutor validates options and constructs a RequestExecutor.
func NewRequestExecutor(opts RequestExecutorOptions) (*RequestExecutor, error) {
	switch {
	case opts.HTTPClient == nil:
		return nil, errors.New("organizze: HTTPClient is required")
	case opts.BaseURL == "":
		return nil, errors.New("organizze: BaseURL is required")
	case opts.Email == "":
		return nil, errors.New("organizze: Email is required")
	case opts.APIKey == "":
		return nil, errors.New("organizze: APIKey is required")
	case opts.UserAgent == "":
		return nil, errors.New("organizze: UserAgent is required")
	}
	w := opts.LogWriter
	if w == nil {
		w = os.Stderr
	}
	return &RequestExecutor{
		client:      opts.HTTPClient,
		baseURL:     opts.BaseURL,
		email:       opts.Email,
		apiKey:      opts.APIKey,
		userAgent:   opts.UserAgent,
		logRequests: opts.LogRequests,
		logWriter:   w,
	}, nil
}

// Get performs a GET and decodes the JSON response into out (or discards it if nil).
func (e *RequestExecutor) Get(ctx context.Context, path string, out any) error {
	return e.do(ctx, http.MethodGet, path, nil, out)
}

// Post performs a POST with JSON body and decodes the response into out.
func (e *RequestExecutor) Post(ctx context.Context, path string, body, out any) error {
	return e.do(ctx, http.MethodPost, path, body, out)
}

// Put performs a PUT with JSON body and decodes the response into out.
func (e *RequestExecutor) Put(ctx context.Context, path string, body, out any) error {
	return e.do(ctx, http.MethodPut, path, body, out)
}

// Delete performs a DELETE. If body is non-nil it is JSON-encoded and sent as
// the request body (some Organizze endpoints, e.g. DELETE /categories/:id with
// replacement_id, require this). If out is non-nil the JSON response is decoded
// into it; a 204 response is treated as success and leaves out untouched.
func (e *RequestExecutor) Delete(ctx context.Context, path string, body, out any) error {
	return e.do(ctx, http.MethodDelete, path, body, out)
}

// do is the single point of contact with the HTTP layer.
func (e *RequestExecutor) do(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	var bodyBytes []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("organizze: marshal body: %w", err)
		}
		bodyBytes = b
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, e.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("organizze: build request: %w", err)
	}
	req.SetBasicAuth(e.email, e.apiKey)
	req.Header.Set("User-Agent", e.userAgent)
	req.Header.Set("Accept", "application/json")
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Single-bool fast path: when logging is off, allocate nothing extra.
	if e.logRequests {
		if len(bodyBytes) == 0 {
			fmt.Fprintf(e.logWriter, "organizze: --> %s %s\n", method, path)
		} else {
			fmt.Fprintf(e.logWriter, "organizze: --> %s %s body=%s\n", method, path, bodyBytes)
		}
	}

	resp, err := e.client.Do(req)
	if err != nil {
		if e.logRequests {
			fmt.Fprintf(e.logWriter, "organizze: <-- %s %s error=%v\n", method, path, err)
		}
		return fmt.Errorf("organizze: do request: %w", err)
	}
	defer resp.Body.Close()

	// When logging is enabled, peek up to 2KB of the response body and
	// re-wrap it so downstream decode/error paths see the original stream.
	if e.logRequests {
		const maxLogBytes = 2048
		peek, _ := io.ReadAll(io.LimitReader(resp.Body, maxLogBytes))
		fmt.Fprintf(e.logWriter, "organizze: <-- %s %s status=%d body=%s\n", method, path, resp.StatusCode, peek)
		// Re-attach the peeked bytes + remainder so downstream decoders
		// behave identically to the non-logging path. The original
		// resp.Body still satisfies io.Closer for the deferred close.
		resp.Body = struct {
			io.Reader
			io.Closer
		}{
			Reader: io.MultiReader(bytes.NewReader(peek), resp.Body),
			Closer: resp.Body,
		}
	}

	if resp.StatusCode >= 400 {
		return parseAPIError(resp)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("organizze: decode response: %w", err)
	}
	return nil
}
