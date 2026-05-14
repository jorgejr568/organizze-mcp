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
	cases := []struct {
		name string
		in   domain.CreateTransactionParams
	}{
		{"description missing", domain.CreateTransactionParams{}},
		{"date missing", domain.CreateTransactionParams{Description: "x"}},
		{"amount_cents zero", domain.CreateTransactionParams{Description: "x", Date: "2026-05-14"}},
		{"account_id zero", domain.CreateTransactionParams{Description: "x", Date: "2026-05-14", AmountCents: -1500}},
		{"category_id zero", domain.CreateTransactionParams{Description: "x", Date: "2026-05-14", AmountCents: -1500, AccountID: 1}},
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

func TestTransactionService_Create_ValidatesRecurrencePeriodicity(t *testing.T) {
	base := domain.CreateTransactionParams{
		Description: "Despesa fixa", Date: "2026-05-14", AmountCents: -10000,
		AccountID: 1, CategoryID: 10,
	}
	t.Run("empty periodicity is rejected", func(t *testing.T) {
		in := base
		in.Recurrence = &domain.RecurrenceAttributes{}
		svc := NewTransactionService(&fakeTransactionRepo{})
		if _, err := svc.Create(context.Background(), in); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("err=%v, want ErrValidation", err)
		}
	})
	t.Run("unknown periodicity is rejected", func(t *testing.T) {
		in := base
		in.Recurrence = &domain.RecurrenceAttributes{Periodicity: "daily"}
		svc := NewTransactionService(&fakeTransactionRepo{})
		if _, err := svc.Create(context.Background(), in); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("err=%v, want ErrValidation", err)
		}
	})
	t.Run("valid periodicity is forwarded", func(t *testing.T) {
		in := base
		in.Recurrence = &domain.RecurrenceAttributes{Periodicity: domain.PeriodicityMonthly}
		repo := &fakeTransactionRepo{}
		svc := NewTransactionService(repo)
		if _, err := svc.Create(context.Background(), in); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if repo.created.Recurrence == nil || repo.created.Recurrence.Periodicity != domain.PeriodicityMonthly {
			t.Errorf("repo received %+v", repo.created)
		}
	})
}

func TestTransactionService_Create_RejectsBothRecurrenceAndInstallments(t *testing.T) {
	svc := NewTransactionService(&fakeTransactionRepo{})
	_, err := svc.Create(context.Background(), domain.CreateTransactionParams{
		Description: "x", Date: "2026-05-14", AmountCents: 1, AccountID: 1, CategoryID: 1,
		Recurrence:   &domain.RecurrenceAttributes{Periodicity: domain.PeriodicityMonthly},
		Installments: &domain.InstallmentsAttributes{Periodicity: domain.PeriodicityMonthly, Total: 12},
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}

func TestTransactionService_Create_RejectsInstallments_BadPeriodicity(t *testing.T) {
	svc := NewTransactionService(&fakeTransactionRepo{})
	_, err := svc.Create(context.Background(), domain.CreateTransactionParams{
		Description: "x", Date: "2026-05-14", AmountCents: 1, AccountID: 1, CategoryID: 1,
		Installments: &domain.InstallmentsAttributes{Periodicity: "daily", Total: 12},
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}

func TestTransactionService_Create_RejectsInstallments_NonPositiveTotal(t *testing.T) {
	svc := NewTransactionService(&fakeTransactionRepo{})
	_, err := svc.Create(context.Background(), domain.CreateTransactionParams{
		Description: "x", Date: "2026-05-14", AmountCents: 1, AccountID: 1, CategoryID: 1,
		Installments: &domain.InstallmentsAttributes{Periodicity: domain.PeriodicityMonthly, Total: 0},
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
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
