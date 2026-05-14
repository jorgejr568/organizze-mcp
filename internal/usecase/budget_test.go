package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeBudgetRepo struct {
	called string // "list" | "year" | "month"
	year   int
	month  int
}

func (f *fakeBudgetRepo) List(context.Context) ([]domain.Budget, error) {
	f.called = "list"
	return nil, nil
}
func (f *fakeBudgetRepo) ListForYear(_ context.Context, y int) ([]domain.Budget, error) {
	f.called = "year"
	f.year = y
	return nil, nil
}
func (f *fakeBudgetRepo) ListForMonth(_ context.Context, y, m int) ([]domain.Budget, error) {
	f.called = "month"
	f.year = y
	f.month = m
	return nil, nil
}

func TestBudgetService_RoutesByPeriod(t *testing.T) {
	cases := []struct {
		name   string
		period domain.BudgetPeriod
		want   string
	}{
		{"current", domain.BudgetPeriod{}, "list"},
		{"year", domain.BudgetPeriod{Year: 2026}, "year"},
		{"month", domain.BudgetPeriod{Year: 2026, Month: 5}, "month"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &fakeBudgetRepo{}
			svc := NewBudgetService(repo)
			if _, err := svc.List(context.Background(), c.period); err != nil {
				t.Fatalf("List: %v", err)
			}
			if repo.called != c.want {
				t.Errorf("called = %q, want %q", repo.called, c.want)
			}
		})
	}
}

func TestBudgetService_RejectsMonthWithoutYear(t *testing.T) {
	repo := &fakeBudgetRepo{}
	svc := NewBudgetService(repo)
	_, err := svc.List(context.Background(), domain.BudgetPeriod{Month: 5})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err should wrap ErrValidation; got %v", err)
	}
	if repo.called != "" {
		t.Errorf("repo should not have been called; got %q", repo.called)
	}
}
