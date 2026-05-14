package mcp

import "testing"

func TestNew_BuildsServerWithoutPanic(t *testing.T) {
	deps := Dependencies{
		User:     nopUserSvc{},
		Account:  nopAccountSvc{},
		Category: nopCategorySvc{},
		Budget:   nopBudgetSvc{},
		// CreditCard, Invoice, Transfer, and Transaction nops are added by
		// Tasks 9-10 alongside their tools_*.go files. The lines below stay
		// commented until then so this test compiles in isolation.
		// CreditCard:  nopCreditCardSvc{},
		// Invoice:     nopInvoiceSvc{},
		// Transfer:    nopTransferSvc{},
		// Transaction: nopTransactionSvc{},
	}
	s := New(deps)
	if s == nil {
		t.Fatal("New returned nil")
	}
}
