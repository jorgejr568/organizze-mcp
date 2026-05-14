# Organizze MCP Server Implementation Plan (Clean Architecture)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go-based MCP server that wraps the Organizze REST API, packaged as a Docker image, configured by environment variables, supporting both `stdio` and Streamable HTTP transports — organized as a layered Clean Architecture with explicit interfaces between domain, application, and infrastructure layers.

**Architecture:** Four layers, dependencies inward only.

```
  cmd/organizze-mcp/main.go         ← composition root
        │ wires
        ▼
  ┌────────────────────┐  ┌─────────────────────────┐
  │  adapter/mcp        │  │  adapter/organizze       │
  │  (MCP tools)        │  │  (HTTPClient + repos)    │
  └─────────┬──────────┘  └────────────┬────────────┘
            │ depends on                │ implements
            ▼                           ▼
        ┌──────────────────────────────────────┐
        │  internal/usecase                     │
        │  (services + repository interfaces)   │
        └──────────────────┬───────────────────┘
                           │ depends on
                           ▼
        ┌──────────────────────────────────────┐
        │  internal/domain                      │
        │  (entities, value objects, errors)    │
        └──────────────────────────────────────┘
```

The `HTTPClient` interface + `Client` struct at `adapter/organizze/client.go` wraps `net/http` so repositories never touch `*http.Client` directly. Tests substitute fakes via that interface.

**Tech Stack:**
- **Go** ≥ 1.23
- **MCP SDK:** `github.com/modelcontextprotocol/go-sdk` v1.5.0+
- **HTTP:** stdlib `net/http` (wrapped behind `HTTPClient`)
- **JSON:** stdlib `encoding/json`
- **Tests:** stdlib `testing` + `net/http/httptest`
- **Container:** multi-stage `golang:1.23-alpine` → `gcr.io/distroless/static:nonroot`
- **CI/CD:** GitHub Actions — test on every PR + multi-arch Docker Hub publish on `v*` tags
- **Registry:** Docker Hub — `jorgejr568/organizze-mcp` (semver tags + `latest`)
- **Repo hosting:** GitHub `jorgejr568/organizze-mcp` with branch protection on `main`

---

## Design principles applied

### Clean Code
- **Names**: every exported symbol explains itself; no abbreviations beyond established (`URL`, `HTTP`, `ID`).
- **Small functions**: one thing at one level of abstraction; cyclomatic depth ≤ 3 in handlers.
- **DRY at one boundary**: Basic-Auth, `User-Agent`, JSON marshaling, and HTTP-error mapping live exactly once — in `adapter/organizze/executor.go`. Repositories never construct an `*http.Request` manually.
- **No magic literals**: status codes are named; the API base URL is a constant.
- **TDD**: every step writes the test first, runs it red, makes it green.

### SOLID
- **S — Single Responsibility**: `client.go` = transport; `executor.go` = auth + JSON; one repository per resource; one service per resource; one tool family per MCP file.
- **O — Open/Closed**: a new resource = new files only; the composition root wires it. No edits to existing repositories or services.
- **L — Liskov Substitution**: any `HTTPClient` impl is interchangeable with `*Client`; any repository impl is interchangeable with the test fake.
- **I — Interface Segregation**: `HTTPClient` has one method. `TransactionRepository` is composed of `TransactionReader` + `TransactionWriter`. Other repos are read-only (2 methods each) — already minimal.
- **D — Dependency Inversion**: every layer depends on interfaces it owns, not on outer concretes. Repository interfaces live in `usecase/`; HTTP impls in `adapter/organizze`. The composition root is the only place concrete wiring happens.

### Clean Architecture
- **Layers**: domain → usecase → adapter → cmd.
- **Dependency Rule**: source code dependencies point inward.
- **Data crossing boundaries**: only `domain.*` types. Wire-format DTOs are unexported and live in `adapter/organizze`.
- **Interfaces at boundaries**: defined by the consumer (inner layer), implemented by the provider (outer layer). Go's implicit interface satisfaction means no explicit annotations.

### Go idiom notes
- "Accept interfaces, return structs" — constructors return `*Concrete`; functions accept `Interface`. Exception: `NewClient` returns the `HTTPClient` interface because `Client` has no other exposed methods.
- No master interfaces; each interface is sized to its call site.
- Names: the user's casual example used `HttpClientCaller` / `httpClientImpl`. This plan uses Go-idiomatic `HTTPClient` (interface) and `Client` (struct) — `Caller`/`Impl` suffixes are a Java/C# convention. One sed pass renames if you prefer.

---

## File structure

```
organizze-mcp/
├── go.mod
├── go.sum
├── Dockerfile
├── .dockerignore
├── .gitignore
├── README.md
├── Makefile
├── .github/
│   └── workflows/
│       ├── ci.yml                     # test on PR + push to main
│       └── release.yml                # multi-arch Docker Hub publish on v* tags
├── cmd/
│   └── organizze-mcp/
│       ├── main.go                    # composition root
│       └── main_test.go
└── internal/
    ├── config/
    │   ├── config.go
    │   └── config_test.go
    ├── domain/                        # LAYER 1
    │   ├── account.go
    │   ├── budget.go
    │   ├── category.go
    │   ├── credit_card.go
    │   ├── errors.go
    │   ├── filters.go
    │   ├── invoice.go
    │   ├── transaction.go
    │   ├── transfer.go
    │   └── user.go
    ├── usecase/                       # LAYER 2
    │   ├── account.go                 # AccountRepository + AccountService
    │   ├── budget.go                  # BudgetRepository + BudgetService (period routing)
    │   ├── category.go
    │   ├── credit_card.go
    │   ├── invoice.go
    │   ├── transaction.go             # TransactionReader + TransactionWriter + TransactionRepository + TransactionService
    │   ├── transfer.go
    │   ├── user.go
    │   └── *_test.go                  # service tests with fake repos
    └── adapter/                       # LAYER 3
        ├── organizze/                 # HTTP repository implementations
        │   ├── client.go              # HTTPClient interface + Client struct
        │   ├── executor.go            # RequestExecutor (auth + UA + JSON)
        │   ├── errors.go              # APIError + maps to domain errors
        │   ├── account_repository.go
        │   ├── budget_repository.go
        │   ├── category_repository.go
        │   ├── credit_card_repository.go
        │   ├── invoice_repository.go
        │   ├── transaction_repository.go
        │   ├── transfer_repository.go
        │   ├── user_repository.go
        │   └── *_test.go              # httptest-backed tests per repo
        └── mcp/                       # MCP tool adapters
            ├── server.go              # New(services...) *mcp.Server
            ├── tools_accounts.go
            ├── tools_budgets.go
            ├── tools_categories.go
            ├── tools_credit_cards.go
            ├── tools_invoices.go
            ├── tools_transactions.go
            ├── tools_transfers.go
            ├── tools_user.go
            ├── *_test.go              # handler tests with fake services
            └── integration_test.go    # full-stack via InMemoryTransports
```

**Tool catalogue (16 tools):**

| Tool | Service.Method | Mutating? |
|---|---|---|
| `get_user` | UserService.Get | no |
| `list_accounts`, `get_account` | AccountService.{List, Get} | no |
| `list_categories`, `get_category` | CategoryService.{List, Get} | no |
| `list_budgets` | BudgetService.List (routes on year/month) | no |
| `list_credit_cards`, `get_credit_card` | CreditCardService.{List, Get} | no |
| `list_credit_card_invoices`, `get_credit_card_invoice` | InvoiceService.{List, Get} | no |
| `list_transfers` | TransferService.List | no |
| `list_transactions`, `get_transaction` | TransactionService.{List, Get} | no |
| `create_transaction`, `update_transaction`, `delete_transaction` | TransactionService.{Create, Update, Delete} | **yes** |

**Env contract:**

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `ORGANIZZE_API_KEY` | yes | — | Basic-Auth password |
| `ORGANIZZE_EMAIL` | yes | — | Basic-Auth username |
| `ORGANIZZE_USER_AGENT` | yes | — | Required by Organizze; format `"App (e@x.com)"` |
| `MCP_TRANSPORT` | no | `stdio` | `stdio` or `http` |
| `MCP_HTTP_ADDR` | no | `:8080` | Listen address when `MCP_TRANSPORT=http` |
| `ORGANIZZE_BASE_URL` | no | `https://api.organizze.com.br/rest/v2` | Override |
| `ORGANIZZE_HTTP_TIMEOUT` | no | `30s` | HTTP client timeout (parsed via `time.ParseDuration`) |

---

## Task 1: Bootstrap

**Files:** `go.mod`, `.gitignore`, `.dockerignore`, `Makefile`, `README.md` (skeleton), git init.

- [ ] **Step 1: Init git + go module + SDK dependency**

```bash
cd /Users/j/src/jorgejr568/organizze-mcp
git init -b main
go mod init github.com/jorgejr568/organizze-mcp
go get github.com/modelcontextprotocol/go-sdk@latest
```

Expected: `git init` creates `.git/`; `go.mod` is created with module path; `go get` adds the SDK on a `v1.x.y` line.

- [ ] **Step 2: Write `.gitignore`**

```gitignore
/bin/
/organizze-mcp
*.exe
coverage.out
coverage.html
.idea/
.vscode/
*.swp
.DS_Store
.env
.env.local
```

- [ ] **Step 3: Write `.dockerignore`**

```
.git
.gitignore
.dockerignore
Dockerfile
README.md
docs/
*.md
bin/
coverage.out
coverage.html
.env*
.idea/
.vscode/
.DS_Store
```

- [ ] **Step 4: Write `Makefile`**

```makefile
.PHONY: build test test-cover lint run-stdio run-http docker docker-run clean

BINARY := organizze-mcp

build:
	go build -o bin/$(BINARY) ./cmd/organizze-mcp

test:
	go test ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	go vet ./...

run-stdio: build
	MCP_TRANSPORT=stdio ./bin/$(BINARY)

run-http: build
	MCP_TRANSPORT=http MCP_HTTP_ADDR=:8080 ./bin/$(BINARY)

docker:
	docker build -t organizze-mcp:latest .

docker-run:
	docker run --rm -i \
		-e ORGANIZZE_API_KEY -e ORGANIZZE_EMAIL -e ORGANIZZE_USER_AGENT \
		organizze-mcp:latest

clean:
	rm -rf bin/ coverage.out coverage.html
```

- [ ] **Step 5: Write skeleton `README.md`**

```markdown
# organizze-mcp

A Model Context Protocol (MCP) server exposing the Organizze REST API, built in Go with a layered Clean Architecture. Final docs at end of project.

## Configuration

See env contract in `docs/superpowers/plans/2026-05-14-organizze-mcp.md`.
```

- [ ] **Step 6: Verify and commit**

```bash
go build ./...
git add .
git commit -m "chore: bootstrap (go.mod, makefile, gitignore)"
```

Expected: empty build (no Go files yet); clean commit.

---

## Task 2: Config package (TDD)

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test**

`internal/config/config_test.go`:

```go
package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoad_DefaultsAndRequiredFields(t *testing.T) {
	t.Setenv("ORGANIZZE_API_KEY", "k")
	t.Setenv("ORGANIZZE_EMAIL", "e@x.com")
	t.Setenv("ORGANIZZE_USER_AGENT", "App (e@x.com)")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIKey != "k" || cfg.Email != "e@x.com" || cfg.UserAgent != "App (e@x.com)" {
		t.Fatalf("required: %+v", cfg)
	}
	if cfg.Transport != "stdio" {
		t.Errorf("Transport default = %q", cfg.Transport)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr default = %q", cfg.HTTPAddr)
	}
	if cfg.BaseURL != "https://api.organizze.com.br/rest/v2" {
		t.Errorf("BaseURL default = %q", cfg.BaseURL)
	}
	if cfg.HTTPTimeout != 30*time.Second {
		t.Errorf("HTTPTimeout default = %v", cfg.HTTPTimeout)
	}
}

func TestLoad_MissingRequiredFails(t *testing.T) {
	t.Setenv("ORGANIZZE_API_KEY", "")
	t.Setenv("ORGANIZZE_EMAIL", "")
	t.Setenv("ORGANIZZE_USER_AGENT", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"ORGANIZZE_API_KEY", "ORGANIZZE_EMAIL", "ORGANIZZE_USER_AGENT"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err missing %q: %v", want, err)
		}
	}
}

func TestLoad_RejectsUnknownTransport(t *testing.T) {
	t.Setenv("ORGANIZZE_API_KEY", "k")
	t.Setenv("ORGANIZZE_EMAIL", "e@x.com")
	t.Setenv("ORGANIZZE_USER_AGENT", "ua")
	t.Setenv("MCP_TRANSPORT", "grpc")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "MCP_TRANSPORT") {
		t.Fatalf("want MCP_TRANSPORT error, got %v", err)
	}
}

func TestLoad_AcceptsHTTPAndCustomTimeout(t *testing.T) {
	t.Setenv("ORGANIZZE_API_KEY", "k")
	t.Setenv("ORGANIZZE_EMAIL", "e@x.com")
	t.Setenv("ORGANIZZE_USER_AGENT", "ua")
	t.Setenv("MCP_TRANSPORT", "HTTP")
	t.Setenv("MCP_HTTP_ADDR", ":9000")
	t.Setenv("ORGANIZZE_HTTP_TIMEOUT", "10s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Transport != "http" || cfg.HTTPAddr != ":9000" || cfg.HTTPTimeout != 10*time.Second {
		t.Errorf("got %+v", cfg)
	}
}

func TestLoad_RejectsInvalidTimeout(t *testing.T) {
	t.Setenv("ORGANIZZE_API_KEY", "k")
	t.Setenv("ORGANIZZE_EMAIL", "e@x.com")
	t.Setenv("ORGANIZZE_USER_AGENT", "ua")
	t.Setenv("ORGANIZZE_HTTP_TIMEOUT", "not-a-duration")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "ORGANIZZE_HTTP_TIMEOUT") {
		t.Fatalf("want timeout error, got %v", err)
	}
}
```

- [ ] **Step 2: Run, verify failure**

```bash
go test ./internal/config/...
```

Expected: build failure (`package config: no Go files`).

- [ ] **Step 3: Write implementation**

`internal/config/config.go`:

```go
// Package config loads and validates the environment configuration
// for the organizze-mcp server.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config is the resolved runtime configuration.
type Config struct {
	APIKey      string
	Email       string
	UserAgent   string
	BaseURL     string
	HTTPTimeout time.Duration

	Transport string // "stdio" | "http"
	HTTPAddr  string // listen address when Transport == "http"
}

// Load reads configuration from environment variables, applying defaults and
// validating required fields and enum-like values. Errors list every problem.
func Load() (*Config, error) {
	cfg := &Config{
		APIKey:    os.Getenv("ORGANIZZE_API_KEY"),
		Email:     os.Getenv("ORGANIZZE_EMAIL"),
		UserAgent: os.Getenv("ORGANIZZE_USER_AGENT"),
		BaseURL:   os.Getenv("ORGANIZZE_BASE_URL"),
		Transport: strings.ToLower(strings.TrimSpace(os.Getenv("MCP_TRANSPORT"))),
		HTTPAddr:  os.Getenv("MCP_HTTP_ADDR"),
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.organizze.com.br/rest/v2"
	}
	if cfg.Transport == "" {
		cfg.Transport = "stdio"
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}

	timeoutStr := os.Getenv("ORGANIZZE_HTTP_TIMEOUT")
	if timeoutStr == "" {
		cfg.HTTPTimeout = 30 * time.Second
	} else {
		d, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return nil, fmt.Errorf("ORGANIZZE_HTTP_TIMEOUT %q: %w", timeoutStr, err)
		}
		cfg.HTTPTimeout = d
	}

	var missing []string
	if cfg.APIKey == "" {
		missing = append(missing, "ORGANIZZE_API_KEY")
	}
	if cfg.Email == "" {
		missing = append(missing, "ORGANIZZE_EMAIL")
	}
	if cfg.UserAgent == "" {
		missing = append(missing, "ORGANIZZE_USER_AGENT")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	switch cfg.Transport {
	case "stdio", "http":
	default:
		return nil, fmt.Errorf("invalid MCP_TRANSPORT %q (expected stdio or http)", cfg.Transport)
	}

	return cfg, nil
}
```

- [ ] **Step 4: Run tests, verify pass**

```bash
go test ./internal/config/... -v
```

Expected: 5 PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): env loading with validation"
```

---

## Task 3: Domain layer (TDD)

**Files:** ten files under `internal/domain/`. The domain is data + sentinel errors only — no behavior beyond constructor-style helpers and stringer-ish methods if useful.

**Test scope:** the domain has almost no logic, so tests are minimal — only the error helpers (`errors.Is` over the sentinels) need exercising. The entity structs are pure data and become testable via the layers that use them.

- [ ] **Step 1: Write failing test for domain errors**

`internal/domain/errors_test.go`:

```go
package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestSentinelsAreDistinct(t *testing.T) {
	if errors.Is(ErrNotFound, ErrUnauthorized) {
		t.Error("ErrNotFound must not match ErrUnauthorized")
	}
	if errors.Is(ErrUnauthorized, ErrValidation) {
		t.Error("ErrUnauthorized must not match ErrValidation")
	}
}

func TestWrappedSentinelMatches(t *testing.T) {
	wrapped := fmt.Errorf("transaction 42: %w", ErrNotFound)
	if !errors.Is(wrapped, ErrNotFound) {
		t.Error("errors.Is must traverse wrapping")
	}
}
```

- [ ] **Step 2: Run, verify failure**

```bash
go test ./internal/domain/...
```

Expected: build failure.

- [ ] **Step 3: Write `errors.go`**

`internal/domain/errors.go`:

```go
// Package domain holds the entities, value objects, and sentinel errors that
// cross layer boundaries. It depends on nothing outside the standard library.
package domain

