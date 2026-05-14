package domain

// Category is an expense/income category.
type Category struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color,omitempty"`
	ParentID *int64 `json:"parent_id,omitempty"`
}

// CreateCategoryParams are the inputs to CategoryService.Create. The Organizze
// API requires name; color and parent_id are optional.
type CreateCategoryParams struct {
	Name     string `json:"name"`
	Color    string `json:"color,omitempty"`
	ParentID *int64 `json:"parent_id,omitempty"`
}

// UpdateCategoryParams describes a partial update; nil pointers are omitted.
type UpdateCategoryParams struct {
	Name     *string `json:"name,omitempty"`
	Color    *string `json:"color,omitempty"`
	ParentID *int64  `json:"parent_id,omitempty"`
}
