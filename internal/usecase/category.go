package usecase

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type CategoryReader interface {
	List(ctx context.Context) ([]domain.Category, error)
	Get(ctx context.Context, id int64) (*domain.Category, error)
}

type CategoryWriter interface {
	Create(ctx context.Context, params domain.CreateCategoryParams) (*domain.Category, error)
	Update(ctx context.Context, id int64, params domain.UpdateCategoryParams) (*domain.Category, error)
	Delete(ctx context.Context, id int64, replacementID *int64) (*domain.Category, error)
}

type CategoryRepository interface {
	CategoryReader
	CategoryWriter
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

func (s *CategoryService) Create(ctx context.Context, p domain.CreateCategoryParams) (*domain.Category, error) {
	if p.Name == "" {
		return nil, fmt.Errorf("%w: name is required", domain.ErrValidation)
	}
	return s.repo.Create(ctx, p)
}

func (s *CategoryService) Update(ctx context.Context, id int64, p domain.UpdateCategoryParams) (*domain.Category, error) {
	return s.repo.Update(ctx, id, p)
}

func (s *CategoryService) Delete(ctx context.Context, id int64, replacementID *int64) (*domain.Category, error) {
	return s.repo.Delete(ctx, id, replacementID)
}
