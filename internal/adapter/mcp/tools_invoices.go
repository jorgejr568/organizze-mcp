package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type InvoiceService interface {
	List(ctx context.Context, creditCardID int64, filter domain.ListInvoicesFilter) ([]domain.Invoice, error)
	Get(ctx context.Context, creditCardID, invoiceID int64) (*domain.Invoice, error)
	Payment(ctx context.Context, creditCardID, invoiceID int64) (*domain.Transaction, error)
}

type ListInvoicesInput struct {
	CreditCardID int64  `json:"credit_card_id" jsonschema:"The numeric credit card id whose invoices to list."`
	StartDate    string `json:"start_date,omitempty" jsonschema:"Optional YYYY-MM-DD lower bound. Without a range, Organizze caps results to the current calendar year."`
	EndDate      string `json:"end_date,omitempty"   jsonschema:"Optional YYYY-MM-DD upper bound."`
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

type GetInvoicePaymentInput struct {
	CreditCardID int64 `json:"credit_card_id" jsonschema:"The numeric credit card id."`
	InvoiceID    int64 `json:"invoice_id"     jsonschema:"The numeric invoice id."`
}

type GetInvoicePaymentOutput struct {
	Payment domain.Transaction `json:"payment"`
}

func listInvoicesHandler(svc InvoiceService) mcpsdk.ToolHandlerFor[ListInvoicesInput, ListInvoicesOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in ListInvoicesInput) (*mcpsdk.CallToolResult, ListInvoicesOutput, error) {
		invs, err := svc.List(ctx, in.CreditCardID, domain.ListInvoicesFilter{StartDate: in.StartDate, EndDate: in.EndDate})
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

func getInvoicePaymentHandler(svc InvoiceService) mcpsdk.ToolHandlerFor[GetInvoicePaymentInput, GetInvoicePaymentOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetInvoicePaymentInput) (*mcpsdk.CallToolResult, GetInvoicePaymentOutput, error) {
		tx, err := svc.Payment(ctx, in.CreditCardID, in.InvoiceID)
		if err != nil {
			return nil, GetInvoicePaymentOutput{}, err
		}
		return nil, GetInvoicePaymentOutput{Payment: *tx}, nil
	}
}

func registerInvoiceTools(s *mcpsdk.Server, inst instrumentation, svc InvoiceService) {
	addInstrumentedTool(s, inst, &mcpsdk.Tool{
		Name:        "list_credit_card_invoices",
		Description: "List invoices for a given credit card. Optional start_date / end_date (YYYY-MM-DD) widen beyond the default current-year window.",
	}, listInvoicesHandler(svc))
	addInstrumentedTool(s, inst, &mcpsdk.Tool{
		Name:        "get_credit_card_invoice",
		Description: "Fetch a specific credit-card invoice.",
	}, getInvoiceHandler(svc))
	addInstrumentedTool(s, inst, &mcpsdk.Tool{
		Name:        "get_credit_card_invoice_payment",
		Description: "Fetch the consolidated payment Transaction for a credit-card invoice (GET /credit_cards/{credit_card_id}/invoices/{invoice_id}/payments).",
	}, getInvoicePaymentHandler(svc))
}
