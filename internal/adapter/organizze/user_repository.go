package organizze

import (
	"context"
	"fmt"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// UserRepository fetches Organizze users.
type UserRepository struct {
	exec *RequestExecutor
}

// NewUserRepository constructs a UserRepository. The returned value satisfies
// usecase.UserRepository implicitly.
func NewUserRepository(exec *RequestExecutor) *UserRepository {
	return &UserRepository{exec: exec}
}

// Get returns the user with the given id.
func (r *UserRepository) Get(ctx context.Context, id int64) (*domain.User, error) {
	var u domain.User
	if err := r.exec.Get(ctx, fmt.Sprintf("/users/%d", id), &u); err != nil {
		return nil, err
	}
	return &u, nil
}
