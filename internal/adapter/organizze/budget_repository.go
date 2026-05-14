package organizze

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// BudgetRepository lists budgets for the current month, an entire year, or a
// specific year+month.
type BudgetRepository struct {
	exec *RequestExecutor
}

func NewBudgetRepository(exec *RequestExecutor) *BudgetRepository {
	return &BudgetRepository{exec: exec}
}

// List returns budgets for the current month.
func (r *BudgetRepository) List(ctx context.Context) ([]domain.Budget, error) {
	var out []domain.Budget
	if err := r.exec.Get(ctx, "/budgets", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListForYear returns budgets for every month of the given year.
func (r *BudgetRepository) ListForYear(ctx context.Context, year int) ([]domain.Budget, error) {
	var out []domain.Budget
	if err := r.exec.Get(ctx, fmt.Sprintf("/budgets/%d", year), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListForMonth returns budgets for a specific year+month (month 1..12).
func (r *BudgetRepository) ListForMonth(ctx context.Context, year, month int) ([]domain.Budget, error) {
	var out []domain.Budget
	if err := r.exec.Get(ctx, fmt.Sprintf("/budgets/%d/%d", year, month), &out); err != nil {
		return nil, err
	}
	return out, nil
}
