package domain

// Category is an expense/income category.
type Category struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color,omitempty"`
	ParentID *int64 `json:"parent_id,omitempty"`
}
