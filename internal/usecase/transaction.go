package usecase

import (
	"context"
	"fmt"

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

func validateCreate(p domain.CreateTransactionParams) error {
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
	// Account routing: bank account vs credit card. Organizze silently drops
	// credit_card_id when account_id is also set, so we reject the ambiguous
	// shape up-front rather than let the transaction land on the wrong account.
	switch {
	case p.AccountID == 0 && p.CreditCardID == nil:
		return fmt.Errorf("%w: exactly one of account_id or credit_card_id is required (bank transaction vs credit-card transaction)", domain.ErrValidation)
	case p.AccountID != 0 && p.CreditCardID != nil:
		return fmt.Errorf("%w: account_id and credit_card_id are mutually exclusive — Organizze silently drops credit_card_id when account_id is also set. To bill a credit card, pass only credit_card_id (optionally with credit_card_invoice_id)", domain.ErrValidation)
	case p.CreditCardInvoiceID != nil && p.CreditCardID == nil:
		return fmt.Errorf("%w: credit_card_invoice_id requires credit_card_id", domain.ErrValidation)
	}
	if p.Recurrence != nil && p.Installments != nil {
		return fmt.Errorf("%w: recurrence_attributes and installments_attributes are mutually exclusive", domain.ErrValidation)
	}
	if p.Recurrence != nil && !p.Recurrence.Periodicity.Valid() {
		return fmt.Errorf("%w: recurrence.periodicity must be one of %v", domain.ErrValidation, domain.AllPeriodicities)
	}
	if p.Installments != nil {
		if !p.Installments.Periodicity.Valid() {
			return fmt.Errorf("%w: installments.periodicity must be one of %v", domain.ErrValidation, domain.AllPeriodicities)
		}
		if p.Installments.Total <= 0 {
			return fmt.Errorf("%w: installments.total must be > 0", domain.ErrValidation)
		}
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
