package mcp_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/adapter/mcp"
	"github.com/jorgejr568/organizze-mcp/internal/adapter/organizze"
	"github.com/jorgejr568/organizze-mcp/internal/usecase"
)

// fakeOrganizze responds to every endpoint touched by any tool. Unknown paths
// fail the test loudly.
func fakeOrganizze(t *testing.T) *httptest.Server {
	t.Helper()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/users/3":
			_, _ = io.WriteString(w, `{"id":3,"name":"Jorge","email":"j@x.com","role":"admin"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/accounts":
			_, _ = io.WriteString(w, `[{"id":1,"name":"Checking","type":"checking"}]`)
		case r.Method == http.MethodGet && r.URL.Path == "/accounts/1":
			_, _ = io.WriteString(w, `{"id":1,"name":"Checking","type":"checking"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/categories":
			_, _ = io.WriteString(w, `[{"id":10,"name":"Food"}]`)
		case r.Method == http.MethodGet && r.URL.Path == "/categories/10":
			_, _ = io.WriteString(w, `{"id":10,"name":"Food"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/budgets":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == http.MethodGet && r.URL.Path == "/budgets/2026":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == http.MethodGet && r.URL.Path == "/budgets/2026/5":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == http.MethodGet && r.URL.Path == "/credit_cards":
			_, _ = io.WriteString(w, `[{"id":1,"name":"Nubank","closing_day":20,"due_day":27,"limit_cents":500000}]`)
		case r.Method == http.MethodGet && r.URL.Path == "/credit_cards/1":
			_, _ = io.WriteString(w, `{"id":1,"name":"Nubank","closing_day":20,"due_day":27,"limit_cents":500000}`)
		case r.Method == http.MethodGet && r.URL.Path == "/credit_cards/1/invoices":
			_, _ = io.WriteString(w, `[{"id":100,"credit_card_id":1,"amount_cents":120000}]`)
		case r.Method == http.MethodGet && r.URL.Path == "/credit_cards/1/invoices/100":
			_, _ = io.WriteString(w, `{"id":100,"credit_card_id":1,"amount_cents":120000}`)
		case r.Method == http.MethodGet && r.URL.Path == "/credit_cards/1/invoices/100/payments":
			_, _ = io.WriteString(w, `{"id":1033,"description":"Pagamento fatura","amount_cents":0,"account_id":1,"category_id":10,"date":"2026-05-14"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/transfers":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == http.MethodGet && r.URL.Path == "/transfers/123":
			_, _ = io.WriteString(w, `{"id":123,"description":"Transferência","amount_cents":-50000,"account_id":2,"oposite_account_id":1,"date":"2026-05-14"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/transactions":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == http.MethodGet && r.URL.Path == "/transactions/55":
			_, _ = io.WriteString(w, `{"id":55,"description":"Pizza","amount_cents":-4500,"account_id":1,"category_id":10,"date":"2026-05-10"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/transactions":
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"id":777,"description":"Coffee","amount_cents":-1500,"account_id":1,"category_id":10,"date":"2026-05-14"}`)
		case r.Method == http.MethodPut && r.URL.Path == "/transactions/55":
			_, _ = io.WriteString(w, `{"id":55,"description":"Pizza-updated","amount_cents":-4500,"account_id":1,"category_id":10,"date":"2026-05-10"}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/transactions/55":
			w.WriteHeader(http.StatusNoContent)

		// accounts write
		case r.Method == http.MethodPost && r.URL.Path == "/accounts":
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"id":18,"name":"Itaú CC","type":"checking","default":true}`)
		case r.Method == http.MethodPut && r.URL.Path == "/accounts/1":
			_, _ = io.WriteString(w, `{"id":1,"name":"Checking-renamed","type":"checking"}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/accounts/1":
			w.WriteHeader(http.StatusNoContent)

		// categories write
		case r.Method == http.MethodPost && r.URL.Path == "/categories":
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"id":42,"name":"Groceries"}`)
		case r.Method == http.MethodPut && r.URL.Path == "/categories/10":
			_, _ = io.WriteString(w, `{"id":10,"name":"Food-updated"}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/categories/10":
			w.WriteHeader(http.StatusNoContent)

		// credit cards write
		case r.Method == http.MethodPost && r.URL.Path == "/credit_cards":
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"id":7,"name":"Nubank","closing_day":20,"due_day":27,"limit_cents":500000}`)
		case r.Method == http.MethodPut && r.URL.Path == "/credit_cards/1":
			_, _ = io.WriteString(w, `{"id":1,"name":"Nubank-renamed","closing_day":20,"due_day":27,"limit_cents":500000}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/credit_cards/1":
			w.WriteHeader(http.StatusNoContent)

		// transfers write
		case r.Method == http.MethodPost && r.URL.Path == "/transfers":
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"id":123,"description":"Transferência","amount_cents":50000,"account_id":2,"oposite_account_id":1,"date":"2026-05-14"}`)
		case r.Method == http.MethodPut && r.URL.Path == "/transfers/123":
			_, _ = io.WriteString(w, `{"id":123,"description":"Reimbursement","amount_cents":50000,"account_id":2,"oposite_account_id":1,"date":"2026-05-14"}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/transfers/123":
			w.WriteHeader(http.StatusNoContent)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

// newRealServer wires every layer with no shortcuts.
func newRealServer(t *testing.T) *mcpsdk.Server {
	t.Helper()
	api := fakeOrganizze(t)
	client := organizze.NewClient(organizze.ClientOptions{})
	exec, err := organizze.NewRequestExecutor(organizze.RequestExecutorOptions{
		HTTPClient: client,
		BaseURL:    api.URL,
		Email:      "test@example.com",
		APIKey:     "k",
		UserAgent:  "Test (e@x.com)",
	})
	if err != nil {
		t.Fatalf("executor: %v", err)
	}

	deps := mcp.Dependencies{
		User:        usecase.NewUserService(organizze.NewUserRepository(exec)),
		Account:     usecase.NewAccountService(organizze.NewAccountRepository(exec)),
		Category:    usecase.NewCategoryService(organizze.NewCategoryRepository(exec)),
		Budget:      usecase.NewBudgetService(organizze.NewBudgetRepository(exec)),
		CreditCard:  usecase.NewCreditCardService(organizze.NewCreditCardRepository(exec)),
		Invoice:     usecase.NewInvoiceService(organizze.NewInvoiceRepository(exec)),
		Transfer:    usecase.NewTransferService(organizze.NewTransferRepository(exec)),
		Transaction: usecase.NewTransactionService(organizze.NewTransactionRepository(exec)),
	}
	return mcp.New(deps)
}

func newConnectedSession(t *testing.T) *mcpsdk.ClientSession {
	t.Helper()
	server := newRealServer(t)
	serverT, clientT := mcpsdk.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if _, err := server.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "integration-test", Version: "0"}, nil)
	sess, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

var allExpectedTools = []string{
	"get_user",
	"list_accounts", "get_account",
	"create_account", "update_account", "delete_account",
	"list_categories", "get_category",
	"create_category", "update_category", "delete_category",
	"list_budgets",
	"list_credit_cards", "get_credit_card",
	"create_credit_card", "update_credit_card", "delete_credit_card",
	"list_credit_card_invoices", "get_credit_card_invoice", "get_credit_card_invoice_payment",
	"list_transfers", "get_transfer",
	"create_transfer", "update_transfer", "delete_transfer",
	"list_transactions", "get_transaction",
	"create_transaction", "update_transaction", "delete_transaction",
}

func TestIntegration_AllToolsRegisteredWithSchemas(t *testing.T) {
	sess := newConnectedSession(t)
	res, err := sess.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	got := make([]string, 0, len(res.Tools))
	by := make(map[string]*mcpsdk.Tool, len(res.Tools))
	for _, tl := range res.Tools {
		got = append(got, tl.Name)
		by[tl.Name] = tl
	}
	sort.Strings(got)
	want := append([]string(nil), allExpectedTools...)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Errorf("got %d tools (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for _, name := range want {
		tl, ok := by[name]
		if !ok {
			t.Errorf("tool %q not registered", name)
			continue
		}
		if tl.Description == "" {
			t.Errorf("tool %q missing Description", name)
		}
		if tl.InputSchema == nil {
			t.Errorf("tool %q missing InputSchema", name)
		}
		if tl.OutputSchema == nil {
			t.Errorf("tool %q missing OutputSchema", name)
		}
	}
}

func TestIntegration_EveryToolRoundtripsThroughProtocol(t *testing.T) {
	sess := newConnectedSession(t)
	cases := []struct {
		label string
		name  string
		args  any
	}{
		{"get_user", "get_user", map[string]any{"id": 3}},
		{"list_accounts", "list_accounts", map[string]any{}},
		{"get_account", "get_account", map[string]any{"id": 1}},
		{"list_categories", "list_categories", map[string]any{}},
		{"get_category", "get_category", map[string]any{"id": 10}},
		{"list_budgets/current", "list_budgets", map[string]any{}},
		{"list_budgets/year", "list_budgets", map[string]any{"year": 2026}},
		{"list_budgets/month", "list_budgets", map[string]any{"year": 2026, "month": 5}},
		{"list_credit_cards", "list_credit_cards", map[string]any{}},
		{"get_credit_card", "get_credit_card", map[string]any{"id": 1}},
		{"list_credit_card_invoices", "list_credit_card_invoices", map[string]any{"credit_card_id": 1}},
		{"get_credit_card_invoice", "get_credit_card_invoice", map[string]any{"credit_card_id": 1, "invoice_id": 100}},
		{"get_credit_card_invoice_payment", "get_credit_card_invoice_payment", map[string]any{"credit_card_id": 1, "invoice_id": 100}},
		{"list_transfers", "list_transfers", map[string]any{}},
		{"get_transfer", "get_transfer", map[string]any{"id": 123}},
		{"list_transactions", "list_transactions", map[string]any{}},
		{"get_transaction", "get_transaction", map[string]any{"id": 55}},
		{"create_transaction", "create_transaction", map[string]any{
			"description":  "Coffee",
			"date":         "2026-05-14",
			"amount_cents": -1500,
			"account_id":   1,
			"category_id":  10,
			"paid":         true,
		}},
		{"update_transaction", "update_transaction", map[string]any{
			"id":          55,
			"description": "Pizza-updated",
		}},
		{"delete_transaction", "delete_transaction", map[string]any{"id": 55}},

		{"create_account", "create_account", map[string]any{
			"name": "Itaú CC", "type": "checking",
		}},
		{"update_account", "update_account", map[string]any{
			"id": 1, "name": "Checking-renamed",
		}},
		{"delete_account", "delete_account", map[string]any{"id": 1}},

		{"create_category", "create_category", map[string]any{"name": "Groceries"}},
		{"update_category", "update_category", map[string]any{
			"id": 10, "name": "Food-updated",
		}},
		{"delete_category", "delete_category", map[string]any{"id": 10}},

		{"create_credit_card", "create_credit_card", map[string]any{
			"name": "Nubank", "due_day": 27, "closing_day": 20,
		}},
		{"update_credit_card", "update_credit_card", map[string]any{
			"id": 1, "name": "Nubank-renamed",
		}},
		{"delete_credit_card", "delete_credit_card", map[string]any{"id": 1}},

		{"create_transfer", "create_transfer", map[string]any{
			"credit_account_id": 2, "debit_account_id": 1,
			"amount_cents": 50000, "date": "2026-05-14", "paid": true,
		}},
		{"update_transfer", "update_transfer", map[string]any{
			"id": 123, "description": "Reimbursement",
		}},
		{"delete_transfer", "delete_transfer", map[string]any{"id": 123}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.label, func(t *testing.T) {
			res, err := sess.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: tc.name, Arguments: tc.args})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if res.IsError {
				t.Fatalf("IsError=true; content=%v", res.Content)
			}
			if len(res.Content) == 0 {
				t.Errorf("no content")
			}
		})
	}
}

func TestIntegration_BudgetMonthWithoutYear_ReturnsToolError(t *testing.T) {
	sess := newConnectedSession(t)
	res, err := sess.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "list_budgets", Arguments: map[string]any{"month": 5},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true; content=%v", res.Content)
	}
}

func TestIntegration_CreateTransactionMissingFields_ReturnsToolError(t *testing.T) {
	sess := newConnectedSession(t)
	res, err := sess.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: "create_transaction", Arguments: map[string]any{"description": "x"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true; content=%v", res.Content)
	}
}
