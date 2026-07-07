package usecase

import (
	"context"
	"fmt"
	"sync"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// TransactionReader is the read-only slice of TransactionRepository.
type TransactionReader interface {
	List(ctx context.Context, filter domain.ListTransactionsFilter) ([]domain.Transaction, error)
	Get(ctx context.Context, id int64) (*domain.Transaction, error)
}

// TransactionWriter is the mutating slice of TransactionRepository.
type TransactionWriter interface {
	Create(ctx context.Context, params domain.CreateTransactionParams) (*domain.Transaction, error)
	Update(ctx context.Context, id int64, params domain.UpdateTransactionParams) (*domain.Transaction, error)
	Delete(ctx context.Context, id int64, params domain.DeleteTransactionParams) (*domain.Transaction, error)
}

// TransactionRepository composes reader and writer for callers that need both.
type TransactionRepository interface {
	TransactionReader
	TransactionWriter
}

// TransactionService orchestrates transaction operations. Create validates that
// the four required Organizze fields are present before hitting the repo.
type TransactionService struct {
	repo TransactionRepository
}

func NewTransactionService(repo TransactionRepository) *TransactionService {
	return &TransactionService{repo: repo}
}

func (s *TransactionService) List(ctx context.Context, filter domain.ListTransactionsFilter) ([]domain.Transaction, error) {
	return s.repo.List(ctx, filter)
}

func (s *TransactionService) Get(ctx context.Context, id int64) (*domain.Transaction, error) {
	return s.repo.Get(ctx, id)
}

func (s *TransactionService) Create(ctx context.Context, p domain.CreateTransactionParams) (*domain.Transaction, error) {
	if err := validateCreate(p); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, p)
}

// batchConcurrency bounds how many item requests are in flight at once
// during batch operations. Organizze has no batch endpoint and rate-limits, so we
// fan out with a small fixed worker pool rather than all-at-once.
const batchConcurrency = 5

// CreateBatch creates up to domain.MaxBatchCreateTransactions transactions,
// each via the same validated single-create path as Create. It is best-effort:
// every item is attempted, and per-item validation or API failures (including
// domain.ErrRateLimited) land in that item's BatchCreateResult.Err rather than
// aborting the batch. The returned slice is index-aligned with items regardless
// of the order goroutines finish in. A non-nil top-level error is returned only
// when the batch itself is invalid (empty or over the cap); in that case no
// item is attempted.
func (s *TransactionService) CreateBatch(ctx context.Context, items []domain.CreateTransactionParams) ([]domain.BatchCreateResult, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: at least one transaction is required", domain.ErrValidation)
	}
	if len(items) > domain.MaxBatchCreateTransactions {
		return nil, fmt.Errorf("%w: at most %d transactions per batch, got %d", domain.ErrValidation, domain.MaxBatchCreateTransactions, len(items))
	}

	results := make([]domain.BatchCreateResult, len(items))
	sem := make(chan struct{}, batchConcurrency)
	var wg sync.WaitGroup
	for i := range items {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			// Each goroutine writes only results[i]; no shared mutation, no mutex.
			tx, err := s.Create(ctx, items[i])
			results[i] = domain.BatchCreateResult{Index: i, Transaction: tx, Err: err}
		}(i)
	}
	wg.Wait()
	return results, nil
}

func (s *TransactionService) Update(ctx context.Context, id int64, p domain.UpdateTransactionParams) (*domain.Transaction, error) {
	if err := validateUpdate(p); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, id, p)
}

func (s *TransactionService) Delete(ctx context.Context, id int64, p domain.DeleteTransactionParams) (*domain.Transaction, error) {
	if p.UpdateFuture != nil && p.UpdateAll != nil {
		return nil, fmt.Errorf("%w: update_future and update_all are mutually exclusive", domain.ErrValidation)
	}
	return s.repo.Delete(ctx, id, p)
}

func (s *TransactionService) UpdateBatch(ctx context.Context, items []domain.UpdateTransactionBatchItem) ([]domain.BatchUpdateResult, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: at least one transaction is required", domain.ErrValidation)
	}
	if len(items) > domain.MaxBatchUpdateTransactions {
		return nil, fmt.Errorf("%w: at most %d transactions per batch, got %d", domain.ErrValidation, domain.MaxBatchUpdateTransactions, len(items))
	}

	results := make([]domain.BatchUpdateResult, len(items))
	sem := make(chan struct{}, batchConcurrency)
	var wg sync.WaitGroup
	for i := range items {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			tx, err := s.Update(ctx, items[i].ID, items[i].Params)
			results[i] = domain.BatchUpdateResult{Index: i, Transaction: tx, Err: err}
		}(i)
	}
	wg.Wait()
	return results, nil
}