import "errors"

// Sentinel errors. Outer layers wrap these so callers can match with errors.Is.
var (
	ErrNotFound     = errors.New("domain: not found")
	ErrUnauthorized = errors.New("domain: unauthorized")
	ErrValidation   = errors.New("domain: validation failed")
	ErrUpstream     = errors.New("domain: upstream API error")
)
```

- [ ] **Step 4: Write `user.go`**

`internal/domain/user.go`:

```go
package domain

// User is an Organizze account holder.
type User struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}
```

- [ ] **Step 5: Write `account.go`**

`internal/domain/account.go`:

```go
package domain

import "time"

// Account is a bank or cash account.
type Account struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Type        string    `json:"type"`
	Default     bool      `json:"default"`
	Archived    bool      `json:"archived"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}
```

- [ ] **Step 6: Write `category.go`**

`internal/domain/category.go`:

```go
package domain

// Category is an expense/income category.
type Category struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color,omitempty"`
	ParentID *int64 `json:"parent_id,omitempty"`
}
```

- [ ] **Step 7: Write `budget.go`**

`internal/domain/budget.go`:

```go
package domain

// Budget is a planned spend for a category in a given period.
type Budget struct {
	AmountInCents  int64  `json:"amount_in_cents"`
	CategoryID     int64  `json:"category_id"`
	Date           string `json:"date"` // YYYY-MM-DD (period start)
	ActivityType   int    `json:"activity_type,omitempty"`
	Total          int64  `json:"total"`
	PredictedTotal int64  `json:"predicted_total"`
	Percentage     string `json:"percentage"`
}
```

- [ ] **Step 8: Write `credit_card.go`**

`internal/domain/credit_card.go`:

```go
package domain

import "time"

// CreditCard represents a credit card configured in Organizze.
type CreditCard struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	CardNetwork *string   `json:"card_network,omitempty"`
	ClosingDay  int       `json:"closing_day"`
	DueDay      int       `json:"due_day"`
	LimitCents  int64     `json:"limit_cents"`
	Kind        string    `json:"kind,omitempty"`
	Archived    bool      `json:"archived"`
	Default     bool      `json:"default"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}
```

- [ ] **Step 9: Write `invoice.go`**

`internal/domain/invoice.go`:

```go
package domain

// Invoice is a monthly credit-card bill.
type Invoice struct {
	ID                   int64         `json:"id"`
	Date                 string        `json:"date"`
	StartingDate         string        `json:"starting_date"`
	ClosingDate          string        `json:"closing_date"`
	AmountCents          int64         `json:"amount_cents"`
	PaymentAmountCents   int64         `json:"payment_amount_cents,omitempty"`
	BalanceCents         int64         `json:"balance_cents,omitempty"`
	PreviousBalanceCents int64         `json:"previous_balance_cents,omitempty"`
	CreditCardID         int64         `json:"credit_card_id"`
	Transactions         []Transaction `json:"transactions,omitempty"`
	Payments             []Transaction `json:"payments,omitempty"`
}
```

- [ ] **Step 10: Write `transaction.go`**

`internal/domain/transaction.go`:

```go
package domain

// Tag is a lightweight tag attached to a transaction.
type Tag struct {
	Name string `json:"name"`
}

// Transaction is a ledger entry. AmountCents is negative for expenses,
// positive for income, per the Organizze API.
type Transaction struct {
	ID                      int64  `json:"id"`
	Description             string `json:"description"`
	Date                    string `json:"date"`
	Paid                    bool   `json:"paid"`
	AmountCents             int64  `json:"amount_cents"`
	TotalInstallments       int    `json:"total_installments,omitempty"`
	Installment             int    `json:"installment,omitempty"`
	Recurring               bool   `json:"recurring,omitempty"`
	AccountID               int64  `json:"account_id"`
	AccountType             string `json:"account_type,omitempty"`
	CategoryID              int64  `json:"category_id"`
	ContactID               *int64 `json:"contact_id,omitempty"`
	Notes                   string `json:"notes,omitempty"`
	AttachmentsCount        int    `json:"attachments_count,omitempty"`
	CreditCardID            *int64 `json:"credit_card_id,omitempty"`
	CreditCardInvoiceID     *int64 `json:"credit_card_invoice_id,omitempty"`
	PaidCreditCardID        *int64 `json:"paid_credit_card_id,omitempty"`
	PaidCreditCardInvoiceID *int64 `json:"paid_credit_card_invoice_id,omitempty"`
	OppositeTransactionID   *int64 `json:"oposite_transaction_id,omitempty"`
	OppositeAccountID       *int64 `json:"oposite_account_id,omitempty"`
	RecurrenceID            *int64 `json:"recurrence_id,omitempty"`
	Tags                    []Tag  `json:"tags,omitempty"`
	CreatedAt               string `json:"created_at,omitempty"`
	UpdatedAt               string `json:"updated_at,omitempty"`
}

// CreateTransactionParams are the inputs to TransactionService.Create.
// Shape mirrors the Organizze POST body but is owned by the domain layer.
type CreateTransactionParams struct {
	Description string `json:"description"`
	Date        string `json:"date"`
	AmountCents int64  `json:"amount_cents"`
	AccountID   int64  `json:"account_id"`
	CategoryID  int64  `json:"category_id"`
	Paid        bool   `json:"paid"`
	Notes       string `json:"notes,omitempty"`
	ContactID   *int64 `json:"contact_id,omitempty"`
	Tags        []Tag  `json:"tags,omitempty"`
}

// UpdateTransactionParams describe a partial update; nil pointers are omitted.
type UpdateTransactionParams struct {
	Description *string `json:"description,omitempty"`
	Date        *string `json:"date,omitempty"`
	AmountCents *int64  `json:"amount_cents,omitempty"`
	AccountID   *int64  `json:"account_id,omitempty"`
	CategoryID  *int64  `json:"category_id,omitempty"`
	Paid        *bool   `json:"paid,omitempty"`
	Notes       *string `json:"notes,omitempty"`
	ContactID   *int64  `json:"contact_id,omitempty"`
	Tags        []Tag   `json:"tags,omitempty"`
}
```

- [ ] **Step 11: Write `transfer.go`**

`internal/domain/transfer.go`:

```go
package domain

// Transfer is a movement of money between two accounts.
type Transfer struct {
	ID                    int64  `json:"id"`
	Description           string `json:"description"`
	Date                  string `json:"date"`
	Paid                  bool   `json:"paid"`
	AmountCents           int64  `json:"amount_cents"`
	AccountID             int64  `json:"account_id"`
	OppositeAccountID     int64  `json:"oposite_account_id"`
	OppositeTransactionID int64  `json:"oposite_transaction_id,omitempty"`
	CategoryID            int64  `json:"category_id"`
	Notes                 string `json:"notes,omitempty"`
	RecurrenceID          *int64 `json:"recurrence_id,omitempty"`
}
```

- [ ] **Step 12: Write `filters.go`**

`internal/domain/filters.go`:

```go
package domain

// ListTransactionsFilter is the filter for TransactionService.List.
// Zero values are treated as "no filter".
type ListTransactionsFilter struct {
	StartDate string // YYYY-MM-DD
	EndDate   string // YYYY-MM-DD
	AccountID int64
}

// ListTransfersFilter is the filter for TransferService.List.
type ListTransfersFilter struct {
	StartDate string
	EndDate   string
}

// BudgetPeriod selects which budget view to return.
//
//	BudgetPeriod{}                             -> current month
//	BudgetPeriod{Year: 2026}                   -> entire 2026
//	BudgetPeriod{Year: 2026, Month: 5}         -> May 2026
type BudgetPeriod struct {
	Year  int
	Month int // 1..12
}
```

- [ ] **Step 13: Run tests, verify pass**

```bash
go test ./internal/domain/... -v
```

Expected: `TestSentinelsAreDistinct PASS`, `TestWrappedSentinelMatches PASS`.

- [ ] **Step 14: Commit**

```bash
git add internal/domain/
git commit -m "feat(domain): entities, value objects, sentinel errors"
```

---

## Task 4: Adapter/organizze foundation — `HTTPClient`, executor, errors (TDD)

This task installs the three building blocks every repository will use:

1. **`HTTPClient` interface** + concrete `Client` struct — the user-requested abstraction over `*http.Client`.
2. **`RequestExecutor`** — owns Basic Auth, `User-Agent`, JSON marshaling, base-URL composition, and HTTP-status → domain-error mapping. Repositories call its methods (`Get`, `Post`, `Put`, `Delete`); they never construct `http.Request` directly.
3. **`APIError`** — typed error returned from the executor on non-2xx, with helpers to map it to domain sentinels.

**Files:**
- Create: `internal/adapter/organizze/client.go`
- Create: `internal/adapter/organizze/client_test.go`
- Create: `internal/adapter/organizze/errors.go`
- Create: `internal/adapter/organizze/errors_test.go`
- Create: `internal/adapter/organizze/executor.go`
- Create: `internal/adapter/organizze/executor_test.go`
- Create: `internal/adapter/organizze/testhelper_test.go`

- [ ] **Step 1: Write failing test for `HTTPClient` + `Client`**

`internal/adapter/organizze/client_test.go`:

```go
package organizze

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_DoForwardsToInner(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	}))
	defer ts.Close()

	c := NewClient(ClientOptions{Timeout: 5 * time.Second})
	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()
	if !called {
		t.Error("inner client not invoked")
	}
	if resp.StatusCode != http.StatusTeapot {
		t.Errorf("StatusCode = %d", resp.StatusCode)
	}
}

func TestClient_SatisfiesHTTPClient(t *testing.T) {
	// Compile-time guarantee: *Client implements HTTPClient.
	var _ HTTPClient = NewClient(ClientOptions{})
}

func TestNewClient_AppliesDefaultTimeout(t *testing.T) {
	c := NewClient(ClientOptions{})
	if c.Inner().Timeout == 0 {
		t.Error("default timeout should be non-zero")
	}
}
```

- [ ] **Step 2: Run, verify failure**

```bash
go test ./internal/adapter/organizze/...
```

Expected: undefined symbols.

- [ ] **Step 3: Write `client.go`**

`internal/adapter/organizze/client.go`:

```go
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

// Inner exposes the underlying *http.Client for advanced callers that need to
// configure transports (proxies, TLS) directly. Most code should not use this.
func (c *Client) Inner() *http.Client {
	return c.inner
}

// Compile-time check.
var _ HTTPClient = (*Client)(nil)
```

- [ ] **Step 4: Run, verify pass**

```bash
go test ./internal/adapter/organizze/... -run Client -v
```

Expected: 3 PASS.

- [ ] **Step 5: Write failing tests for errors + executor**

`internal/adapter/organizze/errors_test.go`:

```go
package organizze

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

func TestAPIError_Error_FormatsStatusAndMessage(t *testing.T) {
	e := &APIError{StatusCode: 422, Message: "validation failed"}
	if got := e.Error(); got != "organizze: 422 validation failed" {
		t.Errorf("Error() = %q", got)
	}
}

func TestAPIError_MapsToDomainSentinels(t *testing.T) {
	cases := []struct {
		status int
		sentinel error
	}{
		{http.StatusNotFound, domain.ErrNotFound},
		{http.StatusUnauthorized, domain.ErrUnauthorized},
		{http.StatusForbidden, domain.ErrUnauthorized},
		{http.StatusUnprocessableEntity, domain.ErrValidation},
		{http.StatusBadRequest, domain.ErrValidation},
		{http.StatusInternalServerError, domain.ErrUpstream},
	}
	for _, c := range cases {
		err := &APIError{StatusCode: c.status}
		if !errors.Is(err, c.sentinel) {
			t.Errorf("status %d should map to %v; got Is=false", c.status, c.sentinel)
		}
	}
}

func TestAPIError_UnknownStatusMapsToUpstream(t *testing.T) {
	err := &APIError{StatusCode: 418}
	if !errors.Is(err, domain.ErrUpstream) {
		t.Error("unknown status should map to ErrUpstream")
	}
}
```

`internal/adapter/organizze/testhelper_test.go`:

```go
package organizze

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestExecutor spins up an httptest.Server backed by handler and returns
// a fully-wired RequestExecutor pointing at it.
func newTestExecutor(t *testing.T, handler http.HandlerFunc) (*RequestExecutor, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	exec, err := NewRequestExecutor(RequestExecutorOptions{
		HTTPClient: NewClient(ClientOptions{}),
		BaseURL:    ts.URL,
		Email:      "test@example.com",
		APIKey:     "test-key",
		UserAgent:  "Test (test@example.com)",
	})
	if err != nil {
		t.Fatalf("NewRequestExecutor: %v", err)
	}
	return exec, ts
}
```

`internal/adapter/organizze/executor_test.go`:

```go
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

	var out struct{ OK bool `json:"ok"` }
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
	type body struct{ Hello string `json:"hello"` }
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
	if err := exec.Delete(context.Background(), "/x/1"); err != nil {
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
	var out struct{ X int `json:"x"` }
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
```

- [ ] **Step 6: Run, verify failure**

```bash
go test ./internal/adapter/organizze/...
```

Expected: undefined symbols.

- [ ] **Step 7: Write `errors.go`**

`internal/adapter/organizze/errors.go`:

```go
package organizze

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// APIError is returned by the executor for any non-2xx Organizze response.
// Use errors.As to inspect details; errors.Is matches it to domain sentinels.
type APIError struct {
	StatusCode int    // HTTP status
	Message    string // best-effort message pulled from the JSON body
	Body       string // raw body (truncated to 1 KiB)
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("organizze: %d %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("organizze: %d", e.StatusCode)
}

// Is supports errors.Is(err, domain.ErrXxx) — maps HTTP status to domain sentinels.
func (e *APIError) Is(target error) bool {
	switch e.StatusCode {
	case http.StatusNotFound:
		return target == domain.ErrNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return target == domain.ErrUnauthorized
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return target == domain.ErrValidation
	default:
		return target == domain.ErrUpstream
	}
}

// parseAPIError reads a non-2xx response and constructs an APIError.
func parseAPIError(resp *http.Response) *APIError {
	const maxBody = 1 << 10
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	apiErr := &APIError{StatusCode: resp.StatusCode, Body: string(raw)}

	var payload map[string]any
	if json.Unmarshal(raw, &payload) == nil {
		if m, ok := payload["message"].(string); ok && m != "" {
			apiErr.Message = m
		} else if m, ok := payload["error"].(string); ok && m != "" {
			apiErr.Message = m
		}
	}
	return apiErr
}
```

- [ ] **Step 8: Write `executor.go`**

`internal/adapter/organizze/executor.go`:

```go
package organizze

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// RequestExecutorOptions configures a RequestExecutor.
type RequestExecutorOptions struct {
	HTTPClient HTTPClient // required
	BaseURL    string     // required (no trailing slash)
	Email      string     // required (Basic-Auth username)
	APIKey     string     // required (Basic-Auth password)
	UserAgent  string     // required (Organizze rejects requests without it)
}

// RequestExecutor encapsulates Basic Auth, User-Agent, JSON marshaling,
// base-URL composition, and HTTP-error mapping. Repositories call its methods;
// they never construct an *http.Request.
type RequestExecutor struct {
	client    HTTPClient
	baseURL   string
	email     string
	apiKey    string
	userAgent string
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
	return &RequestExecutor{
		client:    opts.HTTPClient,
		baseURL:   opts.BaseURL,
		email:     opts.Email,
		apiKey:    opts.APIKey,
		userAgent: opts.UserAgent,
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

// Delete performs a DELETE; discards any response body.
func (e *RequestExecutor) Delete(ctx context.Context, path string) error {
	return e.do(ctx, http.MethodDelete, path, nil, nil)
}

// do is the single point of contact with the HTTP layer.
func (e *RequestExecutor) do(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("organizze: marshal body: %w", err)
		}
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

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("organizze: do request: %w", err)
	}
	defer resp.Body.Close()

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
```

- [ ] **Step 9: Run all tests in package, verify pass**

```bash
go test ./internal/adapter/organizze/... -v
```

Expected: every test PASS.

- [ ] **Step 10: Commit**

```bash
git add internal/adapter/organizze/
git commit -m "feat(adapter/organizze): HTTPClient interface, RequestExecutor, APIError"
```

---

## Task 5: Adapter/organizze — read-only repositories (TDD)

Implement the 7 read-only repositories: User, Account, Category, Budget, CreditCard, Invoice, Transfer. Each follows the same shape: small struct holding `*RequestExecutor`, methods returning `domain.*` types, tests using the shared httptest helper.

**Files (create one of each):**
- `internal/adapter/organizze/user_repository.go` + `_test.go`
- `internal/adapter/organizze/account_repository.go` + `_test.go`
- `internal/adapter/organizze/category_repository.go` + `_test.go`
- `internal/adapter/organizze/budget_repository.go` + `_test.go`
- `internal/adapter/organizze/credit_card_repository.go` + `_test.go`
- `internal/adapter/organizze/invoice_repository.go` + `_test.go`
- `internal/adapter/organizze/transfer_repository.go` + `_test.go`

- [ ] **Step 1: Write failing tests**

`internal/adapter/organizze/user_repository_test.go`:

