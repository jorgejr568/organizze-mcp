package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type CreditCardService interface {
	List(ctx context.Context) ([]domain.CreditCard, error)
	Get(ctx context.Context, id int64) (*domain.CreditCard, error)
}

type ListCreditCardsOutput struct {
	CreditCards []domain.CreditCard `json:"credit_cards"`
}

type GetCreditCardInput struct {
	ID int64 `json:"id" jsonschema:"The numeric credit card id."`
}

type GetCreditCardOutput struct {
	CreditCard domain.CreditCard `json:"credit_card"`
}

func listCreditCardsHandler(svc CreditCardService) mcpsdk.ToolHandlerFor[struct{}, ListCreditCardsOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, ListCreditCardsOutput, error) {
		cards, err := svc.List(ctx)
		if err != nil {
			return nil, ListCreditCardsOutput{}, err
		}
		return nil, ListCreditCardsOutput{CreditCards: cards}, nil
	}
}

func getCreditCardHandler(svc CreditCardService) mcpsdk.ToolHandlerFor[GetCreditCardInput, GetCreditCardOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetCreditCardInput) (*mcpsdk.CallToolResult, GetCreditCardOutput, error) {
		card, err := svc.Get(ctx, in.ID)
		if err != nil {
			return nil, GetCreditCardOutput{}, err
		}
		return nil, GetCreditCardOutput{CreditCard: *card}, nil
	}
}

func registerCreditCardTools(s *mcpsdk.Server, svc CreditCardService) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_credit_cards",
		Description: "List all Organizze credit cards.",
	}, listCreditCardsHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_credit_card",
		Description: "Fetch a single Organizze credit card by id.",
	}, getCreditCardHandler(svc))
}
