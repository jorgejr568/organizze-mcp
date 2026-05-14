package usecase

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// BudgetRepository exposes the three list flavors of the Organizze budgets API.
type BudgetRepository interface {
	List(ctx context.Context) ([]domain.Budget, error)
	ListForYear(ctx context.Context, year int) ([]domain.Budget, error)
	ListForMonth(ctx context.Context, year, month int) ([]domain.Budget, error)
}

// BudgetService routes the period requested by the caller to the right
// repository method. Month without Year is a validation error.
type BudgetService struct {
	repo BudgetRepository
}

func NewBudgetService(repo BudgetRepository) *BudgetService {
	return &BudgetService{repo: repo}
}

// List returns budgets for the period selected by p.
func (s *BudgetService) List(ctx context.Context, p domain.BudgetPeriod) ([]domain.Budget, error) {
	if p.Month != 0 && p.Year == 0 {
		return nil, fmt.Errorf("%w: month requires year", domain.ErrValidation)
	}
	switch {
	case p.Year == 0:
		return s.repo.List(ctx)
	case p.Month == 0:
		return s.repo.ListForYear(ctx, p.Year)
	default:
		if p.Month < 1 || p.Month > 12 {
			return nil, fmt.Errorf("%w: month must be 1..12", domain.ErrValidation)
		}
		return s.repo.ListForMonth(ctx, p.Year, p.Month)
	}
}
