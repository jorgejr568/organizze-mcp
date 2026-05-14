package usecase

import (
	"context"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type AccountRepository interface {
	List(ctx context.Context) ([]domain.Account, error)
	Get(ctx context.Context, id int64) (*domain.Account, error)
}

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
