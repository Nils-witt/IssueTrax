package memory

import (
	"bytes"
	"cmp"
	"context"
	"slices"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that APIKeyRepository satisfies the driven port.
var _ port.APIKeyRepository = (*APIKeyRepository)(nil)

// APIKeyRepository implements port.APIKeyRepository on a Store.
type APIKeyRepository struct{ s *Store }

// NewAPIKeyRepository returns an APIKeyRepository backed by s.
func NewAPIKeyRepository(s *Store) *APIKeyRepository { return &APIKeyRepository{s: s} }

// Create inserts key.
func (r *APIKeyRepository) Create(_ context.Context, key domain.APIKey) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	r.s.data.apiKeys[key.ID] = cloneAPIKey(key)

	return nil
}

// Get returns the key with id.
func (r *APIKeyRepository) Get(_ context.Context, id domain.APIKeyID) (domain.APIKey, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	k, ok := r.s.data.apiKeys[id]
	if !ok {
		return domain.APIKey{}, domain.ErrAPIKeyNotFound
	}

	return cloneAPIKey(k), nil
}

// GetByHash returns the key whose token hashes to tokenHash.
func (r *APIKeyRepository) GetByHash(_ context.Context, tokenHash []byte) (domain.APIKey, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	for _, k := range r.s.data.apiKeys {
		if bytes.Equal(k.TokenHash, tokenHash) {
			return cloneAPIKey(k), nil
		}
	}

	return domain.APIKey{}, domain.ErrAPIKeyNotFound
}

// ListByUser returns the keys of user, newest first.
func (r *APIKeyRepository) ListByUser(_ context.Context, user domain.UserID) ([]domain.APIKey, error) {
	r.s.mu.RLock()

	keys := []domain.APIKey{}

	for _, k := range r.s.data.apiKeys {
		if k.UserID == user {
			keys = append(keys, cloneAPIKey(k))
		}
	}

	r.s.mu.RUnlock()

	slices.SortFunc(keys, func(a, b domain.APIKey) int {
		return cmp.Or(b.CreatedAt.Compare(a.CreatedAt), cmp.Compare(b.ID.String(), a.ID.String()))
	})

	return keys, nil
}

// Update stores the usage and revocation times of key.
func (r *APIKeyRepository) Update(_ context.Context, key domain.APIKey) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	k, ok := r.s.data.apiKeys[key.ID]
	if !ok {
		return domain.ErrAPIKeyNotFound
	}

	k.LastUsedAt, k.RevokedAt = key.LastUsedAt, key.RevokedAt
	r.s.data.apiKeys[key.ID] = cloneAPIKey(k)

	return nil
}

// cloneAPIKey copies k's pointer and slice fields so stored keys never
// alias caller memory.
func cloneAPIKey(k domain.APIKey) domain.APIKey {
	k.TokenHash = bytes.Clone(k.TokenHash)
	k.ExpiresAt = clonePtr(k.ExpiresAt)
	k.LastUsedAt = clonePtr(k.LastUsedAt)
	k.RevokedAt = clonePtr(k.RevokedAt)

	return k
}

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}

	v := *p

	return &v
}
