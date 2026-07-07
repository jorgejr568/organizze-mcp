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
	deletedID   int64
	createErr   error
	batchParams []domain.CreateTransactionParams
	batchErr    error
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
func (f *fakeTransactionSvc) CreateBatch(_ context.Context, params []domain.CreateTransactionParams) ([]domain.BatchCreateResult, error) {
	if f.batchErr != nil {
		return nil, f.batchErr
	}
	f.batchParams = params
	results := make([]domain.BatchCreateResult, len(params))
	for i, p := range params {
		if p.Description == "fail" {
			results[i] = domain.BatchCreateResult{Index: i, Err: domain.ErrValidation}
			continue
		}
		results[i] = domain.BatchCreateResult{
			Index:       i,
			Transaction: &domain.Transaction{ID: int64(700 + i), Description: p.Description, AmountCents: p.AmountCents},
		}
	}
	return results, nil
}
func (f *fakeTransactionSvc) Update(_ context.Context, id int64, p domain.UpdateTransactionParams) (*domain.Transaction, error) {
	f.updated.id, f.updated.params = id, p
	return &domain.Transaction{ID: id}, nil
}
func (f *fakeTransactionSvc) Delete(_ context.Context, id int64, _ domain.DeleteTransactionParams) (*domain.Transaction, error) {
	f.deletedID = id
	return &domain.Transaction{ID: id}, nil
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
func (nopTransactionSvc) CreateBatch(context.Context, []domain.CreateTransactionParams) ([]domain.BatchCreateResult, error) {
	return nil, nil
}
func (nopTransactionSvc) Update(context.Context, int64, domain.UpdateTransactionParams) (*domain.Transaction, error) {
	return &domain.Transaction{}, nil
}
func (nopTransactionSvc) Delete(context.Context, int64, domain.DeleteTransactionParams) (*domain.Transaction, error) {
	return nil, nil
}

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

func TestCreateTransactionHandler_PlumbsRecurrence(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := createTransactionHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateTransactionInput{
		Description: "Despesa fixa", Date: "2026-05-14", AmountCents: -10000,
		AccountID: 3, CategoryID: 21,
		Recurrence: &RecurrenceInput{Periodicity: "monthly"},
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if svc.created.Recurrence == nil {
		t.Fatalf("recurrence not forwarded: %+v", svc.created)
	}
	if svc.created.Recurrence.Periodicity != domain.PeriodicityMonthly {
		t.Errorf("periodicity = %q, want monthly", svc.created.Recurrence.Periodicity)
	}
}

func TestCreateTransactionHandler_PlumbsInstallments(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := createTransactionHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateTransactionInput{
		Description: "Computador", Date: "2026-05-14", AmountCents: -100000,
		AccountID: 1, CategoryID: 10,
		Installments: &InstallmentsInput{Periodicity: "monthly", Total: 12},
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if svc.created.Installments == nil {
		t.Fatalf("installments not forwarded: %+v", svc.created)
	}
	if svc.created.Installments.Periodicity != domain.PeriodicityMonthly {
		t.Errorf("periodicity = %q, want monthly", svc.created.Installments.Periodicity)
	}
	if svc.created.Installments.Total != 12 {
		t.Errorf("total = %d, want 12", svc.created.Installments.Total)
	}
}

func TestCreateTransactionHandler_PlumbsCreditCardFields(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := createTransactionHandler(svc)
	cardID := int64(1287765)
	invoiceID := int64(276)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateTransactionInput{
		Description: "Coffee", Date: "2026-05-14", AmountCents: -1500,
		AccountID: 1, CategoryID: 10,
		CreditCardID:        &cardID,
		CreditCardInvoiceID: &invoiceID,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if svc.created.CreditCardID == nil || *svc.created.CreditCardID != 1287765 {
		t.Errorf("CreditCardID = %v, want 1287765", svc.created.CreditCardID)
	}
	if svc.created.CreditCardInvoiceID == nil || *svc.created.CreditCardInvoiceID != 276 {
		t.Errorf("CreditCardInvoiceID = %v, want 276", svc.created.CreditCardInvoiceID)
	}
}

func TestUpdateTransactionHandler_PlumbsCreditCardID(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := updateTransactionHandler(svc)
	cardID := int64(1287765)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, UpdateTransactionInput{
		ID: 777, CreditCardID: &cardID,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if svc.updated.params.CreditCardID == nil || *svc.updated.params.CreditCardID != 1287765 {
		t.Errorf("params.CreditCardID = %v", svc.updated.params.CreditCardID)
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

func TestUpdateTransactionHandler_PlumbsCreditCardInvoiceID(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := updateTransactionHandler(svc)
	cardID := int64(386176)
	invoiceID := int64(317)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, UpdateTransactionInput{
		ID:                  777,
		CreditCardID:        &cardID,
		CreditCardInvoiceID: &invoiceID,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if svc.updated.params.CreditCardInvoiceID == nil || *svc.updated.params.CreditCardInvoiceID != 317 {
		t.Errorf("params.CreditCardInvoiceID = %v, want 317", svc.updated.params.CreditCardInvoiceID)
	}
}

func TestCreateTransactionsHandler_MapsResultsAndCounts(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := createTransactionsHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateTransactionsInput{
		Transactions: []CreateTransactionInput{
			{Description: "a", Date: "2026-05-14", AmountCents: -1500, AccountID: 1, CategoryID: 10},
			{Description: "fail", Date: "2026-05-14", AmountCents: -1500, AccountID: 1, CategoryID: 10},
			{Description: "c", Date: "2026-05-14", AmountCents: -2500, AccountID: 1, CategoryID: 10},
		},
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.Created != 2 || out.Failed != 1 {
		t.Errorf("created=%d failed=%d, want 2/1", out.Created, out.Failed)
	}
	if len(out.Results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(out.Results))
	}
	if !out.Results[0].Success || out.Results[0].Transaction == nil {
		t.Errorf("results[0] = %+v, want success", out.Results[0])
	}
	if out.Results[1].Success || out.Results[1].Error == "" {
		t.Errorf("results[1] = %+v, want failure with error text", out.Results[1])
	}
	if out.Results[2].Index != 2 {
		t.Errorf("results[2].Index = %d, want 2", out.Results[2].Index)
	}
	// toCreateParams was applied to each item before dispatch.
	if len(svc.batchParams) != 3 || svc.batchParams[2].AmountCents != -2500 {
		t.Errorf("batchParams = %+v", svc.batchParams)
	}
}

func TestCreateTransactionsHandler_PlumbsInstallments(t *testing.T) {
	svc := &fakeTransactionSvc{}
	h := createTransactionsHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateTransactionsInput{
		Transactions: []CreateTransactionInput{
			{
				Description: "Computador", Date: "2026-05-14", AmountCents: -100000,
				AccountID: 1, CategoryID: 10,
				Installments: &InstallmentsInput{Periodicity: "monthly", Total: 12},
			},
		},
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(svc.batchParams) != 1 || svc.batchParams[0].Installments == nil {
		t.Fatalf("installments not forwarded: %+v", svc.batchParams)
	}
	if svc.batchParams[0].Installments.Total != 12 {
		t.Errorf("total = %d, want 12", svc.batchParams[0].Installments.Total)
	}
}

func TestCreateTransactionsHandler_PropagatesTopLevelError(t *testing.T) {
	svc := &fakeTransactionSvc{batchErr: domain.ErrValidation}
	h := createTransactionsHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateTransactionsInput{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
}
