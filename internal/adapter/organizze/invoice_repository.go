package organizze

import (
	"context"
	"fmt"
	"net/url"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// InvoiceRepository lists and fetches credit-card invoices.
type InvoiceRepository struct {
	exec *RequestExecutor
}

func NewInvoiceRepository(exec *RequestExecutor) *InvoiceRepository {
	return &InvoiceRepository{exec: exec}
}

func (r *InvoiceRepository) List(ctx context.Context, cardID int64, f domain.ListInvoicesFilter) ([]domain.Invoice, error) {
	q := url.Values{}
	if f.StartDate != "" {
		q.Set("start_date", f.StartDate)
	}
	if f.EndDate != "" {
		q.Set("end_date", f.EndDate)
	}
	path := fmt.Sprintf("/credit_cards/%d/invoices", cardID)
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var out []domain.Invoice
	if err := r.exec.Get(ctx, path, &out); err != nil {
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
