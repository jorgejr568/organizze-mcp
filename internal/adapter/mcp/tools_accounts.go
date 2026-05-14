package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type AccountService interface {
	List(ctx context.Context) ([]domain.Account, error)
	Get(ctx context.Context, id int64) (*domain.Account, error)
}

type ListAccountsOutput struct {
	Accounts []domain.Account `json:"accounts"`
}

type GetAccountInput struct {
	ID int64 `json:"id" jsonschema:"The numeric Organizze account id."`
}

type GetAccountOutput struct {
	Account domain.Account `json:"account"`
}

func listAccountsHandler(svc AccountService) mcpsdk.ToolHandlerFor[struct{}, ListAccountsOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, ListAccountsOutput, error) {
		accounts, err := svc.List(ctx)
		if err != nil {
			return nil, ListAccountsOutput{}, err
		}
		return nil, ListAccountsOutput{Accounts: accounts}, nil
	}
}

func getAccountHandler(svc AccountService) mcpsdk.ToolHandlerFor[GetAccountInput, GetAccountOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetAccountInput) (*mcpsdk.CallToolResult, GetAccountOutput, error) {
		a, err := svc.Get(ctx, in.ID)
		if err != nil {
			return nil, GetAccountOutput{}, err
		}
		return nil, GetAccountOutput{Account: *a}, nil
	}
}

func registerAccountTools(s *mcpsdk.Server, svc AccountService) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_accounts",
		Description: "List all bank/cash accounts in Organizze.",
	}, listAccountsHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_account",
		Description: "Fetch a single Organizze account by id.",
	}, getAccountHandler(svc))
}
