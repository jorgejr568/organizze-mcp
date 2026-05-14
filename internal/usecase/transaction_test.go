package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeTransactionRepo struct {
	listFilter domain.ListTransactionsFilter
	created    domain.CreateTransactionParams
	updatedID  int64
	deletedID  int64
}

func (f *fakeTransactionRepo) List(_ context.Context, fl domain.ListTransactionsFilter) ([]domain.Transaction, error) {
	f.listFilter = fl
	return nil, nil
}
func (f *fakeTransactionRepo) Get(_ context.Context, id int64) (*domain.Transaction, error) {
	return &domain.Transaction{ID: id}, nil
}
func (f *fakeTransactionRepo) Create(_ context.Context, p domain.CreateTransactionParams) (*domain.Transaction, error) {
	f.created = p
	return &domain.Transaction{ID: 777, Description: p.Description, AmountCents: p.AmountCents}, nil
}
func (f *fakeTransactionRepo) Update(_ context.Context, id int64, _ domain.UpdateTransactionParams) (*domain.Transaction, error) {
	f.updatedID = id
	return &domain.Transaction{ID: id}, nil
}
func (f *fakeTransactionRepo) Delete(_ context.Context, id int64) error {
	f.deletedID = id
	return nil
}

func TestTransactionService_ListAndGet(t *testing.T) {
	repo := &fakeTransactionRepo{}
	svc := NewTransactionService(repo)
	if _, err := svc.List(context.Background(), domain.ListTransactionsFilter{AccountID: 5}); err != nil {
		t.Fatalf("List: %v", err)
	}
	if repo.listFilter.AccountID != 5 {
		t.Errorf("filter not forwarded: %+v", repo.listFilter)
	}
	if tx, _ := svc.Get(context.Background(), 9); tx.ID != 9 {
		t.Errorf("Get: %+v", tx)
	}
}

func TestTransactionService_Create_ValidatesRequiredFields(t *testing.T) {
	svc := NewTransactionService(&fakeTransactionRepo{})
	cases := []domain.CreateTransactionParams{
		{}, // all zero
		{Description: "x"},
		{Description: "x", Date: "2026-05-14"},
		{Description: "x", Date: "2026-05-14", AccountID: 1},
		{Description: "x", Date: "2026-05-14", AccountID: 1, CategoryID: 2}, // AmountCents == 0
	}
	for i, p := range cases {
		_, err := svc.Create(context.Background(), p)
		if !errors.Is(err, domain.ErrValidation) {
			t.Errorf("case %d: err=%v, want ErrValidation", i, err)
		}
	}
}

func TestTransactionService_Create_Succeeds(t *testing.T) {
	repo := &fakeTransactionRepo{}
	svc := NewTransactionService(repo)
	tx, err := svc.Create(context.Background(), domain.CreateTransactionParams{
		Description: "Coffee", Date: "2026-05-14", AmountCents: -1500,
		AccountID: 1, CategoryID: 10, Paid: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tx.ID != 777 {
		t.Errorf("got %+v", tx)
	}
	if repo.created.AmountCents != -1500 {
		t.Errorf("repo received %+v", repo.created)
	}
}

func TestTransactionService_UpdateDelete(t *testing.T) {
	repo := &fakeTransactionRepo{}
	svc := NewTransactionService(repo)
	if _, err := svc.Update(context.Background(), 42, domain.UpdateTransactionParams{}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if repo.updatedID != 42 {
		t.Errorf("repo.updatedID = %d", repo.updatedID)
	}
	if err := svc.Delete(context.Background(), 42); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.deletedID != 42 {
		t.Errorf("repo.deletedID = %d", repo.deletedID)
	}
}
