package usecase

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type CreditCardReader interface {
	List(ctx context.Context) ([]domain.CreditCard, error)
	Get(ctx context.Context, id int64) (*domain.CreditCard, error)
}

type CreditCardWriter interface {
	Create(ctx context.Context, params domain.CreateCreditCardParams) (*domain.CreditCard, error)
	Update(ctx context.Context, id int64, params domain.UpdateCreditCardParams) (*domain.CreditCard, error)
	Delete(ctx context.Context, id int64) error
}

type CreditCardRepository interface {
	CreditCardReader
	CreditCardWriter
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

func (s *CreditCardService) Create(ctx context.Context, p domain.CreateCreditCardParams) (*domain.CreditCard, error) {
	switch {
	case p.Name == "":
		return nil, fmt.Errorf("%w: name is required", domain.ErrValidation)
	case p.DueDay < 1 || p.DueDay > 31:
		return nil, fmt.Errorf("%w: due_day must be between 1 and 31", domain.ErrValidation)
	case p.ClosingDay < 1 || p.ClosingDay > 31:
		return nil, fmt.Errorf("%w: closing_day must be between 1 and 31", domain.ErrValidation)
	}
	return s.repo.Create(ctx, p)
}

func (s *CreditCardService) Update(ctx context.Context, id int64, p domain.UpdateCreditCardParams) (*domain.CreditCard, error) {
	return s.repo.Update(ctx, id, p)
}

func (s *CreditCardService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
