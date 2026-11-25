package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nilswitt.dev/issuetrax/internal/adapter/storage/postgres/sqlc"
	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that APIKeyRepository satisfies the driven port.
var _ port.APIKeyRepository = (*APIKeyRepository)(nil)

// APIKeyRepository implements port.APIKeyRepository.
type APIKeyRepository struct{ db *DB }

// NewAPIKeyRepository returns an APIKeyRepository on db.
func NewAPIKeyRepository(db *DB) *APIKeyRepository { return &APIKeyRepository{db: db} }

// Create inserts key.
func (r *APIKeyRepository) Create(ctx context.Context, k domain.APIKey) error {
	err := r.db.queries(ctx).CreateAPIKey(ctx, sqlc.CreateAPIKeyParams{
		ID:         uuid.UUID(k.ID),
		UserID:     uuid.UUID(k.UserID),
		Name:       k.Name,
		TokenHash:  k.TokenHash,
		Prefix:     k.Prefix,
		CreatedBy:  uuid.UUID(k.CreatedBy),
		CreatedAt:  k.CreatedAt,
		ExpiresAt:  k.ExpiresAt,
		LastUsedAt: k.LastUsedAt,
		RevokedAt:  k.RevokedAt,
	})
	if err != nil {
		return fmt.Errorf("insert api key: %w", err)
	}

	return nil
}

// Get returns the key with id.
func (r *APIKeyRepository) Get(ctx context.Context, id domain.APIKeyID) (domain.APIKey, error) {
	row, err := r.db.queries(ctx).GetAPIKey(ctx, uuid.UUID(id))

	return oneAPIKey(row, err)
}

// GetByHash returns the key whose token hashes to tokenHash.
func (r *APIKeyRepository) GetByHash(ctx context.Context, tokenHash []byte) (domain.APIKey, error) {
	row, err := r.db.queries(ctx).GetAPIKeyByHash(ctx, tokenHash)

	return oneAPIKey(row, err)
}

// ListByUser returns the keys of user, newest first.
func (r *APIKeyRepository) ListByUser(ctx context.Context, user domain.UserID) ([]domain.APIKey, error) {
	rows, err := r.db.queries(ctx).ListAPIKeysByUser(ctx, uuid.UUID(user))
	if err != nil {
		return nil, fmt.Errorf("select api keys: %w", err)
	}

	keys := make([]domain.APIKey, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, toAPIKey(row))
	}

	return keys, nil
}

// Update stores the usage and revocation times of key.
func (r *APIKeyRepository) Update(ctx context.Context, k domain.APIKey) error {
	n, err := r.db.queries(ctx).UpdateAPIKey(ctx, sqlc.UpdateAPIKeyParams{
		ID:         uuid.UUID(k.ID),
		LastUsedAt: k.LastUsedAt,
		RevokedAt:  k.RevokedAt,
	})
	if err != nil {
		return fmt.Errorf("update api key: %w", err)
	}

	if n == 0 {
		return domain.ErrAPIKeyNotFound
	}

	return nil
}

func oneAPIKey(row sqlc.ApiKey, err error) (domain.APIKey, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.APIKey{}, domain.ErrAPIKeyNotFound
	}

	if err != nil {
		return domain.APIKey{}, fmt.Errorf("select api key: %w", err)
	}

	return toAPIKey(row), nil
}

func toAPIKey(row sqlc.ApiKey) domain.APIKey {
	return domain.APIKey{
		ID:         domain.APIKeyID(row.ID),
		UserID:     domain.UserID(row.UserID),
		Name:       row.Name,
		TokenHash:  row.TokenHash,
		Prefix:     row.Prefix,
		CreatedBy:  domain.UserID(row.CreatedBy),
		CreatedAt:  row.CreatedAt.UTC(),
		ExpiresAt:  utcPtr(row.ExpiresAt),
		LastUsedAt: utcPtr(row.LastUsedAt),
		RevokedAt:  utcPtr(row.RevokedAt),
	}
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}

	u := t.UTC()

	return &u
}
