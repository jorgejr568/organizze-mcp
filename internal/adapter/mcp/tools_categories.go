package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type CategoryService interface {
	List(ctx context.Context) ([]domain.Category, error)
	Get(ctx context.Context, id int64) (*domain.Category, error)
	Create(ctx context.Context, params domain.CreateCategoryParams) (*domain.Category, error)
	Update(ctx context.Context, id int64, params domain.UpdateCategoryParams) (*domain.Category, error)
	Delete(ctx context.Context, id int64, replacementID *int64) (*domain.Category, error)
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

type CreateCategoryInput struct {
	Name     string `json:"name"                jsonschema:"Category name."`
	Color    string `json:"color,omitempty"     jsonschema:"Optional hex color (e.g. #abcdef)."`
	ParentID *int64 `json:"parent_id,omitempty" jsonschema:"Optional parent category id."`
}

type CreateCategoryOutput struct {
	Category domain.Category `json:"category"`
}

type UpdateCategoryInput struct {
	ID       int64   `json:"id"                 jsonschema:"The numeric Organizze category id to update."`
	Name     *string `json:"name,omitempty"     jsonschema:"New category name."`
	Color    *string `json:"color,omitempty"    jsonschema:"New hex color."`
	ParentID *int64  `json:"parent_id,omitempty" jsonschema:"New parent category id."`
}

type UpdateCategoryOutput struct {
	Category domain.Category `json:"category"`
}

type DeleteCategoryInput struct {
	ID            int64  `json:"id" jsonschema:"The numeric Organizze category id to delete."`
	ReplacementID *int64 `json:"replacement_id,omitempty" jsonschema:"Optional category id to reassign affected transactions to."`
}

type DeleteCategoryOutput struct {
	Deleted  bool             `json:"deleted"`
	ID       int64            `json:"id"`
	Category *domain.Category `json:"category,omitempty"`
}

func listCategoriesHandler(svc CategoryService) mcpsdk.ToolHandlerFor[struct{}, ListCategoriesOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, ListCategoriesOutput, error) {
		cs, err := svc.List(ctx)
		if err != nil {
			return nil, ListCategoriesOutput{}, err
		}
		return nil, ListCategoriesOutput{Categories: cs}, nil
	}
}

func getCategoryHandler(svc CategoryService) mcpsdk.ToolHandlerFor[GetCategoryInput, GetCategoryOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetCategoryInput) (*mcpsdk.CallToolResult, GetCategoryOutput, error) {
		c, err := svc.Get(ctx, in.ID)
		if err != nil {
			return nil, GetCategoryOutput{}, err
		}
		return nil, GetCategoryOutput{Category: *c}, nil
	}
}

func createCategoryHandler(svc CategoryService) mcpsdk.ToolHandlerFor[CreateCategoryInput, CreateCategoryOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in CreateCategoryInput) (*mcpsdk.CallToolResult, CreateCategoryOutput, error) {
		c, err := svc.Create(ctx, domain.CreateCategoryParams{Name: in.Name, Color: in.Color, ParentID: in.ParentID})
		if err != nil {
			return nil, CreateCategoryOutput{}, err
		}
		return nil, CreateCategoryOutput{Category: *c}, nil
	}
}

func updateCategoryHandler(svc CategoryService) mcpsdk.ToolHandlerFor[UpdateCategoryInput, UpdateCategoryOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in UpdateCategoryInput) (*mcpsdk.CallToolResult, UpdateCategoryOutput, error) {
		c, err := svc.Update(ctx, in.ID, domain.UpdateCategoryParams{Name: in.Name, Color: in.Color, ParentID: in.ParentID})
		if err != nil {
			return nil, UpdateCategoryOutput{}, err
		}
		return nil, UpdateCategoryOutput{Category: *c}, nil
	}
}

func deleteCategoryHandler(svc CategoryService) mcpsdk.ToolHandlerFor[DeleteCategoryInput, DeleteCategoryOutput] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in DeleteCategoryInput) (*mcpsdk.CallToolResult, DeleteCategoryOutput, error) {
		c, err := svc.Delete(ctx, in.ID, in.ReplacementID)
		if err != nil {
			return nil, DeleteCategoryOutput{}, err
		}
		return nil, DeleteCategoryOutput{Deleted: true, ID: in.ID, Category: c}, nil
	}
}

func registerCategoryTools(s *mcpsdk.Server, inst instrumentation, svc CategoryService) {
	addInstrumentedTool(s, inst, &mcpsdk.Tool{
		Name:        "list_categories",
		Description: "List all Organizze categories.",
	}, listCategoriesHandler(svc))
	addInstrumentedTool(s, inst, &mcpsdk.Tool{
		Name:        "get_category",
		Description: "Fetch a single Organizze category by id.",
	}, getCategoryHandler(svc))
	addInstrumentedTool(s, inst, &mcpsdk.Tool{
		Name:        "create_category",
		Description: "Create a new Organizze category. Required: name. Color and parent_id are optional.",
	}, createCategoryHandler(svc))
	addInstrumentedTool(s, inst, &mcpsdk.Tool{
		Name:        "update_category",
		Description: "Update fields on an existing Organizze category. Only fields you provide are changed.",
	}, updateCategoryHandler(svc))
	addInstrumentedTool(s, inst, &mcpsdk.Tool{
		Name:        "delete_category",
		Description: "Permanently delete an Organizze category by id. Optionally pass replacement_id (numeric id of another category) to reassign affected transactions to that category — without this, Organizze falls back to the default category. The deleted category snapshot is returned in the 'category' field when the API provides one.",
	}, deleteCategoryHandler(svc))
}
