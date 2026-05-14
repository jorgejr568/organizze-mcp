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

// Create issues a POST and returns the persisted credit card.
func (r *CreditCardRepository) Create(ctx context.Context, params domain.CreateCreditCardParams) (*domain.CreditCard, error) {
	var cc domain.CreditCard
	if err := r.exec.Post(ctx, "/credit_cards", params, &cc); err != nil {
		return nil, err
	}
	return &cc, nil
}

// Update issues a PUT with only the non-nil fields from params.
func (r *CreditCardRepository) Update(ctx context.Context, id int64, params domain.UpdateCreditCardParams) (*domain.CreditCard, error) {
	var cc domain.CreditCard
	if err := r.exec.Put(ctx, fmt.Sprintf("/credit_cards/%d", id), params, &cc); err != nil {
		return nil, err
	}
	return &cc, nil
}

// Delete issues a DELETE.
func (r *CreditCardRepository) Delete(ctx context.Context, id int64) error {
	return r.exec.Delete(ctx, fmt.Sprintf("/credit_cards/%d", id), nil, nil)
}
