package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

func TestTransfer_UnmarshalsAllDocumentedFields(t *testing.T) {
	in := `{
		"id": 10, "description": "Transferência", "date": "2015-09-01",
		"paid": true, "amount_cents": -10000,
		"total_installments": 1, "installment": 1, "recurring": false,
		"account_id": 3, "category_id": 21, "notes": null,
		"attachments_count": 1,
		"credit_card_id": null, "credit_card_invoice_id": null,
		"paid_credit_card_id": null, "paid_credit_card_invoice_id": null,
		"oposite_transaction_id": 11, "oposite_account_id": 4,
		"created_at": "2015-09-01T23:42:29-03:00",
		"updated_at": "2015-09-01T23:42:29-03:00",
		"tags": [], "attachments": ["https://example.com/receipt.pdf"], "recurrence_id": null,
		"deleted": true
	}`
	var tr domain.Transfer
	if err := json.Unmarshal([]byte(in), &tr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	checks := map[string]bool{
		"ID":                         tr.ID == 10,
		"TotalInstallments":          tr.TotalInstallments == 1,
		"Installment":                tr.Installment == 1,
		"AttachmentsCount=1":         tr.AttachmentsCount == 1,
		"OppositeTransactionID":      tr.OppositeTransactionID != nil && *tr.OppositeTransactionID == 11,
		"OppositeAccountID":          tr.OppositeAccountID != nil && *tr.OppositeAccountID == 4,
		"CreatedAt":                  tr.CreatedAt == "2015-09-01T23:42:29-03:00",
		"UpdatedAt":                  tr.UpdatedAt == "2015-09-01T23:42:29-03:00",
		"Tags non-nil":               tr.Tags != nil,
		"Attachments len=1":          len(tr.Attachments) == 1,
		"Attachments[0]=receipt-url": len(tr.Attachments) == 1 && tr.Attachments[0] == "https://example.com/receipt.pdf",
		"Deleted":                    tr.Deleted,
	}
	for label, ok := range checks {
		if !ok {
			t.Errorf("check failed: %s; transfer = %+v", label, tr)
		}
	}
}
