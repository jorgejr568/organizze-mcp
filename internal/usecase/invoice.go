package usecase

import (
	"context"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type InvoiceRepository interface {
	List(ctx context.Context, creditCardID int64, filter domain.ListInvoicesFilter) ([]domain.Invoice, error)
	Get(ctx context.Context, creditCardID, invoiceID int64) (*domain.Invoice, error)
	Payment(ctx context.Context, creditCardID, invoiceID int64) (*domain.Transaction, error)
}

type InvoiceService struct {
	repo InvoiceRepository
}

func NewInvoiceService(repo InvoiceRepository) *InvoiceService {
	return &InvoiceService{repo: repo}
}

func (s *InvoiceService) List(ctx context.Context, creditCardID int64, filter domain.ListInvoicesFilter) ([]domain.Invoice, error) {
	return s.repo.List(ctx, creditCardID, filter)
}

func (s *InvoiceService) Get(ctx context.Context, creditCardID, invoiceID int64) (*domain.Invoice, error) {
	return s.repo.Get(ctx, creditCardID, invoiceID)
}

func (s *InvoiceService) Payment(ctx context.Context, creditCardID, invoiceID int64) (*domain.Transaction, error) {
	return s.repo.Payment(ctx, creditCardID, invoiceID)
}
