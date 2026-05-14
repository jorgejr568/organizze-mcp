package organizze

import (
	"context"
	"fmt"

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
