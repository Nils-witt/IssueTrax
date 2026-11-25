package memory

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"strings"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that UserRepository satisfies the driven port.
var _ port.UserRepository = (*UserRepository)(nil)

// UserRepository implements port.UserRepository on a Store.
type UserRepository struct{ s *Store }

// NewUserRepository returns a UserRepository backed by s.
func NewUserRepository(s *Store) *UserRepository { return &UserRepository{s: s} }

// Create inserts user.
func (r *UserRepository) Create(_ context.Context, user domain.User) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	r.s.data.users[user.ID] = user

	return nil
}

// Update replaces an existing user.
func (r *UserRepository) Update(_ context.Context, user domain.User) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	if _, ok := r.s.data.users[user.ID]; !ok {
		return domain.ErrUserNotFound
	}

	r.s.data.users[user.ID] = user

	return nil
}

// Get returns the user with id.
func (r *UserRepository) Get(_ context.Context, id domain.UserID) (domain.User, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	u, ok := r.s.data.users[id]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}

	return u, nil
}

// GetByIdentity returns the user with the given OIDC issuer and subject.
func (r *UserRepository) GetByIdentity(_ context.Context, issuer, subject string) (domain.User, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	for _, u := range r.s.data.users {
		if u.Issuer == issuer && u.Subject == subject {
			return u, nil
		}
	}

	return domain.User{}, domain.ErrUserNotFound
}

// GetMany returns the users with the given IDs, skipping unknown ones.
func (r *UserRepository) GetMany(_ context.Context, ids []domain.UserID) ([]domain.User, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	users := make([]domain.User, 0, len(ids))

	for _, id := range ids {
		if u, ok := r.s.data.users[id]; ok {
			users = append(users, u)
		}
	}

	return users, nil
}

// List returns all users ordered by display name.
func (r *UserRepository) List(_ context.Context) ([]domain.User, error) {
	r.s.mu.RLock()
	users := slices.Collect(maps.Values(r.s.data.users))
	r.s.mu.RUnlock()

	slices.SortFunc(users, func(a, b domain.User) int {
		return cmp.Or(
			cmp.Compare(strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName)),
			cmp.Compare(a.ID.String(), b.ID.String()),
		)
	})

	return users, nil
}
