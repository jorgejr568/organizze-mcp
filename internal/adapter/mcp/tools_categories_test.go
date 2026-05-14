package mcp

import (
	"context"
	"errors"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeCategorySvc struct {
	listed  bool
	gotID   int64
	created domain.CreateCategoryParams
	updated struct {
		id     int64
		params domain.UpdateCategoryParams
	}
	deletedID    int64
	deletedRepID *int64
	createErr    error
}

func (f *fakeCategorySvc) List(_ context.Context) ([]domain.Category, error) {
	f.listed = true
	return []domain.Category{{ID: 1}}, nil
}
func (f *fakeCategorySvc) Get(_ context.Context, id int64) (*domain.Category, error) {
	f.gotID = id
	return &domain.Category{ID: id}, nil
}
func (f *fakeCategorySvc) Create(_ context.Context, p domain.CreateCategoryParams) (*domain.Category, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.created = p
	return &domain.Category{ID: 42, Name: p.Name}, nil
}
func (f *fakeCategorySvc) Update(_ context.Context, id int64, p domain.UpdateCategoryParams) (*domain.Category, error) {
	f.updated.id, f.updated.params = id, p
	return &domain.Category{ID: id}, nil
}
func (f *fakeCategorySvc) Delete(_ context.Context, id int64, replacementID *int64) (*domain.Category, error) {
	f.deletedID, f.deletedRepID = id, replacementID
	return &domain.Category{ID: id, Name: "Marketing"}, nil
}

type nopCategorySvc struct{}

func (nopCategorySvc) List(context.Context) ([]domain.Category, error) { return nil, nil }
func (nopCategorySvc) Get(context.Context, int64) (*domain.Category, error) {
	return &domain.Category{}, nil
}
func (nopCategorySvc) Create(context.Context, domain.CreateCategoryParams) (*domain.Category, error) {
	return &domain.Category{}, nil
}
func (nopCategorySvc) Update(context.Context, int64, domain.UpdateCategoryParams) (*domain.Category, error) {
	return &domain.Category{}, nil
}
func (nopCategorySvc) Delete(context.Context, int64, *int64) (*domain.Category, error) {
	return nil, nil
}

func TestCategoryHandlers(t *testing.T) {
	svc := &fakeCategorySvc{}
	hList := listCategoriesHandler(svc)
	if _, out, err := hList(context.Background(), &mcpsdk.CallToolRequest{}, struct{}{}); err != nil || len(out.Categories) != 1 {
		t.Fatalf("list: out=%+v err=%v", out, err)
	}
	hGet := getCategoryHandler(svc)
	if _, out, err := hGet(context.Background(), &mcpsdk.CallToolRequest{}, GetCategoryInput{ID: 10}); err != nil || out.Category.ID != 10 {
		t.Fatalf("get: out=%+v err=%v", out, err)
	}
}

func TestCreateCategoryHandler(t *testing.T) {
	svc := &fakeCategorySvc{}
	h := createCategoryHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateCategoryInput{Name: "Groceries"})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Category.ID != 42 || svc.created.Name != "Groceries" {
		t.Errorf("out=%+v svc.created=%+v", out, svc.created)
	}
}

func TestCreateCategoryHandler_PropagatesValidationError(t *testing.T) {
	svc := &fakeCategorySvc{createErr: domain.ErrValidation}
	h := createCategoryHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateCategoryInput{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
}

func TestUpdateCategoryHandler(t *testing.T) {
	svc := &fakeCategorySvc{}
	h := updateCategoryHandler(svc)
	name := "Food"
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, UpdateCategoryInput{
		ID: 42, Name: &name,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Category.ID != 42 || svc.updated.id != 42 || *svc.updated.params.Name != "Food" {
		t.Errorf("out=%+v svc.updated=%+v", out, svc.updated)
	}
}

func TestDeleteCategoryHandler_NoReplacement(t *testing.T) {
	svc := &fakeCategorySvc{}
	h := deleteCategoryHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, DeleteCategoryInput{ID: 42})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !out.Deleted || out.ID != 42 || svc.deletedID != 42 || svc.deletedRepID != nil {
		t.Errorf("out=%+v svc.deletedID=%d svc.deletedRepID=%v", out, svc.deletedID, svc.deletedRepID)
	}
	if out.Category == nil || out.Category.ID != 42 {
		t.Errorf("out.Category = %+v, want category with ID 42", out.Category)
	}
}

func TestDeleteCategoryHandler_WithReplacement(t *testing.T) {
	svc := &fakeCategorySvc{}
	h := deleteCategoryHandler(svc)
	rep := int64(99)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, DeleteCategoryInput{ID: 42, ReplacementID: &rep})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if svc.deletedRepID == nil || *svc.deletedRepID != 99 {
		t.Errorf("svc.deletedRepID = %v", svc.deletedRepID)
	}
}
