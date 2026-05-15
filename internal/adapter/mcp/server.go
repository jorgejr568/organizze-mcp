// Package mcp is the MCP adapter — it composes use-case services into MCP tools.
// It depends on usecase (interfaces it owns locally) and domain (types crossing
// the boundary). It does NOT import internal/adapter/organizze.
package mcp

import (
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

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
	r := deps.Reporter
	if r == nil {
		r = stats.NoopReporter{}
	}

	s := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "organizze-mcp",
		Version: Version,
	}, nil)

	registerUserTools(s, r, deps.User)
	registerAccountTools(s, r, deps.Account)
	registerCategoryTools(s, r, deps.Category)
	registerBudgetTools(s, r, deps.Budget)
	registerCreditCardTools(s, r, deps.CreditCard)
	registerInvoiceTools(s, r, deps.Invoice)
	registerTransferTools(s, r, deps.Transfer)
	registerTransactionTools(s, r, deps.Transaction)

	return s
}
