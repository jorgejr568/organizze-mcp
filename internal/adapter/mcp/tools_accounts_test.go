package mcp

import (
	"context"
	"errors"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeAccountSvc struct {
	list      []domain.Account
	one       *domain.Account
	listed    bool
	gotID     int64
	created   domain.CreateAccountParams
	updated   struct {
		id     int64
		params domain.UpdateAccountParams
	}
	deletedID int64
	createErr error
}

func (f *fakeAccountSvc) List(context.Context) ([]domain.Account, error) {
	f.listed = true
	return f.list, nil
}
func (f *fakeAccountSvc) Get(_ context.Context, id int64) (*domain.Account, error) {
	f.gotID = id
	if f.one != nil {
		return f.one, nil
	}
	return &domain.Account{ID: id}, nil
}
func (f *fakeAccountSvc) Create(_ context.Context, p domain.CreateAccountParams) (*domain.Account, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.created = p
	return &domain.Account{ID: 18, Name: p.Name, Type: p.Type}, nil
}
func (f *fakeAccountSvc) Update(_ context.Context, id int64, p domain.UpdateAccountParams) (*domain.Account, error) {
	f.updated.id, f.updated.params = id, p
	return &domain.Account{ID: id}, nil
}
func (f *fakeAccountSvc) Delete(_ context.Context, id int64) error {
	f.deletedID = id
	return nil
}

type nopAccountSvc struct{}

func (nopAccountSvc) List(context.Context) ([]domain.Account, error)      { return nil, nil }
func (nopAccountSvc) Get(context.Context, int64) (*domain.Account, error) { return &domain.Account{}, nil }
func (nopAccountSvc) Create(context.Context, domain.CreateAccountParams) (*domain.Account, error) {
	return &domain.Account{}, nil
}
func (nopAccountSvc) Update(context.Context, int64, domain.UpdateAccountParams) (*domain.Account, error) {
	return &domain.Account{}, nil
}
func (nopAccountSvc) Delete(context.Context, int64) error { return nil }

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

func TestCreateAccountHandler(t *testing.T) {
	svc := &fakeAccountSvc{}
	h := createAccountHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateAccountInput{
		Name: "Itaú CC", Type: "checking",
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Account.ID != 18 || svc.created.Type != "checking" {
		t.Errorf("out=%+v svc.created=%+v", out, svc.created)
	}
}

func TestCreateAccountHandler_PropagatesValidationError(t *testing.T) {
	svc := &fakeAccountSvc{createErr: domain.ErrValidation}
	h := createAccountHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateAccountInput{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
}

func TestUpdateAccountHandler(t *testing.T) {
	svc := &fakeAccountSvc{}
	h := updateAccountHandler(svc)
	name := "Renamed"
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, UpdateAccountInput{
		ID: 18, Name: &name,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Account.ID != 18 {
		t.Errorf("out = %+v", out)
	}
	if svc.updated.id != 18 || svc.updated.params.Name == nil || *svc.updated.params.Name != "Renamed" {
		t.Errorf("svc.updated = %+v", svc.updated)
	}
}

func TestDeleteAccountHandler(t *testing.T) {
	svc := &fakeAccountSvc{}
	h := deleteAccountHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, DeleteAccountInput{ID: 18})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !out.Deleted || out.ID != 18 || svc.deletedID != 18 {
		t.Errorf("out=%+v svc.deletedID=%d", out, svc.deletedID)
	}
}
