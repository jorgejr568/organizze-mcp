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

// Delete issues a DELETE. If replacementID is non-nil, the request body is
// {"replacement_id": ID} which tells Organizze to reassign affected
// transactions to that category (per ORGANIZZE_API.md "Excluir uma categoria").
// Returns the deleted Category as echoed by the API.
func (r *CategoryRepository) Delete(ctx context.Context, id int64, replacementID *int64) (*domain.Category, error) {
	var body any
	if replacementID != nil {
		body = struct {
			ReplacementID int64 `json:"replacement_id"`
		}{ReplacementID: *replacementID}
	}
	var out domain.Category
	if err := r.exec.Delete(ctx, fmt.Sprintf("/categories/%d", id), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
