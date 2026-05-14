package mcp

import (
	"context"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeTransferSvc struct {
	gotFilter domain.ListTransfersFilter
}

func (f *fakeTransferSvc) List(_ context.Context, fl domain.ListTransfersFilter) ([]domain.Transfer, error) {
	f.gotFilter = fl
	return nil, nil
}

type nopTransferSvc struct{}

func (nopTransferSvc) List(context.Context, domain.ListTransfersFilter) ([]domain.Transfer, error) {
	return nil, nil
}

func TestListTransfersHandler_PassesFilter(t *testing.T) {
	svc := &fakeTransferSvc{}
	h := listTransfersHandler(svc)
	_, _, err := h(context.Background(), &mcpsdk.CallToolRequest{}, ListTransfersInput{
		StartDate: "2026-05-01", EndDate: "2026-05-31",
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if svc.gotFilter.StartDate != "2026-05-01" || svc.gotFilter.EndDate != "2026-05-31" {
		t.Errorf("filter = %+v", svc.gotFilter)
	}
}
