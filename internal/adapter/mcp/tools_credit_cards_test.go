package mcp

import (
	"context"
	"errors"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeCreditCardSvc struct {
	listed  bool
	gotID   int64
	created domain.CreateCreditCardParams
	updated struct {
		id     int64
		params domain.UpdateCreditCardParams
	}
	deletedID int64
	createErr error
}

func (f *fakeCreditCardSvc) List(_ context.Context) ([]domain.CreditCard, error) {
	f.listed = true
	return []domain.CreditCard{{ID: 1}}, nil
}
func (f *fakeCreditCardSvc) Get(_ context.Context, id int64) (*domain.CreditCard, error) {
	f.gotID = id
	return &domain.CreditCard{ID: id}, nil
}
func (f *fakeCreditCardSvc) Create(_ context.Context, p domain.CreateCreditCardParams) (*domain.CreditCard, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.created = p
	return &domain.CreditCard{ID: 7, Name: p.Name}, nil
}
func (f *fakeCreditCardSvc) Update(_ context.Context, id int64, p domain.UpdateCreditCardParams) (*domain.CreditCard, error) {
	f.updated.id, f.updated.params = id, p
	return &domain.CreditCard{ID: id}, nil
}
func (f *fakeCreditCardSvc) Delete(_ context.Context, id int64) (*domain.CreditCard, error) {
	f.deletedID = id
	return &domain.CreditCard{ID: id, Name: "Visa Exclusive", Archived: true}, nil
}

type nopCreditCardSvc struct{}

func (nopCreditCardSvc) List(context.Context) ([]domain.CreditCard, error) { return nil, nil }
func (nopCreditCardSvc) Get(context.Context, int64) (*domain.CreditCard, error) {
	return &domain.CreditCard{}, nil
}
func (nopCreditCardSvc) Create(context.Context, domain.CreateCreditCardParams) (*domain.CreditCard, error) {
	return &domain.CreditCard{}, nil
}
func (nopCreditCardSvc) Update(context.Context, int64, domain.UpdateCreditCardParams) (*domain.CreditCard, error) {
	return &domain.CreditCard{}, nil
}
func (nopCreditCardSvc) Delete(context.Context, int64) (*domain.CreditCard, error) {
	return &domain.CreditCard{}, nil
}

func TestCreditCardHandlers(t *testing.T) {
	svc := &fakeCreditCardSvc{}
	hList := listCreditCardsHandler(svc)
	if _, out, err := hList(context.Background(), &mcpsdk.CallToolRequest{}, struct{}{}); err != nil || len(out.CreditCards) != 1 {
		t.Fatalf("list: out=%+v err=%v", out, err)
	}
	hGet := getCreditCardHandler(svc)
	if _, out, err := hGet(context.Background(), &mcpsdk.CallToolRequest{}, GetCreditCardInput{ID: 1}); err != nil || out.CreditCard.ID != 1 {
		t.Fatalf("get: out=%+v err=%v", out, err)
	}
}

func TestCreateCreditCardHandler(t *testing.T) {
	svc := &fakeCreditCardSvc{}
	h := createCreditCardHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateCreditCardInput{
		Name: "Nubank", DueDay: 27, ClosingDay: 20,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.CreditCard.ID != 7 || svc.created.DueDay != 27 {
		t.Errorf("out=%+v svc.created=%+v", out, svc.created)
	}
}

func TestCreateCreditCardHandler_PropagatesValidationError(t *testing.T) {
	svc := &fakeCreditCardSvc{createErr: domain.ErrValidation}
	h := createCreditCardHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, CreateCreditCardInput{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
}

func TestUpdateCreditCardHandler(t *testing.T) {
	svc := &fakeCreditCardSvc{}
	h := updateCreditCardHandler(svc)
	name := "Renamed"
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, UpdateCreditCardInput{
		ID: 7, Name: &name,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.CreditCard.ID != 7 || svc.updated.id != 7 || *svc.updated.params.Name != "Renamed" {
		t.Errorf("out=%+v svc.updated=%+v", out, svc.updated)
	}
}

func TestDeleteCreditCardHandler(t *testing.T) {
	svc := &fakeCreditCardSvc{}
	h := deleteCreditCardHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, DeleteCreditCardInput{ID: 7})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !out.Deleted || out.ID != 7 || svc.deletedID != 7 {
		t.Errorf("out=%+v svc.deletedID=%d", out, svc.deletedID)
	}
	if out.CreditCard == nil || out.CreditCard.ID != 7 || !out.CreditCard.Archived {
		t.Errorf("out.CreditCard = %+v, want deleted snapshot", out.CreditCard)
	}
}
