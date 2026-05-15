// Command organizze-mcp is the composition root for the Organizze MCP server.
// It wires every layer (domain → usecase → adapter/organizze → adapter/mcp) and
// dispatches to the requested MCP transport.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/adapter/mcp"
	"github.com/jorgejr568/organizze-mcp/internal/adapter/organizze"
	"github.com/jorgejr568/organizze-mcp/internal/config"
	"github.com/jorgejr568/organizze-mcp/internal/usecase"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "organizze-mcp:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cfg.Transport {
	case "stdio":
		return runWithTransport(ctx, cfg, &mcpsdk.StdioTransport{}, "stdio")
	case "http":
		return runHTTP(ctx, cfg)
	default:
		return fmt.Errorf("unsupported transport %q", cfg.Transport)
	}
}

// buildServer is the dependency-injection graph. It is the ONLY place that
// imports both adapter/organizze and adapter/mcp concretes.
func buildServer(cfg *config.Config) (*mcpsdk.Server, error) {
	httpClient := organizze.NewClient(organizze.ClientOptions{Timeout: cfg.HTTPTimeout})

	exec, err := organizze.NewRequestExecutor(organizze.RequestExecutorOptions{
		HTTPClient:  httpClient,
		BaseURL:     cfg.BaseURL,
		Email:       cfg.Email,
		APIKey:      cfg.APIKey,
		UserAgent:   cfg.UserAgent,
		LogRequests: cfg.LogRequests,
		// LogWriter intentionally left zero - executor defaults to os.Stderr.
	})
	if err != nil {
		return nil, fmt.Errorf("build request executor: %w", err)
	}

	deps := mcp.Dependencies{
		User:        usecase.NewUserService(organizze.NewUserRepository(exec)),
		Account:     usecase.NewAccountService(organizze.NewAccountRepository(exec)),
		Category:    usecase.NewCategoryService(organizze.NewCategoryRepository(exec)),
		Budget:      usecase.NewBudgetService(organizze.NewBudgetRepository(exec)),
		CreditCard:  usecase.NewCreditCardService(organizze.NewCreditCardRepository(exec)),
		Invoice:     usecase.NewInvoiceService(organizze.NewInvoiceRepository(exec)),
		Transfer:    usecase.NewTransferService(organizze.NewTransferRepository(exec)),
		Transaction: usecase.NewTransactionService(organizze.NewTransactionRepository(exec)),
	}
	return mcp.New(deps), nil
}

func runWithTransport(ctx context.Context, cfg *config.Config, t mcpsdk.Transport, name string) error {
	s, err := buildServer(cfg)
	if err != nil {
		return err
	}
	log.SetOutput(os.Stderr) // stdout is reserved for MCP protocol on stdio
	log.Printf("organizze-mcp v%s starting on %s", mcp.Version, name)
	return s.Run(ctx, t)
}

func runHTTP(ctx context.Context, cfg *config.Config) error {
	s, err := buildServer(cfg)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpsdk.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcpsdk.Server { return s },
		nil,
	))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("organizze-mcp v%s listening on %s", mcp.Version, cfg.HTTPAddr)
		err := srv.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("organizze-mcp: shutdown: %v", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}
