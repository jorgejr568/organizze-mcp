package usecase

import (
	"context"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeInvoiceRepo struct {
	gotFilter domain.ListInvoicesFilter
}

func (f *fakeInvoiceRepo) List(_ context.Context, _ int64, filter domain.ListInvoicesFilter) ([]domain.Invoice, error) {
	f.gotFilter = filter
	return []domain.Invoice{{ID: 1}}, nil
}
func (f *fakeInvoiceRepo) Get(_ context.Context, _ int64, _ int64) (*domain.Invoice, error) {
	return &domain.Invoice{ID: 1}, nil
}

func TestInvoiceService(t *testing.T) {
	repo := &fakeInvoiceRepo{}
	svc := NewInvoiceService(repo)
	filter := domain.ListInvoicesFilter{StartDate: "2024-01-01", EndDate: "2024-12-31"}
	if xs, _ := svc.List(context.Background(), 9, filter); len(xs) != 1 {
		t.Errorf("List: %v", xs)
	}
	if repo.gotFilter != filter {
		t.Errorf("filter forwarded = %+v, want %+v", repo.gotFilter, filter)
	}
	if v, _ := svc.Get(context.Background(), 9, 1); v.ID != 1 {
		t.Errorf("Get: %+v", v)
	}
}
