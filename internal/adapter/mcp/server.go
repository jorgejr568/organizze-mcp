// Package mcp is the MCP adapter — it composes use-case services into MCP tools.
// It depends on usecase (interfaces it owns locally) and domain (types crossing
// the boundary). It does NOT import internal/adapter/organizze.
package mcp

import (
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"github.com/jorgejr568/organizze-mcp/internal/stats"
)

// Version is reported via the MCP Implementation block on handshake.
//
// Set at link time via:
//
//	go build -ldflags="-X 'github.com/jorgejr568/organizze-mcp/internal/adapter/mcp.Version=<value>'"
//
// Defaults to "dev" for unstamped builds (go run, go test, IDE builds).
var Version = "dev"

// Dependencies bundles every service the MCP server needs. Each field is a
// small interface defined in the matching tools_*.go file. The composition
// root in cmd/organizze-mcp wires usecase.*Service concretes into these slots.
type Dependencies struct {
	Reporter stats.Reporter
	// Logger receives one structured record per tool call (info on success,
	// warn on error). Defaults to zap.NewNop() so tests need not provide one.
	// The log uses the same non-sensitive vocabulary as the stats reporter
	// (tool name, status, error_class, duration_ms) — never tool arguments
	// or return values.
	Logger *zap.Logger

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
	inst := instrumentation{
		reporter: deps.Reporter,
		logger:   deps.Logger,
	}.normalize()

	s := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "organizze-mcp",
		Version: Version,
	}, nil)

	registerUserTools(s, inst, deps.User)
	registerAccountTools(s, inst, deps.Account)
	registerCategoryTools(s, inst, deps.Category)
	registerBudgetTools(s, inst, deps.Budget)
	registerCreditCardTools(s, inst, deps.CreditCard)
	registerInvoiceTools(s, inst, deps.Invoice)
	registerTransferTools(s, inst, deps.Transfer)
	registerTransactionTools(s, inst, deps.Transaction)

	return s
}
