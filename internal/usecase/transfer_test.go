package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeTransferRepo struct {
	listFilter domain.ListTransfersFilter
	created    domain.CreateTransferParams
	updatedID  int64
	deletedID  int64
}

func (f *fakeTransferRepo) List(_ context.Context, fl domain.ListTransfersFilter) ([]domain.Transfer, error) {
	f.listFilter = fl
	return nil, nil
}
func (f *fakeTransferRepo) Create(_ context.Context, p domain.CreateTransferParams) (*domain.Transfer, error) {
	f.created = p
	return &domain.Transfer{ID: 123, AmountCents: p.AmountCents}, nil
}
func (f *fakeTransferRepo) Update(_ context.Context, id int64, _ domain.UpdateTransferParams) (*domain.Transfer, error) {
	f.updatedID = id
	return &domain.Transfer{ID: id}, nil
}
func (f *fakeTransferRepo) Delete(_ context.Context, id int64) error {
	f.deletedID = id
	return nil
}

func TestTransferService_PassesFilter(t *testing.T) {
	repo := &fakeTransferRepo{}
	svc := NewTransferService(repo)
	_, err := svc.List(context.Background(), domain.ListTransfersFilter{StartDate: "2026-05-01"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if repo.listFilter.StartDate != "2026-05-01" {
		t.Errorf("filter not forwarded: %+v", repo.listFilter)
	}
}

func TestTransferService_Create_ValidatesRequiredFields(t *testing.T) {
	svc := NewTransferService(&fakeTransferRepo{})
	cases := []struct {
		name string
		in   domain.CreateTransferParams
	}{
		{"credit_account_id zero", domain.CreateTransferParams{DebitAccountID: 1, AmountCents: 100, Date: "2026-05-14"}},
		{"debit_account_id zero", domain.CreateTransferParams{CreditAccountID: 2, AmountCents: 100, Date: "2026-05-14"}},
		{"amount_cents zero", domain.CreateTransferParams{CreditAccountID: 2, DebitAccountID: 1, Date: "2026-05-14"}},
		{"date missing", domain.CreateTransferParams{CreditAccountID: 2, DebitAccountID: 1, AmountCents: 100}},
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

func TestTransferService_Create_Succeeds(t *testing.T) {
	repo := &fakeTransferRepo{}
	svc := NewTransferService(repo)
	tr, err := svc.Create(context.Background(), domain.CreateTransferParams{
		CreditAccountID: 2, DebitAccountID: 1, AmountCents: 50000, Date: "2026-05-14", Paid: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tr.ID != 123 || repo.created.AmountCents != 50000 {
		t.Errorf("tr=%+v repo.created=%+v", tr, repo.created)
	}
}

func TestTransferService_UpdateDelete(t *testing.T) {
	repo := &fakeTransferRepo{}
	svc := NewTransferService(repo)
	if _, err := svc.Update(context.Background(), 123, domain.UpdateTransferParams{}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if repo.updatedID != 123 {
		t.Errorf("repo.updatedID = %d", repo.updatedID)
	}
	if err := svc.Delete(context.Background(), 123); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.deletedID != 123 {
		t.Errorf("repo.deletedID = %d", repo.deletedID)
	}
}
