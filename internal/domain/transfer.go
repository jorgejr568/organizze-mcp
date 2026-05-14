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
