package organizze

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

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

// Create issues a POST and returns the persisted category.
func (r *CategoryRepository) Create(ctx context.Context, params domain.CreateCategoryParams) (*domain.Category, error) {
	var c domain.Category
	if err := r.exec.Post(ctx, "/categories", params, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Update issues a PUT with only the non-nil fields from params.
func (r *CategoryRepository) Update(ctx context.Context, id int64, params domain.UpdateCategoryParams) (*domain.Category, error) {
	var c domain.Category
	if err := r.exec.Put(ctx, fmt.Sprintf("/categories/%d", id), params, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Delete issues a DELETE. If replacementID is non-nil, the affected transactions
// are reassigned to that category (Organizze "replacement_id" query parameter).
func (r *CategoryRepository) Delete(ctx context.Context, id int64, replacementID *int64) error {
	path := fmt.Sprintf("/categories/%d", id)
	if replacementID != nil {
		q := url.Values{}
		q.Set("replacement_id", strconv.FormatInt(*replacementID, 10))
		path += "?" + q.Encode()
	}
	return r.exec.Delete(ctx, path, nil, nil)
}
