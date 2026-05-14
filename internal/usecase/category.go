package usecase

import (
	"context"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type CategoryRepository interface {
	List(ctx context.Context) ([]domain.Category, error)
	Get(ctx context.Context, id int64) (*domain.Category, error)
}

type CategoryService struct {
	repo CategoryRepository
}

func NewCategoryService(repo CategoryRepository) *CategoryService {
	return &CategoryService{repo: repo}
}

func (s *CategoryService) List(ctx context.Context) ([]domain.Category, error) {
	return s.repo.List(ctx)
}

func (s *CategoryService) Get(ctx context.Context, id int64) (*domain.Category, error) {
	return s.repo.Get(ctx, id)
}
