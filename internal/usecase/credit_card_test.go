package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeCreditCardRepo struct {
	listed    bool
	gotID     int64
	created   domain.CreateCreditCardParams
	updatedID int64
	deletedID int64
}

func (f *fakeCreditCardRepo) List(_ context.Context) ([]domain.CreditCard, error) {
	f.listed = true
	return nil, nil
}
func (f *fakeCreditCardRepo) Get(_ context.Context, id int64) (*domain.CreditCard, error) {
	f.gotID = id
	return &domain.CreditCard{ID: id}, nil
}
func (f *fakeCreditCardRepo) Create(_ context.Context, p domain.CreateCreditCardParams) (*domain.CreditCard, error) {
	f.created = p
	return &domain.CreditCard{ID: 7, Name: p.Name}, nil
}
func (f *fakeCreditCardRepo) Update(_ context.Context, id int64, _ domain.UpdateCreditCardParams) (*domain.CreditCard, error) {
	f.updatedID = id
	return &domain.CreditCard{ID: id}, nil
}
func (f *fakeCreditCardRepo) Delete(_ context.Context, id int64) error {
	f.deletedID = id
	return nil
}

func TestCreditCardService(t *testing.T) {
	repo := &fakeCreditCardRepo{}
	svc := NewCreditCardService(repo)
	if _, err := svc.List(context.Background()); err != nil {
		t.Errorf("List: %v", err)
	}
	if !repo.listed {
		t.Error("repo.List not called")
	}
	if c, _ := svc.Get(context.Background(), 1); c.ID != 1 {
		t.Errorf("Get: %+v", c)
	}
}

func TestCreditCardService_Create_ValidatesRequiredFields(t *testing.T) {
	svc := NewCreditCardService(&fakeCreditCardRepo{})
	cases := []struct {
		name string
		in   domain.CreateCreditCardParams
	}{
		{"name missing", domain.CreateCreditCardParams{DueDay: 27, ClosingDay: 20}},
		{"due_day zero", domain.CreateCreditCardParams{Name: "x", ClosingDay: 20}},
		{"due_day > 31", domain.CreateCreditCardParams{Name: "x", DueDay: 32, ClosingDay: 20}},
		{"closing_day zero", domain.CreateCreditCardParams{Name: "x", DueDay: 27}},
		{"closing_day > 31", domain.CreateCreditCardParams{Name: "x", DueDay: 27, ClosingDay: 99}},
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

func TestCreditCardService_Create_Succeeds(t *testing.T) {
	repo := &fakeCreditCardRepo{}
	svc := NewCreditCardService(repo)
	cc, err := svc.Create(context.Background(), domain.CreateCreditCardParams{
		Name: "Nubank", DueDay: 27, ClosingDay: 20,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if cc.ID != 7 || repo.created.Name != "Nubank" {
		t.Errorf("cc=%+v repo.created=%+v", cc, repo.created)
	}
}

func TestCreditCardService_UpdateDelete(t *testing.T) {
	repo := &fakeCreditCardRepo{}
	svc := NewCreditCardService(repo)
	if _, err := svc.Update(context.Background(), 7, domain.UpdateCreditCardParams{}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if repo.updatedID != 7 {
		t.Errorf("repo.updatedID = %d", repo.updatedID)
	}
	if err := svc.Delete(context.Background(), 7); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.deletedID != 7 {
		t.Errorf("repo.deletedID = %d", repo.deletedID)
	}
}
