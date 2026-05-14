package usecase

import (
	"context"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeCardRepo struct {
	list []domain.CreditCard
	one  *domain.CreditCard
}

func (f *fakeCardRepo) List(context.Context) ([]domain.CreditCard, error) {
	return f.list, nil
}
func (f *fakeCardRepo) Get(context.Context, int64) (*domain.CreditCard, error) {
	return f.one, nil
}

func TestCreditCardService(t *testing.T) {
	repo := &fakeCardRepo{
		list: []domain.CreditCard{{ID: 1, Name: "Nubank"}},
		one:  &domain.CreditCard{ID: 1, Name: "Nubank"},
	}
	svc := NewCreditCardService(repo)
	if xs, _ := svc.List(context.Background()); len(xs) != 1 {
		t.Errorf("List: %v", xs)
	}
	if c, _ := svc.Get(context.Background(), 1); c.ID != 1 {
		t.Errorf("Get: %+v", c)
	}
}
