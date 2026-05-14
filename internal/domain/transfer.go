package domain

// Transfer is a movement of money between two accounts.
type Transfer struct {
	ID                    int64  `json:"id"`
	Description           string `json:"description"`
	Date                  string `json:"date"`
	Paid                  bool   `json:"paid"`
	AmountCents           int64  `json:"amount_cents"`
	AccountID             int64  `json:"account_id"`
	OppositeAccountID     int64  `json:"oposite_account_id"`
	OppositeTransactionID int64  `json:"oposite_transaction_id,omitempty"`
	CategoryID            int64  `json:"category_id"`
	Notes                 string `json:"notes,omitempty"`
	RecurrenceID          *int64 `json:"recurrence_id,omitempty"`
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
