package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// TransactionService is the consumer-side slice this file needs.
type TransactionService interface {
	List(ctx context.Context, filter domain.ListTransactionsFilter) ([]domain.Transaction, error)
	Get(ctx context.Context, id int64) (*domain.Transaction, error)
	Create(ctx context.Context, params domain.CreateTransactionParams) (*domain.Transaction, error)
	Update(ctx context.Context, id int64, params domain.UpdateTransactionParams) (*domain.Transaction, error)
	Delete(ctx context.Context, id int64, params domain.DeleteTransactionParams) (*domain.Transaction, error)
}

// ---------- list / get ----------

type ListTransactionsInput struct {
	StartDate string `json:"start_date,omitempty" jsonschema:"Optional YYYY-MM-DD lower bound."`
	EndDate   string `json:"end_date,omitempty"   jsonschema:"Optional YYYY-MM-DD upper bound."`
	AccountID int64  `json:"account_id,omitempty" jsonschema:"Optional account id to filter by."`
}

type ListTransactionsOutput struct {
	Transactions []domain.Transaction `json:"transactions"`
}

type GetTransactionInput struct {
	ID int64 `json:"id" jsonschema:"The numeric transaction id."`
}

type GetTransactionOutput struct {
	Transaction domain.Transaction `json:"transaction"`
}

// ---------- create ----------

type CreateTransactionInput struct {
	Description         string             `json:"description" jsonschema:"Short transaction description."`
	Date                string             `json:"date"        jsonschema:"YYYY-MM-DD."`
	AmountCents         int64              `json:"amount_cents" jsonschema:"Cents; negative=expense, positive=income. When installments is set, this is the TOTAL across all installments — Organizze divides evenly. To get a per-installment value X with N installments, send amount_cents = X * N."`
	AccountID           int64              `json:"account_id,omitempty" jsonschema:"Bank account id. Required for a bank-account transaction; MUST be omitted when billing to a credit card. account_id and credit_card_id are mutually exclusive — Organizze silently drops credit_card_id when account_id is also set."`
	CategoryID          int64              `json:"category_id"  jsonschema:"Category id."`
	Paid                bool               `json:"paid"         jsonschema:"Whether the transaction is already paid."`
	Notes               string             `json:"notes,omitempty"      jsonschema:"Optional notes."`
	ContactID           *int64             `json:"contact_id,omitempty" jsonschema:"Optional contact id."`
	Tags                []domain.Tag       `json:"tags,omitempty"       jsonschema:"Optional tags."`
	CreditCardID        *int64             `json:"credit_card_id,omitempty"         jsonschema:"Bill this transaction to a credit card by ID. Required for credit-card transactions; MUST NOT be combined with account_id (Organizze silently ignores credit_card_id when account_id is present)."`
	CreditCardInvoiceID *int64             `json:"credit_card_invoice_id,omitempty" jsonschema:"Pin this transaction to a specific credit-card invoice. Requires credit_card_id; omit to let Organizze auto-pick the current invoice."`
	Recurrence          *RecurrenceInput   `json:"recurrence,omitempty"   jsonschema:"Optional. Set to create a fixed recurring transaction (recurrence_attributes). Mutually exclusive with installments."`
	Installments        *InstallmentsInput `json:"installments,omitempty" jsonschema:"Optional. Set to create an installment-plan transaction (installments_attributes). Mutually exclusive with recurrence."`
}

// RecurrenceInput selects the cadence for a fixed recurring transaction.
type RecurrenceInput struct {
	Periodicity string `json:"periodicity" jsonschema:"One of: weekly, biweekly, monthly, bimonthly, trimonthly, yearly."`
}

// InstallmentsInput selects an installment plan for a parcelada create.
type InstallmentsInput struct {
	Periodicity string `json:"periodicity" jsonschema:"One of: weekly, biweekly, monthly, bimonthly, trimonthly, yearly."`
	Total       int    `json:"total"       jsonschema:"Total number of installments (>=1)."`
}

type CreateTransactionOutput struct {
	Transaction domain.Transaction `json:"transaction"`
}

// ---------- update ----------

