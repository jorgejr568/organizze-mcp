package organizze

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestCreditCardRepository_List(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/credit_cards" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[{"id":1,"name":"Nubank","closing_day":20,"due_day":27,"limit_cents":500000}]`)
	})
	repo := NewCreditCardRepository(exec)
	cards, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(cards) != 1 || cards[0].Name != "Nubank" {
		t.Errorf("got %+v", cards)
	}
}

func TestCreditCardRepository_Get(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/credit_cards/9" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":9,"name":"Inter"}`)
	})
	repo := NewCreditCardRepository(exec)
	card, err := repo.Get(context.Background(), 9)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if card.ID != 9 {
		t.Errorf("got %+v", card)
	}
}
