package organizze

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

func TestInvoiceRepository_List(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/credit_cards/9/invoices" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[{"id":100,"credit_card_id":9,"amount_cents":120000}]`)
	})
	repo := NewInvoiceRepository(exec)
	invs, err := repo.List(context.Background(), 9, domain.ListInvoicesFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(invs) != 1 || invs[0].AmountCents != 120000 {
		t.Errorf("got %+v", invs)
	}
}

func TestInvoiceRepository_List_AppendsDateFilters(t *testing.T) {
	var gotURL string
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	})
	repo := NewInvoiceRepository(exec)
	if _, err := repo.List(context.Background(), 7, domain.ListInvoicesFilter{
		StartDate: "2024-01-01", EndDate: "2024-12-31",
	}); err != nil {
		t.Fatalf("List: %v", err)
	}
	want := "/credit_cards/7/invoices?end_date=2024-12-31&start_date=2024-01-01"
	if gotURL != want {
		t.Errorf("URL = %q, want %q", gotURL, want)
	}
}

func TestInvoiceRepository_List_NoFilter_OmitsQuery(t *testing.T) {
	var gotURL string
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	})
	repo := NewInvoiceRepository(exec)
	if _, err := repo.List(context.Background(), 7, domain.ListInvoicesFilter{}); err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotURL != "/credit_cards/7/invoices" {
		t.Errorf("URL = %q, want /credit_cards/7/invoices", gotURL)
	}
}

func TestInvoiceRepository_Get(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/credit_cards/9/invoices/100" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":100,"credit_card_id":9}`)
	})
	repo := NewInvoiceRepository(exec)
	inv, err := repo.Get(context.Background(), 9, 100)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if inv.ID != 100 {
		t.Errorf("got %+v", inv)
	}
}
