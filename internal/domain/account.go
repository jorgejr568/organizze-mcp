package domain

import "time"

// Account is a bank or cash account.
type Account struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Type        string    `json:"type"`
	Default     bool      `json:"default"`
	Archived    bool      `json:"archived"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// CreateAccountParams are the inputs to AccountService.Create. The Organizze
// API requires name and type; description and default are optional.
type CreateAccountParams struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Default     bool   `json:"default,omitempty"`
}

// UpdateAccountParams describes a partial update; nil pointers are omitted.
type UpdateAccountParams struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Default     *bool   `json:"default,omitempty"`
	Type        *string `json:"type,omitempty"`
}
