package organizze

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// TransactionRepository handles transaction reads and writes against the
// Organizze API.
type TransactionRepository struct {
	exec *RequestExecutor
}

func NewTransactionRepository(exec *RequestExecutor) *TransactionRepository {
	return &TransactionRepository{exec: exec}
}

// List returns transactions matching filter. Zero-valued fields are omitted.
func (r *TransactionRepository) List(ctx context.Context, f domain.ListTransactionsFilter) ([]domain.Transaction, error) {
	q := url.Values{}
	if f.StartDate != "" {
		q.Set("start_date", f.StartDate)
	}
	if f.EndDate != "" {
		q.Set("end_date", f.EndDate)
	}
	if f.AccountID != 0 {
		q.Set("account_id", strconv.FormatInt(f.AccountID, 10))
	}
	path := "/transactions"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var out []domain.Transaction
	if err := r.exec.Get(ctx, path, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get returns a single transaction by id.
func (r *TransactionRepository) Get(ctx context.Context, id int64) (*domain.Transaction, error) {
	var tx domain.Transaction
	if err := r.exec.Get(ctx, fmt.Sprintf("/transactions/%d", id), &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

// Create issues a POST and returns the persisted transaction.
func (r *TransactionRepository) Create(ctx context.Context, params domain.CreateTransactionParams) (*domain.Transaction, error) {
	var tx domain.Transaction
	if err := r.exec.Post(ctx, "/transactions", params, &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

// Update issues a PUT with only the non-nil fields from params.
func (r *TransactionRepository) Update(ctx context.Context, id int64, params domain.UpdateTransactionParams) (*domain.Transaction, error) {
	var tx domain.Transaction
	if err := r.exec.Put(ctx, fmt.Sprintf("/transactions/%d", id), params, &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

// Delete issues a DELETE. If params is non-zero, its fields ride along as a
// JSON body so Organizze can apply the deletion to a recurring/installment
// series per ORGANIZZE_API.md "Excluir movimentação". The deleted transaction
// snapshot is returned when the API echoes one; otherwise nil.
func (r *TransactionRepository) Delete(ctx context.Context, id int64, params domain.DeleteTransactionParams) (*domain.Transaction, error) {
	var body any
	if !params.IsZero() {
		body = params
	}
	var out domain.Transaction
	if err := r.exec.Delete(ctx, fmt.Sprintf("/transactions/%d", id), body, &out); err != nil {
		return nil, err
	}
	if out.ID == 0 {
		return nil, nil
	}
	return &out, nil
}
