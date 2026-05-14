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

func TestTransferRepository_List_NoFilter(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transfers" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[]`)
	})
	repo := NewTransferRepository(exec)
	if _, err := repo.List(context.Background(), domain.ListTransfersFilter{}); err != nil {
		t.Fatalf("List: %v", err)
	}
}

func TestTransferRepository_List_WithDateRange(t *testing.T) {
	var got url.Values
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = io.WriteString(w, `[]`)
	})
	repo := NewTransferRepository(exec)
	_, err := repo.List(context.Background(), domain.ListTransfersFilter{
		StartDate: "2026-05-01",
		EndDate:   "2026-05-31",
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got.Get("start_date") != "2026-05-01" || got.Get("end_date") != "2026-05-31" {
		t.Errorf("query = %v", got)
	}
}

func TestTransferRepository_Create(t *testing.T) {
	var gotBody domain.CreateTransferParams
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/transfers" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":123,"description":"Transferência","amount_cents":50000,"account_id":2,"oposite_account_id":1,"date":"2026-05-14"}`)
	})
	repo := NewTransferRepository(exec)
	tr, err := repo.Create(context.Background(), domain.CreateTransferParams{
		CreditAccountID: 2, DebitAccountID: 1, AmountCents: 50000, Date: "2026-05-14", Paid: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tr.ID != 123 {
		t.Errorf("got %+v", tr)
	}
	if gotBody.CreditAccountID != 2 || gotBody.DebitAccountID != 1 || gotBody.AmountCents != 50000 {
		t.Errorf("server received %+v", gotBody)
	}
}

func TestTransferRepository_Update_SendsOnlySetFields(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/transfers/123" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&raw)
		_, _ = io.WriteString(w, `{"id":123,"description":"Reimbursement","amount_cents":50000}`)
	})
	repo := NewTransferRepository(exec)
	desc := "Reimbursement"
	tr, err := repo.Update(context.Background(), 123, domain.UpdateTransferParams{Description: &desc})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if tr.Description != "Reimbursement" {
		t.Errorf("got %+v", tr)
	}
	if _, has := raw["notes"]; has {
		t.Errorf("absent fields must be omitted; body=%v", raw)
	}
	if raw["description"] != "Reimbursement" {
		t.Errorf("body=%v", raw)
	}
}

func TestTransferRepository_Get_HitsTransferURL(t *testing.T) {
	var gotPath string
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":10,"description":"Transferência","amount_cents":-10000,"account_id":3,"date":"2015-09-01"}`)
	})
	repo := NewTransferRepository(exec)
	tr, err := repo.Get(context.Background(), 10)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if gotPath != "/transfers/10" {
		t.Errorf("path = %q", gotPath)
	}
	if tr == nil || tr.ID != 10 {
		t.Errorf("returned = %+v", tr)
	}
}

func TestTransferRepository_Delete_ReturnsDeletedTransferWithDeletedTrue(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/transfers/10" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":10,"description":"Transferência","amount_cents":-10000,"account_id":3,"date":"2015-09-01","deleted":true}`)
	})
	repo := NewTransferRepository(exec)
	tr, err := repo.Delete(context.Background(), 10)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if tr == nil || tr.ID != 10 || !tr.Deleted {
		t.Errorf("returned = %+v", tr)
	}
}