func (s *TransactionService) DeleteBatch(ctx context.Context, items []domain.DeleteTransactionBatchItem) ([]domain.BatchDeleteResult, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: at least one transaction is required", domain.ErrValidation)
	}
	if len(items) > domain.MaxBatchDeleteTransactions {
		return nil, fmt.Errorf("%w: at most %d transactions per batch, got %d", domain.ErrValidation, domain.MaxBatchDeleteTransactions, len(items))
	}

	results := make([]domain.BatchDeleteResult, len(items))
	sem := make(chan struct{}, batchConcurrency)
	var wg sync.WaitGroup
	for i := range items {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			tx, err := s.Delete(ctx, items[i].ID, items[i].Params)
			results[i] = domain.BatchDeleteResult{Index: i, Transaction: tx, Err: err}
		}(i)
	}
	wg.Wait()
	return results, nil
}

func validateCreate(p domain.CreateTransactionParams) error {
	checks := []func(domain.CreateTransactionParams) error{
		validateCreateRequiredFields,
		validateCreateAccountRouting,
		validateCreateRecurrenceInstallmentsExclusive,
		validateCreateRecurrence,
		validateCreateInstallments,
	}
	for _, check := range checks {
		if err := check(p); err != nil {
			return err
		}
	}
	return nil
}

func validateCreateRequiredFields(p domain.CreateTransactionParams) error {
	switch {
	case p.Description == "":
		return fmt.Errorf("%w: description is required", domain.ErrValidation)
	case p.Date == "":
		return fmt.Errorf("%w: date is required", domain.ErrValidation)
	case p.AmountCents == 0:
		return fmt.Errorf("%w: amount_cents must be non-zero", domain.ErrValidation)
	case p.CategoryID == 0:
		return fmt.Errorf("%w: category_id is required", domain.ErrValidation)
	}
	return nil
}

// validateCreateAccountRouting enforces bank-account vs credit-card mutual
// exclusion. Organizze silently drops credit_card_id when account_id is also
// set, so we reject the ambiguous shape up-front rather than let the
// transaction land on the wrong account.
func validateCreateAccountRouting(p domain.CreateTransactionParams) error {
	switch {
	case p.AccountID == 0 && p.CreditCardID == nil:
		return fmt.Errorf("%w: exactly one of account_id or credit_card_id is required (bank transaction vs credit-card transaction)", domain.ErrValidation)
	case p.AccountID != 0 && p.CreditCardID != nil:
		return fmt.Errorf("%w: account_id and credit_card_id are mutually exclusive — Organizze silently drops credit_card_id when account_id is also set. To bill a credit card, pass only credit_card_id (optionally with credit_card_invoice_id)", domain.ErrValidation)
	case p.CreditCardInvoiceID != nil && p.CreditCardID == nil:
		return fmt.Errorf("%w: credit_card_invoice_id requires credit_card_id", domain.ErrValidation)
	}
	return nil
}

func validateCreateRecurrenceInstallmentsExclusive(p domain.CreateTransactionParams) error {
	if p.Recurrence != nil && p.Installments != nil {
		return fmt.Errorf("%w: recurrence_attributes and installments_attributes are mutually exclusive", domain.ErrValidation)
	}
	return nil
}

func validateCreateRecurrence(p domain.CreateTransactionParams) error {
	if p.Recurrence == nil {
		return nil
	}
	return validatePeriodicity("recurrence", p.Recurrence.Periodicity)
}

func validateCreateInstallments(p domain.CreateTransactionParams) error {
	if p.Installments == nil {
		return nil
	}
	if err := validatePeriodicity("installments", p.Installments.Periodicity); err != nil {
		return err
	}
	if p.Installments.Total <= 0 {
		return fmt.Errorf("%w: installments.total must be > 0", domain.ErrValidation)
	}
	return nil
}

// validatePeriodicity is the shared periodicity check used by both recurrence
// and installments. label is the field name prefix used in the error message
// ("recurrence" or "installments"), matching the error messages the previous
// inline checks produced.
func validatePeriodicity(label string, period domain.Periodicity) error {
	if !period.Valid() {
		return fmt.Errorf("%w: %s.periodicity must be one of %v", domain.ErrValidation, label, domain.AllPeriodicities)
	}
	return nil
}

// validateUpdate enforces the account-routing rules on PUT /transactions/{id}.
// Organizze silently drops credit_card_id (and credit_card_invoice_id) when
// account_id is also present in the request body — the same trap closed for
// POST in v0.6.1, verified to also apply to PUT in the v0.6.2 audit.
//
// Unlike validateCreate, both AccountID and CreditCardID may be nil (a partial
// update that touches neither field), so the "neither set" branch is allowed.
func validateUpdate(p domain.UpdateTransactionParams) error {
	switch {
	case p.AccountID != nil && p.CreditCardID != nil:
		return fmt.Errorf("%w: account_id and credit_card_id are mutually exclusive on update — Organizze silently drops credit_card_id when account_id is also set. To move a transaction to a credit card, pass only credit_card_id (optionally with credit_card_invoice_id)", domain.ErrValidation)
	case p.CreditCardInvoiceID != nil && p.CreditCardID == nil:
		return fmt.Errorf("%w: credit_card_invoice_id requires credit_card_id on update", domain.ErrValidation)
	}
	return nil
}
