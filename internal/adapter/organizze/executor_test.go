package organizze

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

func TestNewRequestExecutor_RejectsMissingRequired(t *testing.T) {
	c := NewClient(ClientOptions{})
	cases := []RequestExecutorOptions{
		{HTTPClient: c, Email: "", APIKey: "k", UserAgent: "ua", BaseURL: "https://x"},
		{HTTPClient: c, Email: "e", APIKey: "", UserAgent: "ua", BaseURL: "https://x"},
		{HTTPClient: c, Email: "e", APIKey: "k", UserAgent: "", BaseURL: "https://x"},
		{HTTPClient: c, Email: "e", APIKey: "k", UserAgent: "ua", BaseURL: ""},
		{HTTPClient: nil, Email: "e", APIKey: "k", UserAgent: "ua", BaseURL: "https://x"},
	}
	for i, opt := range cases {
		if _, err := NewRequestExecutor(opt); err == nil {
			t.Errorf("case %d: expected error for %+v", i, opt)
		}
	}
}

func TestExecutor_GET_SetsAuthAndUserAgent(t *testing.T) {
	var gotAuth, gotUA, gotPath string
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, `{"ok":true}`)
	})

	var out struct {
		OK bool `json:"ok"`
	}
	if err := exec.Get(context.Background(), "/users/3", &out); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !strings.HasPrefix(gotAuth, "Basic ") {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotUA != "Test (test@example.com)" {
		t.Errorf("User-Agent = %q", gotUA)
	}
	if gotPath != "/users/3" {
		t.Errorf("path = %q", gotPath)
	}
	if !out.OK {
		t.Errorf("body decode failed: %+v", out)
	}
}

func TestExecutor_POST_RoundTripsBody(t *testing.T) {
	type body struct {
		Hello string `json:"hello"`
	}
	var received body
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"hello":"world"}`)
	})

	var out body
	if err := exec.Post(context.Background(), "/echo", body{Hello: "world"}, &out); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if received.Hello != "world" || out.Hello != "world" {
		t.Errorf("roundtrip failed: received=%+v out=%+v", received, out)
	}
}

func TestExecutor_DELETE_HandlesNoContent(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := exec.Delete(context.Background(), "/x/1", nil, nil); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestRequestExecutor_Delete_WithBody_SendsJSONBodyAndDecodesResponse(t *testing.T) {
	var gotMethod, gotPath, gotCT string
	var gotBody []byte
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":42,"deleted":true}`)
	})

	body := map[string]any{"replacement_id": 18}
	var out struct {
		ID      int64 `json:"id"`
		Deleted bool  `json:"deleted"`
	}
	if err := exec.Delete(context.Background(), "/categories/6", body, &out); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/categories/6" {
		t.Errorf("path = %q", gotPath)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotCT)
	}
	if string(gotBody) != `{"replacement_id":18}` {
		t.Errorf("body = %q, want {\"replacement_id\":18}", string(gotBody))
	}
	if out.ID != 42 || !out.Deleted {
		t.Errorf("decoded = %+v", out)
	}
}

func TestRequestExecutor_Delete_NilBody_OmitsContentType(t *testing.T) {
	var gotCT string
	var gotBody []byte
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	})

	if err := exec.Delete(context.Background(), "/accounts/1", nil, nil); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if gotCT != "" {
		t.Errorf("Content-Type = %q on body-less DELETE, want empty", gotCT)
	}
	if len(gotBody) != 0 {
		t.Errorf("body = %q on body-less DELETE, want empty", string(gotBody))
	}
}

func TestExecutor_PUT_RoundTripsBody(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q", r.Method)
		}
		_, _ = io.WriteString(w, `{"x":1}`)
	})
	var out struct {
		X int `json:"x"`
	}
	if err := exec.Put(context.Background(), "/x/1", map[string]any{"y": 2}, &out); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if out.X != 1 {
		t.Errorf("out.X = %d", out.X)
	}
}

func TestExecutor_4xx_ReturnsTypedAPIError(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"missing"}`)
	})

	err := exec.Get(context.Background(), "/x/99", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err is %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d", apiErr.StatusCode)
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err should match domain.ErrNotFound")
	}
}

func TestExecutor_PropagatesContextCancel(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := exec.Get(ctx, "/slow", nil); err == nil {
		t.Fatal("expected error from cancelled ctx")
	}
}

func TestExecutor_LoggingDisabled_WritesNothing(t *testing.T) {
	var buf bytes.Buffer
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(ts.Close)

	exec, err := NewRequestExecutor(RequestExecutorOptions{
		HTTPClient:  NewClient(ClientOptions{}),
		BaseURL:     ts.URL,
		Email:       "test@example.com",
		APIKey:      "test-key",
		UserAgent:   "Test (test@example.com)",
		LogRequests: false,
		LogWriter:   &buf,
	})
	if err != nil {
		t.Fatalf("NewRequestExecutor: %v", err)
	}

	var out struct {
		OK bool `json:"ok"`
	}
	if err := exec.Get(context.Background(), "/users/3", &out); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("LogWriter received %d bytes with LogRequests=false: %q", buf.Len(), buf.String())
	}
}

func TestExecutor_LoggingEnabled_CapturesMethodPathBody_RedactsAuth(t *testing.T) {
	var buf bytes.Buffer
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":99}`)
	}))
	t.Cleanup(ts.Close)

	exec, err := NewRequestExecutor(RequestExecutorOptions{
		HTTPClient:  NewClient(ClientOptions{}),
		BaseURL:     ts.URL,
		Email:       "test@example.com",
		APIKey:      "super-secret-key",
		UserAgent:   "Test (test@example.com)",
		LogRequests: true,
		LogWriter:   &buf,
	})
	if err != nil {
		t.Fatalf("NewRequestExecutor: %v", err)
	}

	body := map[string]any{"description": "Coffee", "amount_cents": -1500}
	var out struct {
		ID int64 `json:"id"`
	}
	if err := exec.Post(context.Background(), "/transactions", body, &out); err != nil {
		t.Fatalf("Post: %v", err)
	}

	logged := buf.String()
	if logged == "" {
		t.Fatal("LogWriter received nothing with LogRequests=true")
	}

	// Request-line assertions: method, path, body fields all present.
	for _, want := range []string{"POST", "/transactions", `"description":"Coffee"`, `"amount_cents":-1500`} {
		if !strings.Contains(logged, want) {
			t.Errorf("log missing %q; full output:\n%s", want, logged)
		}
	}

	// Response-line assertions: status and response body fragment.
	for _, want := range []string{"201", `"id":99`} {
		if !strings.Contains(logged, want) {
			t.Errorf("log missing %q; full output:\n%s", want, logged)
		}
	}

	// Redaction assertions: Authorization header value and API key MUST NOT appear.
	for _, banned := range []string{"Basic ", "super-secret-key", "Authorization"} {
		if strings.Contains(logged, banned) {
			t.Errorf("log leaked %q; full output:\n%s", banned, logged)
		}
	}
}
