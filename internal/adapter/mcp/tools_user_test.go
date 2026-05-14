package mcp

import (
	"context"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeUserSvc struct {
	gotID int64
	user  *domain.User
}

func (f *fakeUserSvc) Get(_ context.Context, id int64) (*domain.User, error) {
	f.gotID = id
	return f.user, nil
}

type nopUserSvc struct{}

func (nopUserSvc) Get(context.Context, int64) (*domain.User, error) { return &domain.User{}, nil }

func TestGetUserHandler(t *testing.T) {
	svc := &fakeUserSvc{user: &domain.User{ID: 3, Name: "Jorge"}}
	h := getUserHandler(svc)
	_, out, err := h(context.Background(), &mcpsdk.CallToolRequest{}, GetUserInput{ID: 3})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out.User.ID != 3 || svc.gotID != 3 {
		t.Errorf("out=%+v svc=%+v", out, svc)
	}
}
