package organizze

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// TestJSONShape_DomainTypesDecodeRealisticFixtures decodes representative API
// payloads into the matching domain.* type and asserts each load-bearing field
// populates. This catches silent field drops caused by typos in JSON tags or
// upstream renames.
//
// To refresh a fixture: capture a real Organizze response (redacting any PII),
// drop it into testdata/<resource>.json, and ensure the assertion below for
// that case still picks meaningful fields.
func TestJSONShape_DomainTypesDecodeRealisticFixtures(t *testing.T) {
	t.Run("User", func(t *testing.T) {
		var u domain.User
		mustDecodeFixture(t, "user.json", &u)
		if u.ID == 0 || u.Name == "" || u.Email == "" || u.Role == "" {
			t.Errorf("User decode lost fields: %+v", u)
		}
	})

	t.Run("Account", func(t *testing.T) {
		var a domain.Account
		mustDecodeFixture(t, "account.json", &a)
		if a.ID == 0 || a.Name == "" || a.Type == "" {
			t.Errorf("Account decode lost fields: %+v", a)
		}
	})

	t.Run("Category", func(t *testing.T) {
		var c domain.Category
		mustDecodeFixture(t, "category.json", &c)
		if c.ID == 0 || c.Name == "" {
			t.Errorf("Category decode lost fields: %+v", c)
		}
	})

	t.Run("Budget", func(t *testing.T) {
		var b domain.Budget
		mustDecodeFixture(t, "budget.json", &b)
		if b.AmountInCents == 0 || b.CategoryID == 0 || b.Date == "" {
			t.Errorf("Budget decode lost fields: %+v", b)
		}
	})

	t.Run("CreditCard", func(t *testing.T) {
		var c domain.CreditCard
		mustDecodeFixture(t, "credit_card.json", &c)
		if c.ID == 0 || c.Name == "" || c.ClosingDay == 0 || c.DueDay == 0 || c.LimitCents == 0 {
			t.Errorf("CreditCard decode lost fields: %+v", c)
		}
	})

	t.Run("Invoice", func(t *testing.T) {
		var inv domain.Invoice
		mustDecodeFixture(t, "invoice.json", &inv)
		if inv.ID == 0 || inv.CreditCardID == 0 || inv.AmountCents == 0 ||
			inv.Date == "" || inv.StartingDate == "" || inv.ClosingDate == "" {
			t.Errorf("Invoice decode lost fields: %+v", inv)
		}
	})

	t.Run("Transaction", func(t *testing.T) {
		var tx domain.Transaction
		mustDecodeFixture(t, "transaction.json", &tx)
		if tx.ID == 0 || tx.Description == "" || tx.Date == "" ||
			tx.AmountCents == 0 || tx.AccountID == 0 || tx.CategoryID == 0 {
			t.Errorf("Transaction decode lost core fields: %+v", tx)
		}
		// optional but load-bearing in real usage:
		if len(tx.Tags) == 0 {
			t.Errorf("Transaction decode dropped Tags: %+v", tx)
		}
		if len(tx.Attachments) != 1 || tx.Attachments[0] != "https://example.com/receipt.pdf" {
			t.Errorf("Transaction decode dropped Attachments: %+v", tx.Attachments)
		}
	})

	t.Run("Transfer", func(t *testing.T) {
		var tr domain.Transfer
		mustDecodeFixture(t, "transfer.json", &tr)
		if tr.ID == 0 || tr.Description == "" || tr.AmountCents == 0 ||
			tr.AccountID == 0 || tr.OppositeAccountID == nil || *tr.OppositeAccountID == 0 || tr.CategoryID == 0 {
			t.Errorf("Transfer decode lost fields: %+v", tr)
		}
		if len(tr.Attachments) != 1 || tr.Attachments[0] != "https://example.com/transfer-receipt.pdf" {
			t.Errorf("Transfer decode dropped Attachments: %+v", tr.Attachments)
		}
	})
}

func mustDecodeFixture(t *testing.T, name string, into any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("unmarshal %s: %v", name, err)
	}
}