type UpdateTransactionInput struct {
	ID                  int64        `json:"id" jsonschema:"The numeric transaction id to update."`
	Description         *string      `json:"description,omitempty"  jsonschema:"New description."`
	Date                *string      `json:"date,omitempty"         jsonschema:"New date YYYY-MM-DD."`
	AmountCents         *int64       `json:"amount_cents,omitempty" jsonschema:"New amount in cents."`
	AccountID           *int64       `json:"account_id,omitempty"   jsonschema:"New bank account id. MUST NOT be combined with credit_card_id — Organizze silently drops credit_card_id (and credit_card_invoice_id) when account_id is also present in the PUT body. To move a transaction to a credit card, send only credit_card_id."`
	CategoryID          *int64       `json:"category_id,omitempty"  jsonschema:"New category id."`
	Paid                *bool        `json:"paid,omitempty"         jsonschema:"New paid flag."`
	Notes               *string      `json:"notes,omitempty"        jsonschema:"New notes."`
	ContactID           *int64       `json:"contact_id,omitempty"   jsonschema:"New contact id."`
	Tags                []domain.Tag `json:"tags,omitempty"         jsonschema:"Replacement tag list."`
	CreditCardID        *int64       `json:"credit_card_id,omitempty"         jsonschema:"Move this transaction to a credit card by ID. MUST NOT be combined with account_id (Organizze silently ignores credit_card_id when account_id is present)."`
	CreditCardInvoiceID *int64       `json:"credit_card_invoice_id,omitempty" jsonschema:"Pin this transaction to a specific credit-card invoice. Requires credit_card_id; omit to let Organizze auto-pick the current invoice."`
	UpdateFuture        *bool        `json:"update_future,omitempty" jsonschema:"For recurring/installment series: also apply this update to the current and all FUTURE occurrences."`
	UpdateAll           *bool        `json:"update_all,omitempty"    jsonschema:"For recurring/installment series: also apply this update to ALL occurrences, including past ones. May alter the account balance if past entries were already paid."`
}

type UpdateTransactionOutput struct {
	Transaction domain.Transaction `json:"transaction"`
}

// ---------- delete ----------

type DeleteTransactionInput struct {
	ID           int64 `json:"id" jsonschema:"The numeric transaction id to delete."`
	UpdateFuture *bool `json:"update_future,omitempty" jsonschema:"For recurring/installment series: also delete the current and all FUTURE occurrences. Mutually exclusive with update_all."`
	UpdateAll    *bool `json:"update_all,omitempty"    jsonschema:"For recurring/installment series: delete ALL occurrences, including past ones. May alter the account balance if past entries were paid. Mutually exclusive with update_future."`
}

type DeleteTransactionOutput struct {
	Deleted     bool                `json:"deleted"`
	ID          int64               `json:"id"`
	Transaction *domain.Transaction `json:"transaction,omitempty"`
}

// ---------- handlers ----------

func listTransactionsHandler(svc TransactionService) mcpsdk.ToolHandlerFor[ListTransactionsInput, ListTransactionsOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in ListTransactionsInput) (*mcpsdk.CallToolResult, ListTransactionsOutput, error) {
		tx, err := svc.List(ctx, domain.ListTransactionsFilter{
			StartDate: in.StartDate, EndDate: in.EndDate, AccountID: in.AccountID,
		})
		if err != nil {
			return nil, ListTransactionsOutput{}, err
		}
		return nil, ListTransactionsOutput{Transactions: tx}, nil
	}
}

func getTransactionHandler(svc TransactionService) mcpsdk.ToolHandlerFor[GetTransactionInput, GetTransactionOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetTransactionInput) (*mcpsdk.CallToolResult, GetTransactionOutput, error) {
		tx, err := svc.Get(ctx, in.ID)
		if err != nil {
			return nil, GetTransactionOutput{}, err
		}
		return nil, GetTransactionOutput{Transaction: *tx}, nil
	}
}

func createTransactionHandler(svc TransactionService) mcpsdk.ToolHandlerFor[CreateTransactionInput, CreateTransactionOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in CreateTransactionInput) (*mcpsdk.CallToolResult, CreateTransactionOutput, error) {
		params := domain.CreateTransactionParams{
			Description: in.Description, Date: in.Date, AmountCents: in.AmountCents,
			AccountID: in.AccountID, CategoryID: in.CategoryID, Paid: in.Paid,
			Notes: in.Notes, ContactID: in.ContactID, Tags: in.Tags,
			CreditCardID:        in.CreditCardID,
			CreditCardInvoiceID: in.CreditCardInvoiceID,
		}
		if in.Recurrence != nil {
			params.Recurrence = &domain.RecurrenceAttributes{
				Periodicity: domain.Periodicity(in.Recurrence.Periodicity),
			}
		}
		if in.Installments != nil {
			params.Installments = &domain.InstallmentsAttributes{
				Periodicity: domain.Periodicity(in.Installments.Periodicity),
				Total:       in.Installments.Total,
			}
		}
		tx, err := svc.Create(ctx, params)
		if err != nil {
			return nil, CreateTransactionOutput{}, err
		}
		return nil, CreateTransactionOutput{Transaction: *tx}, nil
	}
}

