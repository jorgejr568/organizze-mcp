package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type BudgetService interface {
	List(ctx context.Context, period domain.BudgetPeriod) ([]domain.Budget, error)
}

type ListBudgetsInput struct {
	Year  int `json:"year,omitempty"  jsonschema:"Optional year, e.g. 2026. Omit for current month."`
	Month int `json:"month,omitempty" jsonschema:"Optional month 1..12. Requires year."`
}

type ListBudgetsOutput struct {
	Budgets []domain.Budget `json:"budgets"`
}

func listBudgetsHandler(svc BudgetService) mcpsdk.ToolHandlerFor[ListBudgetsInput, ListBudgetsOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in ListBudgetsInput) (*mcpsdk.CallToolResult, ListBudgetsOutput, error) {
		budgets, err := svc.List(ctx, domain.BudgetPeriod{Year: in.Year, Month: in.Month})
		if err != nil {
			return nil, ListBudgetsOutput{}, err
		}
		return nil, ListBudgetsOutput{Budgets: budgets}, nil
	}
}

func registerBudgetTools(s *mcpsdk.Server, inst instrumentation, svc BudgetService) {
	addInstrumentedTool(s, inst, &mcpsdk.Tool{
		Name:        "list_budgets",
		Description: "List Organizze budgets. With no args, returns the current month. Provide year for an annual view, or year+month for a specific month.",
	}, listBudgetsHandler(svc))
}
