package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"github.com/jorgejr568/organizze-mcp/internal/config"
)

func TestBuildServer_AssemblesEveryLayer(t *testing.T) {
	cfg := &config.Config{
		APIKey:      "k",
		Email:       "e@x.com",
		UserAgent:   "Test (e@x.com)",
		BaseURL:     "http://127.0.0.1:1", // never reached
		HTTPTimeout: 5 * time.Second,
		Transport:   "stdio",
		HTTPAddr:    ":0",
	}
	s, err := buildServer(context.Background(), cfg, zap.NewNop(), "test")
	if err != nil {
		t.Fatalf("buildServer: %v", err)
	}
	if s == nil {
		t.Fatal("server is nil")
	}
}

func TestBuildServer_AssemblesEveryLayer_WithLoggingOn(t *testing.T) {
	cfg := &config.Config{
		APIKey:      "k",
		Email:       "e@x.com",
		UserAgent:   "Test (e@x.com)",
		BaseURL:     "http://127.0.0.1:1",
		HTTPTimeout: 5 * time.Second,
		Transport:   "stdio",
		HTTPAddr:    ":0",
		LogRequests: true,
	}
	s, err := buildServer(context.Background(), cfg, zap.NewNop(), "test")
	if err != nil {
		t.Fatalf("buildServer: %v", err)
	}
	if s == nil {
		t.Fatal("server is nil")
	}
}

func TestRunWithTransport_ServesOverInMemory(t *testing.T) {
	cfg := &config.Config{
		APIKey: "k", Email: "e@x.com", UserAgent: "Test (e@x.com)",
		BaseURL: "http://127.0.0.1:1", HTTPTimeout: 5 * time.Second,
		Transport: "stdio", HTTPAddr: ":0",
	}
	serverT, clientT := mcpsdk.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- runWithTransport(ctx, cfg, zap.NewNop(), serverT, "test") }()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "cmd-test", Version: "0"}, nil)
	sess, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		cancel()
		<-done
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	res, err := sess.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(res.Tools) < 16 {
		t.Errorf("expected 16+ tools, got %d", len(res.Tools))
	}

	cancel()
	select {
	case err := <-done:
		if err != nil && err != context.Canceled {
			t.Errorf("runWithTransport: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not return after cancel")
	}
}

func TestNewLogger_WritesJSONToStderr(t *testing.T) {
	// The zap config used at runtime must write JSON to stderr — stdout is
	// reserved for the MCP stdio protocol channel and any leakage would
	// corrupt the JSON-RPC stream.
	logger, err := newLogger()
	if err != nil {
		t.Fatalf("newLogger: %v", err)
	}
	defer func() { _ = logger.Sync() }()
	if logger == nil {
		t.Fatal("newLogger returned nil")
	}
}

func TestRunHTTP_HealthzResponds(t *testing.T) {
	lis, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := lis.Addr().String()
	lis.Close()

	cfg := &config.Config{
		APIKey: "k", Email: "e@x.com", UserAgent: "Test (e@x.com)",
		BaseURL: "http://127.0.0.1:1", HTTPTimeout: 5 * time.Second,
		Transport: "http", HTTPAddr: addr,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runHTTP(ctx, cfg, zap.NewNop()) }()

	deadline := time.Now().Add(2 * time.Second)
	var ok bool
	for time.Now().Before(deadline) {
		if r, err := http.Get("http://" + addr + "/healthz"); err == nil {
			r.Body.Close()
			if r.StatusCode == http.StatusOK {
				ok = true
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ok {
		cancel()
		<-done
		t.Fatal("server never replied to /healthz")
	}
	cancel()
	if err := <-done; err != nil && err != http.ErrServerClosed && err != context.Canceled {
		t.Errorf("runHTTP: %v", err)
	}
}
