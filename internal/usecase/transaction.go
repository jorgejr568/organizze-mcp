package usecase

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// TransactionReader is the read-only slice of TransactionRepository.
type TransactionReader interface {
	List(ctx context.Context, filter domain.ListTransactionsFilter) ([]domain.Transaction, error)
	Get(ctx context.Context, id int64) (*domain.Transaction, error)
}

// TransactionWriter is the mutating slice of TransactionRepository.
type TransactionWriter interface {
	Create(ctx context.Context, params domain.CreateTransactionParams) (*domain.Transaction, error)
	Update(ctx context.Context, id int64, params domain.UpdateTransactionParams) (*domain.Transaction, error)
	Delete(ctx context.Context, id int64) error
}

// TransactionRepository composes reader and writer for callers that need both.
type TransactionRepository interface {
	TransactionReader
	TransactionWriter
}

// TransactionService orchestrates transaction operations. Create validates that
// the four required Organizze fields are present before hitting the repo.
type TransactionService struct {
	repo TransactionRepository
}

func NewTransactionService(repo TransactionRepository) *TransactionService {
	return &TransactionService{repo: repo}
}

func (s *TransactionService) List(ctx context.Context, filter domain.ListTransactionsFilter) ([]domain.Transaction, error) {
	return s.repo.List(ctx, filter)
}

func (s *TransactionService) Get(ctx context.Context, id int64) (*domain.Transaction, error) {
	return s.repo.Get(ctx, id)
}

func (s *TransactionService) Create(ctx context.Context, p domain.CreateTransactionParams) (*domain.Transaction, error) {
	if err := validateCreate(p); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, p)
}

func (s *TransactionService) Update(ctx context.Context, id int64, p domain.UpdateTransactionParams) (*domain.Transaction, error) {
	return s.repo.Update(ctx, id, p)
}

func (s *TransactionService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func validateCreate(p domain.CreateTransactionParams) error {
	switch {
	case p.Description == "":
		return fmt.Errorf("%w: description is required", domain.ErrValidation)
	case p.Date == "":
		return fmt.Errorf("%w: date is required", domain.ErrValidation)
	case p.AmountCents == 0:
		return fmt.Errorf("%w: amount_cents must be non-zero", domain.ErrValidation)
	case p.AccountID == 0:
		return fmt.Errorf("%w: account_id is required", domain.ErrValidation)
	case p.CategoryID == 0:
		return fmt.Errorf("%w: category_id is required", domain.ErrValidation)
	}
	if p.Recurrence != nil && p.Installments != nil {
		return fmt.Errorf("%w: recurrence_attributes and installments_attributes are mutually exclusive", domain.ErrValidation)
	}
	if p.Recurrence != nil && !p.Recurrence.Periodicity.Valid() {
		return fmt.Errorf("%w: recurrence.periodicity must be one of %v", domain.ErrValidation, domain.AllPeriodicities)
	}
	if p.Installments != nil {
		if !p.Installments.Periodicity.Valid() {
			return fmt.Errorf("%w: installments.periodicity must be one of %v", domain.ErrValidation, domain.AllPeriodicities)
		}
		if p.Installments.Total <= 0 {
			return fmt.Errorf("%w: installments.total must be > 0", domain.ErrValidation)
		}
	}
	return nil
}
