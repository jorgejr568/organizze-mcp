package organizze

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestBudgetRepository_List_CurrentMonth(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/budgets" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[{"amount_in_cents":50000,"category_id":10,"date":"2026-05-01","total":12000,"predicted_total":30000,"percentage":"24"}]`)
	})
	repo := NewBudgetRepository(exec)
	bs, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(bs) != 1 || bs[0].AmountInCents != 50000 {
		t.Errorf("got %+v", bs)
	}
}

func TestBudgetRepository_ListForYear(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/budgets/2026" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[]`)
	})
	repo := NewBudgetRepository(exec)
	if _, err := repo.ListForYear(context.Background(), 2026); err != nil {
		t.Fatalf("ListForYear: %v", err)
	}
}

func TestBudgetRepository_ListForMonth(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/budgets/2026/5" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[]`)
	})
	repo := NewBudgetRepository(exec)
	if _, err := repo.ListForMonth(context.Background(), 2026, 5); err != nil {
		t.Fatalf("ListForMonth: %v", err)
	}
}
