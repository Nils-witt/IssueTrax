package port

import (
	"context"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// ServiceAccountService is the driving port for managing service accounts
// ("bot-<name>" users) and their API keys. Changes are made by people: an
// actor that is itself a service account gets domain.ErrSessionRequired.
type ServiceAccountService interface {
	// List returns all service accounts ordered by display name.
	List(ctx context.Context) ([]domain.User, error)
	// Create adds the service account "bot-<name>". A taken name returns
	// domain.ErrServiceAccountNameTaken.
	Create(ctx context.Context, actor domain.User, name string) (domain.User, error)
	// ListKeys returns the keys (without tokens) of a service account,
	// newest first.
	ListKeys(ctx context.Context, account domain.UserID) ([]domain.APIKey, error)
	// CreateKey issues a key for account. The returned token is the only
	// copy; it cannot be retrieved later.
	CreateKey(ctx context.Context, actor domain.User, account domain.UserID, name string, expiresAt *time.Time) (domain.APIKey, string, error)
	// RevokeKey revokes a key of account. Revoking twice is fine.
	RevokeKey(ctx context.Context, actor domain.User, account domain.UserID, key domain.APIKeyID) (domain.APIKey, error)
}

// APIKeyRepository is the driven port for persisting API keys.
//
// Get and GetByHash return domain.ErrAPIKeyNotFound. ListByUser orders by
// creation time, newest first. Update stores LastUsedAt and RevokedAt.
type APIKeyRepository interface {
	Create(ctx context.Context, key domain.APIKey) error
	Get(ctx context.Context, id domain.APIKeyID) (domain.APIKey, error)
	GetByHash(ctx context.Context, tokenHash []byte) (domain.APIKey, error)
	ListByUser(ctx context.Context, user domain.UserID) ([]domain.APIKey, error)
	Update(ctx context.Context, key domain.APIKey) error
}
