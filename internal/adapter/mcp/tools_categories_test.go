package mcp

import (
	"context"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeCategorySvc struct {
	list []domain.Category
	one  *domain.Category
}

func (f *fakeCategorySvc) List(context.Context) ([]domain.Category, error)         { return f.list, nil }
func (f *fakeCategorySvc) Get(context.Context, int64) (*domain.Category, error)    { return f.one, nil }

type nopCategorySvc struct{}

func (nopCategorySvc) List(context.Context) ([]domain.Category, error)             { return nil, nil }
func (nopCategorySvc) Get(context.Context, int64) (*domain.Category, error)        { return &domain.Category{}, nil }

func TestCategoryHandlers(t *testing.T) {
	svc := &fakeCategorySvc{
		list: []domain.Category{{ID: 10, Name: "Food"}},
		one:  &domain.Category{ID: 10, Name: "Food"},
	}
	hList := listCategoriesHandler(svc)
	if _, out, err := hList(context.Background(), &mcpsdk.CallToolRequest{}, struct{}{}); err != nil || len(out.Categories) != 1 {
		t.Fatalf("list: out=%+v err=%v", out, err)
	}
	hGet := getCategoryHandler(svc)
	if _, out, err := hGet(context.Background(), &mcpsdk.CallToolRequest{}, GetCategoryInput{ID: 10}); err != nil || out.Category.ID != 10 {
		t.Fatalf("get: out=%+v err=%v", out, err)
	}
}
