package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type CreditCardService interface {
	List(ctx context.Context) ([]domain.CreditCard, error)
	Get(ctx context.Context, id int64) (*domain.CreditCard, error)
	Create(ctx context.Context, params domain.CreateCreditCardParams) (*domain.CreditCard, error)
	Update(ctx context.Context, id int64, params domain.UpdateCreditCardParams) (*domain.CreditCard, error)
	Delete(ctx context.Context, id int64) (*domain.CreditCard, error)
}

type ListCreditCardsOutput struct {
	CreditCards []domain.CreditCard `json:"credit_cards"`
}

type GetCreditCardInput struct {
	ID int64 `json:"id" jsonschema:"The numeric Organizze credit card id."`
}

type GetCreditCardOutput struct {
	CreditCard domain.CreditCard `json:"credit_card"`
}

type CreateCreditCardInput struct {
	Name        string `json:"name"                  jsonschema:"Credit card name."`
	DueDay      int    `json:"due_day"               jsonschema:"Bill due day (1-31)."`
	ClosingDay  int    `json:"closing_day"           jsonschema:"Statement closing day (1-31)."`
	CardNetwork string `json:"card_network,omitempty" jsonschema:"Optional card network (visa, mastercard, etc.)."`
	LimitCents  int64  `json:"limit_cents,omitempty"  jsonschema:"Optional credit limit in cents."`
	Description string `json:"description,omitempty"  jsonschema:"Optional description."`
	Default     bool   `json:"default,omitempty"      jsonschema:"Mark as the default credit card."`
}

type CreateCreditCardOutput struct {
	CreditCard domain.CreditCard `json:"credit_card"`
}

type UpdateCreditCardInput struct {
	ID                  int64   `json:"id"                              jsonschema:"The numeric Organizze credit card id to update."`
	Name                *string `json:"name,omitempty"                  jsonschema:"New name."`
	DueDay              *int    `json:"due_day,omitempty"               jsonschema:"New due day (1-31)."`
	ClosingDay          *int    `json:"closing_day,omitempty"           jsonschema:"New closing day (1-31)."`
	Description         *string `json:"description,omitempty"           jsonschema:"New description."`
	UpdateInvoicesSince *string `json:"update_invoices_since,omitempty" jsonschema:"If set (YYYY-MM-DD), Organizze retroactively regenerates invoices from this date."`
	LimitCents          *int64  `json:"limit_cents,omitempty"           jsonschema:"New credit limit in cents."`
	CardNetwork         *string `json:"card_network,omitempty"          jsonschema:"New card network (visa, mastercard, hipercard, etc.)."`
	Archived            *bool   `json:"archived,omitempty"              jsonschema:"Archive (true) or unarchive (false) the card."`
	Default             *bool   `json:"default,omitempty"               jsonschema:"Set as default credit card."`
}

type UpdateCreditCardOutput struct {
	CreditCard domain.CreditCard `json:"credit_card"`
}

type DeleteCreditCardInput struct {
	ID int64 `json:"id" jsonschema:"The numeric Organizze credit card id to delete."`
}

type DeleteCreditCardOutput struct {
	Deleted    bool               `json:"deleted"`
	ID         int64              `json:"id"`
	CreditCard *domain.CreditCard `json:"credit_card,omitempty"`
}

func listCreditCardsHandler(svc CreditCardService) mcpsdk.ToolHandlerFor[struct{}, ListCreditCardsOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, ListCreditCardsOutput, error) {
		cc, err := svc.List(ctx)
		if err != nil {
			return nil, ListCreditCardsOutput{}, err
		}
		return nil, ListCreditCardsOutput{CreditCards: cc}, nil
	}
}

func getCreditCardHandler(svc CreditCardService) mcpsdk.ToolHandlerFor[GetCreditCardInput, GetCreditCardOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetCreditCardInput) (*mcpsdk.CallToolResult, GetCreditCardOutput, error) {
		cc, err := svc.Get(ctx, in.ID)
		if err != nil {
			return nil, GetCreditCardOutput{}, err
		}
		return nil, GetCreditCardOutput{CreditCard: *cc}, nil
	}
}

func createCreditCardHandler(svc CreditCardService) mcpsdk.ToolHandlerFor[CreateCreditCardInput, CreateCreditCardOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in CreateCreditCardInput) (*mcpsdk.CallToolResult, CreateCreditCardOutput, error) {
		cc, err := svc.Create(ctx, domain.CreateCreditCardParams{
			Name: in.Name, DueDay: in.DueDay, ClosingDay: in.ClosingDay,
			CardNetwork: in.CardNetwork, LimitCents: in.LimitCents,
			Description: in.Description, Default: in.Default,
		})
		if err != nil {
			return nil, CreateCreditCardOutput{}, err
		}
		return nil, CreateCreditCardOutput{CreditCard: *cc}, nil
	}
}

func updateCreditCardHandler(svc CreditCardService) mcpsdk.ToolHandlerFor[UpdateCreditCardInput, UpdateCreditCardOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in UpdateCreditCardInput) (*mcpsdk.CallToolResult, UpdateCreditCardOutput, error) {
		cc, err := svc.Update(ctx, in.ID, domain.UpdateCreditCardParams{
			Name: in.Name, DueDay: in.DueDay, ClosingDay: in.ClosingDay,
			Description: in.Description, UpdateInvoicesSince: in.UpdateInvoicesSince,
			LimitCents: in.LimitCents, CardNetwork: in.CardNetwork,
			Archived: in.Archived, Default: in.Default,
		})
		if err != nil {
			return nil, UpdateCreditCardOutput{}, err
		}
		return nil, UpdateCreditCardOutput{CreditCard: *cc}, nil
	}
}

func deleteCreditCardHandler(svc CreditCardService) mcpsdk.ToolHandlerFor[DeleteCreditCardInput, DeleteCreditCardOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in DeleteCreditCardInput) (*mcpsdk.CallToolResult, DeleteCreditCardOutput, error) {
		cc, err := svc.Delete(ctx, in.ID)
		if err != nil {
			return nil, DeleteCreditCardOutput{}, err
		}
		return nil, DeleteCreditCardOutput{Deleted: true, ID: in.ID, CreditCard: cc}, nil
	}
}

func registerCreditCardTools(s *mcpsdk.Server, inst instrumentation, svc CreditCardService) {
	addInstrumentedTool(s, inst, &mcpsdk.Tool{
		Name:        "list_credit_cards",
		Description: "List all Organizze credit cards.",
	}, listCreditCardsHandler(svc))
	addInstrumentedTool(s, inst, &mcpsdk.Tool{
		Name:        "get_credit_card",
		Description: "Fetch a single Organizze credit card by id.",
	}, getCreditCardHandler(svc))
	addInstrumentedTool(s, inst, &mcpsdk.Tool{
		Name:        "create_credit_card",
		Description: "Create a new Organizze credit card. Required: name, due_day (1-31), closing_day (1-31).",
	}, createCreditCardHandler(svc))
	addInstrumentedTool(s, inst, &mcpsdk.Tool{
		Name:        "update_credit_card",
		Description: "Update fields on an existing Organizze credit card. Only fields you provide are changed.",
	}, updateCreditCardHandler(svc))
	addInstrumentedTool(s, inst, &mcpsdk.Tool{
		Name:        "delete_credit_card",
		Description: "Permanently delete an Organizze credit card by id.",
	}, deleteCreditCardHandler(svc))
}
