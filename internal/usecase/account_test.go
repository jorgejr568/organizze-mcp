package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeAccountRepo struct {
	listed    bool
	gotID     int64
	created   domain.CreateAccountParams
	updatedID int64
	deletedID int64
}

func (f *fakeAccountRepo) List(_ context.Context) ([]domain.Account, error) {
	f.listed = true
	return nil, nil
}
func (f *fakeAccountRepo) Get(_ context.Context, id int64) (*domain.Account, error) {
	f.gotID = id
	return &domain.Account{ID: id}, nil
}
func (f *fakeAccountRepo) Create(_ context.Context, p domain.CreateAccountParams) (*domain.Account, error) {
	f.created = p
	return &domain.Account{ID: 18, Name: p.Name, Type: p.Type}, nil
}
func (f *fakeAccountRepo) Update(_ context.Context, id int64, _ domain.UpdateAccountParams) (*domain.Account, error) {
	f.updatedID = id
	return &domain.Account{ID: id}, nil
}
func (f *fakeAccountRepo) Delete(_ context.Context, id int64) error {
	f.deletedID = id
	return nil
}

func TestAccountService_DelegatesBothCalls(t *testing.T) {
	repo := &fakeAccountRepo{}
	svc := NewAccountService(repo)

	if _, err := svc.List(context.Background()); err != nil {
		t.Fatalf("List: %v", err)
	}
	if !repo.listed {
		t.Errorf("expected repo.List to be called")
	}

	a, err := svc.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if a.ID != 1 || repo.gotID != 1 {
		t.Errorf("got a=%+v repo.gotID=%d", a, repo.gotID)
	}
}

func TestAccountService_Create_ValidatesRequiredFields(t *testing.T) {
	svc := NewAccountService(&fakeAccountRepo{})
	cases := []struct {
		name string
		in   domain.CreateAccountParams
	}{
		{"name missing", domain.CreateAccountParams{Type: "checking"}},
		{"type missing", domain.CreateAccountParams{Name: "Checking"}},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if _, err := svc.Create(context.Background(), c.in); !errors.Is(err, domain.ErrValidation) {
				t.Errorf("err=%v, want ErrValidation", err)
			}
		})
	}
}

func TestAccountService_Create_Succeeds(t *testing.T) {
	repo := &fakeAccountRepo{}
	svc := NewAccountService(repo)
	a, err := svc.Create(context.Background(), domain.CreateAccountParams{
		Name: "Itaú CC", Type: "checking",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.ID != 18 || repo.created.Type != "checking" {
		t.Errorf("a=%+v repo.created=%+v", a, repo.created)
	}
}

func TestAccountService_UpdateDelete(t *testing.T) {
	repo := &fakeAccountRepo{}
	svc := NewAccountService(repo)
	if _, err := svc.Update(context.Background(), 18, domain.UpdateAccountParams{}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if repo.updatedID != 18 {
		t.Errorf("repo.updatedID = %d", repo.updatedID)
	}
	if err := svc.Delete(context.Background(), 18); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.deletedID != 18 {
		t.Errorf("repo.deletedID = %d", repo.deletedID)
	}
}
