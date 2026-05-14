package mcp

import (
	"context"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeCreditCardSvc struct {
	list []domain.CreditCard
	one  *domain.CreditCard
}

func (f *fakeCreditCardSvc) List(context.Context) ([]domain.CreditCard, error) { return f.list, nil }
func (f *fakeCreditCardSvc) Get(context.Context, int64) (*domain.CreditCard, error) {
	return f.one, nil
}

type nopCreditCardSvc struct{}

func (nopCreditCardSvc) List(context.Context) ([]domain.CreditCard, error) { return nil, nil }
func (nopCreditCardSvc) Get(context.Context, int64) (*domain.CreditCard, error) {
	return &domain.CreditCard{}, nil
}

func TestCreditCardHandlers(t *testing.T) {
	svc := &fakeCreditCardSvc{
		list: []domain.CreditCard{{ID: 1, Name: "Nubank"}},
		one:  &domain.CreditCard{ID: 1, Name: "Nubank"},
	}
	hList := listCreditCardsHandler(svc)
	if _, out, err := hList(context.Background(), &mcpsdk.CallToolRequest{}, struct{}{}); err != nil || len(out.CreditCards) != 1 {
		t.Fatalf("list: out=%+v err=%v", out, err)
	}
	hGet := getCreditCardHandler(svc)
	if _, out, err := hGet(context.Background(), &mcpsdk.CallToolRequest{}, GetCreditCardInput{ID: 1}); err != nil || out.CreditCard.ID != 1 {
		t.Fatalf("get: out=%+v err=%v", out, err)
	}
}
