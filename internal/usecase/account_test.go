package usecase

import (
	"context"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeAccountRepo struct {
	list []domain.Account
	one  *domain.Account
}

func (f *fakeAccountRepo) List(context.Context) ([]domain.Account, error) {
	return f.list, nil
}
func (f *fakeAccountRepo) Get(_ context.Context, _ int64) (*domain.Account, error) {
	return f.one, nil
}

func TestAccountService_DelegatesBothCalls(t *testing.T) {
	repo := &fakeAccountRepo{
		list: []domain.Account{{ID: 1, Name: "Checking"}},
		one:  &domain.Account{ID: 1, Name: "Checking"},
	}
	svc := NewAccountService(repo)

	xs, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(xs) != 1 {
		t.Errorf("got %d accounts", len(xs))
	}

	a, err := svc.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if a.ID != 1 {
		t.Errorf("got %+v", a)
	}
}
