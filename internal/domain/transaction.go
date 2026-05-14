package domain

// Tag is a lightweight tag attached to a transaction.
type Tag struct {
	Name string `json:"name"`
}

// Transaction is a ledger entry. AmountCents is negative for expenses,
// positive for income, per the Organizze API.
type Transaction struct {
	ID                      int64  `json:"id"`
	Description             string `json:"description"`
	Date                    string `json:"date"`
	Paid                    bool   `json:"paid"`
	AmountCents             int64  `json:"amount_cents"`
	TotalInstallments       int    `json:"total_installments,omitempty"`
	Installment             int    `json:"installment,omitempty"`
	Recurring               bool   `json:"recurring,omitempty"`
	AccountID               int64  `json:"account_id"`
	AccountType             string `json:"account_type,omitempty"`
	CategoryID              int64  `json:"category_id"`
	ContactID               *int64 `json:"contact_id,omitempty"`
	Notes                   string `json:"notes,omitempty"`
	AttachmentsCount        int    `json:"attachments_count,omitempty"`
	CreditCardID            *int64 `json:"credit_card_id,omitempty"`
	CreditCardInvoiceID     *int64 `json:"credit_card_invoice_id,omitempty"`
	PaidCreditCardID        *int64 `json:"paid_credit_card_id,omitempty"`
	PaidCreditCardInvoiceID *int64 `json:"paid_credit_card_invoice_id,omitempty"`
	OppositeTransactionID   *int64 `json:"oposite_transaction_id,omitempty"`
	OppositeAccountID       *int64 `json:"oposite_account_id,omitempty"`
	RecurrenceID            *int64 `json:"recurrence_id,omitempty"`
	Tags                    []Tag  `json:"tags,omitempty"`
	CreatedAt               string `json:"created_at,omitempty"`
	UpdatedAt               string `json:"updated_at,omitempty"`
}

// CreateTransactionParams are the inputs to TransactionService.Create.
// Shape mirrors the Organizze POST body but is owned by the domain layer.
type CreateTransactionParams struct {
	Description string `json:"description"`
	Date        string `json:"date"`
	AmountCents int64  `json:"amount_cents"`
	AccountID   int64  `json:"account_id"`
	CategoryID  int64  `json:"category_id"`
	Paid        bool   `json:"paid"`
	Notes       string `json:"notes,omitempty"`
	ContactID   *int64 `json:"contact_id,omitempty"`
	Tags        []Tag  `json:"tags,omitempty"`
}

// UpdateTransactionParams describe a partial update; nil pointers are omitted
// from the wire body via `omitempty`.
//
// Semantics rely on a load-bearing assumption about the upstream Organizze API:
// fields absent from the PUT body are treated as "leave unchanged", NOT as
// "clear to zero / null". If Organizze ever changes this behaviour, every
// caller of TransactionService.Update becomes destructive. The contract is
// tested at the wire level (TestTransactionRepository_Update_SendsOnlySetFields
// asserts that absent fields are absent from the JSON), but the semantic
// assumption beyond the wire is not tested by anything we control.
//
// Note: `Tags []Tag` has different semantics — because it's not a pointer,
// `omitempty` only drops nil; an explicit `[]Tag{}` will be marshalled and may
// clear server-side tags. Pass nil to leave tags unchanged.
type UpdateTransactionParams struct {
	Description *string `json:"description,omitempty"`
	Date        *string `json:"date,omitempty"`
	AmountCents *int64  `json:"amount_cents,omitempty"`
	AccountID   *int64  `json:"account_id,omitempty"`
	CategoryID  *int64  `json:"category_id,omitempty"`
	Paid        *bool   `json:"paid,omitempty"`
	Notes       *string `json:"notes,omitempty"`
	ContactID   *int64  `json:"contact_id,omitempty"`
	Tags        []Tag   `json:"tags,omitempty"`
}
