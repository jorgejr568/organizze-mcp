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