```go
package organizze

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestUserRepository_Get(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/3" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":3,"name":"Jorge","email":"j@x.com","role":"admin"}`)
	})

	repo := NewUserRepository(exec)
	u, err := repo.Get(context.Background(), 3)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if u.ID != 3 || u.Name != "Jorge" {
		t.Errorf("got %+v", u)
	}
}
```

`internal/adapter/organizze/account_repository_test.go`:

```go
package organizze

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestAccountRepository_List(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[{"id":1,"name":"Checking","type":"checking","default":true}]`)
	})

	repo := NewAccountRepository(exec)
	accounts, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(accounts) != 1 || accounts[0].Name != "Checking" || !accounts[0].Default {
		t.Errorf("got %+v", accounts)
	}
}

func TestAccountRepository_Get(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts/42" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":42,"name":"Itau"}`)
	})
	repo := NewAccountRepository(exec)
	acc, err := repo.Get(context.Background(), 42)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if acc.ID != 42 || acc.Name != "Itau" {
		t.Errorf("got %+v", acc)
	}
}
```

`internal/adapter/organizze/category_repository_test.go`:

```go
package organizze

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestCategoryRepository_List(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/categories" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[{"id":10,"name":"Food","parent_id":null}]`)
	})
	repo := NewCategoryRepository(exec)
	cats, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(cats) != 1 || cats[0].Name != "Food" {
		t.Errorf("got %+v", cats)
	}
}

func TestCategoryRepository_Get(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/categories/10" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":10,"name":"Food"}`)
	})
	repo := NewCategoryRepository(exec)
	cat, err := repo.Get(context.Background(), 10)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if cat.ID != 10 {
		t.Errorf("got %+v", cat)
	}
}
```

`internal/adapter/organizze/budget_repository_test.go`:

```go
package organizze

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestBudgetRepository_List_CurrentMonth(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/budgets" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[{"amount_in_cents":50000,"category_id":10,"date":"2026-05-01","total":12000,"predicted_total":30000,"percentage":"24"}]`)
	})
	repo := NewBudgetRepository(exec)
	bs, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(bs) != 1 || bs[0].AmountInCents != 50000 {
		t.Errorf("got %+v", bs)
	}
}

func TestBudgetRepository_ListForYear(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/budgets/2026" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[]`)
	})
	repo := NewBudgetRepository(exec)
	if _, err := repo.ListForYear(context.Background(), 2026); err != nil {
		t.Fatalf("ListForYear: %v", err)
	}
}

func TestBudgetRepository_ListForMonth(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/budgets/2026/5" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[]`)
	})
	repo := NewBudgetRepository(exec)
	if _, err := repo.ListForMonth(context.Background(), 2026, 5); err != nil {
		t.Fatalf("ListForMonth: %v", err)
	}
}
```

`internal/adapter/organizze/credit_card_repository_test.go`:

```go
package organizze

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestCreditCardRepository_List(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/credit_cards" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[{"id":1,"name":"Nubank","closing_day":20,"due_day":27,"limit_cents":500000}]`)
	})
	repo := NewCreditCardRepository(exec)
	cards, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(cards) != 1 || cards[0].Name != "Nubank" {
		t.Errorf("got %+v", cards)
	}
}

func TestCreditCardRepository_Get(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/credit_cards/9" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":9,"name":"Inter"}`)
	})
	repo := NewCreditCardRepository(exec)
	card, err := repo.Get(context.Background(), 9)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if card.ID != 9 {
		t.Errorf("got %+v", card)
	}
}
```

`internal/adapter/organizze/invoice_repository_test.go`:

```go
package organizze

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestInvoiceRepository_List(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/credit_cards/9/invoices" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[{"id":100,"credit_card_id":9,"amount_cents":120000}]`)
	})
	repo := NewInvoiceRepository(exec)
	invs, err := repo.List(context.Background(), 9)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(invs) != 1 || invs[0].AmountCents != 120000 {
		t.Errorf("got %+v", invs)
	}
}

func TestInvoiceRepository_Get(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/credit_cards/9/invoices/100" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":100,"credit_card_id":9}`)
	})
	repo := NewInvoiceRepository(exec)
	inv, err := repo.Get(context.Background(), 9, 100)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if inv.ID != 100 {
		t.Errorf("got %+v", inv)
	}
}
```

`internal/adapter/organizze/transfer_repository_test.go`:

```go
package organizze

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

func TestTransferRepository_List_NoFilter(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transfers" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[]`)
	})
	repo := NewTransferRepository(exec)
	if _, err := repo.List(context.Background(), domain.ListTransfersFilter{}); err != nil {
		t.Fatalf("List: %v", err)
	}
}

