package usecase

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// AccountReader is the read-only slice of AccountRepository.
type AccountReader interface {
	List(ctx context.Context) ([]domain.Account, error)
	Get(ctx context.Context, id int64) (*domain.Account, error)
}

// AccountWriter is the mutating slice of AccountRepository.
type AccountWriter interface {
	Create(ctx context.Context, params domain.CreateAccountParams) (*domain.Account, error)
	Update(ctx context.Context, id int64, params domain.UpdateAccountParams) (*domain.Account, error)
	Delete(ctx context.Context, id int64) (*domain.Account, error)
}

// AccountRepository composes reader and writer for callers that need both.
type AccountRepository interface {
	AccountReader
	AccountWriter
}

// AccountService orchestrates account operations.
type AccountService struct {
	repo AccountRepository
}

func NewAccountService(repo AccountRepository) *AccountService {
	return &AccountService{repo: repo}
}

func (s *AccountService) List(ctx context.Context) ([]domain.Account, error) {
	return s.repo.List(ctx)
}

func (s *AccountService) Get(ctx context.Context, id int64) (*domain.Account, error) {
	return s.repo.Get(ctx, id)
}

func (s *AccountService) Create(ctx context.Context, p domain.CreateAccountParams) (*domain.Account, error) {
	switch {
	case p.Name == "":
		return nil, fmt.Errorf("%w: name is required", domain.ErrValidation)
	case p.Type == "":
		return nil, fmt.Errorf("%w: type is required", domain.ErrValidation)
	}
	return s.repo.Create(ctx, p)
}

func (s *AccountService) Update(ctx context.Context, id int64, p domain.UpdateAccountParams) (*domain.Account, error) {
	return s.repo.Update(ctx, id, p)
}

func (s *AccountService) Delete(ctx context.Context, id int64) (*domain.Account, error) {
	return s.repo.Delete(ctx, id)
}
