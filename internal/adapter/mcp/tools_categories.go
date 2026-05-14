package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type CategoryService interface {
	List(ctx context.Context) ([]domain.Category, error)
	Get(ctx context.Context, id int64) (*domain.Category, error)
}

type ListCategoriesOutput struct {
	Categories []domain.Category `json:"categories"`
}

type GetCategoryInput struct {
	ID int64 `json:"id" jsonschema:"The numeric Organizze category id."`
}

type GetCategoryOutput struct {
	Category domain.Category `json:"category"`
}

func listCategoriesHandler(svc CategoryService) mcpsdk.ToolHandlerFor[struct{}, ListCategoriesOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, ListCategoriesOutput, error) {
		cats, err := svc.List(ctx)
		if err != nil {
			return nil, ListCategoriesOutput{}, err
		}
		return nil, ListCategoriesOutput{Categories: cats}, nil
	}
}

func getCategoryHandler(svc CategoryService) mcpsdk.ToolHandlerFor[GetCategoryInput, GetCategoryOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetCategoryInput) (*mcpsdk.CallToolResult, GetCategoryOutput, error) {
		cat, err := svc.Get(ctx, in.ID)
		if err != nil {
			return nil, GetCategoryOutput{}, err
		}
		return nil, GetCategoryOutput{Category: *cat}, nil
	}
}

func registerCategoryTools(s *mcpsdk.Server, svc CategoryService) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_categories",
		Description: "List all Organizze categories.",
	}, listCategoriesHandler(svc))
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_category",
		Description: "Fetch a single Organizze category by id.",
	}, getCategoryHandler(svc))
}
