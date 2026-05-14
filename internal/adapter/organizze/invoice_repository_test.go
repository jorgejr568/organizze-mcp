package organizze

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestInvoiceRepository_List(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/credit_cards/9/invoices" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[{"id":100,"credit_card_id":9,"amount_cents":120000}]`)
	})
	repo := NewInvoiceRepository(exec)
	invs, err := repo.List(context.Background(), 9)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(invs) != 1 || invs[0].AmountCents != 120000 {
		t.Errorf("got %+v", invs)
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
