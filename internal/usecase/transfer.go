package usecase

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type TransferReader interface {
	List(ctx context.Context, filter domain.ListTransfersFilter) ([]domain.Transfer, error)
	Get(ctx context.Context, id int64) (*domain.Transfer, error)
}

type TransferWriter interface {
	Create(ctx context.Context, params domain.CreateTransferParams) (*domain.Transfer, error)
	Update(ctx context.Context, id int64, params domain.UpdateTransferParams) (*domain.Transfer, error)
	Delete(ctx context.Context, id int64) (*domain.Transfer, error)
}

type TransferRepository interface {
	TransferReader
	TransferWriter
}

type TransferService struct {
	repo TransferRepository
}

func NewTransferService(repo TransferRepository) *TransferService {
	return &TransferService{repo: repo}
}

func (s *TransferService) List(ctx context.Context, filter domain.ListTransfersFilter) ([]domain.Transfer, error) {
	return s.repo.List(ctx, filter)
}

func (s *TransferService) Get(ctx context.Context, id int64) (*domain.Transfer, error) {
	return s.repo.Get(ctx, id)
}

func (s *TransferService) Create(ctx context.Context, p domain.CreateTransferParams) (*domain.Transfer, error) {
	switch {
	case p.CreditAccountID == 0:
		return nil, fmt.Errorf("%w: credit_account_id is required", domain.ErrValidation)
	case p.DebitAccountID == 0:
		return nil, fmt.Errorf("%w: debit_account_id is required", domain.ErrValidation)
	case p.AmountCents == 0:
		return nil, fmt.Errorf("%w: amount_cents must be non-zero", domain.ErrValidation)
	case p.Date == "":
		return nil, fmt.Errorf("%w: date is required", domain.ErrValidation)
	}
	return s.repo.Create(ctx, p)
}

func (s *TransferService) Update(ctx context.Context, id int64, p domain.UpdateTransferParams) (*domain.Transfer, error) {
	return s.repo.Update(ctx, id, p)
}

func (s *TransferService) Delete(ctx context.Context, id int64) (*domain.Transfer, error) {
	return s.repo.Delete(ctx, id)
}
