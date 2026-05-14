package usecase

import (
	"context"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeInvoiceRepo struct{}

func (f *fakeInvoiceRepo) List(_ context.Context, _ int64) ([]domain.Invoice, error) {
	return []domain.Invoice{{ID: 1}}, nil
}
func (f *fakeInvoiceRepo) Get(_ context.Context, _ int64, _ int64) (*domain.Invoice, error) {
	return &domain.Invoice{ID: 1}, nil
}

func TestInvoiceService(t *testing.T) {
	svc := NewInvoiceService(&fakeInvoiceRepo{})
	if xs, _ := svc.List(context.Background(), 9); len(xs) != 1 {
		t.Errorf("List: %v", xs)
	}
	if v, _ := svc.Get(context.Background(), 9, 1); v.ID != 1 {
		t.Errorf("Get: %+v", v)
	}
}
