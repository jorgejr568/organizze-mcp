package domain

// Budget is a planned spend for a category in a given period.
type Budget struct {
	AmountInCents  int64  `json:"amount_in_cents"`
	CategoryID     int64  `json:"category_id"`
	Date           string `json:"date"` // YYYY-MM-DD (period start)
	ActivityType   int    `json:"activity_type,omitempty"`
	Total          int64  `json:"total"`
	PredictedTotal int64  `json:"predicted_total"`
	Percentage     string `json:"percentage"`
}
