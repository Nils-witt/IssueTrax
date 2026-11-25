package service

import (
	"context"
	"fmt"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that UserService satisfies the driving port.
var _ port.UserService = (*UserService)(nil)

// UserService implements port.UserService.
type UserService struct {
	repos Repositories
}

// NewUserService returns a UserService backed by repos.
func NewUserService(repos Repositories) *UserService {
	return &UserService{repos: repos}
}

// List returns all known users ordered by display name.
func (s *UserService) List(ctx context.Context) ([]domain.User, error) {
	users, err := s.repos.Users.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	return users, nil
}
