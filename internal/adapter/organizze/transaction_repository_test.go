package organizze

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

func TestTransactionRepository_List_PassesAllFilters(t *testing.T) {
	var got url.Values
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = io.WriteString(w, `[]`)
	})
	repo := NewTransactionRepository(exec)
	_, err := repo.List(context.Background(), domain.ListTransactionsFilter{
		StartDate: "2026-05-01", EndDate: "2026-05-31", AccountID: 7,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got.Get("start_date") != "2026-05-01" ||
		got.Get("end_date") != "2026-05-31" ||
		got.Get("account_id") != "7" {
		t.Errorf("query = %v", got)
	}
}

func TestTransactionRepository_Get(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transactions/55" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":55,"description":"Pizza","amount_cents":-4500}`)
	})
	repo := NewTransactionRepository(exec)
	tx, err := repo.Get(context.Background(), 55)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tx.ID != 55 || tx.AmountCents != -4500 {
		t.Errorf("got %+v", tx)
	}
}

func TestTransactionRepository_Create(t *testing.T) {
	var gotBody domain.CreateTransactionParams
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/transactions" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":777,"description":"Coffee","amount_cents":-1500}`)
	})
	repo := NewTransactionRepository(exec)
	tx, err := repo.Create(context.Background(), domain.CreateTransactionParams{
		Description: "Coffee", Date: "2026-05-14", AmountCents: -1500,
		AccountID: 1, CategoryID: 10, Paid: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tx.ID != 777 {
		t.Errorf("got %+v", tx)
	}
	if gotBody.Description != "Coffee" || gotBody.AccountID != 1 {
		t.Errorf("server received %+v", gotBody)
	}
}

func TestTransactionRepository_Create_IncludesRecurrenceAttributes(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":97,"description":"Despesa fixa","recurring":true}`)
	})
	repo := NewTransactionRepository(exec)
	_, err := repo.Create(context.Background(), domain.CreateTransactionParams{
		Description: "Despesa fixa", Date: "2026-05-14", AmountCents: -10000,
		AccountID: 3, CategoryID: 21,
		Recurrence: &domain.RecurrenceAttributes{Periodicity: domain.PeriodicityMonthly},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	rec, ok := raw["recurrence_attributes"].(map[string]any)
	if !ok {
		t.Fatalf("recurrence_attributes missing from body: %v", raw)
	}
	if rec["periodicity"] != "monthly" {
		t.Errorf("periodicity = %v, want monthly", rec["periodicity"])
	}
}

func TestTransactionRepository_Create_OmitsRecurrenceWhenNil(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":1}`)
	})
	repo := NewTransactionRepository(exec)
	_, err := repo.Create(context.Background(), domain.CreateTransactionParams{
		Description: "Coffee", Date: "2026-05-14", AmountCents: -1500,
		AccountID: 1, CategoryID: 10,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, has := raw["recurrence_attributes"]; has {
		t.Errorf("recurrence_attributes should be omitted when nil; body=%v", raw)
	}
}

func TestTransactionRepository_Update_SendsOnlySetFields(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/transactions/777" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&raw)
		_, _ = io.WriteString(w, `{"id":777,"description":"Tea"}`)
	})
	repo := NewTransactionRepository(exec)
	desc := "Tea"
	tx, err := repo.Update(context.Background(), 777, domain.UpdateTransactionParams{
		Description: &desc,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if tx.Description != "Tea" {
		t.Errorf("got %+v", tx)
	}
	if _, has := raw["amount_cents"]; has {
		t.Errorf("absent fields must be omitted; body=%v", raw)
	}
	if raw["description"] != "Tea" {
		t.Errorf("body=%v", raw)
	}
}

func TestTransactionRepository_Delete(t *testing.T) {
	called := false
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodDelete || r.URL.Path != "/transactions/777" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	repo := NewTransactionRepository(exec)
	if err := repo.Delete(context.Background(), 777); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !called {
		t.Error("handler not invoked")
	}
}
