package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type TransferService interface {
	List(ctx context.Context, filter domain.ListTransfersFilter) ([]domain.Transfer, error)
	Get(ctx context.Context, id int64) (*domain.Transfer, error)
	Create(ctx context.Context, params domain.CreateTransferParams) (*domain.Transfer, error)
	Update(ctx context.Context, id int64, params domain.UpdateTransferParams) (*domain.Transfer, error)
	Delete(ctx context.Context, id int64) (*domain.Transfer, error)
}

type ListTransfersInput struct {
	StartDate string `json:"start_date,omitempty" jsonschema:"Optional YYYY-MM-DD lower bound."`
	EndDate   string `json:"end_date,omitempty"   jsonschema:"Optional YYYY-MM-DD upper bound."`
}

type ListTransfersOutput struct {
	Transfers []domain.Transfer `json:"transfers"`
}

type GetTransferInput struct {
	ID int64 `json:"id" jsonschema:"The numeric Organizze transfer id."`
}

type GetTransferOutput struct {
	Transfer domain.Transfer `json:"transfer"`
}

type CreateTransferInput struct {
	CreditAccountID int64        `json:"credit_account_id" jsonschema:"Receiving account id (bank account; not a credit card)."`
	DebitAccountID  int64        `json:"debit_account_id"  jsonschema:"Sending account id (bank account; not a credit card)."`
	AmountCents     int64        `json:"amount_cents"      jsonschema:"Transfer amount in cents (non-zero)."`
	Date            string       `json:"date"              jsonschema:"YYYY-MM-DD."`
	Paid            bool         `json:"paid"              jsonschema:"Whether the transfer is already settled."`
	Tags            []domain.Tag `json:"tags,omitempty"    jsonschema:"Optional tags."`
}

type CreateTransferOutput struct {
	Transfer domain.Transfer `json:"transfer"`
}

type UpdateTransferInput struct {
	ID          int64        `json:"id"                  jsonschema:"The numeric Organizze transfer id to update."`
	Description *string      `json:"description,omitempty" jsonschema:"New description."`
	Notes       *string      `json:"notes,omitempty"       jsonschema:"New notes."`
	Tags        []domain.Tag `json:"tags,omitempty"        jsonschema:"Replacement tag list."`
}

type UpdateTransferOutput struct {
	Transfer domain.Transfer `json:"transfer"`
}

type DeleteTransferInput struct {
	ID int64 `json:"id" jsonschema:"The numeric Organizze transfer id to delete."`
}

type DeleteTransferOutput struct {
	Deleted  bool             `json:"deleted"`
	ID       int64            `json:"id"`
	Transfer *domain.Transfer `json:"transfer,omitempty"`
}

func listTransfersHandler(svc TransferService) mcpsdk.ToolHandlerFor[ListTransfersInput, ListTransfersOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in ListTransfersInput) (*mcpsdk.CallToolResult, ListTransfersOutput, error) {
		ts, err := svc.List(ctx, domain.ListTransfersFilter{StartDate: in.StartDate, EndDate: in.EndDate})
		if err != nil {
			return nil, ListTransfersOutput{}, err
		}
		return nil, ListTransfersOutput{Transfers: ts}, nil
	}
}

func getTransferHandler(svc TransferService) mcpsdk.ToolHandlerFor[GetTransferInput, GetTransferOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetTransferInput) (*mcpsdk.CallToolResult, GetTransferOutput, error) {
		tr, err := svc.Get(ctx, in.ID)
		if err != nil {
			return nil, GetTransferOutput{}, err
		}
		return nil, GetTransferOutput{Transfer: *tr}, nil
	}
}

func createTransferHandler(svc TransferService) mcpsdk.ToolHandlerFor[CreateTransferInput, CreateTransferOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in CreateTransferInput) (*mcpsdk.CallToolResult, CreateTransferOutput, error) {
		tr, err := svc.Create(ctx, domain.CreateTransferParams{
			CreditAccountID: in.CreditAccountID, DebitAccountID: in.DebitAccountID,
			AmountCents: in.AmountCents, Date: in.Date, Paid: in.Paid, Tags: in.Tags,
		})
		if err != nil {
			return nil, CreateTransferOutput{}, err
		}
		return nil, CreateTransferOutput{Transfer: *tr}, nil
	}
}

func updateTransferHandler(svc TransferService) mcpsdk.ToolHandlerFor[UpdateTransferInput, UpdateTransferOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in UpdateTransferInput) (*mcpsdk.CallToolResult, UpdateTransferOutput, error) {
		tr, err := svc.Update(ctx, in.ID, domain.UpdateTransferParams{
			Description: in.Description, Notes: in.Notes, Tags: in.Tags,
		})
		if err != nil {
			return nil, UpdateTransferOutput{}, err
		}
		return nil, UpdateTransferOutput{Transfer: *tr}, nil
	}
}

func deleteTransferHandler(svc TransferService) mcpsdk.ToolHandlerFor[DeleteTransferInput, DeleteTransferOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in DeleteTransferInput) (*mcpsdk.CallToolResult, DeleteTransferOutput, error) {
		tr, err := svc.Delete(ctx, in.ID)
		if err != nil {
			return nil, DeleteTransferOutput{}, err
		}
		return nil, DeleteTransferOutput{Deleted: true, ID: in.ID, Transfer: tr}, nil
	}
}

func registerTransferTools(s *mcpsdk.Server, inst instrumentation, svc TransferService) {
	addInstrumentedTool(s, inst, &mcpsdk.Tool{
		Name:        "list_transfers",
		Description: "List Organizze transfers, optionally filtered by date range.",
	}, listTransfersHandler(svc))
	addInstrumentedTool(s, inst, &mcpsdk.Tool{
		Name:        "get_transfer",
		Description: "Fetch a single Organizze transfer by id.",
	}, getTransferHandler(svc))
	addInstrumentedTool(s, inst, &mcpsdk.Tool{
		Name:        "create_transfer",
		Description: "Create a new Organizze transfer between two bank accounts. Required: credit_account_id (receiving), debit_account_id (sending), amount_cents, date. Credit cards are NOT accepted as source or destination.",
	}, createTransferHandler(svc))
	addInstrumentedTool(s, inst, &mcpsdk.Tool{
		Name:        "update_transfer",
		Description: "Update fields on an existing Organizze transfer. Only description, notes, and tags can be modified.",
	}, updateTransferHandler(svc))
	addInstrumentedTool(s, inst, &mcpsdk.Tool{
		Name:        "delete_transfer",
		Description: "Permanently delete an Organizze transfer by id.",
	}, deleteTransferHandler(svc))
}
