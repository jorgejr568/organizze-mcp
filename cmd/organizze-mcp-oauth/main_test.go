package main

// This test runs the full OAuth flow against a real Postgres (skipped without
// OAUTH_DATABASE_URL) and a fake Organizze upstream (httptest). It exercises:
//   1. POST /oauth/register → client_id
//   2. POST /oauth/authorize (form) → 303 with code
//   3. POST /oauth/token (authorization_code) → access_token
//   4. POST /mcp with Bearer access_token → tools/list response

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/adapter/mcp"
	"github.com/jorgejr568/organizze-mcp/internal/adapter/organizze"
	"github.com/jorgejr568/organizze-mcp/internal/oauth/credprovider"
	"github.com/jorgejr568/organizze-mcp/internal/oauth/server"
	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
	"github.com/jorgejr568/organizze-mcp/internal/stats"
	"github.com/jorgejr568/organizze-mcp/internal/usecase"
)

func TestEndToEnd_OAuthThenToolCall(t *testing.T) {
	dsn := os.Getenv("OAUTH_DATABASE_URL")
	if dsn == "" {
		t.Skip("OAUTH_DATABASE_URL not set; skipping e2e test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := storage.ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Wipe between runs so reruns aren't poisoned by prior state.
	for _, tbl := range []string{"oauth_tokens", "oauth_codes", "oauth_sessions", "oauth_clients", "oauth_users"} {
		_, _ = pool.Exec(ctx, "TRUNCATE TABLE "+tbl+" RESTART IDENTITY CASCADE")
	}
	store := storage.NewPostgres(pool)

	// Fake Organizze upstream. Only /accounts is wired — that's all the
	// validator and the tools/list path need for this e2e.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/accounts":
			_, _ = io.WriteString(w, `[{"id":1,"name":"Checking","type":"checking"}]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(upstream.Close)

	cipher, err := storage.NewCipher(make([]byte, 32))
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	httpClient := organizze.NewClient(organizze.ClientOptions{Timeout: 5 * time.Second})
	exec, err := organizze.NewRequestExecutor(organizze.RequestExecutorOptions{
		HTTPClient:  httpClient,
		BaseURL:     upstream.URL,
		Credentials: credprovider.FromContext,
	})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	validate := func(ctx context.Context, email, apiKey, ua string) error {
		v, err := organizze.NewRequestExecutor(organizze.RequestExecutorOptions{
			HTTPClient:  httpClient,
			BaseURL:     upstream.URL,
			Credentials: credprovider.Static(email, apiKey, ua),
		})
		if err != nil {
			return err
		}
		var ignored []map[string]any
		return v.Get(ctx, "/accounts", &ignored)
	}

	oauthSrv := server.New(server.Config{
		PublicURL:         "http://oauth.example.com",
		Store:             store,
		Cipher:            cipher,
		ValidateOrganizze: validate,
		CookieSecret:      bytes.Repeat([]byte("c"), 32),
	})
	// Wire every Dependencies field. The Bearer middleware will short-circuit
	// 401-protected calls, but tools/list invokes every registration callback
	// and panics if any sub-service is nil — so populate the whole graph just
	// as the production main.go does.
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
	mux := http.NewServeMux()
	mux.Handle("/mcp", oauthSrv.Bearer(mcpsdk.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcpsdk.Server { return mcpServer }, nil)))
	mux.Handle("/", oauthSrv)
	front := httptest.NewServer(mux)
	t.Cleanup(front.Close)

	// 1. DCR — Dynamic Client Registration.
	dcrBody := `{"client_name":"E2E","redirect_uris":["https://chat.example.com/cb"]}`
	resp, err := http.Post(front.URL+"/oauth/register", "application/json", strings.NewReader(dcrBody))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("register status: %d body=%s", resp.StatusCode, body)
	}
	var dcr struct {
		ClientID string `json:"client_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&dcr); err != nil {
		t.Fatalf("decode dcr: %v", err)
	}
	resp.Body.Close()
	if dcr.ClientID == "" {
		t.Fatalf("empty client_id from DCR")
	}

	// 2. Authorize POST — skips the GET form render; ChatGPT would render that
	// in a browser. We POST the same fields the form would submit.
	verifier := "the-pkce-verifier-which-is-long-enough-to-pass-checks-1234"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	form := url.Values{
		"client_id":             {dcr.ClientID},
		"redirect_uri":          {"https://chat.example.com/cb"},
		"state":                 {"st"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"email":                 {"e@x.com"},
		"api_key":               {"key"},
		"user_agent":            {"E2E (e@x.com)"},
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err = client.PostForm(front.URL+"/oauth/authorize", form)
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if resp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("authorize status: %d body=%s", resp.StatusCode, body)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse location: %v", err)
	}
	resp.Body.Close()
	code := loc.Query().Get("code")
	if code == "" {
		t.Fatalf("no code in %s", resp.Header.Get("Location"))
	}

	// 3. Token exchange — authorization_code grant with PKCE verifier.
	resp, err = http.PostForm(front.URL+"/oauth/token", url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {"https://chat.example.com/cb"},
		"client_id":     {dcr.ClientID},
		"code_verifier": {verifier},
	})
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("token status: %d body=%s", resp.StatusCode, body)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		t.Fatalf("decode token: %v", err)
	}
	resp.Body.Close()
	if tok.AccessToken == "" {
		t.Fatalf("no access_token in token response")
	}

	// 4. /mcp tools/list with the bearer token (raw JSON-RPC against the
	// Streamable HTTP handler). The handler streams responses as SSE when
	// the client offers text/event-stream; setting both Accept media types
	// lets it pick either, but most importantly avoids a 406.
	body := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	req, _ := http.NewRequest(http.MethodPost, front.URL+"/mcp", body)
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("mcp call: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("mcp call status: %d body=%s", resp.StatusCode, raw)
	}
}
