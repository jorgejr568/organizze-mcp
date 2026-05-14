package organizze

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// CategoryRepository lists and fetches categories.
type CategoryRepository struct {
	exec *RequestExecutor
}

func NewCategoryRepository(exec *RequestExecutor) *CategoryRepository {
	return &CategoryRepository{exec: exec}
}

func (r *CategoryRepository) List(ctx context.Context) ([]domain.Category, error) {
	var out []domain.Category
	if err := r.exec.Get(ctx, "/categories", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *CategoryRepository) Get(ctx context.Context, id int64) (*domain.Category, error) {
	var c domain.Category
	if err := r.exec.Get(ctx, fmt.Sprintf("/categories/%d", id), &c); err != nil {
		return nil, err
	}
	return &c, nil
}
