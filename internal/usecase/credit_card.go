package usecase

import (
	"context"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type CreditCardRepository interface {
	List(ctx context.Context) ([]domain.CreditCard, error)
	Get(ctx context.Context, id int64) (*domain.CreditCard, error)
}

type CreditCardService struct {
	repo CreditCardRepository
}

func NewCreditCardService(repo CreditCardRepository) *CreditCardService {
	return &CreditCardService{repo: repo}
}

func (s *CreditCardService) List(ctx context.Context) ([]domain.CreditCard, error) {
	return s.repo.List(ctx)
}

func (s *CreditCardService) Get(ctx context.Context, id int64) (*domain.CreditCard, error) {
	return s.repo.Get(ctx, id)
}
