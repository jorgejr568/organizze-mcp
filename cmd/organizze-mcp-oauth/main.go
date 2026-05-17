// Command organizze-mcp-oauth is the multi-tenant variant of organizze-mcp.
// It hosts an OAuth 2.1 Authorization Server and serves the same MCP toolset
// over Streamable HTTP, resolving each caller's Organizze credentials from
// the validated bearer token instead of process-wide env vars.
// See AGENTS.md and cmd/organizze-mcp-oauth/README.md for the operator runbook.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/jorgejr568/organizze-mcp/internal/adapter/mcp"
	"github.com/jorgejr568/organizze-mcp/internal/adapter/organizze"
	"github.com/jorgejr568/organizze-mcp/internal/config"
	"github.com/jorgejr568/organizze-mcp/internal/oauth/credprovider"
	"github.com/jorgejr568/organizze-mcp/internal/oauth/server"
	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
	"github.com/jorgejr568/organizze-mcp/internal/stats"
	"github.com/jorgejr568/organizze-mcp/internal/usecase"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "organizze-mcp-oauth:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadOAuth()
	if err != nil {
		return err
	}
	logger, err := newLogger()
	if err != nil {
		return fmt.Errorf("build logger: %w", err)
	}
	defer func() { _ = logger.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("pgxpool: %w", err)
	}
	defer pool.Close()
	if err := storage.ApplyMigrations(ctx, pool); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	store := storage.NewPostgres(pool)
	cipher, err := storage.NewCipher(cfg.EncryptionKey)
	if err != nil {
		return fmt.Errorf("cipher: %w", err)
	}

	// Upstream Organizze client is shared across all tenants — per-request
	// credentials arrive via the request context (populated by the bearer
	// middleware) and are pulled by credprovider.FromContext.
	httpClient := organizze.NewClient(organizze.ClientOptions{Timeout: cfg.HTTPTimeout})
	exec, err := organizze.NewRequestExecutor(organizze.RequestExecutorOptions{
		HTTPClient:  httpClient,
		BaseURL:     cfg.OrganizzeBase,
		Credentials: credprovider.FromContext,
		LogRequests: cfg.LogRequests,
		Logger:      logger,
	})
	if err != nil {
		return fmt.Errorf("executor: %w", err)
	}

	// validate probes the supplied credentials against the live Organizze API
	// by calling GET /accounts. Cheap, read-only, no side effects. Used by the
	// /oauth/authorize handler to reject bad credential pairs up-front rather
	// than minting a token that fails on the first tool call.
	validate := func(ctx context.Context, email, apiKey, ua string) error {
		validator, err := organizze.NewRequestExecutor(organizze.RequestExecutorOptions{
			HTTPClient:  httpClient,
			BaseURL:     cfg.OrganizzeBase,
			Credentials: credprovider.Static(email, apiKey, ua),
		})
		if err != nil {
			return err
		}
		var ignored []map[string]any
		return validator.Get(ctx, "/accounts", &ignored)
	}

	oauthSrv := server.New(server.Config{
		PublicURL:         cfg.PublicURL,
		Store:             store,
		Cipher:            cipher,
		ValidateOrganizze: validate,
		AccessTokenTTL:    cfg.AccessTokenTTL,
		RefreshTokenTTL:   cfg.RefreshTTL,
		SessionTTL:        cfg.SessionTTL,
		CookieSecret:      cfg.CookieSecret,
		Logger:            logger,
	})

	mcpServer := mcp.New(mcp.Dependencies{
		Reporter:    stats.NoopReporter{},
		User:        usecase.NewUserService(organizze.NewUserRepository(exec)),
		Account:     usecase.NewAccountService(organizze.NewAccountRepository(exec)),
		Category:    usecase.NewCategoryService(organizze.NewCategoryRepository(exec)),
		Budget:      usecase.NewBudgetService(organizze.NewBudgetRepository(exec)),
		CreditCard:  usecase.NewCreditCardService(organizze.NewCreditCardRepository(exec)),
		Invoice:     usecase.NewInvoiceService(organizze.NewInvoiceRepository(exec)),
		Transfer:    usecase.NewTransferService(organizze.NewTransferRepository(exec)),
		Transaction: usecase.NewTransactionService(organizze.NewTransactionRepository(exec)),
	})

	// Serve both MCP transports under one bearer-protected surface:
	//   /mcp                — Streamable HTTP (current spec)
	//   /sse + /sse/<session> — legacy SSE transport
	// ChatGPT's MCP connector probes the SSE path first; without it the
	// discovery step fails with `MCP_ACTION_DISCOVERY_FAILED` even though
	// the OAuth flow itself completes.
	getServer := func(_ *http.Request) *mcpsdk.Server { return mcpServer }
	streamableHandler := mcpsdk.NewStreamableHTTPHandler(getServer, nil)
	sseHandler := mcpsdk.NewSSEHandler(getServer, nil)

	bearerSSE := oauthSrv.Bearer(sseHandler)

	// Root dispatcher: if the request looks like an MCP SSE probe (GET with
	// Accept: text/event-stream — or any POST against a `?sessionid=` URL),
	// route to the SSE handler. Otherwise fall through to OAuth + .well-known
	// + the original 404 surface. ChatGPT's connector probes the *URL the
	// user typed*, which is frequently the bare origin "/" — without this
	// dispatch the probe hits a Go-mux 404 and the connector aborts even
	// when the OAuth flow itself completes.
	rootHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && looksLikeSSEProbe(r) {
			bearerSSE.ServeHTTP(w, r)
			return
		}
		oauthSrv.ServeHTTP(w, r)
	})

	mux := http.NewServeMux()
	mux.Handle("/mcp", oauthSrv.Bearer(streamableHandler))
	mux.Handle("/sse", bearerSSE)
	mux.Handle("/sse/", bearerSSE)         // per-session message POSTs (advertised endpoint)
	mux.Handle("/mcp/sse", bearerSSE)      // clients that probe relative to the resource URL
	mux.Handle("/mcp/sse/", bearerSSE)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/", rootHandler)

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		logger.Info("organizze-mcp-oauth listening",
			zap.String("addr", cfg.HTTPAddr),
			zap.String("public_url", cfg.PublicURL),
		)
		err := httpSrv.ListenAndServe()
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
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown failed", zap.Error(err))
		}
		return nil
	case err := <-errCh:
		return err
	}
}

// looksLikeSSEProbe heuristically identifies the requests an MCP SSE client
// would send. A GET asking for text/event-stream is the canonical probe; a
// POST carrying a sessionid is the messages-back channel for an open SSE
// session. Everything else falls through to OAuth/well-known/etc.
func looksLikeSSEProbe(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet:
		for _, v := range r.Header.Values("Accept") {
			if strings.Contains(strings.ToLower(v), "text/event-stream") {
				return true
			}
		}
		return false
	case http.MethodPost:
		return r.URL.Query().Get("sessionid") != ""
	default:
		return false
	}
}

// newLogger returns a JSON-formatted production logger writing to stderr.
// Mirrors cmd/organizze-mcp/main.go so all binaries emit the same shape.
func newLogger() (*zap.Logger, error) {
	c := zap.NewProductionConfig()
	c.OutputPaths = []string{"stderr"}
	c.ErrorOutputPaths = []string{"stderr"}
	c.EncoderConfig.TimeKey = "ts"
	c.EncoderConfig.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	return c.Build()
}
