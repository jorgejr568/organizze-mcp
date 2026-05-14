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
	Delete(ctx context.Context, id int64) error
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
	Description string       `json:"description" jsonschema:"Short transaction description."`
	Date        string       `json:"date"        jsonschema:"YYYY-MM-DD."`
	AmountCents int64        `json:"amount_cents" jsonschema:"Cents; negative=expense, positive=income."`
	AccountID   int64        `json:"account_id"   jsonschema:"Source account id."`
	CategoryID  int64        `json:"category_id"  jsonschema:"Category id."`
	Paid        bool         `json:"paid"         jsonschema:"Whether the transaction is already paid."`
	Notes       string       `json:"notes,omitempty"      jsonschema:"Optional notes."`
	ContactID   *int64       `json:"contact_id,omitempty" jsonschema:"Optional contact id."`
	Tags        []domain.Tag `json:"tags,omitempty"      jsonschema:"Optional tags."`
}

type CreateTransactionOutput struct {
	Transaction domain.Transaction `json:"transaction"`
}

// ---------- update ----------

type UpdateTransactionInput struct {
	ID          int64        `json:"id" jsonschema:"The numeric transaction id to update."`
	Description *string      `json:"description,omitempty"  jsonschema:"New description."`
	Date        *string      `json:"date,omitempty"         jsonschema:"New date YYYY-MM-DD."`
	AmountCents *int64       `json:"amount_cents,omitempty" jsonschema:"New amount in cents."`
	AccountID   *int64       `json:"account_id,omitempty"   jsonschema:"New account id."`
	CategoryID  *int64       `json:"category_id,omitempty"  jsonschema:"New category id."`
	Paid        *bool        `json:"paid,omitempty"         jsonschema:"New paid flag."`
	Notes       *string      `json:"notes,omitempty"        jsonschema:"New notes."`
	ContactID   *int64       `json:"contact_id,omitempty"   jsonschema:"New contact id."`
	Tags        []domain.Tag `json:"tags,omitempty"         jsonschema:"Replacement tag list."`
}

type UpdateTransactionOutput struct {
	Transaction domain.Transaction `json:"transaction"`
}

// ---------- delete ----------

type DeleteTransactionInput struct {
	ID int64 `json:"id" jsonschema:"The numeric transaction id to delete."`
}

type DeleteTransactionOutput struct {
	Deleted bool  `json:"deleted"`
	ID      int64 `json:"id"`
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
		tx, err := svc.Create(ctx, domain.CreateTransactionParams{
			Description: in.Description, Date: in.Date, AmountCents: in.AmountCents,
			AccountID: in.AccountID, CategoryID: in.CategoryID, Paid: in.Paid,
			Notes: in.Notes, ContactID: in.ContactID, Tags: in.Tags,
		})
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
		})
		if err != nil {
			return nil, UpdateTransactionOutput{}, err
		}
		return nil, UpdateTransactionOutput{Transaction: *tx}, nil
	}
}

func deleteTransactionHandler(svc TransactionService) mcpsdk.ToolHandlerFor[DeleteTransactionInput, DeleteTransactionOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in DeleteTransactionInput) (*mcpsdk.CallToolResult, DeleteTransactionOutput, error) {
		if err := svc.Delete(ctx, in.ID); err != nil {
			return nil, DeleteTransactionOutput{}, err
		}
		return nil, DeleteTransactionOutput{Deleted: true, ID: in.ID}, nil
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
		Name:        "create_transaction",
		Description: "Create a new Organizze transaction. amount_cents is negative for expenses, positive for income.",
	}, createTransactionHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "update_transaction",
		Description: "Update fields on an existing Organizze transaction. Only fields you provide are changed; omitted fields are left unchanged (not cleared). To clear notes, pass an empty string; to replace tags, pass the full new tag list (omitting tags leaves them alone, but passing an empty array clears them).",
	}, updateTransactionHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "delete_transaction",
		Description: "Permanently delete an Organizze transaction by id. There is no soft-delete; the row is gone after this call returns successfully. Calling delete on an already-deleted id returns a not-found error rather than re-deleting.",
	}, deleteTransactionHandler(svc))
}
