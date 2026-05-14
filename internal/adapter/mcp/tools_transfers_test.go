package mcp

import (
	"context"
	"errors"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeTransferSvc struct {
	listFilter domain.ListTransfersFilter
	created    domain.CreateTransferParams
	updated    struct {
		id     int64
		params domain.UpdateTransferParams
	}
	deletedID int64
	createErr error
}

func (f *fakeTransferSvc) List(_ context.Context, fl domain.ListTransfersFilter) ([]domain.Transfer, error) {
	f.listFilter = fl
	return []domain.Transfer{{ID: 1}}, nil
}
func (f *fakeTransferSvc) Create(_ context.Context, p domain.CreateTransferParams) (*domain.Transfer, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.created = p
	return &domain.Transfer{ID: 123, AmountCents: p.AmountCents}, nil
}
func (f *fakeTransferSvc) Update(_ context.Context, id int64, p domain.UpdateTransferParams) (*domain.Transfer, error) {
	f.updated.id, f.updated.params = id, p
	return &domain.Transfer{ID: id}, nil
}
func (f *fakeTransferSvc) Delete(_ context.Context, id int64) error {
	f.deletedID = id
	return nil
}

type nopTransferSvc struct{}

func (nopTransferSvc) List(context.Context, domain.ListTransfersFilter) ([]domain.Transfer, error) {
	return nil, nil
}
func (nopTransferSvc) Create(context.Context, domain.CreateTransferParams) (*domain.Transfer, error) {
	return nil, nil
}
func (nopTransferSvc) Update(context.Context, int64, domain.UpdateTransferParams) (*domain.Transfer, error) {
	return nil, nil
}
func (nopTransferSvc) Delete(context.Context, int64) error { return nil }

func TestListTransfersHandler_PassesFilter(t *testing.T) {
	svc := &fakeTransferSvc{}
	h := listTransfersHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, ListTransfersInput{
		StartDate: "2026-05-01", EndDate: "2026-05-31",
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if svc.listFilter.StartDate != "2026-05-01" || svc.listFilter.EndDate != "2026-05-31" {
		t.Errorf("filter = %+v", svc.listFilter)
	}
}

func TestCreateTransferHandler(t *testing.T) {
	svc := &fakeTransferSvc{}
	h := createTransferHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateTransferInput{
		CreditAccountID: 2, DebitAccountID: 1, AmountCents: 50000, Date: "2026-05-14", Paid: true,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Transfer.ID != 123 || svc.created.AmountCents != 50000 {
		t.Errorf("out=%+v svc.created=%+v", out, svc.created)
	}
}

func TestCreateTransferHandler_PropagatesValidationError(t *testing.T) {
	svc := &fakeTransferSvc{createErr: domain.ErrValidation}
	h := createTransferHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateTransferInput{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
}

func TestUpdateTransferHandler(t *testing.T) {
	svc := &fakeTransferSvc{}
	h := updateTransferHandler(svc)
	desc := "Reimbursement"
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, UpdateTransferInput{
		ID: 123, Description: &desc,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Transfer.ID != 123 || svc.updated.id != 123 || *svc.updated.params.Description != "Reimbursement" {
		t.Errorf("out=%+v svc.updated=%+v", out, svc.updated)
	}
}

func TestDeleteTransferHandler(t *testing.T) {
	svc := &fakeTransferSvc{}
	h := deleteTransferHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, DeleteTransferInput{ID: 123})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !out.Deleted || out.ID != 123 || svc.deletedID != 123 {
		t.Errorf("out=%+v svc.deletedID=%d", out, svc.deletedID)
	}
}
