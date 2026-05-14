package usecase

import (
	"context"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type TransferRepository interface {
	List(ctx context.Context, filter domain.ListTransfersFilter) ([]domain.Transfer, error)
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
