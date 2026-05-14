package domain

import "encoding/json"

// Transfer is a movement of money between two accounts. The JSON shape mirrors
// `ORGANIZZE_API.md` "Transfers" — see Listar/Detalhar/Criar/Atualizar/Excluir.
type Transfer struct {
	ID                      int64             `json:"id"`
	Description             string            `json:"description"`
	Date                    string            `json:"date"`
	Paid                    bool              `json:"paid"`
	AmountCents             int64             `json:"amount_cents"`
	TotalInstallments       int               `json:"total_installments,omitempty"`
	Installment             int               `json:"installment,omitempty"`
	Recurring               bool              `json:"recurring,omitempty"`
	AccountID               int64             `json:"account_id"`
	CategoryID              int64             `json:"category_id"`
	Notes                   string            `json:"notes,omitempty"`
	AttachmentsCount        int               `json:"attachments_count,omitempty"`
	CreditCardID            *int64            `json:"credit_card_id,omitempty"`
	CreditCardInvoiceID     *int64            `json:"credit_card_invoice_id,omitempty"`
	PaidCreditCardID        *int64            `json:"paid_credit_card_id,omitempty"`
	PaidCreditCardInvoiceID *int64            `json:"paid_credit_card_invoice_id,omitempty"`
	OppositeTransactionID   *int64            `json:"oposite_transaction_id,omitempty"`
	OppositeAccountID       *int64            `json:"oposite_account_id,omitempty"`
	RecurrenceID            *int64            `json:"recurrence_id,omitempty"`
	Tags                    []Tag             `json:"tags,omitempty"`
	Attachments             []json.RawMessage `json:"attachments,omitempty"`
	CreatedAt               string            `json:"created_at,omitempty"`
	UpdatedAt               string            `json:"updated_at,omitempty"`
	Deleted                 bool              `json:"deleted,omitempty"`
}

// CreateTransferParams are the inputs to TransferService.Create. The Organizze
// API requires credit_account_id, debit_account_id, amount_cents, date.
// "credit" is the receiving account; "debit" is the sending account.
// Only bank accounts are allowed (credit cards are rejected upstream).
type CreateTransferParams struct {
	CreditAccountID int64  `json:"credit_account_id"`
	DebitAccountID  int64  `json:"debit_account_id"`
	AmountCents     int64  `json:"amount_cents"`
	Date            string `json:"date"`
	Paid            bool   `json:"paid"`
	Tags            []Tag  `json:"tags,omitempty"`
}

// UpdateTransferParams describes a partial update; nil pointers are omitted.
// Organizze's PUT /transfers/{id} only accepts description, notes, and tags.
type UpdateTransferParams struct {
	Description *string `json:"description,omitempty"`
	Notes       *string `json:"notes,omitempty"`
	Tags        []Tag   `json:"tags,omitempty"`
}
