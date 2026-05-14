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

func (nopInvoiceSvc) List(context.Context, int64) ([]domain.Invoice, error)      { return nil, nil }
func (nopInvoiceSvc) Get(context.Context, int64, int64) (*domain.Invoice, error) { return &domain.Invoice{}, nil }

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
