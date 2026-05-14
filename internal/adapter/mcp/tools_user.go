package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// UserService is the consumer-side slice this file needs from usecase.UserService.
type UserService interface {
	Get(ctx context.Context, id int64) (*domain.User, error)
}

type GetUserInput struct {
	ID int64 `json:"id" jsonschema:"The numeric Organizze user id to fetch."`
}

type GetUserOutput struct {
	User domain.User `json:"user"`
}

func getUserHandler(svc UserService) mcpsdk.ToolHandlerFor[GetUserInput, GetUserOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetUserInput) (*mcpsdk.CallToolResult, GetUserOutput, error) {
		u, err := svc.Get(ctx, in.ID)
		if err != nil {
			return nil, GetUserOutput{}, err
		}
		return nil, GetUserOutput{User: *u}, nil
	}
}

func registerUserTools(s *mcpsdk.Server, svc UserService) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_user",
		Description: "Fetch details for an Organizze user by numeric id.",
	}, getUserHandler(svc))
}
