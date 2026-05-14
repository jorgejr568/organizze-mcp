package organizze

import (
	"context"
	"fmt"
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

// Create issues a POST and returns the persisted transfer.
func (r *TransferRepository) Create(ctx context.Context, params domain.CreateTransferParams) (*domain.Transfer, error) {
	var tr domain.Transfer
	if err := r.exec.Post(ctx, "/transfers", params, &tr); err != nil {
		return nil, err
	}
	return &tr, nil
}

// Update issues a PUT with only the non-nil fields from params.
func (r *TransferRepository) Update(ctx context.Context, id int64, params domain.UpdateTransferParams) (*domain.Transfer, error) {
	var tr domain.Transfer
	if err := r.exec.Put(ctx, fmt.Sprintf("/transfers/%d", id), params, &tr); err != nil {
		return nil, err
	}
	return &tr, nil
}

// Delete issues a DELETE.
func (r *TransferRepository) Delete(ctx context.Context, id int64) error {
	return r.exec.Delete(ctx, fmt.Sprintf("/transfers/%d", id), nil, nil)
}
