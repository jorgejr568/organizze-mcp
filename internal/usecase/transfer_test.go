package usecase

import (
	"context"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeTransferRepo struct {
	gotFilter domain.ListTransfersFilter
}

func (f *fakeTransferRepo) List(_ context.Context, fl domain.ListTransfersFilter) ([]domain.Transfer, error) {
	f.gotFilter = fl
	return []domain.Transfer{}, nil
}

func TestTransferService_PassesFilter(t *testing.T) {
	repo := &fakeTransferRepo{}
	svc := NewTransferService(repo)
	_, err := svc.List(context.Background(), domain.ListTransfersFilter{StartDate: "2026-05-01"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if repo.gotFilter.StartDate != "2026-05-01" {
		t.Errorf("filter not forwarded: %+v", repo.gotFilter)
	}
}
