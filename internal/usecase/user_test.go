package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

type fakeUserRepo struct {
	gotID int64
	user  *domain.User
	err   error
}

func (f *fakeUserRepo) Get(_ context.Context, id int64) (*domain.User, error) {
	f.gotID = id
	return f.user, f.err
}

func TestUserService_Get_DelegatesToRepo(t *testing.T) {
	repo := &fakeUserRepo{user: &domain.User{ID: 7, Name: "Jorge"}}
	svc := NewUserService(repo)
	got, err := svc.Get(context.Background(), 7)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != 7 || repo.gotID != 7 {
		t.Errorf("svc=%+v repo=%+v", got, repo)
	}
}

func TestUserService_Get_PropagatesError(t *testing.T) {
	want := errors.New("boom")
	repo := &fakeUserRepo{err: want}
	svc := NewUserService(repo)
	if _, err := svc.Get(context.Background(), 1); !errors.Is(err, want) {
		t.Errorf("err = %v, want wraps %v", err, want)
	}
}
