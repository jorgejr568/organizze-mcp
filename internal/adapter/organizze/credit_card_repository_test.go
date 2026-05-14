package organizze

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
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

func TestCreditCardRepository_Create(t *testing.T) {
	var gotBody domain.CreateCreditCardParams
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/credit_cards" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":7,"name":"Nubank","closing_day":20,"due_day":27,"limit_cents":500000}`)
	})
	repo := NewCreditCardRepository(exec)
	cc, err := repo.Create(context.Background(), domain.CreateCreditCardParams{
		Name: "Nubank", DueDay: 27, ClosingDay: 20, LimitCents: 500000,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if cc.ID != 7 || cc.LimitCents != 500000 {
		t.Errorf("got %+v", cc)
	}
	if gotBody.DueDay != 27 || gotBody.ClosingDay != 20 {
		t.Errorf("server received %+v", gotBody)
	}
}

func TestCreditCardRepository_Update_SendsOnlySetFields(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/credit_cards/7" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&raw)
		_, _ = io.WriteString(w, `{"id":7,"name":"Renamed","closing_day":20,"due_day":27}`)
	})
	repo := NewCreditCardRepository(exec)
	name := "Renamed"
	cc, err := repo.Update(context.Background(), 7, domain.UpdateCreditCardParams{Name: &name})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if cc.Name != "Renamed" {
		t.Errorf("got %+v", cc)
	}
	if _, has := raw["due_day"]; has {
		t.Errorf("absent fields must be omitted; body=%v", raw)
	}
}

func TestCreditCardRepository_Update_SendsAllOptionalFields(t *testing.T) {
	var gotBody []byte
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":3,"name":"Visa","closing_day":4,"due_day":17,"limit_cents":2000000,"archived":false,"default":true}`)
	})
	repo := NewCreditCardRepository(exec)
	limit := int64(2000000)
	network := "mastercard"
	archived := false
	defaultCard := true
	if _, err := repo.Update(context.Background(), 3, domain.UpdateCreditCardParams{
		LimitCents:  &limit,
		CardNetwork: &network,
		Archived:    &archived,
		Default:     &defaultCard,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got := string(gotBody)
	for _, want := range []string{`"limit_cents":2000000`, `"card_network":"mastercard"`, `"archived":false`, `"default":true`} {
		if !strings.Contains(got, want) {
			t.Errorf("body = %q, missing %s", got, want)
		}
	}
}

func TestCreditCardRepository_Delete(t *testing.T) {
	called := false
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodDelete || r.URL.Path != "/credit_cards/7" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	repo := NewCreditCardRepository(exec)
	if err := repo.Delete(context.Background(), 7); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !called {
		t.Error("handler not invoked")
	}
}
