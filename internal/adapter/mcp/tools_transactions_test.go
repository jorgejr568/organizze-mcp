package mcp

import (
	"context"
	"errors"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeTransactionSvc struct {
	listFilter domain.ListTransactionsFilter
	created    domain.CreateTransactionParams
	updated    struct {
		id     int64
		params domain.UpdateTransactionParams
	}
	deletedID int64
	createErr error
}

func (f *fakeTransactionSvc) List(_ context.Context, fl domain.ListTransactionsFilter) ([]domain.Transaction, error) {
	f.listFilter = fl
	return []domain.Transaction{{ID: 1}}, nil
}
func (f *fakeTransactionSvc) Get(_ context.Context, id int64) (*domain.Transaction, error) {
	return &domain.Transaction{ID: id}, nil
}
func (f *fakeTransactionSvc) Create(_ context.Context, p domain.CreateTransactionParams) (*domain.Transaction, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.created = p
	return &domain.Transaction{ID: 777, Description: p.Description, AmountCents: p.AmountCents}, nil
}
func (f *fakeTransactionSvc) Update(_ context.Context, id int64, p domain.UpdateTransactionParams) (*domain.Transaction, error) {
	f.updated.id, f.updated.params = id, p
	return &domain.Transaction{ID: id}, nil
}
func (f *fakeTransactionSvc) Delete(_ context.Context, id int64) error {
	f.deletedID = id
	return nil
}

type nopTransactionSvc struct{}

func (nopTransactionSvc) List(context.Context, domain.ListTransactionsFilter) ([]domain.Transaction, error) {
	return nil, nil
}
func (nopTransactionSvc) Get(context.Context, int64) (*domain.Transaction, error) {
	return &domain.Transaction{}, nil
}
func (nopTransactionSvc) Create(context.Context, domain.CreateTransactionParams) (*domain.Transaction, error) {
	return &domain.Transaction{}, nil
}
func (nopTransactionSvc) Update(context.Context, int64, domain.UpdateTransactionParams) (*domain.Transaction, error) {
	return &domain.Transaction{}, nil
}
func (nopTransactionSvc) Delete(context.Context, int64) error { return nil }

func TestListTransactionsHandler_PassesAllFilters(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := listTransactionsHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, ListTransactionsInput{
		StartDate: "2026-05-01", EndDate: "2026-05-31", AccountID: 7,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(out.Transactions) != 1 {
		t.Errorf("len = %d", len(out.Transactions))
	}
	if svc.listFilter.AccountID != 7 || svc.listFilter.StartDate != "2026-05-01" {
		t.Errorf("filter = %+v", svc.listFilter)
	}
}

func TestGetTransactionHandler(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := getTransactionHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, GetTransactionInput{ID: 55})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Transaction.ID != 55 {
		t.Errorf("got %+v", out)
	}
}

func TestCreateTransactionHandler(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := createTransactionHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateTransactionInput{
		Description: "Coffee", Date: "2026-05-14", AmountCents: -1500,
		AccountID: 1, CategoryID: 10, Paid: true,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Transaction.ID != 777 || svc.created.AmountCents != -1500 {
		t.Errorf("out=%+v svc=%+v", out, svc.created)
	}
}

func TestCreateTransactionHandler_PropagatesValidationError(t *testing.T) {
	svc := &fakeTransactionSvc{createErr: domain.ErrValidation}
	h := createTransactionHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateTransactionInput{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
}

func TestUpdateTransactionHandler(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := updateTransactionHandler(svc)
	desc := "Tea"
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, UpdateTransactionInput{
		ID: 55, Description: &desc,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Transaction.ID != 55 {
		t.Errorf("out = %+v", out)
	}
	if svc.updated.id != 55 || svc.updated.params.Description == nil || *svc.updated.params.Description != "Tea" {
		t.Errorf("svc.updated = %+v", svc.updated)
	}
}

func TestDeleteTransactionHandler(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := deleteTransactionHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, DeleteTransactionInput{ID: 55})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !out.Deleted || out.ID != 55 || svc.deletedID != 55 {
		t.Errorf("out=%+v svc.deletedID=%d", out, svc.deletedID)
	}
}
