package organizze

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
	"github.com/jorgejr568/organizze-mcp/internal/oauth/credprovider"
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

func TestTransactionRepository_Create_IncludesCreditCardFields(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":1}`)
	})
	repo := NewTransactionRepository(exec)
	cardID := int64(1287765)
	invoiceID := int64(276)
	_, err := repo.Create(context.Background(), domain.CreateTransactionParams{
		Description: "Coffee", Date: "2026-05-14", AmountCents: -1500,
		AccountID: 1, CategoryID: 10,
		CreditCardID:        &cardID,
		CreditCardInvoiceID: &invoiceID,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if raw["credit_card_id"] != float64(1287765) {
		t.Errorf("credit_card_id = %v, want 1287765", raw["credit_card_id"])
	}
	if raw["credit_card_invoice_id"] != float64(276) {
		t.Errorf("credit_card_invoice_id = %v, want 276", raw["credit_card_invoice_id"])
	}
}

func TestTransactionRepository_Create_OmitsCreditCardFieldsWhenNil(t *testing.T) {
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
	for _, k := range []string{"credit_card_id", "credit_card_invoice_id"} {
		if _, has := raw[k]; has {
			t.Errorf("%s must be omitted when nil; body=%v", k, raw)
		}
	}
}

func TestTransactionRepository_Update_IncludesCreditCardID(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		_, _ = io.WriteString(w, `{"id":777}`)
	})
	repo := NewTransactionRepository(exec)
	cardID := int64(1287765)
	_, err := repo.Update(context.Background(), 777, domain.UpdateTransactionParams{
		CreditCardID: &cardID,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if raw["credit_card_id"] != float64(1287765) {
		t.Errorf("credit_card_id = %v, want 1287765", raw["credit_card_id"])
	}
}

func TestTransactionRepository_Create_OmitsAccountIDWhenZero(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":1}`)
	})
	repo := NewTransactionRepository(exec)
	cardID := int64(386176)
	_, err := repo.Create(context.Background(), domain.CreateTransactionParams{
		Description: "Card buy", Date: "2026-05-14", AmountCents: -1500,
		CategoryID:   10,
		CreditCardID: &cardID,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, has := raw["account_id"]; has {
		t.Errorf("account_id must be absent from body when zero (so Organizze does not drop credit_card_id); body=%v", raw)
	}
	if raw["credit_card_id"] != float64(386176) {
		t.Errorf("credit_card_id = %v, want 386176", raw["credit_card_id"])
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

func TestTransactionRepository_Update_SendsUpdateFuture(t *testing.T) {
	var gotBody []byte
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":101,"description":"x","amount_cents":1,"account_id":1,"category_id":1,"date":"2026-05-14"}`)
	})
	repo := NewTransactionRepository(exec)
	uf := true
	if _, err := repo.Update(context.Background(), 101, domain.UpdateTransactionParams{UpdateFuture: &uf}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if string(gotBody) != `{"update_future":true}` {
		t.Errorf("body = %q", string(gotBody))
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
	if _, err := repo.Delete(context.Background(), 777, domain.DeleteTransactionParams{}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !called {
		t.Error("handler not invoked")
	}
}

func TestTransactionRepository_Delete_SendsBodyWhenFlagsSet(t *testing.T) {
	var gotBody []byte
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	})
	repo := NewTransactionRepository(exec)
	uf := true
	if _, err := repo.Delete(context.Background(), 101, domain.DeleteTransactionParams{UpdateFuture: &uf}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if string(gotBody) != `{"update_future":true}` {
		t.Errorf("body = %q", string(gotBody))
	}
}

func TestTransactionRepository_Delete_NoFlags_SendsNoBody(t *testing.T) {
	var gotBody []byte
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	})
	repo := NewTransactionRepository(exec)
	if _, err := repo.Delete(context.Background(), 101, domain.DeleteTransactionParams{}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(gotBody) != 0 {
		t.Errorf("body = %q on empty params, want empty", string(gotBody))
	}
}

func TestTransactionRepository_Update_IncludesCreditCardInvoiceID(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		_, _ = io.WriteString(w, `{"id":777}`)
	})
	repo := NewTransactionRepository(exec)
	cardID := int64(386176)
	invoiceID := int64(317)
	_, err := repo.Update(context.Background(), 777, domain.UpdateTransactionParams{
		CreditCardID:        &cardID,
		CreditCardInvoiceID: &invoiceID,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if raw["credit_card_id"] != float64(386176) {
		t.Errorf("credit_card_id = %v, want 386176", raw["credit_card_id"])
	}
	if raw["credit_card_invoice_id"] != float64(317) {
		t.Errorf("credit_card_invoice_id = %v, want 317", raw["credit_card_invoice_id"])
	}
}

func TestTransactionRepository_Update_OmitsAccountIDAndCreditCardInvoiceIDWhenNil(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		_, _ = io.WriteString(w, `{"id":777}`)
	})
	repo := NewTransactionRepository(exec)
	cardID := int64(386176)
	_, err := repo.Update(context.Background(), 777, domain.UpdateTransactionParams{
		CreditCardID: &cardID,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	for _, k := range []string{"account_id", "credit_card_invoice_id"} {
		if _, has := raw[k]; has {
			t.Errorf("%s must be omitted when nil; body=%v", k, raw)
		}
	}
}

func TestTransactionRepository_Create_LoggingEnabled_EmitsBodyLine(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":1234,"description":"MCP test","amount_cents":-1}`)
	}))
	t.Cleanup(ts.Close)

	exec, err := NewRequestExecutor(RequestExecutorOptions{
		HTTPClient:  NewClient(ClientOptions{}),
		BaseURL:     ts.URL,
		Credentials: credprovider.Static("test@example.com", "test-key", "Test (test@example.com)"),
		LogRequests: true,
		Logger:      zap.New(core),
	})
	if err != nil {
		t.Fatalf("NewRequestExecutor: %v", err)
	}
	repo := NewTransactionRepository(exec)

	params := domain.CreateTransactionParams{
		Description: "MCP test - auto-delete",
		AmountCents: -1,
		Date:        "2026-05-15",
		CategoryID:  42,
		AccountID:   7,
	}
	tx, err := repo.Create(context.Background(), params)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tx.ID != 1234 {
		t.Errorf("tx.ID = %d, want 1234", tx.ID)
	}

	// Request-side: outbound body contains every populated field.
	reqEntries := logs.FilterMessage("organizze request").All()
	if len(reqEntries) != 1 {
		t.Fatalf("expected 1 request entry, got %d", len(reqEntries))
	}
	reqFields := reqEntries[0].ContextMap()
	if reqFields["method"] != "POST" || reqFields["path"] != "/transactions" {
		t.Errorf("request method/path: %v / %v", reqFields["method"], reqFields["path"])
	}
	reqBody, _ := reqFields["body"].(string)
	for _, want := range []string{
		`"description":"MCP test - auto-delete"`,
		`"amount_cents":-1`,
		`"date":"2026-05-15"`,
		`"category_id":42`,
		`"account_id":7`,
	} {
		if !strings.Contains(reqBody, want) {
			t.Errorf("request body missing %q; got %q", want, reqBody)
		}
	}

	// Response-side: status and id appear.
	respEntries := logs.FilterMessage("organizze response").All()
	if len(respEntries) != 1 {
		t.Fatalf("expected 1 response entry, got %d", len(respEntries))
	}
	respFields := respEntries[0].ContextMap()
	if got, _ := respFields["status"].(int64); got != 201 {
		t.Errorf("response status = %v, want 201", respFields["status"])
	}
	respBody, _ := respFields["body"].(string)
	if !strings.Contains(respBody, `"id":1234`) {
		t.Errorf("response body missing id:1234; got %q", respBody)
	}

	// Redaction guard still holds at the repository layer.
	dump := dumpEntries(logs.All())
	for _, banned := range []string{"Basic ", "Authorization", "test-key"} {
		if strings.Contains(dump, banned) {
			t.Errorf("log leaked %q; full output:\n%s", banned, dump)
		}
	}
}
