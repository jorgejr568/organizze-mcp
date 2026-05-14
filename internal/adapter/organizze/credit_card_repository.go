package organizze

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// CreditCardRepository lists and fetches credit cards.
type CreditCardRepository struct {
	exec *RequestExecutor
}

func NewCreditCardRepository(exec *RequestExecutor) *CreditCardRepository {
	return &CreditCardRepository{exec: exec}
}

func (r *CreditCardRepository) List(ctx context.Context) ([]domain.CreditCard, error) {
	var out []domain.CreditCard
	if err := r.exec.Get(ctx, "/credit_cards", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *CreditCardRepository) Get(ctx context.Context, id int64) (*domain.CreditCard, error) {
	var c domain.CreditCard
	if err := r.exec.Get(ctx, fmt.Sprintf("/credit_cards/%d", id), &c); err != nil {
		return nil, err
	}
	return &c, nil
}
