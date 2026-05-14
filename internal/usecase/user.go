// Package usecase contains application services and the repository interfaces
// they consume. Repositories are defined here (the consumer) and implemented
// by outer-layer packages; Go's implicit interface satisfaction handles the
// inversion automatically.
package usecase

import (
	"context"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// UserRepository is the consumer-owned port for User reads.
type UserRepository interface {
	Get(ctx context.Context, id int64) (*domain.User, error)
}

// UserService exposes user operations to outer layers.
type UserService struct {
	repo UserRepository
}

func NewUserService(repo UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) Get(ctx context.Context, id int64) (*domain.User, error) {
	return s.repo.Get(ctx, id)
}
