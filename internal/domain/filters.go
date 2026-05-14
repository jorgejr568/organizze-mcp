package domain

// ListTransactionsFilter is the filter for TransactionService.List.
// Zero values are treated as "no filter".
type ListTransactionsFilter struct {
	StartDate string // YYYY-MM-DD
	EndDate   string // YYYY-MM-DD
	AccountID int64
}

// ListTransfersFilter is the filter for TransferService.List.
type ListTransfersFilter struct {
	StartDate string
	EndDate   string
}

// BudgetPeriod selects which budget view to return.
//
//	BudgetPeriod{}                             -> current month
//	BudgetPeriod{Year: 2026}                   -> entire 2026
//	BudgetPeriod{Year: 2026, Month: 5}         -> May 2026
type BudgetPeriod struct {
	Year  int
	Month int // 1..12
}
