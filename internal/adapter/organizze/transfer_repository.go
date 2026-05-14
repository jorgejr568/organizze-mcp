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
