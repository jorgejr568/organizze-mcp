package mcp

import "testing"

func TestNew_BuildsServerWithoutPanic(t *testing.T) {
	deps := Dependencies{
		User:       nopUserSvc{},
		Account:    nopAccountSvc{},
		Category:   nopCategorySvc{},
		Budget:     nopBudgetSvc{},
		CreditCard: nopCreditCardSvc{},
		Invoice:    nopInvoiceSvc{},
		Transfer:   nopTransferSvc{},
		// Transaction nop is added by Task 10 alongside its tools_*.go file.
		// The line below stays commented until then so this test compiles in
		// isolation.
		// Transaction: nopTransactionSvc{},
	}
	s := New(deps)
	if s == nil {
		t.Fatal("New returned nil")
	}
}
