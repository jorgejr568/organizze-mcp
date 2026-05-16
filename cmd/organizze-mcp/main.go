// Command organizze-mcp is the composition root for the Organizze MCP server.
// It wires every layer (domain → usecase → adapter/organizze → adapter/mcp) and
// dispatches to the requested MCP transport.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/jorgejr568/organizze-mcp/internal/adapter/mcp"
	"github.com/jorgejr568/organizze-mcp/internal/adapter/organizze"
	"github.com/jorgejr568/organizze-mcp/internal/config"
	"github.com/jorgejr568/organizze-mcp/internal/stats"
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

	logger, err := newLogger()
	if err != nil {
		return fmt.Errorf("build logger: %w", err)
	}
	defer func() { _ = logger.Sync() }()

	switch cfg.Transport {
	case "stdio":
		return runWithTransport(ctx, cfg, logger, &mcpsdk.StdioTransport{}, "stdio")
	case "http":
		return runHTTP(ctx, cfg, logger)
	default:
		return fmt.Errorf("unsupported transport %q", cfg.Transport)
	}
}

// newLogger returns a JSON-formatted production logger that writes to stderr.
// Stdout is reserved for the MCP stdio protocol; routing logs anywhere else
// would corrupt the JSON-RPC stream.
func newLogger() (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{"stderr"}
	cfg.ErrorOutputPaths = []string{"stderr"}
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	return cfg.Build()
}

// buildServer is the dependency-injection graph. It is the ONLY place that
// imports both adapter/organizze and adapter/mcp concretes.
func buildServer(ctx context.Context, cfg *config.Config, logger *zap.Logger, transport string) (*mcpsdk.Server, error) {
	httpClient := organizze.NewClient(organizze.ClientOptions{Timeout: cfg.HTTPTimeout})

	exec, err := organizze.NewRequestExecutor(organizze.RequestExecutorOptions{
		HTTPClient:  httpClient,
		BaseURL:     cfg.BaseURL,
		Email:       cfg.Email,
		APIKey:      cfg.APIKey,
		UserAgent:   cfg.UserAgent,
		LogRequests: cfg.LogRequests,
		Logger:      logger,
	})
	if err != nil {
		return nil, fmt.Errorf("build request executor: %w", err)
	}

	deps := mcp.Dependencies{
		Reporter:    buildStatsReporter(ctx, logger, transport),
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

func runWithTransport(ctx context.Context, cfg *config.Config, logger *zap.Logger, t mcpsdk.Transport, name string) error {
	s, err := buildServer(ctx, cfg, logger, name)
	if err != nil {
		return err
	}
	logger.Info("organizze-mcp starting",
		zap.String("version", mcp.Version),
		zap.String("transport", name),
	)
	return s.Run(ctx, t)
}

func runHTTP(ctx context.Context, cfg *config.Config, logger *zap.Logger) error {
	s, err := buildServer(ctx, cfg, logger, "http")
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
		logger.Info("organizze-mcp listening",
			zap.String("version", mcp.Version),
			zap.String("addr", cfg.HTTPAddr),
		)
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
			logger.Error("shutdown failed", zap.Error(err))
		}
		return nil
	case err := <-errCh:
		return err
	}
}

// buildStatsReporter picks the right stats reporter for this process.
// Returns NoopReporter when stats are opted-out (MCP_STATS_OPTOUT=1) or when
// neither the env override nor the build-time default supplies an ingest
// URL + token pair. Released artifacts ship with the URL/token baked in via
// -ldflags; un-stamped dev builds therefore default to NoopReporter unless
// the operator sets the env vars at runtime.
func buildStatsReporter(ctx context.Context, logger *zap.Logger, transport string) stats.Reporter {
	if os.Getenv("MCP_STATS_OPTOUT") != "" {
		return stats.NoopReporter{}
	}
	url := envOr("MCP_STATS_INGEST_URL", stats.DefaultIngestURL)
	token := envOr("MCP_STATS_INGEST_TOKEN", stats.DefaultIngestToken)
	if url == "" || token == "" {
		return stats.NoopReporter{}
	}
	return stats.NewHTTPReporter(ctx, url, token, mcp.Version, transport, 256, logger)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
