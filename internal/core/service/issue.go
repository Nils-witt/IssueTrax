// Package service implements the IssueTrax use cases (driving ports) on top
// of the driven ports.
package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that IssueService satisfies the driving port.
var _ port.IssueService = (*IssueService)(nil)

// IssueService implements port.IssueService.
type IssueService struct {
	repo  port.IssueRepository
	now   func() time.Time
	newID func() domain.IssueID
}

// Option customizes an IssueService.
type Option func(*IssueService)

// WithClock overrides the time source (useful in tests).
func WithClock(now func() time.Time) Option {
	return func(s *IssueService) { s.now = now }
}

// WithIDGenerator overrides how new issue IDs are generated.
func WithIDGenerator(newID func() domain.IssueID) Option {
	return func(s *IssueService) { s.newID = newID }
}

// NewIssueService returns an IssueService backed by repo.
func NewIssueService(repo port.IssueRepository, opts ...Option) *IssueService {
	s := &IssueService{
		repo:  repo,
		now:   func() time.Time { return time.Now().UTC() },
		newID: func() domain.IssueID { return domain.IssueID(rand.Text()) },
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Create validates and stores a new open issue.
func (s *IssueService) Create(ctx context.Context, in port.CreateIssueInput) (domain.Issue, error) {
	issue, err := domain.NewIssue(s.newID(), in.Title, in.Description, s.now())
	if err != nil {
		return domain.Issue{}, err
	}

	if err := s.repo.Save(ctx, issue); err != nil {
		return domain.Issue{}, fmt.Errorf("save issue: %w", err)
	}

	return issue, nil
}

// Get returns the issue with the given ID.
func (s *IssueService) Get(ctx context.Context, id domain.IssueID) (domain.Issue, error) {
	issue, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Issue{}, fmt.Errorf("get issue %q: %w", id, err)
	}

	return issue, nil
}

// List returns all issues, oldest first.
func (s *IssueService) List(ctx context.Context) ([]domain.Issue, error) {
	issues, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}

	return issues, nil
}
