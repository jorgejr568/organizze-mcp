package domain

// Invoice is a monthly credit-card bill.
type Invoice struct {
	ID                   int64         `json:"id"`
	Date                 string        `json:"date"`
	StartingDate         string        `json:"starting_date"`
	ClosingDate          string        `json:"closing_date"`
	AmountCents          int64         `json:"amount_cents"`
	PaymentAmountCents   int64         `json:"payment_amount_cents,omitempty"`
	BalanceCents         int64         `json:"balance_cents,omitempty"`
	PreviousBalanceCents int64         `json:"previous_balance_cents,omitempty"`
	CreditCardID         int64         `json:"credit_card_id"`
	Transactions         []Transaction `json:"transactions,omitempty"`
	Payments             []Transaction `json:"payments,omitempty"`
}
