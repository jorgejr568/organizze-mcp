package mcp

import "testing"

func TestNew_BuildsServerWithoutPanic(t *testing.T) {
	deps := Dependencies{
		User:        nopUserSvc{},
		Account:     nopAccountSvc{},
		Category:    nopCategorySvc{},
		Budget:      nopBudgetSvc{},
		CreditCard:  nopCreditCardSvc{},
		Invoice:     nopInvoiceSvc{},
		Transfer:    nopTransferSvc{},
		Transaction: nopTransactionSvc{},
	}
	s := New(deps)
	if s == nil {
		t.Fatal("New returned nil")
	}
}
