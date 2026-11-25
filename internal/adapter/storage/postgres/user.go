package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nilswitt.dev/issuetrax/internal/adapter/storage/postgres/sqlc"
	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that UserRepository satisfies the driven port.
var _ port.UserRepository = (*UserRepository)(nil)

// UserRepository implements port.UserRepository.
type UserRepository struct{ db *DB }

// NewUserRepository returns a UserRepository on db.
func NewUserRepository(db *DB) *UserRepository { return &UserRepository{db: db} }

// Create inserts user.
func (r *UserRepository) Create(ctx context.Context, u domain.User) error {
	err := r.db.queries(ctx).CreateUser(ctx, sqlc.CreateUserParams{
		ID:          uuid.UUID(u.ID),
		Issuer:      u.Issuer,
		Subject:     u.Subject,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		CreatedAt:   u.CreatedAt,
		LastLoginAt: u.LastLoginAt,
	})
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	return nil
}

// Update stores the profile fields and login time of user.
func (r *UserRepository) Update(ctx context.Context, u domain.User) error {
	n, err := r.db.queries(ctx).UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:          uuid.UUID(u.ID),
		Email:       u.Email,
		DisplayName: u.DisplayName,
		LastLoginAt: u.LastLoginAt,
	})
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	if n == 0 {
		return domain.ErrUserNotFound
	}

	return nil
}

// Get returns the user with id.
func (r *UserRepository) Get(ctx context.Context, id domain.UserID) (domain.User, error) {
	row, err := r.db.queries(ctx).GetUser(ctx, uuid.UUID(id))

	return oneUser(row, err)
}

// GetByIdentity returns the user with the given OIDC issuer and subject.
func (r *UserRepository) GetByIdentity(ctx context.Context, issuer, subject string) (domain.User, error) {
	row, err := r.db.queries(ctx).GetUserByIdentity(ctx, sqlc.GetUserByIdentityParams{Issuer: issuer, Subject: subject})

	return oneUser(row, err)
}

// GetMany returns the users with the given IDs, skipping unknown ones.
func (r *UserRepository) GetMany(ctx context.Context, ids []domain.UserID) ([]domain.User, error) {
	if len(ids) == 0 {
		return []domain.User{}, nil
	}

	rows, err := r.db.queries(ctx).GetUsers(ctx, uuids(ids))
	if err != nil {
		return nil, fmt.Errorf("select users: %w", err)
	}

	return toUsers(rows), nil
}

// List returns all users ordered by display name.
func (r *UserRepository) List(ctx context.Context) ([]domain.User, error) {
	rows, err := r.db.queries(ctx).ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("select users: %w", err)
	}

	return toUsers(rows), nil
}

func oneUser(row sqlc.User, err error) (domain.User, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrUserNotFound
	}

	if err != nil {
		return domain.User{}, fmt.Errorf("select user: %w", err)
	}

	return toUser(row), nil
}

func toUser(row sqlc.User) domain.User {
	return domain.User{
		ID:          domain.UserID(row.ID),
		Issuer:      row.Issuer,
		Subject:     row.Subject,
		Email:       row.Email,
		DisplayName: row.DisplayName,
		CreatedAt:   row.CreatedAt.UTC(),
		LastLoginAt: row.LastLoginAt.UTC(),
	}
}

func toUsers(rows []sqlc.User) []domain.User {
	users := make([]domain.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, toUser(row))
	}

	return users
}
