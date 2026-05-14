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
