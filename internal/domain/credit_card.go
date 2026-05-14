package domain

import "time"

// CreditCard represents a credit card configured in Organizze.
type CreditCard struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	CardNetwork *string   `json:"card_network,omitempty"`
	ClosingDay  int       `json:"closing_day"`
	DueDay      int       `json:"due_day"`
	LimitCents  int64     `json:"limit_cents"`
	Kind        string    `json:"kind,omitempty"`
	Archived    bool      `json:"archived"`
	Default     bool      `json:"default"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// CreateCreditCardParams are the inputs to CreditCardService.Create. The
// Organizze API requires name, due_day (1-31), and closing_day (1-31).
type CreateCreditCardParams struct {
	Name        string `json:"name"`
	DueDay      int    `json:"due_day"`
	ClosingDay  int    `json:"closing_day"`
	CardNetwork string `json:"card_network,omitempty"`
	LimitCents  int64  `json:"limit_cents,omitempty"`
	Description string `json:"description,omitempty"`
	Default     bool   `json:"default,omitempty"`
}

// UpdateCreditCardParams describes a partial update; nil pointers are omitted.
// update_invoices_since is a YYYY-MM-DD date string: when set, Organizze
// retroactively regenerates invoices since that date.
type UpdateCreditCardParams struct {
	Name                *string `json:"name,omitempty"`
	DueDay              *int    `json:"due_day,omitempty"`
	ClosingDay          *int    `json:"closing_day,omitempty"`
	Description         *string `json:"description,omitempty"`
	UpdateInvoicesSince *string `json:"update_invoices_since,omitempty"`
}
