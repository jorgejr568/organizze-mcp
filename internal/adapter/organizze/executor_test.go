package organizze

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
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
