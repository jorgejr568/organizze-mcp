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
