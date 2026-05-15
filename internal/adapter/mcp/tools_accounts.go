package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
	"github.com/jorgejr568/organizze-mcp/internal/stats"
)

type AccountService interface {
	List(ctx context.Context) ([]domain.Account, error)
	Get(ctx context.Context, id int64) (*domain.Account, error)
	Create(ctx context.Context, params domain.CreateAccountParams) (*domain.Account, error)
	Update(ctx context.Context, id int64, params domain.UpdateAccountParams) (*domain.Account, error)
	Delete(ctx context.Context, id int64) (*domain.Account, error)
}

// ---------- list / get ----------

type ListAccountsOutput struct {
	Accounts []domain.Account `json:"accounts"`
}

type GetAccountInput struct {
	ID int64 `json:"id" jsonschema:"The numeric Organizze account id."`
}

type GetAccountOutput struct {
	Account domain.Account `json:"account"`
}

// ---------- create ----------

type CreateAccountInput struct {
	Name        string `json:"name"                  jsonschema:"Account name."`
	Type        string `json:"type"                  jsonschema:"Account type: checking, savings, or other."`
	Description string `json:"description,omitempty" jsonschema:"Optional description."`
	Default     bool   `json:"default,omitempty"     jsonschema:"Whether to mark as the default account."`
}

type CreateAccountOutput struct {
	Account domain.Account `json:"account"`
}

// ---------- update ----------

type UpdateAccountInput struct {
	ID          int64   `json:"id"                    jsonschema:"The numeric Organizze account id to update."`
	Name        *string `json:"name,omitempty"        jsonschema:"New account name."`
	Description *string `json:"description,omitempty" jsonschema:"New description."`
	Default     *bool   `json:"default,omitempty"     jsonschema:"New default flag."`
	Type        *string `json:"type,omitempty"        jsonschema:"New type (checking|savings|other)."`
	Archived    *bool   `json:"archived,omitempty"    jsonschema:"Archive (true) or unarchive (false) the account."`
}

type UpdateAccountOutput struct {
	Account domain.Account `json:"account"`
}

// ---------- delete ----------

type DeleteAccountInput struct {
	ID int64 `json:"id" jsonschema:"The numeric Organizze account id to delete."`
}

type DeleteAccountOutput struct {
	Deleted bool            `json:"deleted"`
	ID      int64           `json:"id"`
	Account *domain.Account `json:"account,omitempty"`
}

// ---------- handlers ----------

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

func createAccountHandler(svc AccountService) mcpsdk.ToolHandlerFor[CreateAccountInput, CreateAccountOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in CreateAccountInput) (*mcpsdk.CallToolResult, CreateAccountOutput, error) {
		a, err := svc.Create(ctx, domain.CreateAccountParams{
			Name: in.Name, Type: in.Type, Description: in.Description, Default: in.Default,
		})
		if err != nil {
			return nil, CreateAccountOutput{}, err
		}
		return nil, CreateAccountOutput{Account: *a}, nil
	}
}

func updateAccountHandler(svc AccountService) mcpsdk.ToolHandlerFor[UpdateAccountInput, UpdateAccountOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in UpdateAccountInput) (*mcpsdk.CallToolResult, UpdateAccountOutput, error) {
		a, err := svc.Update(ctx, in.ID, domain.UpdateAccountParams{
			Name: in.Name, Description: in.Description, Default: in.Default, Type: in.Type, Archived: in.Archived,
		})
		if err != nil {
			return nil, UpdateAccountOutput{}, err
		}
		return nil, UpdateAccountOutput{Account: *a}, nil
	}
}

func deleteAccountHandler(svc AccountService) mcpsdk.ToolHandlerFor[DeleteAccountInput, DeleteAccountOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in DeleteAccountInput) (*mcpsdk.CallToolResult, DeleteAccountOutput, error) {
		a, err := svc.Delete(ctx, in.ID)
		if err != nil {
			return nil, DeleteAccountOutput{}, err
		}
		return nil, DeleteAccountOutput{Deleted: true, ID: in.ID, Account: a}, nil
	}
}

func registerAccountTools(s *mcpsdk.Server, r stats.Reporter, svc AccountService) {
	addInstrumentedTool(s, r, &mcpsdk.Tool{
		Name:        "list_accounts",
		Description: "List all bank/cash accounts in Organizze.",
	}, listAccountsHandler(svc))
	addInstrumentedTool(s, r, &mcpsdk.Tool{
		Name:        "get_account",
		Description: "Fetch a single Organizze account by id.",
	}, getAccountHandler(svc))
	addInstrumentedTool(s, r, &mcpsdk.Tool{
		Name:        "create_account",
		Description: "Create a new Organizze bank/cash account. Required: name, type (checking|savings|other).",
	}, createAccountHandler(svc))
	addInstrumentedTool(s, r, &mcpsdk.Tool{
		Name:        "update_account",
		Description: "Update fields on an existing Organizze account. Only fields you provide are changed. Set archived=true to archive (or false to unarchive).",
	}, updateAccountHandler(svc))
	addInstrumentedTool(s, r, &mcpsdk.Tool{
		Name:        "delete_account",
		Description: "Permanently delete an Organizze account by id.",
	}, deleteAccountHandler(svc))
}