func TestTransferRepository_List_WithDateRange(t *testing.T) {
	var got url.Values
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = io.WriteString(w, `[]`)
	})
	repo := NewTransferRepository(exec)
	_, err := repo.List(context.Background(), domain.ListTransfersFilter{
		StartDate: "2026-05-01",
		EndDate:   "2026-05-31",
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got.Get("start_date") != "2026-05-01" || got.Get("end_date") != "2026-05-31" {
		t.Errorf("query = %v", got)
	}
}
```

- [ ] **Step 2: Run, verify failures**

```bash
go test ./internal/adapter/organizze/... -run "Repository"
```

Expected: undefined symbols across all seven repos.

- [ ] **Step 3: Implement repositories**

`internal/adapter/organizze/user_repository.go`:

```go
package organizze

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// UserRepository fetches Organizze users.
type UserRepository struct {
	exec *RequestExecutor
}

// NewUserRepository constructs a UserRepository. The returned value satisfies
// usecase.UserRepository implicitly.
func NewUserRepository(exec *RequestExecutor) *UserRepository {
	return &UserRepository{exec: exec}
}

// Get returns the user with the given id.
func (r *UserRepository) Get(ctx context.Context, id int64) (*domain.User, error) {
	var u domain.User
	if err := r.exec.Get(ctx, fmt.Sprintf("/users/%d", id), &u); err != nil {
		return nil, err
	}
	return &u, nil
}
```

`internal/adapter/organizze/account_repository.go`:

```go
package organizze

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// AccountRepository lists and fetches bank/cash accounts.
type AccountRepository struct {
	exec *RequestExecutor
}

func NewAccountRepository(exec *RequestExecutor) *AccountRepository {
	return &AccountRepository{exec: exec}
}

func (r *AccountRepository) List(ctx context.Context) ([]domain.Account, error) {
	var out []domain.Account
	if err := r.exec.Get(ctx, "/accounts", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *AccountRepository) Get(ctx context.Context, id int64) (*domain.Account, error) {
	var a domain.Account
	if err := r.exec.Get(ctx, fmt.Sprintf("/accounts/%d", id), &a); err != nil {
		return nil, err
	}
	return &a, nil
}
```

`internal/adapter/organizze/category_repository.go`:

```go
package organizze

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// CategoryRepository lists and fetches categories.
type CategoryRepository struct {
	exec *RequestExecutor
}

func NewCategoryRepository(exec *RequestExecutor) *CategoryRepository {
	return &CategoryRepository{exec: exec}
}

func (r *CategoryRepository) List(ctx context.Context) ([]domain.Category, error) {
	var out []domain.Category
	if err := r.exec.Get(ctx, "/categories", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *CategoryRepository) Get(ctx context.Context, id int64) (*domain.Category, error) {
	var c domain.Category
	if err := r.exec.Get(ctx, fmt.Sprintf("/categories/%d", id), &c); err != nil {
		return nil, err
	}
	return &c, nil
}
```

`internal/adapter/organizze/budget_repository.go`:

```go
package organizze

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// BudgetRepository lists budgets for the current month, an entire year, or a
// specific year+month.
type BudgetRepository struct {
	exec *RequestExecutor
}

func NewBudgetRepository(exec *RequestExecutor) *BudgetRepository {
	return &BudgetRepository{exec: exec}
}

// List returns budgets for the current month.
func (r *BudgetRepository) List(ctx context.Context) ([]domain.Budget, error) {
	var out []domain.Budget
	if err := r.exec.Get(ctx, "/budgets", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListForYear returns budgets for every month of the given year.
func (r *BudgetRepository) ListForYear(ctx context.Context, year int) ([]domain.Budget, error) {
	var out []domain.Budget
	if err := r.exec.Get(ctx, fmt.Sprintf("/budgets/%d", year), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListForMonth returns budgets for a specific year+month (month 1..12).
func (r *BudgetRepository) ListForMonth(ctx context.Context, year, month int) ([]domain.Budget, error) {
	var out []domain.Budget
	if err := r.exec.Get(ctx, fmt.Sprintf("/budgets/%d/%d", year, month), &out); err != nil {
		return nil, err
	}
	return out, nil
}
```

`internal/adapter/organizze/credit_card_repository.go`:

```go
package organizze

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// CreditCardRepository lists and fetches credit cards.
type CreditCardRepository struct {
	exec *RequestExecutor
}

func NewCreditCardRepository(exec *RequestExecutor) *CreditCardRepository {
	return &CreditCardRepository{exec: exec}
}

func (r *CreditCardRepository) List(ctx context.Context) ([]domain.CreditCard, error) {
	var out []domain.CreditCard
	if err := r.exec.Get(ctx, "/credit_cards", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *CreditCardRepository) Get(ctx context.Context, id int64) (*domain.CreditCard, error) {
	var c domain.CreditCard
	if err := r.exec.Get(ctx, fmt.Sprintf("/credit_cards/%d", id), &c); err != nil {
		return nil, err
	}
	return &c, nil
}
```

`internal/adapter/organizze/invoice_repository.go`:

```go
package organizze

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// InvoiceRepository lists and fetches credit-card invoices.
type InvoiceRepository struct {
	exec *RequestExecutor
}

func NewInvoiceRepository(exec *RequestExecutor) *InvoiceRepository {
	return &InvoiceRepository{exec: exec}
}

func (r *InvoiceRepository) List(ctx context.Context, cardID int64) ([]domain.Invoice, error) {
	var out []domain.Invoice
	if err := r.exec.Get(ctx, fmt.Sprintf("/credit_cards/%d/invoices", cardID), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *InvoiceRepository) Get(ctx context.Context, cardID, invoiceID int64) (*domain.Invoice, error) {
	var inv domain.Invoice
	if err := r.exec.Get(ctx, fmt.Sprintf("/credit_cards/%d/invoices/%d", cardID, invoiceID), &inv); err != nil {
		return nil, err
	}
	return &inv, nil
}
```

`internal/adapter/organizze/transfer_repository.go`:

```go
package organizze

import (
	"context"
	"net/url"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// TransferRepository lists transfers between accounts.
type TransferRepository struct {
	exec *RequestExecutor
}

func NewTransferRepository(exec *RequestExecutor) *TransferRepository {
	return &TransferRepository{exec: exec}
}

func (r *TransferRepository) List(ctx context.Context, f domain.ListTransfersFilter) ([]domain.Transfer, error) {
	q := url.Values{}
	if f.StartDate != "" {
		q.Set("start_date", f.StartDate)
	}
	if f.EndDate != "" {
		q.Set("end_date", f.EndDate)
	}
	path := "/transfers"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var out []domain.Transfer
	if err := r.exec.Get(ctx, path, &out); err != nil {
		return nil, err
	}
	return out, nil
}
```

- [ ] **Step 4: Run all tests in package, verify pass**

```bash
go test ./internal/adapter/organizze/... -v
```

Expected: every test PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/organizze/
git commit -m "feat(adapter/organizze): read-only repositories for all 7 resources"
```

---

## Task 6: Adapter/organizze — TransactionRepository (read + write) (TDD)

**Files:**
- Create: `internal/adapter/organizze/transaction_repository.go`
- Create: `internal/adapter/organizze/transaction_repository_test.go`

- [ ] **Step 1: Write failing tests**

`internal/adapter/organizze/transaction_repository_test.go`:

```go
package organizze

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

func TestTransactionRepository_List_PassesAllFilters(t *testing.T) {
	var got url.Values
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = io.WriteString(w, `[]`)
	})
	repo := NewTransactionRepository(exec)
	_, err := repo.List(context.Background(), domain.ListTransactionsFilter{
		StartDate: "2026-05-01", EndDate: "2026-05-31", AccountID: 7,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got.Get("start_date") != "2026-05-01" ||
		got.Get("end_date") != "2026-05-31" ||
		got.Get("account_id") != "7" {
		t.Errorf("query = %v", got)
	}
}

func TestTransactionRepository_Get(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transactions/55" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":55,"description":"Pizza","amount_cents":-4500}`)
	})
	repo := NewTransactionRepository(exec)
	tx, err := repo.Get(context.Background(), 55)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tx.ID != 55 || tx.AmountCents != -4500 {
		t.Errorf("got %+v", tx)
	}
}

func TestTransactionRepository_Create(t *testing.T) {
	var gotBody domain.CreateTransactionParams
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/transactions" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":777,"description":"Coffee","amount_cents":-1500}`)
	})
	repo := NewTransactionRepository(exec)
	tx, err := repo.Create(context.Background(), domain.CreateTransactionParams{
		Description: "Coffee", Date: "2026-05-14", AmountCents: -1500,
		AccountID: 1, CategoryID: 10, Paid: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tx.ID != 777 {
		t.Errorf("got %+v", tx)
	}
	if gotBody.Description != "Coffee" || gotBody.AccountID != 1 {
		t.Errorf("server received %+v", gotBody)
	}
}

func TestTransactionRepository_Update_SendsOnlySetFields(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/transactions/777" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&raw)
		_, _ = io.WriteString(w, `{"id":777,"description":"Tea"}`)
	})
	repo := NewTransactionRepository(exec)
	desc := "Tea"
	tx, err := repo.Update(context.Background(), 777, domain.UpdateTransactionParams{
		Description: &desc,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if tx.Description != "Tea" {
		t.Errorf("got %+v", tx)
	}
	if _, has := raw["amount_cents"]; has {
		t.Errorf("absent fields must be omitted; body=%v", raw)
	}
	if raw["description"] != "Tea" {
		t.Errorf("body=%v", raw)
	}
}

func TestTransactionRepository_Delete(t *testing.T) {
	called := false
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodDelete || r.URL.Path != "/transactions/777" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	repo := NewTransactionRepository(exec)
	if err := repo.Delete(context.Background(), 777); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !called {
		t.Error("handler not invoked")
	}
}
```

- [ ] **Step 2: Run, verify failure**

```bash
go test ./internal/adapter/organizze/... -run Transaction
```

Expected: undefined symbols.

- [ ] **Step 3: Implement repository**

`internal/adapter/organizze/transaction_repository.go`:

```go
package organizze

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// TransactionRepository handles transaction reads and writes against the
// Organizze API.
type TransactionRepository struct {
	exec *RequestExecutor
}

func NewTransactionRepository(exec *RequestExecutor) *TransactionRepository {
	return &TransactionRepository{exec: exec}
}

// List returns transactions matching filter. Zero-valued fields are omitted.
func (r *TransactionRepository) List(ctx context.Context, f domain.ListTransactionsFilter) ([]domain.Transaction, error) {
	q := url.Values{}
	if f.StartDate != "" {
		q.Set("start_date", f.StartDate)
	}
	if f.EndDate != "" {
		q.Set("end_date", f.EndDate)
	}
	if f.AccountID != 0 {
		q.Set("account_id", strconv.FormatInt(f.AccountID, 10))
	}
	path := "/transactions"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var out []domain.Transaction
	if err := r.exec.Get(ctx, path, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get returns a single transaction by id.
func (r *TransactionRepository) Get(ctx context.Context, id int64) (*domain.Transaction, error) {
	var tx domain.Transaction
	if err := r.exec.Get(ctx, fmt.Sprintf("/transactions/%d", id), &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

// Create issues a POST and returns the persisted transaction.
func (r *TransactionRepository) Create(ctx context.Context, params domain.CreateTransactionParams) (*domain.Transaction, error) {
	var tx domain.Transaction
	if err := r.exec.Post(ctx, "/transactions", params, &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

// Update issues a PUT with only the non-nil fields from params.
func (r *TransactionRepository) Update(ctx context.Context, id int64, params domain.UpdateTransactionParams) (*domain.Transaction, error) {
	var tx domain.Transaction
	if err := r.exec.Put(ctx, fmt.Sprintf("/transactions/%d", id), params, &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

// Delete issues a DELETE.
func (r *TransactionRepository) Delete(ctx context.Context, id int64) error {
	return r.exec.Delete(ctx, fmt.Sprintf("/transactions/%d", id))
}
```

- [ ] **Step 4: Run tests, verify pass**

```bash
go test ./internal/adapter/organizze/... -v
```

Expected: all PASS, including the entire package suite.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/organizze/transaction_repository.go internal/adapter/organizze/transaction_repository_test.go
git commit -m "feat(adapter/organizze): TransactionRepository with read+write"
```

---

## Task 7: Usecase services + repository interfaces (TDD)

The use case layer defines:
1. **Repository interfaces** — consumer-owned, per ISP. The HTTP repos from Tasks 5–6 implicitly satisfy them.
2. **Service structs** — orchestrate the repositories, contain any business logic.

Services for read-only resources are thin delegates; `BudgetService` and `TransactionService` carry actual logic (period routing and basic validation respectively).

**Files (8 pairs, one per resource):**
- `internal/usecase/user.go` + `_test.go`
- `internal/usecase/account.go` + `_test.go`
- `internal/usecase/category.go` + `_test.go`
- `internal/usecase/budget.go` + `_test.go`
- `internal/usecase/credit_card.go` + `_test.go`
- `internal/usecase/invoice.go` + `_test.go`
- `internal/usecase/transfer.go` + `_test.go`
- `internal/usecase/transaction.go` + `_test.go`

- [ ] **Step 1: Write failing tests with fake repos**

Each test file defines a tiny fake satisfying the consumer interface, then verifies the service routes inputs correctly.

`internal/usecase/user_test.go`:

```go
package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeUserRepo struct {
	gotID int64
	user  *domain.User
	err   error
}

func (f *fakeUserRepo) Get(_ context.Context, id int64) (*domain.User, error) {
	f.gotID = id
	return f.user, f.err
}

func TestUserService_Get_DelegatesToRepo(t *testing.T) {
	repo := &fakeUserRepo{user: &domain.User{ID: 7, Name: "Jorge"}}
	svc := NewUserService(repo)
	got, err := svc.Get(context.Background(), 7)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != 7 || repo.gotID != 7 {
		t.Errorf("svc=%+v repo=%+v", got, repo)
	}
}

func TestUserService_Get_PropagatesError(t *testing.T) {
	want := errors.New("boom")
	repo := &fakeUserRepo{err: want}
	svc := NewUserService(repo)
	if _, err := svc.Get(context.Background(), 1); !errors.Is(err, want) {
		t.Errorf("err = %v, want wraps %v", err, want)
	}
}
```

`internal/usecase/account_test.go`:

```go
package usecase

import (
	"context"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeAccountRepo struct {
	list []domain.Account
	one  *domain.Account
}

func (f *fakeAccountRepo) List(context.Context) ([]domain.Account, error) {
	return f.list, nil
}
func (f *fakeAccountRepo) Get(_ context.Context, _ int64) (*domain.Account, error) {
	return f.one, nil
}

func TestAccountService_DelegatesBothCalls(t *testing.T) {
	repo := &fakeAccountRepo{
		list: []domain.Account{{ID: 1, Name: "Checking"}},
		one:  &domain.Account{ID: 1, Name: "Checking"},
	}
	svc := NewAccountService(repo)

	xs, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(xs) != 1 {
		t.Errorf("got %d accounts", len(xs))
	}

	a, err := svc.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if a.ID != 1 {
		t.Errorf("got %+v", a)
	}
}
```

`internal/usecase/category_test.go`:

```go
package usecase

import (
	"context"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeCategoryRepo struct {
	list []domain.Category
	one  *domain.Category
}

func (f *fakeCategoryRepo) List(context.Context) ([]domain.Category, error) {
	return f.list, nil
}
func (f *fakeCategoryRepo) Get(context.Context, int64) (*domain.Category, error) {
	return f.one, nil
}

func TestCategoryService_DelegatesBothCalls(t *testing.T) {
	repo := &fakeCategoryRepo{
		list: []domain.Category{{ID: 10, Name: "Food"}},
		one:  &domain.Category{ID: 10, Name: "Food"},
	}
	svc := NewCategoryService(repo)
	if xs, _ := svc.List(context.Background()); len(xs) != 1 {
		t.Errorf("List: %v", xs)
	}
	if c, _ := svc.Get(context.Background(), 10); c.ID != 10 {
		t.Errorf("Get: %+v", c)
	}
}
```

`internal/usecase/budget_test.go`:

```go
package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeBudgetRepo struct {
	called string // "list" | "year" | "month"
	year   int
	month  int
}

func (f *fakeBudgetRepo) List(context.Context) ([]domain.Budget, error) {
	f.called = "list"
	return nil, nil
}
func (f *fakeBudgetRepo) ListForYear(_ context.Context, y int) ([]domain.Budget, error) {
	f.called = "year"
	f.year = y
	return nil, nil
}
func (f *fakeBudgetRepo) ListForMonth(_ context.Context, y, m int) ([]domain.Budget, error) {
	f.called = "month"
	f.year = y
	f.month = m
	return nil, nil
}

func TestBudgetService_RoutesByPeriod(t *testing.T) {
	cases := []struct {
		name   string
		period domain.BudgetPeriod
		want   string
	}{
		{"current", domain.BudgetPeriod{}, "list"},
		{"year", domain.BudgetPeriod{Year: 2026}, "year"},
		{"month", domain.BudgetPeriod{Year: 2026, Month: 5}, "month"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &fakeBudgetRepo{}
			svc := NewBudgetService(repo)
			if _, err := svc.List(context.Background(), c.period); err != nil {
				t.Fatalf("List: %v", err)
			}
			if repo.called != c.want {
				t.Errorf("called = %q, want %q", repo.called, c.want)
			}
		})
	}
}

func TestBudgetService_RejectsMonthWithoutYear(t *testing.T) {
	repo := &fakeBudgetRepo{}
	svc := NewBudgetService(repo)
	_, err := svc.List(context.Background(), domain.BudgetPeriod{Month: 5})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err should wrap ErrValidation; got %v", err)
	}
	if repo.called != "" {
		t.Errorf("repo should not have been called; got %q", repo.called)
	}
}
```

`internal/usecase/credit_card_test.go`:

```go
package usecase

import (
	"context"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeCardRepo struct {
	list []domain.CreditCard
	one  *domain.CreditCard
}

func (f *fakeCardRepo) List(context.Context) ([]domain.CreditCard, error) {
	return f.list, nil
}
func (f *fakeCardRepo) Get(context.Context, int64) (*domain.CreditCard, error) {
	return f.one, nil
}

func TestCreditCardService(t *testing.T) {
	repo := &fakeCardRepo{
		list: []domain.CreditCard{{ID: 1, Name: "Nubank"}},
		one:  &domain.CreditCard{ID: 1, Name: "Nubank"},
	}
	svc := NewCreditCardService(repo)
	if xs, _ := svc.List(context.Background()); len(xs) != 1 {
		t.Errorf("List: %v", xs)
	}
	if c, _ := svc.Get(context.Background(), 1); c.ID != 1 {
		t.Errorf("Get: %+v", c)
	}
}
```

`internal/usecase/invoice_test.go`:

```go
package usecase

import (
	"context"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeInvoiceRepo struct{}

func (f *fakeInvoiceRepo) List(_ context.Context, _ int64) ([]domain.Invoice, error) {
	return []domain.Invoice{{ID: 1}}, nil
}
func (f *fakeInvoiceRepo) Get(_ context.Context, _ int64, _ int64) (*domain.Invoice, error) {
	return &domain.Invoice{ID: 1}, nil
}

func TestInvoiceService(t *testing.T) {
	svc := NewInvoiceService(&fakeInvoiceRepo{})
	if xs, _ := svc.List(context.Background(), 9); len(xs) != 1 {
		t.Errorf("List: %v", xs)
	}
	if v, _ := svc.Get(context.Background(), 9, 1); v.ID != 1 {
		t.Errorf("Get: %+v", v)
	}
}
```

`internal/usecase/transfer_test.go`:

```go
package usecase

import (
	"context"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeTransferRepo struct {
	gotFilter domain.ListTransfersFilter
}

func (f *fakeTransferRepo) List(_ context.Context, fl domain.ListTransfersFilter) ([]domain.Transfer, error) {
	f.gotFilter = fl
	return []domain.Transfer{}, nil
}

func TestTransferService_PassesFilter(t *testing.T) {
	repo := &fakeTransferRepo{}
	svc := NewTransferService(repo)
	_, err := svc.List(context.Background(), domain.ListTransfersFilter{StartDate: "2026-05-01"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if repo.gotFilter.StartDate != "2026-05-01" {
		t.Errorf("filter not forwarded: %+v", repo.gotFilter)
	}
}
```

`internal/usecase/transaction_test.go`:

```go
package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeTransactionRepo struct {
	listFilter domain.ListTransactionsFilter
	created    domain.CreateTransactionParams
	updatedID  int64
	deletedID  int64
}

func (f *fakeTransactionRepo) List(_ context.Context, fl domain.ListTransactionsFilter) ([]domain.Transaction, error) {
	f.listFilter = fl
	return nil, nil
}
func (f *fakeTransactionRepo) Get(_ context.Context, id int64) (*domain.Transaction, error) {
	return &domain.Transaction{ID: id}, nil
}
func (f *fakeTransactionRepo) Create(_ context.Context, p domain.CreateTransactionParams) (*domain.Transaction, error) {
	f.created = p
	return &domain.Transaction{ID: 777, Description: p.Description, AmountCents: p.AmountCents}, nil
}
func (f *fakeTransactionRepo) Update(_ context.Context, id int64, _ domain.UpdateTransactionParams) (*domain.Transaction, error) {
	f.updatedID = id
	return &domain.Transaction{ID: id}, nil
}
func (f *fakeTransactionRepo) Delete(_ context.Context, id int64) error {
	f.deletedID = id
	return nil
}

func TestTransactionService_ListAndGet(t *testing.T) {
	repo := &fakeTransactionRepo{}
	svc := NewTransactionService(repo)
	if _, err := svc.List(context.Background(), domain.ListTransactionsFilter{AccountID: 5}); err != nil {
		t.Fatalf("List: %v", err)
	}
	if repo.listFilter.AccountID != 5 {
		t.Errorf("filter not forwarded: %+v", repo.listFilter)
	}
	if tx, _ := svc.Get(context.Background(), 9); tx.ID != 9 {
		t.Errorf("Get: %+v", tx)
	}
}

func TestTransactionService_Create_ValidatesRequiredFields(t *testing.T) {
	svc := NewTransactionService(&fakeTransactionRepo{})
	cases := []domain.CreateTransactionParams{
		{}, // all zero
		{Description: "x"},
		{Description: "x", Date: "2026-05-14"},
		{Description: "x", Date: "2026-05-14", AccountID: 1},
		{Description: "x", Date: "2026-05-14", AccountID: 1, CategoryID: 2}, // AmountCents == 0
	}
	for i, p := range cases {
		_, err := svc.Create(context.Background(), p)
		if !errors.Is(err, domain.ErrValidation) {
			t.Errorf("case %d: err=%v, want ErrValidation", i, err)
		}
	}
}

func TestTransactionService_Create_Succeeds(t *testing.T) {
	repo := &fakeTransactionRepo{}
	svc := NewTransactionService(repo)
	tx, err := svc.Create(context.Background(), domain.CreateTransactionParams{
		Description: "Coffee", Date: "2026-05-14", AmountCents: -1500,
		AccountID: 1, CategoryID: 10, Paid: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tx.ID != 777 {
		t.Errorf("got %+v", tx)
	}
	if repo.created.AmountCents != -1500 {
		t.Errorf("repo received %+v", repo.created)
	}
}

func TestTransactionService_UpdateDelete(t *testing.T) {
	repo := &fakeTransactionRepo{}
	svc := NewTransactionService(repo)
	if _, err := svc.Update(context.Background(), 42, domain.UpdateTransactionParams{}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if repo.updatedID != 42 {
		t.Errorf("repo.updatedID = %d", repo.updatedID)
	}
	if err := svc.Delete(context.Background(), 42); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.deletedID != 42 {
		t.Errorf("repo.deletedID = %d", repo.deletedID)
	}
}
```

- [ ] **Step 2: Run, verify failures**

```bash
go test ./internal/usecase/...
```

Expected: undefined types/constructors.

- [ ] **Step 3: Implement services**

`internal/usecase/user.go`:

```go
// Package usecase contains application services and the repository interfaces
// they consume. Repositories are defined here (the consumer) and implemented
// by outer-layer packages; Go's implicit interface satisfaction handles the
// inversion automatically.
package usecase

import (
	"context"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// UserRepository is the consumer-owned port for User reads.
type UserRepository interface {
	Get(ctx context.Context, id int64) (*domain.User, error)
}

// UserService exposes user operations to outer layers.
type UserService struct {
	repo UserRepository
}

func NewUserService(repo UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) Get(ctx context.Context, id int64) (*domain.User, error) {
	return s.repo.Get(ctx, id)
}
```

`internal/usecase/account.go`:

```go
package usecase

import (
	"context"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type AccountRepository interface {
	List(ctx context.Context) ([]domain.Account, error)
	Get(ctx context.Context, id int64) (*domain.Account, error)
}

type AccountService struct {
	repo AccountRepository
}

func NewAccountService(repo AccountRepository) *AccountService {
	return &AccountService{repo: repo}
}

func (s *AccountService) List(ctx context.Context) ([]domain.Account, error) {
	return s.repo.List(ctx)
}

func (s *AccountService) Get(ctx context.Context, id int64) (*domain.Account, error) {
	return s.repo.Get(ctx, id)
}
```

`internal/usecase/category.go`:

```go
package usecase

import (
	"context"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type CategoryRepository interface {
	List(ctx context.Context) ([]domain.Category, error)
	Get(ctx context.Context, id int64) (*domain.Category, error)
}

type CategoryService struct {
	repo CategoryRepository
}

func NewCategoryService(repo CategoryRepository) *CategoryService {
	return &CategoryService{repo: repo}
}

func (s *CategoryService) List(ctx context.Context) ([]domain.Category, error) {
	return s.repo.List(ctx)
}

func (s *CategoryService) Get(ctx context.Context, id int64) (*domain.Category, error) {
	return s.repo.Get(ctx, id)
}
```

`internal/usecase/budget.go`:

```go
package usecase

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// BudgetRepository exposes the three list flavors of the Organizze budgets API.
type BudgetRepository interface {
	List(ctx context.Context) ([]domain.Budget, error)
	ListForYear(ctx context.Context, year int) ([]domain.Budget, error)
	ListForMonth(ctx context.Context, year, month int) ([]domain.Budget, error)
}

// BudgetService routes the period requested by the caller to the right
// repository method. Month without Year is a validation error.
type BudgetService struct {
	repo BudgetRepository
}

func NewBudgetService(repo BudgetRepository) *BudgetService {
	return &BudgetService{repo: repo}
}

// List returns budgets for the period selected by p.
func (s *BudgetService) List(ctx context.Context, p domain.BudgetPeriod) ([]domain.Budget, error) {
	if p.Month != 0 && p.Year == 0 {
		return nil, fmt.Errorf("%w: month requires year", domain.ErrValidation)
	}
	switch {
	case p.Year == 0:
		return s.repo.List(ctx)
	case p.Month == 0:
		return s.repo.ListForYear(ctx, p.Year)
	default:
		if p.Month < 1 || p.Month > 12 {
			return nil, fmt.Errorf("%w: month must be 1..12", domain.ErrValidation)
		}
		return s.repo.ListForMonth(ctx, p.Year, p.Month)
	}
}
```

`internal/usecase/credit_card.go`:

```go
package usecase

import (
	"context"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type CreditCardRepository interface {
	List(ctx context.Context) ([]domain.CreditCard, error)
	Get(ctx context.Context, id int64) (*domain.CreditCard, error)
}

type CreditCardService struct {
	repo CreditCardRepository
}

func NewCreditCardService(repo CreditCardRepository) *CreditCardService {
	return &CreditCardService{repo: repo}
}

func (s *CreditCardService) List(ctx context.Context) ([]domain.CreditCard, error) {
	return s.repo.List(ctx)
}

func (s *CreditCardService) Get(ctx context.Context, id int64) (*domain.CreditCard, error) {
	return s.repo.Get(ctx, id)
}
```

`internal/usecase/invoice.go`:

```go
package usecase

import (
	"context"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type InvoiceRepository interface {
	List(ctx context.Context, creditCardID int64) ([]domain.Invoice, error)
	Get(ctx context.Context, creditCardID, invoiceID int64) (*domain.Invoice, error)
}

type InvoiceService struct {
	repo InvoiceRepository
}

func NewInvoiceService(repo InvoiceRepository) *InvoiceService {
	return &InvoiceService{repo: repo}
}

func (s *InvoiceService) List(ctx context.Context, creditCardID int64) ([]domain.Invoice, error) {
	return s.repo.List(ctx, creditCardID)
}

func (s *InvoiceService) Get(ctx context.Context, creditCardID, invoiceID int64) (*domain.Invoice, error) {
	return s.repo.Get(ctx, creditCardID, invoiceID)
}
```

`internal/usecase/transfer.go`:

```go
package usecase

import (
	"context"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type TransferRepository interface {
	List(ctx context.Context, filter domain.ListTransfersFilter) ([]domain.Transfer, error)
}

type TransferService struct {
	repo TransferRepository
}

func NewTransferService(repo TransferRepository) *TransferService {
	return &TransferService{repo: repo}
}

func (s *TransferService) List(ctx context.Context, filter domain.ListTransfersFilter) ([]domain.Transfer, error) {
	return s.repo.List(ctx, filter)
}
```

`internal/usecase/transaction.go`:

```go
package usecase

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// TransactionReader is the read-only slice of TransactionRepository.
type TransactionReader interface {
	List(ctx context.Context, filter domain.ListTransactionsFilter) ([]domain.Transaction, error)
	Get(ctx context.Context, id int64) (*domain.Transaction, error)
}

// TransactionWriter is the mutating slice of TransactionRepository.
type TransactionWriter interface {
	Create(ctx context.Context, params domain.CreateTransactionParams) (*domain.Transaction, error)
	Update(ctx context.Context, id int64, params domain.UpdateTransactionParams) (*domain.Transaction, error)
	Delete(ctx context.Context, id int64) error
}

// TransactionRepository composes reader and writer for callers that need both.
type TransactionRepository interface {
	TransactionReader
	TransactionWriter
}

// TransactionService orchestrates transaction operations. Create validates that
// the four required Organizze fields are present before hitting the repo.
type TransactionService struct {
	repo TransactionRepository
}

func NewTransactionService(repo TransactionRepository) *TransactionService {
	return &TransactionService{repo: repo}
}

func (s *TransactionService) List(ctx context.Context, filter domain.ListTransactionsFilter) ([]domain.Transaction, error) {
	return s.repo.List(ctx, filter)
}

func (s *TransactionService) Get(ctx context.Context, id int64) (*domain.Transaction, error) {
	return s.repo.Get(ctx, id)
}

func (s *TransactionService) Create(ctx context.Context, p domain.CreateTransactionParams) (*domain.Transaction, error) {
	if err := validateCreate(p); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, p)
}

func (s *TransactionService) Update(ctx context.Context, id int64, p domain.UpdateTransactionParams) (*domain.Transaction, error) {
	return s.repo.Update(ctx, id, p)
}

func (s *TransactionService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func validateCreate(p domain.CreateTransactionParams) error {
	switch {
	case p.Description == "":
		return fmt.Errorf("%w: description is required", domain.ErrValidation)
	case p.Date == "":
		return fmt.Errorf("%w: date is required", domain.ErrValidation)
	case p.AmountCents == 0:
		return fmt.Errorf("%w: amount_cents must be non-zero", domain.ErrValidation)
	case p.AccountID == 0:
		return fmt.Errorf("%w: account_id is required", domain.ErrValidation)
	case p.CategoryID == 0:
		return fmt.Errorf("%w: category_id is required", domain.ErrValidation)
	}
	return nil
}
```

- [ ] **Step 4: Run all tests, verify pass**

```bash
go test ./internal/usecase/... -v
```

Expected: every test PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/usecase/
git commit -m "feat(usecase): services + repository interfaces (ports) for every resource"
```

---

## Task 8: Adapter/mcp — server scaffold + tool families (TDD)

The MCP adapter exposes use case services as MCP tools. Each tool family file:
1. Declares input/output types with `jsonschema` tags.
2. Defines a small consumer-side service interface (ISP — only what this file calls).
3. Provides a handler factory `xxxHandler(svc XxxService) mcp.ToolHandlerFor[In, Out]`.
4. Provides a `registerXxxTools(s *mcp.Server, svc XxxService)` that calls `mcp.AddTool` per tool.

Tests use **fake services** (no HTTP). The integration test in Task 11 wires the real stack.

**Files:**
- Create: `internal/adapter/mcp/server.go` + `server_test.go`
- Create: `internal/adapter/mcp/tools_user.go` + `_test.go`
- Create: `internal/adapter/mcp/tools_accounts.go` + `_test.go`
- Create: `internal/adapter/mcp/tools_categories.go` + `_test.go`
- Create: `internal/adapter/mcp/tools_budgets.go` + `_test.go`

- [ ] **Step 1: Write failing test for the server constructor**

`internal/adapter/mcp/server_test.go`:

```go
package mcp

import "testing"

func TestNew_BuildsServerWithoutPanic(t *testing.T) {
	deps := Dependencies{
		User:        nopUserSvc{},
		Account:     nopAccountSvc{},
		Category:    nopCategorySvc{},
		Budget:      nopBudgetSvc{},
		CreditCard:  nopCreditCardSvc{},
		Invoice:     nopInvoiceSvc{},
		Transfer:    nopTransferSvc{},
		Transaction: nopTransactionSvc{},
	}
	s := New(deps)
	if s == nil {
		t.Fatal("New returned nil")
	}
}
```

> **Note:** the `nop*Svc` types are defined in the per-tool-family test files below. Each test file declares the nop satisfying the interface for that family. Go's package-level visibility means they're shared across `*_test.go` in the same package.

`internal/adapter/mcp/tools_user_test.go`:

```go
package mcp

import (
	"context"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeUserSvc struct {
	gotID int64
	user  *domain.User
}

func (f *fakeUserSvc) Get(_ context.Context, id int64) (*domain.User, error) {
	f.gotID = id
	return f.user, nil
}

type nopUserSvc struct{}

func (nopUserSvc) Get(context.Context, int64) (*domain.User, error) { return &domain.User{}, nil }

func TestGetUserHandler(t *testing.T) {
	svc := &fakeUserSvc{user: &domain.User{ID: 3, Name: "Jorge"}}
	h := getUserHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, GetUserInput{ID: 3})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.User.ID != 3 || svc.gotID != 3 {
		t.Errorf("out=%+v svc=%+v", out, svc)
	}
}
```

`internal/adapter/mcp/tools_accounts_test.go`:

```go
package mcp

import (
	"context"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeAccountSvc struct {
	list []domain.Account
	one  *domain.Account
}

func (f *fakeAccountSvc) List(context.Context) ([]domain.Account, error) { return f.list, nil }
func (f *fakeAccountSvc) Get(context.Context, int64) (*domain.Account, error) { return f.one, nil }

type nopAccountSvc struct{}

func (nopAccountSvc) List(context.Context) ([]domain.Account, error)            { return nil, nil }
func (nopAccountSvc) Get(context.Context, int64) (*domain.Account, error)       { return &domain.Account{}, nil }

func TestListAccountsHandler(t *testing.T) {
	svc := &fakeAccountSvc{list: []domain.Account{{ID: 1, Name: "Checking"}}}
	h := listAccountsHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, struct{}{})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(out.Accounts) != 1 || out.Accounts[0].Name != "Checking" {
		t.Errorf("got %+v", out)
	}
}

func TestGetAccountHandler(t *testing.T) {
	svc := &fakeAccountSvc{one: &domain.Account{ID: 42, Name: "Itau"}}
	h := getAccountHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, GetAccountInput{ID: 42})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Account.ID != 42 {
		t.Errorf("got %+v", out)
	}
}
```

`internal/adapter/mcp/tools_categories_test.go`:

```go
package mcp

import (
	"context"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeCategorySvc struct {
	list []domain.Category
	one  *domain.Category
}

func (f *fakeCategorySvc) List(context.Context) ([]domain.Category, error)         { return f.list, nil }
func (f *fakeCategorySvc) Get(context.Context, int64) (*domain.Category, error)    { return f.one, nil }

type nopCategorySvc struct{}

func (nopCategorySvc) List(context.Context) ([]domain.Category, error)             { return nil, nil }
func (nopCategorySvc) Get(context.Context, int64) (*domain.Category, error)        { return &domain.Category{}, nil }

func TestCategoryHandlers(t *testing.T) {
	svc := &fakeCategorySvc{
		list: []domain.Category{{ID: 10, Name: "Food"}},
		one:  &domain.Category{ID: 10, Name: "Food"},
	}
	hList := listCategoriesHandler(svc)
	if _, out, err := hList(context.Background(), &mcpsdk.CallToolRequest{}, struct{}{}); err != nil || len(out.Categories) != 1 {
		t.Fatalf("list: out=%+v err=%v", out, err)
	}
	hGet := getCategoryHandler(svc)
	if _, out, err := hGet(context.Background(), &mcpsdk.CallToolRequest{}, GetCategoryInput{ID: 10}); err != nil || out.Category.ID != 10 {
		t.Fatalf("get: out=%+v err=%v", out, err)
	}
}
```

`internal/adapter/mcp/tools_budgets_test.go`:

```go
package mcp

import (
	"context"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeBudgetSvc struct {
	gotPeriod domain.BudgetPeriod
}

func (f *fakeBudgetSvc) List(_ context.Context, p domain.BudgetPeriod) ([]domain.Budget, error) {
	f.gotPeriod = p
	return nil, nil
}

type nopBudgetSvc struct{}

func (nopBudgetSvc) List(context.Context, domain.BudgetPeriod) ([]domain.Budget, error) {
	return nil, nil
}

func TestListBudgetsHandler_ForwardsPeriod(t *testing.T) {
	svc := &fakeBudgetSvc{}
	h := listBudgetsHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, ListBudgetsInput{Year: 2026, Month: 5})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if svc.gotPeriod.Year != 2026 || svc.gotPeriod.Month != 5 {
		t.Errorf("forwarded = %+v", svc.gotPeriod)
	}
}
```

- [ ] **Step 2: Run, verify failure**

```bash
go test ./internal/adapter/mcp/...
```

Expected: undefined symbols across the new files.

- [ ] **Step 3: Implement `server.go`**

`internal/adapter/mcp/server.go`:

```go
// Package mcp is the MCP adapter — it composes use-case services into MCP tools.
// It depends on usecase (interfaces it owns locally) and domain (types crossing
// the boundary). It does NOT import internal/adapter/organizze.
package mcp

import (
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Version is reported via the MCP Implementation block on handshake.
const Version = "0.1.0"

// Dependencies bundles every service the MCP server needs. Each field is a
// small interface defined in the matching tools_*.go file. The composition
// root in cmd/organizze-mcp wires usecase.*Service concretes into these slots.
type Dependencies struct {
	User        UserService
	Account     AccountService
	Category    CategoryService
	Budget      BudgetService
	CreditCard  CreditCardService
	Invoice     InvoiceService
	Transfer    TransferService
	Transaction TransactionService
}

// New builds an *mcp.Server with every Organizze tool registered.
func New(deps Dependencies) *mcpsdk.Server {
	s := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "organizze-mcp",
		Version: Version,
	}, nil)

	registerUserTools(s, deps.User)
	registerAccountTools(s, deps.Account)
	registerCategoryTools(s, deps.Category)
	registerBudgetTools(s, deps.Budget)
	registerCreditCardTools(s, deps.CreditCard)
	registerInvoiceTools(s, deps.Invoice)
	registerTransferTools(s, deps.Transfer)
	registerTransactionTools(s, deps.Transaction)

	return s
}
```

- [ ] **Step 4: Implement `tools_user.go`**

`internal/adapter/mcp/tools_user.go`:

```go
package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// UserService is the consumer-side slice this file needs from usecase.UserService.
type UserService interface {
	Get(ctx context.Context, id int64) (*domain.User, error)
}

type GetUserInput struct {
	ID int64 `json:"id" jsonschema:"The numeric Organizze user id to fetch."`
}

type GetUserOutput struct {
	User domain.User `json:"user"`
}

func getUserHandler(svc UserService) mcpsdk.ToolHandlerFor[GetUserInput, GetUserOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetUserInput) (*mcpsdk.CallToolResult, GetUserOutput, error) {
		u, err := svc.Get(ctx, in.ID)
		if err != nil {
			return nil, GetUserOutput{}, err
		}
		return nil, GetUserOutput{User: *u}, nil
	}
}

func registerUserTools(s *mcpsdk.Server, svc UserService) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_user",
		Description: "Fetch details for an Organizze user by numeric id.",
	}, getUserHandler(svc))
}
```

- [ ] **Step 5: Implement `tools_accounts.go`**

`internal/adapter/mcp/tools_accounts.go`:

```go
package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type AccountService interface {
	List(ctx context.Context) ([]domain.Account, error)
	Get(ctx context.Context, id int64) (*domain.Account, error)
}

type ListAccountsOutput struct {
	Accounts []domain.Account `json:"accounts"`
}

type GetAccountInput struct {
	ID int64 `json:"id" jsonschema:"The numeric Organizze account id."`
}

type GetAccountOutput struct {
	Account domain.Account `json:"account"`
}

func listAccountsHandler(svc AccountService) mcpsdk.ToolHandlerFor[struct{}, ListAccountsOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, ListAccountsOutput, error) {
		accounts, err := svc.List(ctx)
		if err != nil {
			return nil, ListAccountsOutput{}, err
		}
		return nil, ListAccountsOutput{Accounts: accounts}, nil
	}
}

func getAccountHandler(svc AccountService) mcpsdk.ToolHandlerFor[GetAccountInput, GetAccountOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetAccountInput) (*mcpsdk.CallToolResult, GetAccountOutput, error) {
		a, err := svc.Get(ctx, in.ID)
		if err != nil {
			return nil, GetAccountOutput{}, err
		}
		return nil, GetAccountOutput{Account: *a}, nil
	}
}

func registerAccountTools(s *mcpsdk.Server, svc AccountService) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_accounts",
		Description: "List all bank/cash accounts in Organizze.",
	}, listAccountsHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_account",
		Description: "Fetch a single Organizze account by id.",
	}, getAccountHandler(svc))
}
```

- [ ] **Step 6: Implement `tools_categories.go`**

`internal/adapter/mcp/tools_categories.go`:

```go
package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type CategoryService interface {
	List(ctx context.Context) ([]domain.Category, error)
	Get(ctx context.Context, id int64) (*domain.Category, error)
}

type ListCategoriesOutput struct {
	Categories []domain.Category `json:"categories"`
}

type GetCategoryInput struct {
	ID int64 `json:"id" jsonschema:"The numeric Organizze category id."`
}

type GetCategoryOutput struct {
	Category domain.Category `json:"category"`
}

func listCategoriesHandler(svc CategoryService) mcpsdk.ToolHandlerFor[struct{}, ListCategoriesOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, ListCategoriesOutput, error) {
		cats, err := svc.List(ctx)
		if err != nil {
			return nil, ListCategoriesOutput{}, err
		}
		return nil, ListCategoriesOutput{Categories: cats}, nil
	}
}

func getCategoryHandler(svc CategoryService) mcpsdk.ToolHandlerFor[GetCategoryInput, GetCategoryOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetCategoryInput) (*mcpsdk.CallToolResult, GetCategoryOutput, error) {
		cat, err := svc.Get(ctx, in.ID)
		if err != nil {
			return nil, GetCategoryOutput{}, err
		}
		return nil, GetCategoryOutput{Category: *cat}, nil
	}
}

func registerCategoryTools(s *mcpsdk.Server, svc CategoryService) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_categories",
		Description: "List all Organizze categories.",
	}, listCategoriesHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_category",
		Description: "Fetch a single Organizze category by id.",
	}, getCategoryHandler(svc))
}
```

- [ ] **Step 7: Implement `tools_budgets.go`**

`internal/adapter/mcp/tools_budgets.go`:

```go
package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type BudgetService interface {
	List(ctx context.Context, period domain.BudgetPeriod) ([]domain.Budget, error)
}

type ListBudgetsInput struct {
	Year  int `json:"year,omitempty"  jsonschema:"Optional year, e.g. 2026. Omit for current month."`
	Month int `json:"month,omitempty" jsonschema:"Optional month 1..12. Requires year."`
}

type ListBudgetsOutput struct {
	Budgets []domain.Budget `json:"budgets"`
}

func listBudgetsHandler(svc BudgetService) mcpsdk.ToolHandlerFor[ListBudgetsInput, ListBudgetsOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in ListBudgetsInput) (*mcpsdk.CallToolResult, ListBudgetsOutput, error) {
		budgets, err := svc.List(ctx, domain.BudgetPeriod{Year: in.Year, Month: in.Month})
		if err != nil {
			return nil, ListBudgetsOutput{}, err
		}
		return nil, ListBudgetsOutput{Budgets: budgets}, nil
	}
}

func registerBudgetTools(s *mcpsdk.Server, svc BudgetService) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_budgets",
		Description: "List Organizze budgets. With no args, returns the current month. Provide year for an annual view, or year+month for a specific month.",
	}, listBudgetsHandler(svc))
}
```

> **Note on `New()` calling functions not yet defined:** `server.go` calls `registerCreditCardTools`, `registerInvoiceTools`, `registerTransferTools`, and `registerTransactionTools` which are added in Tasks 9 and 10. Until those tasks complete, the package will fail to build. To run *this* task's tests in isolation, temporarily comment out the four pending `register*Tools` calls in `server.go`, run the tests, then uncomment them before moving on. The integration test in Task 11 is the gate that requires all eight families wired.

- [ ] **Step 8: Run tests with the temporary comment-out**

```bash
go test ./internal/adapter/mcp/... -v
```

Expected: tests for user/accounts/categories/budgets PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/adapter/mcp/
git commit -m "feat(adapter/mcp): server scaffold and 4 tool families (user, accounts, categories, budgets)"
```

---

## Task 9: Adapter/mcp — credit cards, invoices, transfers (TDD)

**Files:**
- Create: `internal/adapter/mcp/tools_credit_cards.go` + `_test.go`
- Create: `internal/adapter/mcp/tools_invoices.go` + `_test.go`
- Create: `internal/adapter/mcp/tools_transfers.go` + `_test.go`

- [ ] **Step 1: Write failing tests**

`internal/adapter/mcp/tools_credit_cards_test.go`:

```go
package mcp

import (
	"context"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeCreditCardSvc struct {
	list []domain.CreditCard
	one  *domain.CreditCard
}

func (f *fakeCreditCardSvc) List(context.Context) ([]domain.CreditCard, error) { return f.list, nil }
func (f *fakeCreditCardSvc) Get(context.Context, int64) (*domain.CreditCard, error) { return f.one, nil }

type nopCreditCardSvc struct{}

func (nopCreditCardSvc) List(context.Context) ([]domain.CreditCard, error) { return nil, nil }
func (nopCreditCardSvc) Get(context.Context, int64) (*domain.CreditCard, error) {
	return &domain.CreditCard{}, nil
}

func TestCreditCardHandlers(t *testing.T) {
	svc := &fakeCreditCardSvc{
		list: []domain.CreditCard{{ID: 1, Name: "Nubank"}},
		one:  &domain.CreditCard{ID: 1, Name: "Nubank"},
	}
	hList := listCreditCardsHandler(svc)
	if _, out, err := hList(context.Background(), &mcpsdk.CallToolRequest{}, struct{}{}); err != nil || len(out.CreditCards) != 1 {
		t.Fatalf("list: out=%+v err=%v", out, err)
	}
	hGet := getCreditCardHandler(svc)
	if _, out, err := hGet(context.Background(), &mcpsdk.CallToolRequest{}, GetCreditCardInput{ID: 1}); err != nil || out.CreditCard.ID != 1 {
		t.Fatalf("get: out=%+v err=%v", out, err)
	}
}
```

`internal/adapter/mcp/tools_invoices_test.go`:

```go
package mcp

import (
	"context"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeInvoiceSvc struct {
	gotCard, gotInvoice int64
}

func (f *fakeInvoiceSvc) List(_ context.Context, cardID int64) ([]domain.Invoice, error) {
	f.gotCard = cardID
	return []domain.Invoice{{ID: 100}}, nil
}
func (f *fakeInvoiceSvc) Get(_ context.Context, cardID, invID int64) (*domain.Invoice, error) {
	f.gotCard, f.gotInvoice = cardID, invID
	return &domain.Invoice{ID: invID}, nil
}

type nopInvoiceSvc struct{}

func (nopInvoiceSvc) List(context.Context, int64) ([]domain.Invoice, error)              { return nil, nil }
func (nopInvoiceSvc) Get(context.Context, int64, int64) (*domain.Invoice, error)         { return &domain.Invoice{}, nil }

func TestInvoiceHandlers(t *testing.T) {
	svc := &fakeInvoiceSvc{}
	hList := listInvoicesHandler(svc)
	if _, out, err := hList(context.Background(), &mcpsdk.CallToolRequest{}, ListInvoicesInput{CreditCardID: 9}); err != nil || len(out.Invoices) != 1 {
		t.Fatalf("list: out=%+v err=%v", out, err)
	}
	if svc.gotCard != 9 {
		t.Errorf("svc.gotCard = %d", svc.gotCard)
	}
	hGet := getInvoiceHandler(svc)
	if _, out, err := hGet(context.Background(), &mcpsdk.CallToolRequest{}, GetInvoiceInput{CreditCardID: 9, InvoiceID: 100}); err != nil || out.Invoice.ID != 100 {
		t.Fatalf("get: out=%+v err=%v", out, err)
	}
}
```

`internal/adapter/mcp/tools_transfers_test.go`:

```go
package mcp

import (
	"context"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeTransferSvc struct {
	gotFilter domain.ListTransfersFilter
}

func (f *fakeTransferSvc) List(_ context.Context, fl domain.ListTransfersFilter) ([]domain.Transfer, error) {
	f.gotFilter = fl
	return nil, nil
}

type nopTransferSvc struct{}

func (nopTransferSvc) List(context.Context, domain.ListTransfersFilter) ([]domain.Transfer, error) {
	return nil, nil
}

func TestListTransfersHandler_PassesFilter(t *testing.T) {
	svc := &fakeTransferSvc{}
	h := listTransfersHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, ListTransfersInput{
		StartDate: "2026-05-01", EndDate: "2026-05-31",
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if svc.gotFilter.StartDate != "2026-05-01" || svc.gotFilter.EndDate != "2026-05-31" {
		t.Errorf("filter = %+v", svc.gotFilter)
	}
}
```

- [ ] **Step 2: Run, verify failure**

```bash
go test ./internal/adapter/mcp/...
```

Expected: undefined symbols.

- [ ] **Step 3: Implement `tools_credit_cards.go`**

```go
package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type CreditCardService interface {
	List(ctx context.Context) ([]domain.CreditCard, error)
	Get(ctx context.Context, id int64) (*domain.CreditCard, error)
}

type ListCreditCardsOutput struct {
	CreditCards []domain.CreditCard `json:"credit_cards"`
}

type GetCreditCardInput struct {
	ID int64 `json:"id" jsonschema:"The numeric credit card id."`
}

type GetCreditCardOutput struct {
	CreditCard domain.CreditCard `json:"credit_card"`
}

func listCreditCardsHandler(svc CreditCardService) mcpsdk.ToolHandlerFor[struct{}, ListCreditCardsOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, ListCreditCardsOutput, error) {
		cards, err := svc.List(ctx)
		if err != nil {
			return nil, ListCreditCardsOutput{}, err
		}
		return nil, ListCreditCardsOutput{CreditCards: cards}, nil
	}
}

func getCreditCardHandler(svc CreditCardService) mcpsdk.ToolHandlerFor[GetCreditCardInput, GetCreditCardOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetCreditCardInput) (*mcpsdk.CallToolResult, GetCreditCardOutput, error) {
		card, err := svc.Get(ctx, in.ID)
		if err != nil {
			return nil, GetCreditCardOutput{}, err
		}
		return nil, GetCreditCardOutput{CreditCard: *card}, nil
	}
}

func registerCreditCardTools(s *mcpsdk.Server, svc CreditCardService) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_credit_cards",
		Description: "List all Organizze credit cards.",
	}, listCreditCardsHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_credit_card",
		Description: "Fetch a single Organizze credit card by id.",
	}, getCreditCardHandler(svc))
}
```

- [ ] **Step 4: Implement `tools_invoices.go`**

```go
package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type InvoiceService interface {
	List(ctx context.Context, creditCardID int64) ([]domain.Invoice, error)
	Get(ctx context.Context, creditCardID, invoiceID int64) (*domain.Invoice, error)
}

type ListInvoicesInput struct {
	CreditCardID int64 `json:"credit_card_id" jsonschema:"The numeric credit card id whose invoices to list."`
}

type ListInvoicesOutput struct {
	Invoices []domain.Invoice `json:"invoices"`
}

type GetInvoiceInput struct {
	CreditCardID int64 `json:"credit_card_id" jsonschema:"The numeric credit card id."`
	InvoiceID    int64 `json:"invoice_id"     jsonschema:"The numeric invoice id."`
}

type GetInvoiceOutput struct {
	Invoice domain.Invoice `json:"invoice"`
}

func listInvoicesHandler(svc InvoiceService) mcpsdk.ToolHandlerFor[ListInvoicesInput, ListInvoicesOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in ListInvoicesInput) (*mcpsdk.CallToolResult, ListInvoicesOutput, error) {
		invs, err := svc.List(ctx, in.CreditCardID)
		if err != nil {
			return nil, ListInvoicesOutput{}, err
		}
		return nil, ListInvoicesOutput{Invoices: invs}, nil
	}
}

func getInvoiceHandler(svc InvoiceService) mcpsdk.ToolHandlerFor[GetInvoiceInput, GetInvoiceOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetInvoiceInput) (*mcpsdk.CallToolResult, GetInvoiceOutput, error) {
		inv, err := svc.Get(ctx, in.CreditCardID, in.InvoiceID)
		if err != nil {
			return nil, GetInvoiceOutput{}, err
		}
		return nil, GetInvoiceOutput{Invoice: *inv}, nil
	}
}

func registerInvoiceTools(s *mcpsdk.Server, svc InvoiceService) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_credit_card_invoices",
		Description: "List invoices for a given credit card.",
	}, listInvoicesHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_credit_card_invoice",
		Description: "Fetch a specific credit-card invoice.",
	}, getInvoiceHandler(svc))
}
```

- [ ] **Step 5: Implement `tools_transfers.go`**

```go
package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type TransferService interface {
	List(ctx context.Context, filter domain.ListTransfersFilter) ([]domain.Transfer, error)
}

type ListTransfersInput struct {
	StartDate string `json:"start_date,omitempty" jsonschema:"Optional YYYY-MM-DD lower bound."`
	EndDate   string `json:"end_date,omitempty"   jsonschema:"Optional YYYY-MM-DD upper bound."`
}

type ListTransfersOutput struct {
	Transfers []domain.Transfer `json:"transfers"`
}

func listTransfersHandler(svc TransferService) mcpsdk.ToolHandlerFor[ListTransfersInput, ListTransfersOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in ListTransfersInput) (*mcpsdk.CallToolResult, ListTransfersOutput, error) {
		ts, err := svc.List(ctx, domain.ListTransfersFilter{StartDate: in.StartDate, EndDate: in.EndDate})
		if err != nil {
			return nil, ListTransfersOutput{}, err
		}
		return nil, ListTransfersOutput{Transfers: ts}, nil
	}
}

func registerTransferTools(s *mcpsdk.Server, svc TransferService) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_transfers",
		Description: "List Organizze transfers, optionally filtered by date range.",
	}, listTransfersHandler(svc))
}
```

- [ ] **Step 6: Run tests, verify pass**

```bash
go test ./internal/adapter/mcp/... -v
```

Expected: every PASS. (Transaction handlers still pending — comment them out in `server.go` if you haven't already.)

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/mcp/tools_credit_cards.go internal/adapter/mcp/tools_credit_cards_test.go \
        internal/adapter/mcp/tools_invoices.go     internal/adapter/mcp/tools_invoices_test.go \
        internal/adapter/mcp/tools_transfers.go    internal/adapter/mcp/tools_transfers_test.go
git commit -m "feat(adapter/mcp): credit cards, invoices, transfers tool families"
```

---

## Task 10: Adapter/mcp — transactions (read + write) (TDD)

**Files:**
- Create: `internal/adapter/mcp/tools_transactions.go` + `_test.go`
- Modify: `internal/adapter/mcp/server.go` to ensure `registerTransactionTools` is wired (uncomment if you commented it).

- [ ] **Step 1: Write failing tests**

`internal/adapter/mcp/tools_transactions_test.go`:

```go
package mcp

import (
	"context"
	"errors"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeTransactionSvc struct {
	listFilter domain.ListTransactionsFilter
	created    domain.CreateTransactionParams
	updated    struct {
		id     int64
		params domain.UpdateTransactionParams
	}
	deletedID int64
	createErr error
}

func (f *fakeTransactionSvc) List(_ context.Context, fl domain.ListTransactionsFilter) ([]domain.Transaction, error) {
	f.listFilter = fl
	return []domain.Transaction{{ID: 1}}, nil
}
func (f *fakeTransactionSvc) Get(_ context.Context, id int64) (*domain.Transaction, error) {
	return &domain.Transaction{ID: id}, nil
}
func (f *fakeTransactionSvc) Create(_ context.Context, p domain.CreateTransactionParams) (*domain.Transaction, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.created = p
	return &domain.Transaction{ID: 777, Description: p.Description, AmountCents: p.AmountCents}, nil
}
func (f *fakeTransactionSvc) Update(_ context.Context, id int64, p domain.UpdateTransactionParams) (*domain.Transaction, error) {
	f.updated.id, f.updated.params = id, p
	return &domain.Transaction{ID: id}, nil
}
func (f *fakeTransactionSvc) Delete(_ context.Context, id int64) error {
	f.deletedID = id
	return nil
}

type nopTransactionSvc struct{}

func (nopTransactionSvc) List(context.Context, domain.ListTransactionsFilter) ([]domain.Transaction, error) {
	return nil, nil
}
func (nopTransactionSvc) Get(context.Context, int64) (*domain.Transaction, error) {
	return &domain.Transaction{}, nil
}
func (nopTransactionSvc) Create(context.Context, domain.CreateTransactionParams) (*domain.Transaction, error) {
	return &domain.Transaction{}, nil
}
func (nopTransactionSvc) Update(context.Context, int64, domain.UpdateTransactionParams) (*domain.Transaction, error) {
	return &domain.Transaction{}, nil
}
func (nopTransactionSvc) Delete(context.Context, int64) error { return nil }

func TestListTransactionsHandler_PassesAllFilters(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := listTransactionsHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, ListTransactionsInput{
		StartDate: "2026-05-01", EndDate: "2026-05-31", AccountID: 7,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(out.Transactions) != 1 {
		t.Errorf("len = %d", len(out.Transactions))
	}
	if svc.listFilter.AccountID != 7 || svc.listFilter.StartDate != "2026-05-01" {
		t.Errorf("filter = %+v", svc.listFilter)
	}
}

func TestGetTransactionHandler(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := getTransactionHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, GetTransactionInput{ID: 55})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Transaction.ID != 55 {
		t.Errorf("got %+v", out)
	}
}

func TestCreateTransactionHandler(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := createTransactionHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateTransactionInput{
		Description: "Coffee", Date: "2026-05-14", AmountCents: -1500,
		AccountID: 1, CategoryID: 10, Paid: true,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Transaction.ID != 777 || svc.created.AmountCents != -1500 {
		t.Errorf("out=%+v svc=%+v", out, svc.created)
	}
}

func TestCreateTransactionHandler_PropagatesValidationError(t *testing.T) {
	svc := &fakeTransactionSvc{createErr: domain.ErrValidation}
	h := createTransactionHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateTransactionInput{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
}

func TestUpdateTransactionHandler(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := updateTransactionHandler(svc)
	desc := "Tea"
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, UpdateTransactionInput{
		ID: 55, Description: &desc,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Transaction.ID != 55 {
		t.Errorf("out = %+v", out)
	}
	if svc.updated.id != 55 || svc.updated.params.Description == nil || *svc.updated.params.Description != "Tea" {
		t.Errorf("svc.updated = %+v", svc.updated)
	}
}

func TestDeleteTransactionHandler(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := deleteTransactionHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, DeleteTransactionInput{ID: 55})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !out.Deleted || out.ID != 55 || svc.deletedID != 55 {
		t.Errorf("out=%+v svc.deletedID=%d", out, svc.deletedID)
	}
}
```

- [ ] **Step 2: Run, verify failure**

```bash
go test ./internal/adapter/mcp/... -run Transaction
```

Expected: undefined symbols.

- [ ] **Step 3: Implement `tools_transactions.go`**

```go
package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// TransactionService is the consumer-side slice this file needs.
type TransactionService interface {
	List(ctx context.Context, filter domain.ListTransactionsFilter) ([]domain.Transaction, error)
	Get(ctx context.Context, id int64) (*domain.Transaction, error)
	Create(ctx context.Context, params domain.CreateTransactionParams) (*domain.Transaction, error)
	Update(ctx context.Context, id int64, params domain.UpdateTransactionParams) (*domain.Transaction, error)
	Delete(ctx context.Context, id int64) error
}

// ---------- list / get ----------

type ListTransactionsInput struct {
	StartDate string `json:"start_date,omitempty" jsonschema:"Optional YYYY-MM-DD lower bound."`
	EndDate   string `json:"end_date,omitempty"   jsonschema:"Optional YYYY-MM-DD upper bound."`
	AccountID int64  `json:"account_id,omitempty" jsonschema:"Optional account id to filter by."`
}

type ListTransactionsOutput struct {
	Transactions []domain.Transaction `json:"transactions"`
}

type GetTransactionInput struct {
	ID int64 `json:"id" jsonschema:"The numeric transaction id."`
}

type GetTransactionOutput struct {
	Transaction domain.Transaction `json:"transaction"`
}

// ---------- create ----------

type CreateTransactionInput struct {
	Description string      `json:"description" jsonschema:"Short transaction description."`
	Date        string      `json:"date"        jsonschema:"YYYY-MM-DD."`
	AmountCents int64       `json:"amount_cents" jsonschema:"Cents; negative=expense, positive=income."`
	AccountID   int64       `json:"account_id"   jsonschema:"Source account id."`
	CategoryID  int64       `json:"category_id"  jsonschema:"Category id."`
	Paid        bool        `json:"paid"         jsonschema:"Whether the transaction is already paid."`
	Notes       string      `json:"notes,omitempty"      jsonschema:"Optional notes."`
	ContactID   *int64      `json:"contact_id,omitempty" jsonschema:"Optional contact id."`
	Tags        []domain.Tag `json:"tags,omitempty"      jsonschema:"Optional tags."`
}

type CreateTransactionOutput struct {
	Transaction domain.Transaction `json:"transaction"`
}

// ---------- update ----------

type UpdateTransactionInput struct {
	ID          int64        `json:"id" jsonschema:"The numeric transaction id to update."`
	Description *string      `json:"description,omitempty"  jsonschema:"New description."`
	Date        *string      `json:"date,omitempty"         jsonschema:"New date YYYY-MM-DD."`
	AmountCents *int64       `json:"amount_cents,omitempty" jsonschema:"New amount in cents."`
	AccountID   *int64       `json:"account_id,omitempty"   jsonschema:"New account id."`
	CategoryID  *int64       `json:"category_id,omitempty"  jsonschema:"New category id."`
	Paid        *bool        `json:"paid,omitempty"         jsonschema:"New paid flag."`
	Notes       *string      `json:"notes,omitempty"        jsonschema:"New notes."`
	ContactID   *int64       `json:"contact_id,omitempty"   jsonschema:"New contact id."`
	Tags        []domain.Tag `json:"tags,omitempty"         jsonschema:"Replacement tag list."`
}

type UpdateTransactionOutput struct {
	Transaction domain.Transaction `json:"transaction"`
}

// ---------- delete ----------

type DeleteTransactionInput struct {
	ID int64 `json:"id" jsonschema:"The numeric transaction id to delete."`
}

type DeleteTransactionOutput struct {
	Deleted bool  `json:"deleted"`
	ID      int64 `json:"id"`
}

// ---------- handlers ----------

func listTransactionsHandler(svc TransactionService) mcpsdk.ToolHandlerFor[ListTransactionsInput, ListTransactionsOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in ListTransactionsInput) (*mcpsdk.CallToolResult, ListTransactionsOutput, error) {
		tx, err := svc.List(ctx, domain.ListTransactionsFilter{
			StartDate: in.StartDate, EndDate: in.EndDate, AccountID: in.AccountID,
		})
		if err != nil {
			return nil, ListTransactionsOutput{}, err
		}
		return nil, ListTransactionsOutput{Transactions: tx}, nil
	}
}

func getTransactionHandler(svc TransactionService) mcpsdk.ToolHandlerFor[GetTransactionInput, GetTransactionOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetTransactionInput) (*mcpsdk.CallToolResult, GetTransactionOutput, error) {
		tx, err := svc.Get(ctx, in.ID)
		if err != nil {
			return nil, GetTransactionOutput{}, err
		}
		return nil, GetTransactionOutput{Transaction: *tx}, nil
	}
}

func createTransactionHandler(svc TransactionService) mcpsdk.ToolHandlerFor[CreateTransactionInput, CreateTransactionOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in CreateTransactionInput) (*mcpsdk.CallToolResult, CreateTransactionOutput, error) {
		tx, err := svc.Create(ctx, domain.CreateTransactionParams{
			Description: in.Description, Date: in.Date, AmountCents: in.AmountCents,
			AccountID: in.AccountID, CategoryID: in.CategoryID, Paid: in.Paid,
			Notes: in.Notes, ContactID: in.ContactID, Tags: in.Tags,
		})
		if err != nil {
			return nil, CreateTransactionOutput{}, err
		}
		return nil, CreateTransactionOutput{Transaction: *tx}, nil
	}
}

func updateTransactionHandler(svc TransactionService) mcpsdk.ToolHandlerFor[UpdateTransactionInput, UpdateTransactionOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in UpdateTransactionInput) (*mcpsdk.CallToolResult, UpdateTransactionOutput, error) {
		tx, err := svc.Update(ctx, in.ID, domain.UpdateTransactionParams{
			Description: in.Description, Date: in.Date, AmountCents: in.AmountCents,
			AccountID: in.AccountID, CategoryID: in.CategoryID, Paid: in.Paid,
			Notes: in.Notes, ContactID: in.ContactID, Tags: in.Tags,
		})
		if err != nil {
			return nil, UpdateTransactionOutput{}, err
		}
		return nil, UpdateTransactionOutput{Transaction: *tx}, nil
	}
}

func deleteTransactionHandler(svc TransactionService) mcpsdk.ToolHandlerFor[DeleteTransactionInput, DeleteTransactionOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in DeleteTransactionInput) (*mcpsdk.CallToolResult, DeleteTransactionOutput, error) {
		if err := svc.Delete(ctx, in.ID); err != nil {
			return nil, DeleteTransactionOutput{}, err
		}
		return nil, DeleteTransactionOutput{Deleted: true, ID: in.ID}, nil
	}
}

func registerTransactionTools(s *mcpsdk.Server, svc TransactionService) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_transactions",
		Description: "List Organizze transactions. Filters: start_date, end_date (YYYY-MM-DD), account_id. amount_cents is negative for expenses, positive for income.",
	}, listTransactionsHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_transaction",
		Description: "Fetch a single Organizze transaction by id.",
	}, getTransactionHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "create_transaction",
		Description: "Create a new Organizze transaction. amount_cents is negative for expenses, positive for income.",
	}, createTransactionHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "update_transaction",
		Description: "Update fields on an existing Organizze transaction. Only fields you provide are changed.",
	}, updateTransactionHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "delete_transaction",
		Description: "Permanently delete an Organizze transaction by id.",
	}, deleteTransactionHandler(svc))
}
```

- [ ] **Step 4: Run all tests in package, verify pass**

```bash
go test ./internal/adapter/mcp/... -v
go test ./... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/mcp/tools_transactions.go internal/adapter/mcp/tools_transactions_test.go internal/adapter/mcp/server.go
git commit -m "feat(adapter/mcp): transactions tool family (read + write)"
```

---

## Task 11: MCP integration tests (real services, fake HTTP) (TDD)

This task wires the FULL stack — domain → usecase → adapter/organizze → adapter/mcp — and exercises every MCP tool through real JSON-RPC against a paired in-memory transport. The Organizze API is the only fake (`httptest`). This catches:
- Missing `register*Tools` calls in `mcp.New`.
- Missing or wrong input/output schemas.
- Wiring mistakes between layers.
- JSON-RPC encoding issues.

**Files:**
- Create: `internal/adapter/mcp/integration_test.go` (`package mcp_test`, external — exercises only public API).

- [ ] **Step 1: Write the integration test**

`internal/adapter/mcp/integration_test.go`:

```go
package mcp_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/adapter/mcp"
	"github.com/jorgejr568/organizze-mcp/internal/adapter/organizze"
	"github.com/jorgejr568/organizze-mcp/internal/usecase"
)

// fakeOrganizze responds to every endpoint touched by any tool. Unknown paths
// fail the test loudly.
func fakeOrganizze(t *testing.T) *httptest.Server {
	t.Helper()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/users/3":
			_, _ = io.WriteString(w, `{"id":3,"name":"Jorge","email":"j@x.com","role":"admin"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/accounts":
			_, _ = io.WriteString(w, `[{"id":1,"name":"Checking","type":"checking"}]`)
		case r.Method == http.MethodGet && r.URL.Path == "/accounts/1":
			_, _ = io.WriteString(w, `{"id":1,"name":"Checking","type":"checking"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/categories":
			_, _ = io.WriteString(w, `[{"id":10,"name":"Food"}]`)
		case r.Method == http.MethodGet && r.URL.Path == "/categories/10":
			_, _ = io.WriteString(w, `{"id":10,"name":"Food"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/budgets":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == http.MethodGet && r.URL.Path == "/budgets/2026":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == http.MethodGet && r.URL.Path == "/budgets/2026/5":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == http.MethodGet && r.URL.Path == "/credit_cards":
			_, _ = io.WriteString(w, `[{"id":1,"name":"Nubank","closing_day":20,"due_day":27,"limit_cents":500000}]`)
		case r.Method == http.MethodGet && r.URL.Path == "/credit_cards/1":
			_, _ = io.WriteString(w, `{"id":1,"name":"Nubank","closing_day":20,"due_day":27,"limit_cents":500000}`)
		case r.Method == http.MethodGet && r.URL.Path == "/credit_cards/1/invoices":
			_, _ = io.WriteString(w, `[{"id":100,"credit_card_id":1,"amount_cents":120000}]`)
		case r.Method == http.MethodGet && r.URL.Path == "/credit_cards/1/invoices/100":
			_, _ = io.WriteString(w, `{"id":100,"credit_card_id":1,"amount_cents":120000}`)
		case r.Method == http.MethodGet && r.URL.Path == "/transfers":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == http.MethodGet && r.URL.Path == "/transactions":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == http.MethodGet && r.URL.Path == "/transactions/55":
			_, _ = io.WriteString(w, `{"id":55,"description":"Pizza","amount_cents":-4500,"account_id":1,"category_id":10,"date":"2026-05-10"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/transactions":
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"id":777,"description":"Coffee","amount_cents":-1500,"account_id":1,"category_id":10,"date":"2026-05-14"}`)
		case r.Method == http.MethodPut && r.URL.Path == "/transactions/55":
			_, _ = io.WriteString(w, `{"id":55,"description":"Pizza-updated","amount_cents":-4500,"account_id":1,"category_id":10,"date":"2026-05-10"}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/transactions/55":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

// newRealServer wires every layer with no shortcuts.
func newRealServer(t *testing.T) *mcpsdk.Server {
	t.Helper()
	api := fakeOrganizze(t)
	client := organizze.NewClient(organizze.ClientOptions{})
	exec, err := organizze.NewRequestExecutor(organizze.RequestExecutorOptions{
		HTTPClient: client,
		BaseURL:    api.URL,
		Email:      "test@example.com",
		APIKey:     "k",
		UserAgent:  "Test (e@x.com)",
	})
	if err != nil {
		t.Fatalf("executor: %v", err)
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
	return mcp.New(deps)
}

func newConnectedSession(t *testing.T) *mcpsdk.ClientSession {
	t.Helper()
	server := newRealServer(t)
	serverT, clientT := mcpsdk.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if _, err := server.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "integration-test", Version: "0"}, nil)
	sess, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

var allExpectedTools = []string{
	"get_user",
	"list_accounts", "get_account",
	"list_categories", "get_category",
	"list_budgets",
	"list_credit_cards", "get_credit_card",
	"list_credit_card_invoices", "get_credit_card_invoice",
	"list_transfers",
	"list_transactions", "get_transaction",
	"create_transaction", "update_transaction", "delete_transaction",
}

func TestIntegration_AllToolsRegisteredWithSchemas(t *testing.T) {
	sess := newConnectedSession(t)
	res, err := sess.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	got := make([]string, 0, len(res.Tools))
	by := make(map[string]*mcpsdk.Tool, len(res.Tools))
	for _, tl := range res.Tools {
		got = append(got, tl.Name)
		by[tl.Name] = tl
	}
	sort.Strings(got)
	want := append([]string(nil), allExpectedTools...)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Errorf("got %d tools (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for _, name := range want {
		tl, ok := by[name]
		if !ok {
			t.Errorf("tool %q not registered", name)
			continue
		}
		if tl.Description == "" {
			t.Errorf("tool %q missing Description", name)
		}
		if tl.InputSchema == nil {
			t.Errorf("tool %q missing InputSchema", name)
		}
		if tl.OutputSchema == nil {
			t.Errorf("tool %q missing OutputSchema", name)
		}
	}
}

func TestIntegration_EveryToolRoundtripsThroughProtocol(t *testing.T) {
	sess := newConnectedSession(t)
	cases := []struct {
		label string
		name  string
		args  any
	}{
		{"get_user", "get_user", map[string]any{"id": 3}},
		{"list_accounts", "list_accounts", map[string]any{}},
		{"get_account", "get_account", map[string]any{"id": 1}},
		{"list_categories", "list_categories", map[string]any{}},
		{"get_category", "get_category", map[string]any{"id": 10}},
		{"list_budgets/current", "list_budgets", map[string]any{}},
		{"list_budgets/year", "list_budgets", map[string]any{"year": 2026}},
		{"list_budgets/month", "list_budgets", map[string]any{"year": 2026, "month": 5}},
		{"list_credit_cards", "list_credit_cards", map[string]any{}},
		{"get_credit_card", "get_credit_card", map[string]any{"id": 1}},
		{"list_credit_card_invoices", "list_credit_card_invoices", map[string]any{"credit_card_id": 1}},
		{"get_credit_card_invoice", "get_credit_card_invoice", map[string]any{"credit_card_id": 1, "invoice_id": 100}},
		{"list_transfers", "list_transfers", map[string]any{}},
		{"list_transactions", "list_transactions", map[string]any{}},
		{"get_transaction", "get_transaction", map[string]any{"id": 55}},
		{"create_transaction", "create_transaction", map[string]any{
			"description":  "Coffee",
			"date":         "2026-05-14",
			"amount_cents": -1500,
			"account_id":   1,
			"category_id":  10,
			"paid":         true,
		}},
		{"update_transaction", "update_transaction", map[string]any{
			"id":          55,
			"description": "Pizza-updated",
		}},
		{"delete_transaction", "delete_transaction", map[string]any{"id": 55}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.label, func(t *testing.T) {
			res, err := sess.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: tc.name, Arguments: tc.args})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if res.IsError {
				t.Fatalf("IsError=true; content=%v", res.Content)
			}
			if len(res.Content) == 0 {
				t.Errorf("no content")
			}
		})
	}
}

func TestIntegration_BudgetMonthWithoutYear_ReturnsToolError(t *testing.T) {
	sess := newConnectedSession(t)
	res, err := sess.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "list_budgets", Arguments: map[string]any{"month": 5},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true; content=%v", res.Content)
	}
}

func TestIntegration_CreateTransactionMissingFields_ReturnsToolError(t *testing.T) {
	sess := newConnectedSession(t)
	res, err := sess.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "create_transaction", Arguments: map[string]any{"description": "x"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true; content=%v", res.Content)
	}
}
```

> **SDK note:** the test depends on `NewInMemoryTransports`, `Server.Connect`, `Client.Connect`, `ClientSession.ListTools`, `ClientSession.CallTool`, `ClientSession.Close`, `CallToolResult.IsError`, `CallToolResult.Content`. If the pinned SDK release renames `IsError` (e.g. into a `GetError()` method), adjust those references — the test shape is correct.

- [ ] **Step 2: Run, verify pass**

```bash
go test ./internal/adapter/mcp/... -v
```

Expected: every integration test PASS.

- [ ] **Step 3: Regression-bite check**

Comment out one `register*Tools` line in `internal/adapter/mcp/server.go`, e.g. `registerTransferTools(s, deps.Transfer)`. Re-run:

```bash
go test ./internal/adapter/mcp/... -run TestIntegration_AllToolsRegisteredWithSchemas -v
```

Expected: FAIL with `tool "list_transfers" not registered`. Restore the line, re-run, observe PASS. Documents the test bites.

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/mcp/integration_test.go
git commit -m "test(adapter/mcp): full-stack protocol integration tests via in-memory transport"
```

---

## Task 12: Composition root — `cmd/organizze-mcp/main.go` (TDD)

The composition root is the **only** place concrete types from outer layers are bound to inner-layer interfaces. The build function (`buildServer`) does the wiring; `runWithTransport` boots the MCP server over an arbitrary transport (stdio or in-memory); `runHTTP` adds the HTTP listener.

**Files:**
- Create: `cmd/organizze-mcp/main.go`
- Create: `cmd/organizze-mcp/main_test.go`

- [ ] **Step 1: Write failing tests**

`cmd/organizze-mcp/main_test.go`:

```go
package main

import (
	"context"
	"net"
	"net/http"
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
```

- [ ] **Step 2: Run, verify failure**

```bash
go test ./cmd/organizze-mcp/...
```

Expected: undefined `buildServer`, `runWithTransport`, `runHTTP`.

- [ ] **Step 3: Implement `main.go`**

`cmd/organizze-mcp/main.go`:

```go
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
		HTTPClient: httpClient,
		BaseURL:    cfg.BaseURL,
		Email:      cfg.Email,
		APIKey:     cfg.APIKey,
		UserAgent:  cfg.UserAgent,
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
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		return err
	}
}
```

- [ ] **Step 4: Run tests, verify pass**

```bash
go test ./cmd/organizze-mcp/... -v
go test ./... -count=1
```

Expected: every test PASS across the whole project.

- [ ] **Step 5: Verify the binary builds and smoke-runs**

```bash
go build -o bin/organizze-mcp ./cmd/organizze-mcp
ORGANIZZE_API_KEY=x ORGANIZZE_EMAIL=x@x.com ORGANIZZE_USER_AGENT='Test (x@x.com)' \
  ORGANIZZE_BASE_URL=http://127.0.0.1:1 \
  timeout 1 ./bin/organizze-mcp </dev/null || true
```

Expected: stderr line `organizze-mcp v0.1.0 starting on stdio`; clean exit at timeout/EOF.

- [ ] **Step 6: Commit**

```bash
git add cmd/
git commit -m "feat(cmd): composition root wiring every layer with transport dispatch"
```

---

## Task 13: Dockerfile + final README

**Files:**
- Create: `Dockerfile`
- Modify: `README.md`

- [ ] **Step 1: Write `Dockerfile`**

```dockerfile
# syntax=docker/dockerfile:1.7

FROM golang:1.23-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/organizze-mcp ./cmd/organizze-mcp

FROM gcr.io/distroless/static:nonroot
LABEL org.opencontainers.image.source="https://github.com/jorgejr568/organizze-mcp"
LABEL org.opencontainers.image.description="MCP server for the Organizze API (Clean Architecture)"

COPY --from=build /out/organizze-mcp /usr/local/bin/organizze-mcp

USER nonroot:nonroot
ENV MCP_TRANSPORT=stdio
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/organizze-mcp"]
```

- [ ] **Step 2: Build and smoke-test the image**

```bash
docker build -t organizze-mcp:dev .
docker images organizze-mcp:dev

docker run --rm -i \
  -e ORGANIZZE_API_KEY=x \
  -e ORGANIZZE_EMAIL=x@x.com \
  -e "ORGANIZZE_USER_AGENT=Test (x@x.com)" \
  -e ORGANIZZE_BASE_URL=http://127.0.0.1:1 \
  organizze-mcp:dev </dev/null

docker run --rm -d --name omcp-smoke \
  -p 8080:8080 \
  -e MCP_TRANSPORT=http \
  -e ORGANIZZE_API_KEY=x -e ORGANIZZE_EMAIL=x@x.com \
  -e "ORGANIZZE_USER_AGENT=Test (x@x.com)" \
  -e ORGANIZZE_BASE_URL=http://127.0.0.1:1 \
  organizze-mcp:dev
sleep 1
curl -fsS http://localhost:8080/healthz
docker stop omcp-smoke
```

Expected: image is ~10–15 MB. stdio smoke prints startup line and exits on EOF. HTTP smoke returns `ok` from `/healthz`.

- [ ] **Step 3: Finalize `README.md`**

Replace the skeleton from Task 1 with:

```markdown
# organizze-mcp

A Model Context Protocol (MCP) server exposing the [Organizze](https://www.organizze.com.br/) REST API to LLM clients (Claude Desktop, Claude Code, etc.), built in Go with a layered Clean Architecture.

## Architecture

```
cmd ──► adapter/mcp ──► usecase ──► domain
        adapter/organizze ──► usecase (interfaces)
```

- **`internal/domain`** — entities, value objects, sentinel errors. Imports nothing.
- **`internal/usecase`** — application services + repository interfaces. Imports `domain`.
- **`internal/adapter/organizze`** — HTTP/REST repository implementations + `HTTPClient` abstraction. Imports `usecase`, `domain`.
- **`internal/adapter/mcp`** — MCP tool adapters. Imports `usecase`, `domain`.
- **`cmd/organizze-mcp`** — composition root.

`adapter/organizze.HTTPClient` is the interface every repository uses for HTTP transport; `Client` (concrete) wraps stdlib `*http.Client` and is the only place timeouts/retries can be centralized.

## Configuration

| Variable | Required | Default | Notes |
|---|---|---|---|
| `ORGANIZZE_API_KEY` | yes | — | https://app.organizze.com.br/configuracoes/api-keys |
| `ORGANIZZE_EMAIL` | yes | — | Account email (Basic-Auth username) |
| `ORGANIZZE_USER_AGENT` | yes | — | `"App (e@x.com)"` — Organizze rejects requests without it |
| `MCP_TRANSPORT` | no | `stdio` | `stdio` or `http` |
| `MCP_HTTP_ADDR` | no | `:8080` | Listen address for HTTP transport |
| `ORGANIZZE_BASE_URL` | no | `https://api.organizze.com.br/rest/v2` | Override |
| `ORGANIZZE_HTTP_TIMEOUT` | no | `30s` | `time.ParseDuration` format |

## Quickstart — Docker stdio (Claude Desktop)

`~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "organizze": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "ORGANIZZE_API_KEY",
        "-e", "ORGANIZZE_EMAIL",
        "-e", "ORGANIZZE_USER_AGENT",
        "ghcr.io/jorgejr568/organizze-mcp:latest"
      ],
      "env": {
        "ORGANIZZE_API_KEY": "your-token-here",
        "ORGANIZZE_EMAIL": "you@example.com",
        "ORGANIZZE_USER_AGENT": "ClaudeDesktop (you@example.com)"
      }
    }
  }
}
```

## Quickstart — Docker HTTP

```bash
docker run -d --name organizze-mcp \
  -p 8080:8080 \
  -e MCP_TRANSPORT=http \
  -e ORGANIZZE_API_KEY=... \
  -e ORGANIZZE_EMAIL=... \
  -e "ORGANIZZE_USER_AGENT=YourApp (you@example.com)" \
  ghcr.io/jorgejr568/organizze-mcp:latest
```

Endpoints: `http://localhost:8080/mcp` (MCP), `http://localhost:8080/healthz` (liveness).

## Quickstart — From source

```bash
git clone https://github.com/jorgejr568/organizze-mcp
cd organizze-mcp
make build
ORGANIZZE_API_KEY=... ORGANIZZE_EMAIL=... ORGANIZZE_USER_AGENT='App (e@x.com)' \
  ./bin/organizze-mcp
```

## Tool catalogue

16 tools. `amount_cents` is **negative for expenses, positive for income**.

| Tool | Service.Method |
|---|---|
| `get_user` | UserService.Get |
| `list_accounts` / `get_account` | AccountService.{List, Get} |
| `list_categories` / `get_category` | CategoryService.{List, Get} |
| `list_budgets` | BudgetService.List (year/month routing) |
| `list_credit_cards` / `get_credit_card` | CreditCardService.{List, Get} |
| `list_credit_card_invoices` / `get_credit_card_invoice` | InvoiceService.{List, Get} |
| `list_transfers` | TransferService.List |
| `list_transactions` / `get_transaction` | TransactionService.{List, Get} |
| `create_transaction` / `update_transaction` / `delete_transaction` | TransactionService.{Create, Update, Delete} |

## Development

```bash
make test        # full suite
make test-cover  # with coverage report
make lint        # go vet
make build       # binary at bin/organizze-mcp
make docker      # container image
```

## License

MIT (or your preferred license).
```

- [ ] **Step 4: Final whole-project check**

```bash
go test ./... -count=1 -v
go vet ./...
go build ./...
```

Expected: every test PASS, no vet warnings, clean build.

- [ ] **Step 5: Commit**

```bash
git add Dockerfile README.md
git commit -m "feat: dockerfile and finalized readme"
```

The first release tag is pushed in Task 17 — after the GitHub Actions release workflow exists. Tagging earlier would not trigger a build.

---

## Task 14: Create the GitHub repository and push initial commits

Push the 13 commits from Tasks 1–13 to a fresh `jorgejr568/organizze-mcp` repository on GitHub.

> **Timing note:** this task can be run immediately after Task 1 instead of at the end. Doing so gives you off-machine backup of work in progress; each subsequent task's commit can be followed by `git push`. The plan places it at Task 14 only to keep the linear flow simple — pick what suits your workflow.

**Prerequisites:** the `gh` CLI installed and authenticated against the target account (`gh auth status` shows a `github.com` login for `jorgejr568`).

- [ ] **Step 1: Verify gh authentication**

```bash
gh auth status
```

Expected: a line listing `github.com` and `Logged in to github.com account jorgejr568`. If not, run `gh auth login` and complete the browser flow.

- [ ] **Step 2: Create the repository and push**

```bash
gh repo create jorgejr568/organizze-mcp \
  --public \
  --description "Model Context Protocol server exposing the Organizze REST API — Go, Clean Architecture" \
  --source=. \
  --remote=origin \
  --push
```

Expected:
- Creates the repository at https://github.com/jorgejr568/organizze-mcp.
- Adds `origin` remote pointing at it.
- Pushes the local `main` branch with all 13 commits from Tasks 1–13.

- [ ] **Step 3: Verify the push**

```bash
git remote -v
gh repo view jorgejr568/organizze-mcp --json url --jq .url
gh api repos/jorgejr568/organizze-mcp/commits --jq '.[0:5] | .[].commit.message'
```

Expected:
- `origin` points to the new repository.
- The repo URL prints.
- The five most recent commit messages are listed (newest first), matching the last five tasks.

---

## Task 15: GitHub Actions CI — test on every PR

Add a CI workflow that runs `go test`, `go vet`, and `go build` on every PR to `main` and every push to `main`. The job is named `Test`; Task 17 references that exact name when configuring branch protection.

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Write `.github/workflows/ci.yml`**

```yaml
name: CI

on:
  pull_request:
    branches: [main]
  push:
    branches: [main]

permissions:
  contents: read

concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'
          cache: true

      - name: Verify go.mod is tidy
        run: |
          go mod tidy
          if ! git diff --exit-code go.mod go.sum; then
            echo "::error::go.mod or go.sum is not tidy. Run 'go mod tidy' locally."
            exit 1
          fi

      - name: Vet
        run: go vet ./...

      - name: Test
        run: go test -race -count=1 -coverprofile=coverage.out ./...

      - name: Build
        run: go build -o /dev/null ./cmd/organizze-mcp

      - name: Upload coverage artifact
        if: success()
        uses: actions/upload-artifact@v4
        with:
          name: coverage
          path: coverage.out
          retention-days: 14
```

Design notes:
- `concurrency` cancels superseded runs for the same ref when a new commit is pushed (saves Actions minutes).
- The `go mod tidy` check fails the run if `go.mod`/`go.sum` aren't tidy — keeps dep hygiene honest at PR time.
- `-race` enables the race detector; `-count=1` disables the test cache so every run is fresh.
- Coverage is uploaded as an artifact for inspection from the run page.

- [ ] **Step 2: Commit and push**

```bash
mkdir -p .github/workflows
# (write the file above)
git add .github/workflows/ci.yml
git commit -m "ci: add GitHub Actions workflow for go test on PR + push"
git push
```

- [ ] **Step 3: Verify the workflow ran**

```bash
gh run list --workflow=ci.yml --limit=1
gh run watch
```

Expected: a single CI run completes in ~1–2 minutes with conclusion `success`. If it fails, inspect with `gh run view --log`.

---

## Task 16: GitHub Actions release — Docker Hub publishing with versioning

Add a release workflow that, on every push of a `v*` git tag, runs the test suite once and then builds a multi-architecture (linux/amd64 + linux/arm64) Docker image, pushing it to Docker Hub with semver-derived tags.

**Resulting tags for a `v1.2.3` release:** `1.2.3`, `1.2`, `1`, `latest`.

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Write `.github/workflows/release.yml`**

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: read

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
          cache: true
      - name: Test
        run: go test -race -count=1 ./...

  docker:
    name: Build and push Docker image
    needs: test
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: jorgejr568/organizze-mcp
          tags: |
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=semver,pattern={{major}}
            type=raw,value=latest

      - name: Build and push
        uses: docker/build-push-action@v6
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

Design notes:
- The `test` job runs the suite again before publishing. It guards against tags pointing at un-CI'd commits — possible when admins bypass branch protection.
- `docker/metadata-action@v5` derives the four-way tag set (`{version}`, `{major}.{minor}`, `{major}`, `latest`) from one semver tag.
- `docker/build-push-action@v6` uses the GitHub Actions cache for fast incremental builds.
- Multi-arch covers Apple Silicon (`arm64`) and standard cloud servers (`amd64`).

- [ ] **Step 2: Commit and push the workflow (do not tag yet)**

```bash
git add .github/workflows/release.yml
git commit -m "ci: add Docker Hub release workflow for v* tags"
git push
```

- [ ] **Step 3: Verify the workflow is registered**

```bash
gh workflow list
```

Expected: both `CI` and `Release` listed. `Release` is dormant until the first `v*` tag is pushed (Task 17).

---

## Task 17: Docker Hub secrets, branch protection, and first release

Final operational pieces: wire the Docker Hub credentials, lock down `main` requiring `Test` to pass, push the first tag, and verify the image lands on Docker Hub.

**Prerequisites:** a Docker Hub access token. Generate one at https://hub.docker.com/settings/security → "New Access Token" with scope `Read & Write` on the `organizze-mcp` repository. Create the Docker Hub repository at the same URL if it doesn't exist yet.

- [ ] **Step 1: Set the Docker Hub username secret**

```bash
gh secret set DOCKERHUB_USERNAME --body "jorgejr568" --repo jorgejr568/organizze-mcp
```

Expected: `✓ Set Actions secret DOCKERHUB_USERNAME for jorgejr568/organizze-mcp`.

- [ ] **Step 2: Set the Docker Hub access-token secret (interactive)**

```bash
gh secret set DOCKERHUB_TOKEN --repo jorgejr568/organizze-mcp
# Paste the access token at the prompt, press Enter, then Ctrl-D to submit.
```

Avoid putting the token on the command line — `--body "$TOKEN"` would leak it into shell history. The interactive form sends the value only over the gh API.

Expected: `✓ Set Actions secret DOCKERHUB_TOKEN for jorgejr568/organizze-mcp`.

Verify both exist (names only — values are write-only):

```bash
gh secret list --repo jorgejr568/organizze-mcp
```

Expected: both `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` listed with recent `Updated` timestamps.

- [ ] **Step 3: Enable branch protection on `main`**

```bash
gh api -X PUT repos/jorgejr568/organizze-mcp/branches/main/protection \
  --input - <<'EOF'
{
  "required_status_checks": {
    "strict": true,
    "contexts": ["Test"]
  },
  "enforce_admins": false,
  "required_pull_request_reviews": null,
  "restrictions": null,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "required_conversation_resolution": true,
  "lock_branch": false,
  "required_linear_history": false
}
EOF
```

Field-by-field:
- `required_status_checks.strict: true` — branch must be up-to-date before merging.
- `required_status_checks.contexts: ["Test"]` — the CI workflow's `Test` job must be green.
- `enforce_admins: false` — solo-repo escape hatch; admins can push directly when needed. Flip to `true` once you have collaborators.
- `required_pull_request_reviews: null` — don't require PR reviews (impractical solo). Add `{"required_approving_review_count": 1}` once you have a reviewer.
- `restrictions: null` — no user/team push restrictions.
- `allow_force_pushes: false`, `allow_deletions: false` — standard "don't lose history" defaults.
- `required_conversation_resolution: true` — PR comments must be resolved before merge.

Verify:

```bash
gh api repos/jorgejr568/organizze-mcp/branches/main/protection \
  --jq '{checks: .required_status_checks.contexts, enforce_admins: .enforce_admins.enabled, conv_resolution: .required_conversation_resolution.enabled}'
```

Expected: `{"checks":["Test"],"enforce_admins":false,"conv_resolution":true}`.

- [ ] **Step 4: Tag the first release**

```bash
git tag -a v0.1.0 -m "v0.1.0 — initial Clean-Architecture MCP server"
git push origin v0.1.0
```

Expected: tag pushed; within a few seconds the Release workflow starts.

- [ ] **Step 5: Watch the release workflow run**

```bash
gh run watch
```

Expected: a `Release` run completes in ~5–8 minutes (multi-arch build is the long pole). Both jobs (`Test` and `Build and push Docker image`) end green.

If the docker job fails with `unauthorized` or `denied`, double-check the secrets from Steps 1–2 and confirm the access token has `Read & Write` on the target repository.

- [ ] **Step 6: Verify the image on Docker Hub**

```bash
docker pull jorgejr568/organizze-mcp:v0.1.0
docker pull jorgejr568/organizze-mcp:latest
docker buildx imagetools inspect jorgejr568/organizze-mcp:latest
```

Expected: pulls succeed; `imagetools inspect` shows both `linux/amd64` and `linux/arm64` in the manifest list. Also browse https://hub.docker.com/r/jorgejr568/organizze-mcp/tags — each tag should display two architectures.

- [ ] **Step 7: Add status badges (optional polish; also exercises branch protection)**

Append to `README.md`:

```markdown
## Status

[![CI](https://github.com/jorgejr568/organizze-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/jorgejr568/organizze-mcp/actions/workflows/ci.yml)
[![Docker Hub](https://img.shields.io/docker/v/jorgejr568/organizze-mcp?label=docker&sort=semver)](https://hub.docker.com/r/jorgejr568/organizze-mcp)
```

Commit on a feature branch and open a PR (you can't push directly to `main` anymore — that's the point):

```bash
git checkout -b chore/readme-badges
git add README.md
git commit -m "docs: add CI and Docker Hub badges"
git push -u origin chore/readme-badges
gh pr create --title "docs: add CI and Docker Hub badges" \
             --body "Adds status badges to the README." \
             --base main
```

Wait for CI to go green, then merge — this proves branch protection is enforced end-to-end:

```bash
gh pr checks --watch
gh pr merge --squash --delete-branch
```

Expected: `gh pr merge` succeeds only after the `Test` check is green. If you try `gh pr merge` while CI is still running or red, it fails with a protection-rule error.

---

## Self-Review Notes

**Spec coverage:**
- [x] Build an MCP server → Tasks 8–10 register all 16 tools.
- [x] Dockerfile → Task 13.
- [x] env API key → Task 2 + Task 12 composition root.
- [x] Consume Organizze API → Tasks 4–6 cover client + every resource.
- [x] Go-based → entire plan.
- [x] Both transports → Task 12 dispatches on `MCP_TRANSPORT`.
- [x] Read + write transactions, read everything else → tool catalogue matches.
- [x] **Clean Code** → small files (≤ ~150 LOC each), meaningful names, DRY (HTTP boilerplate consolidated in `executor.go`), no magic literals, TDD throughout.
- [x] **SOLID** → SRP (one file per concern), OCP (new resources = new files only), LSP (interface fakes substitute), ISP (`TransactionReader`/`TransactionWriter` split; one-method `HTTPClient`), DIP (interfaces consumer-owned; outer impls inject via composition root).
- [x] **Clean Architecture** → 4 layers, dependencies inward only, domain pure, wire-format DTOs unexported in `adapter/organizze`.
- [x] **`HTTPClient` interface + concrete struct wrapping `*http.Client`** → Task 4.
- [x] **GitHub repo `jorgejr568/organizze-mcp` created via `gh repo create`** → Task 14.
- [x] **GitHub Actions test on PR** → Task 15 (`.github/workflows/ci.yml`, job name `Test`).
- [x] **Multi-arch Docker Hub publish with versioning** → Task 16 (`.github/workflows/release.yml`, `v*` tag trigger; tags `{version}`, `{major}.{minor}`, `{major}`, `latest`; platforms `linux/amd64,linux/arm64`).
- [x] **Branch protection on `main`** → Task 17 Step 3 (requires `Test` status check; PR-only path; force-push/delete blocked).
- [x] **Docker Hub credentials via GitHub Actions secrets** → Task 17 Steps 1–2 (`DOCKERHUB_USERNAME` via `--body`; `DOCKERHUB_TOKEN` via interactive stdin so it doesn't leak into shell history).

**Test coverage:**
- `internal/domain`: trivial logic; tests cover sentinel-error wrapping.
- `internal/usecase`: every service tested with fake repos; `BudgetService` and `TransactionService` cover their branches.
- `internal/adapter/organizze`: every repository + `RequestExecutor` + `APIError` mapping tested with `httptest`.
- `internal/adapter/mcp`: every handler tested with fake services; full-stack integration via `InMemoryTransports` covers protocol roundtrip + schemas + `IsError` paths.
- `cmd/organizze-mcp`: composition root tested via `buildServer`, `runWithTransport` (in-memory MCP), `runHTTP` (real TCP + `/healthz`).
- Untested: `main()` (calls `run()` + `os.Exit`), `signal.NotifyContext` glue.

**Type consistency:**
- `domain.Transaction`, `domain.Tag`, `domain.CreateTransactionParams`, `domain.UpdateTransactionParams` defined once in Task 3 and used unchanged through every later task.
- `usecase.TransactionReader` + `TransactionWriter` + `TransactionRepository` composition defined in Task 7 and satisfied by `*organizze.TransactionRepository` from Task 6 (verified at composition-root wire-up in Task 12).
- MCP-layer service interfaces (`UserService`, `AccountService`, etc.) defined per `tools_*.go` file are satisfied by `*usecase.*Service` concretes implicitly — no `implements` annotations needed.
- `HTTPClient` interface (Task 4) is satisfied by `*organizze.Client` (compile-time check `var _ HTTPClient = (*Client)(nil)`).
- `mcp.NewInMemoryTransports`, `Server.Connect`, `Client.Connect`, `ClientSession.{ListTools, CallTool, Close}`, `CallToolResult.IsError`/`Content` — used per SDK v1.5.0+ docs; integration test in Task 11 carries an "if the SDK renames…" note.

**Placeholder scan:** no "TBD", "implement later", or vague "handle errors". Every code block compiles in isolation against dependencies declared in prior tasks. The only forward references are deliberate within-package ones (Task 8's `server.go` calls `register*Tools` functions added in Tasks 9–10; this is flagged in Task 8's Step 7 note with a comment-out workaround for running its tests in isolation).
