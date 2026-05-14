package main

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

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
	s, err := buildServer(cfg)
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
	go func() { done <- runWithTransport(ctx, cfg, serverT, "test") }()

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

func TestRunWithTransport_RedirectsLogToStderr(t *testing.T) {
	cfg := &config.Config{
		APIKey: "k", Email: "e@x.com", UserAgent: "Test (e@x.com)",
		BaseURL: "http://127.0.0.1:1", HTTPTimeout: 5 * time.Second,
		Transport: "stdio", HTTPAddr: ":0",
	}
	serverT, _ := mcpsdk.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- runWithTransport(ctx, cfg, serverT, "test") }()

	// Give the goroutine a moment to call log.SetOutput.
	deadline := time.Now().Add(500 * time.Millisecond)
	var writer io.Writer
	for time.Now().Before(deadline) {
		writer = log.Writer()
		if writer == os.Stderr {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if writer != os.Stderr {
		t.Errorf("log.Writer() = %T, want os.Stderr", writer)
	}

	cancel()
	<-done
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
	go func() { done <- runHTTP(ctx, cfg) }()

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
