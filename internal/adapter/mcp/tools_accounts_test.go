package mcp

import (
	"context"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeAccountSvc struct {
	list []domain.Account
	one  *domain.Account
}

func (f *fakeAccountSvc) List(context.Context) ([]domain.Account, error) { return f.list, nil }
func (f *fakeAccountSvc) Get(context.Context, int64) (*domain.Account, error) { return f.one, nil }

type nopAccountSvc struct{}

func (nopAccountSvc) List(context.Context) ([]domain.Account, error)            { return nil, nil }
func (nopAccountSvc) Get(context.Context, int64) (*domain.Account, error)       { return &domain.Account{}, nil }

func TestListAccountsHandler(t *testing.T) {
	svc := &fakeAccountSvc{list: []domain.Account{{ID: 1, Name: "Checking"}}}
	h := listAccountsHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, struct{}{})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(out.Accounts) != 1 || out.Accounts[0].Name != "Checking" {
		t.Errorf("got %+v", out)
	}
}

func TestGetAccountHandler(t *testing.T) {
	svc := &fakeAccountSvc{one: &domain.Account{ID: 42, Name: "Itau"}}
	h := getAccountHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, GetAccountInput{ID: 42})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Account.ID != 42 {
		t.Errorf("got %+v", out)
	}
}
