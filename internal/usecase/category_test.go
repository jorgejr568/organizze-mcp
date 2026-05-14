package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeCategoryRepo struct {
	listed       bool
	gotID        int64
	created      domain.CreateCategoryParams
	updatedID    int64
	deletedID    int64
	deletedRepID *int64
}

func (f *fakeCategoryRepo) List(_ context.Context) ([]domain.Category, error) {
	f.listed = true
	return nil, nil
}
func (f *fakeCategoryRepo) Get(_ context.Context, id int64) (*domain.Category, error) {
	f.gotID = id
	return &domain.Category{ID: id}, nil
}
func (f *fakeCategoryRepo) Create(_ context.Context, p domain.CreateCategoryParams) (*domain.Category, error) {
	f.created = p
	return &domain.Category{ID: 42, Name: p.Name}, nil
}
func (f *fakeCategoryRepo) Update(_ context.Context, id int64, _ domain.UpdateCategoryParams) (*domain.Category, error) {
	f.updatedID = id
	return &domain.Category{ID: id}, nil
}
func (f *fakeCategoryRepo) Delete(_ context.Context, id int64, replacementID *int64) (*domain.Category, error) {
	f.deletedID, f.deletedRepID = id, replacementID
	return &domain.Category{ID: id, Name: "Marketing"}, nil
}

func TestCategoryService_DelegatesBothCalls(t *testing.T) {
	repo := &fakeCategoryRepo{}
	svc := NewCategoryService(repo)
	if _, err := svc.List(context.Background()); err != nil {
		t.Errorf("List: %v", err)
	}
	if !repo.listed {
		t.Error("List not called")
	}
	if c, _ := svc.Get(context.Background(), 10); c.ID != 10 {
		t.Errorf("Get: %+v", c)
	}
}

func TestCategoryService_Create_RequiresName(t *testing.T) {
	svc := NewCategoryService(&fakeCategoryRepo{})
	if _, err := svc.Create(context.Background(), domain.CreateCategoryParams{}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err=%v, want ErrValidation", err)
	}
}

func TestCategoryService_Create_Succeeds(t *testing.T) {
	repo := &fakeCategoryRepo{}
	svc := NewCategoryService(repo)
	c, err := svc.Create(context.Background(), domain.CreateCategoryParams{Name: "Groceries"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.ID != 42 || repo.created.Name != "Groceries" {
		t.Errorf("c=%+v repo.created=%+v", c, repo.created)
	}
}

func TestCategoryService_UpdateDelete(t *testing.T) {
	repo := &fakeCategoryRepo{}
	svc := NewCategoryService(repo)
	if _, err := svc.Update(context.Background(), 42, domain.UpdateCategoryParams{}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if repo.updatedID != 42 {
		t.Errorf("repo.updatedID = %d", repo.updatedID)
	}
	rep := int64(99)
	if _, err := svc.Delete(context.Background(), 42, &rep); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.deletedID != 42 || repo.deletedRepID == nil || *repo.deletedRepID != 99 {
		t.Errorf("repo.deletedID=%d repo.deletedRepID=%v", repo.deletedID, repo.deletedRepID)
	}
}

func TestCategoryService_Delete_NoReplacement(t *testing.T) {
	repo := &fakeCategoryRepo{}
	svc := NewCategoryService(repo)
	if _, err := svc.Delete(context.Background(), 42, nil); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.deletedRepID != nil {
		t.Errorf("repo.deletedRepID = %v, want nil", repo.deletedRepID)
	}
}

func TestCategoryService_Delete_ReturnsDeletedCategory(t *testing.T) {
	repo := &fakeCategoryRepo{}
	svc := NewCategoryService(repo)
	c, err := svc.Delete(context.Background(), 6, nil)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if c == nil || c.ID != 6 {
		t.Errorf("returned = %+v", c)
	}
}
