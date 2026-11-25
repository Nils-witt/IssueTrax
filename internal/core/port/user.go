package port

import (
	"context"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// UserService is the driving port for user use cases. Users are created by
// logging in, so there is no Create.
type UserService interface {
	List(ctx context.Context) ([]domain.User, error)
}

// UserRepository is the driven port for persisting users.
//
// Get and GetByIdentity return domain.ErrUserNotFound. List orders by
// display name. GetMany ignores unknown IDs.
type UserRepository interface {
	Create(ctx context.Context, user domain.User) error
	Update(ctx context.Context, user domain.User) error
	Get(ctx context.Context, id domain.UserID) (domain.User, error)
	GetByIdentity(ctx context.Context, issuer, subject string) (domain.User, error)
	GetMany(ctx context.Context, ids []domain.UserID) ([]domain.User, error)
	List(ctx context.Context) ([]domain.User, error)
}
