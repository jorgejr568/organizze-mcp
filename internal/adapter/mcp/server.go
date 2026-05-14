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
