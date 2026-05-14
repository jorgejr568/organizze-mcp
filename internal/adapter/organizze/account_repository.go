package organizze

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// AccountRepository lists and fetches bank/cash accounts.
type AccountRepository struct {
	exec *RequestExecutor
}

func NewAccountRepository(exec *RequestExecutor) *AccountRepository {
	return &AccountRepository{exec: exec}
}

func (r *AccountRepository) List(ctx context.Context) ([]domain.Account, error) {
	var out []domain.Account
	if err := r.exec.Get(ctx, "/accounts", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *AccountRepository) Get(ctx context.Context, id int64) (*domain.Account, error) {
	var a domain.Account
	if err := r.exec.Get(ctx, fmt.Sprintf("/accounts/%d", id), &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// Create issues a POST and returns the persisted account.
func (r *AccountRepository) Create(ctx context.Context, params domain.CreateAccountParams) (*domain.Account, error) {
	var a domain.Account
	if err := r.exec.Post(ctx, "/accounts", params, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// Update issues a PUT with only the non-nil fields from params.
func (r *AccountRepository) Update(ctx context.Context, id int64, params domain.UpdateAccountParams) (*domain.Account, error) {
	var a domain.Account
	if err := r.exec.Put(ctx, fmt.Sprintf("/accounts/%d", id), params, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// Delete issues a DELETE and returns the deleted account snapshot as echoed by Organizze.
func (r *AccountRepository) Delete(ctx context.Context, id int64) (*domain.Account, error) {
	var out domain.Account
	if err := r.exec.do(ctx, http.MethodDelete, fmt.Sprintf("/accounts/%d", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
