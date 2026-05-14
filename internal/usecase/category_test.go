package usecase

import (
	"context"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeCategoryRepo struct {
	list []domain.Category
	one  *domain.Category
}

func (f *fakeCategoryRepo) List(context.Context) ([]domain.Category, error) {
	return f.list, nil
}
func (f *fakeCategoryRepo) Get(context.Context, int64) (*domain.Category, error) {
	return f.one, nil
}

func TestCategoryService_DelegatesBothCalls(t *testing.T) {
	repo := &fakeCategoryRepo{
		list: []domain.Category{{ID: 10, Name: "Food"}},
		one:  &domain.Category{ID: 10, Name: "Food"},
	}
	svc := NewCategoryService(repo)
	if xs, _ := svc.List(context.Background()); len(xs) != 1 {
		t.Errorf("List: %v", xs)
	}
	if c, _ := svc.Get(context.Background(), 10); c.ID != 10 {
		t.Errorf("Get: %+v", c)
	}
}
