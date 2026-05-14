package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type TransferService interface {
	List(ctx context.Context, filter domain.ListTransfersFilter) ([]domain.Transfer, error)
}

type ListTransfersInput struct {
	StartDate string `json:"start_date,omitempty" jsonschema:"Optional YYYY-MM-DD lower bound."`
	EndDate   string `json:"end_date,omitempty"   jsonschema:"Optional YYYY-MM-DD upper bound."`
}

type ListTransfersOutput struct {
	Transfers []domain.Transfer `json:"transfers"`
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

func registerTransferTools(s *mcpsdk.Server, svc TransferService) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_transfers",
		Description: "List Organizze transfers, optionally filtered by date range.",
	}, listTransfersHandler(svc))
}
