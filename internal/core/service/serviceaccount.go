package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that BotService satisfies the driving port.
var _ port.ServiceAccountService = (*BotService)(nil)

// apiKeyDisplayChars is how many token characters after the "itx_" prefix
// are kept in APIKey.Prefix to help people tell keys apart.
const apiKeyDisplayChars = 8

// BotService implements port.ServiceAccountService: it manages service
// accounts ("bot-<name>" users) and their API keys.
type BotService struct {
	repos Repositories
	opts  options
}

// NewBotService returns a BotService backed by repos.
func NewBotService(repos Repositories, opts ...Option) *BotService {
	return &BotService{repos: repos, opts: newOptions(opts)}
}

// List returns all service accounts ordered by display name.
func (s *BotService) List(ctx context.Context) ([]domain.User, error) {
	users, err := s.repos.Users.ListByIssuer(ctx, domain.ServiceAccountIssuer)
	if err != nil {
		return nil, fmt.Errorf("list service accounts: %w", err)
	}

	return users, nil
}

// Create adds the service account "bot-<name>".
func (s *BotService) Create(ctx context.Context, actor domain.User, name string) (domain.User, error) {
	if actor.IsServiceAccount() {
		return domain.User{}, domain.ErrSessionRequired
	}

	user, err := domain.NewServiceAccount(domain.UserID(s.opts.newID()), name, s.opts.now())
	if err != nil {
		return domain.User{}, err
	}

	err = s.repos.Users.Create(ctx, user)
	if errors.Is(err, domain.ErrUserIdentityTaken) {
		return domain.User{}, domain.ErrServiceAccountNameTaken
	}

	if err != nil {
		return domain.User{}, fmt.Errorf("create service account: %w", err)
	}

	return user, nil
}

// ListKeys returns the keys of a service account, newest first.
func (s *BotService) ListKeys(ctx context.Context, account domain.UserID) ([]domain.APIKey, error) {
	if _, err := s.account(ctx, account); err != nil {
		return nil, err
	}

	keys, err := s.repos.APIKeys.ListByUser(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}

	return keys, nil
}

// CreateKey issues a new key for account and returns it with its token.
func (s *BotService) CreateKey(
	ctx context.Context, actor domain.User, account domain.UserID, name string, expiresAt *time.Time,
) (domain.APIKey, string, error) {
	if actor.IsServiceAccount() {
		return domain.APIKey{}, "", domain.ErrSessionRequired
	}

	if _, err := s.account(ctx, account); err != nil {
		return domain.APIKey{}, "", err
	}

	token := domain.APIKeyTokenPrefix + randomToken()
	prefix := token[:len(domain.APIKeyTokenPrefix)+apiKeyDisplayChars]

	key, err := domain.NewAPIKey(
		domain.APIKeyID(s.opts.newID()), account, name, hashToken(token), prefix, actor.ID, expiresAt, s.opts.now(),
	)
	if err != nil {
		return domain.APIKey{}, "", err
	}

	if err := s.repos.APIKeys.Create(ctx, key); err != nil {
		return domain.APIKey{}, "", fmt.Errorf("create api key: %w", err)
	}

	return key, token, nil
}

// RevokeKey revokes a key of account.
func (s *BotService) RevokeKey(
	ctx context.Context, actor domain.User, account domain.UserID, id domain.APIKeyID,
) (domain.APIKey, error) {
	if actor.IsServiceAccount() {
		return domain.APIKey{}, domain.ErrSessionRequired
	}

	var key domain.APIKey

	err := s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error

		key, err = s.repos.APIKeys.Get(ctx, id)
		if err != nil {
			return fmt.Errorf("get api key: %w", err)
		}

		if key.UserID != account {
			return domain.ErrAPIKeyNotFound
		}

		if key.Revoked() {
			return nil
		}

		key.Revoke(s.opts.now())

		if err := s.repos.APIKeys.Update(ctx, key); err != nil {
			return fmt.Errorf("revoke api key: %w", err)
		}

		return nil
	})
	if err != nil {
		return domain.APIKey{}, err
	}

	return key, nil
}

// account loads a service account; people and unknown IDs are not found.
func (s *BotService) account(ctx context.Context, id domain.UserID) (domain.User, error) {
	user, err := s.repos.Users.Get(ctx, id)

	switch {
	case errors.Is(err, domain.ErrUserNotFound), err == nil && !user.IsServiceAccount():
		return domain.User{}, domain.ErrServiceAccountNotFound
	case err != nil:
		return domain.User{}, fmt.Errorf("get service account: %w", err)
	}

	return user, nil
}