func updateTransactionHandler(svc TransactionService) mcpsdk.ToolHandlerFor[UpdateTransactionInput, UpdateTransactionOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in UpdateTransactionInput) (*mcpsdk.CallToolResult, UpdateTransactionOutput, error) {
		tx, err := svc.Update(ctx, in.ID, domain.UpdateTransactionParams{
			Description: in.Description, Date: in.Date, AmountCents: in.AmountCents,
			AccountID: in.AccountID, CategoryID: in.CategoryID, Paid: in.Paid,
			Notes: in.Notes, ContactID: in.ContactID, Tags: in.Tags,
			CreditCardID:        in.CreditCardID,
			CreditCardInvoiceID: in.CreditCardInvoiceID,
			UpdateFuture:        in.UpdateFuture,
			UpdateAll:           in.UpdateAll,
		})
		if err != nil {
			return nil, UpdateTransactionOutput{}, err
		}
		return nil, UpdateTransactionOutput{Transaction: *tx}, nil
	}
}

func deleteTransactionHandler(svc TransactionService) mcpsdk.ToolHandlerFor[DeleteTransactionInput, DeleteTransactionOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in DeleteTransactionInput) (*mcpsdk.CallToolResult, DeleteTransactionOutput, error) {
		tx, err := svc.Delete(ctx, in.ID, domain.DeleteTransactionParams{
			UpdateFuture: in.UpdateFuture, UpdateAll: in.UpdateAll,
		})
		if err != nil {
			return nil, DeleteTransactionOutput{}, err
		}
		return nil, DeleteTransactionOutput{Deleted: true, ID: in.ID, Transaction: tx}, nil
	}
}

func registerTransactionTools(s *mcpsdk.Server, svc TransactionService) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_transactions",
		Description: "List Organizze transactions. Filters: start_date, end_date (YYYY-MM-DD), account_id. amount_cents is negative for expenses, positive for income.",
	}, listTransactionsHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_transaction",
		Description: "Fetch a single Organizze transaction by id.",
	}, getTransactionHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "create_transaction",
		Description: "Create a new Organizze transaction. amount_cents is in cents (negative for expenses, positive for income). " +
			"Account routing: for a BANK transaction set `account_id`; for a CREDIT-CARD transaction set `credit_card_id` (optionally pinned to a specific invoice via `credit_card_invoice_id`). Exactly one of `account_id` or `credit_card_id` must be set — if both are sent, Organizze silently drops `credit_card_id` and the transaction lands on the bank account. " +
			"For a fixed recurring transaction, pass `recurrence` with a `periodicity` (weekly, biweekly, monthly, bimonthly, trimonthly, yearly). " +
			"For a parcelada (installment) transaction, pass `installments` with `periodicity` and `total`; IMPORTANT: when `installments` is set, Organizze treats `amount_cents` as the TOTAL across all installments and divides evenly, so each generated installment will be amount_cents/total. To get per-installment value X with N installments, send amount_cents = X * N. " +
			"`recurrence` and `installments` are mutually exclusive.",
	}, createTransactionHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "update_transaction",
		Description: "Update fields on an existing Organizze transaction. Only fields you provide are changed; omitted fields are left unchanged. " +
			"Account routing: to keep the transaction on the same bearer, omit both account_id and credit_card_id. To move it to a different bank account, set only account_id. To move it to a credit card, set only credit_card_id (optionally pinned to a specific invoice via credit_card_invoice_id; omit invoice to let Organizze auto-pick). account_id and credit_card_id are mutually exclusive — if both are sent, Organizze silently drops credit_card_id (and credit_card_invoice_id) and the transaction stays on / moves to the bank account. " +
			"For recurring (fixa) or installment (parcelada) series, set update_future=true to propagate the change to this and all future occurrences, or update_all=true to propagate to every occurrence (may alter past-paid balances).",
	}, updateTransactionHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "delete_transaction",
		Description: "Permanently delete an Organizze transaction by id. For recurring (fixa) or installment (parcelada) series, set update_future=true to also delete this and all future occurrences, or update_all=true to delete every occurrence (may alter past-paid balances). The two flags are mutually exclusive.",
	}, deleteTransactionHandler(svc))
}
