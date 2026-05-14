package mcp

import (
	"context"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeBudgetSvc struct {
	gotPeriod domain.BudgetPeriod
}

func (f *fakeBudgetSvc) List(_ context.Context, p domain.BudgetPeriod) ([]domain.Budget, error) {
	f.gotPeriod = p
	return nil, nil
}

type nopBudgetSvc struct{}

func (nopBudgetSvc) List(context.Context, domain.BudgetPeriod) ([]domain.Budget, error) {
	return nil, nil
}

func TestListBudgetsHandler_ForwardsPeriod(t *testing.T) {
	svc := &fakeBudgetSvc{}
	h := listBudgetsHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, ListBudgetsInput{Year: 2026, Month: 5})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if svc.gotPeriod.Year != 2026 || svc.gotPeriod.Month != 5 {
		t.Errorf("forwarded = %+v", svc.gotPeriod)
	}
}
