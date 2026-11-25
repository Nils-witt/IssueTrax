// Package memory is an in-memory implementation of the storage ports. It is
// meant for development and tests; data is lost on restart.
package memory

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"sync"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that IssueRepository satisfies the driven port.
var _ port.IssueRepository = (*IssueRepository)(nil)

// IssueRepository stores issues in a map guarded by a RWMutex.
type IssueRepository struct {
	mu     sync.RWMutex
	issues map[domain.IssueID]domain.Issue
}

// NewIssueRepository returns an empty IssueRepository.
func NewIssueRepository() *IssueRepository {
	return &IssueRepository{issues: make(map[domain.IssueID]domain.Issue)}
}

// Save inserts or replaces issue.
func (r *IssueRepository) Save(_ context.Context, issue domain.Issue) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.issues[issue.ID] = issue

	return nil
}

// Get returns the issue with the given ID, or domain.ErrIssueNotFound.
func (r *IssueRepository) Get(_ context.Context, id domain.IssueID) (domain.Issue, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	issue, ok := r.issues[id]
	if !ok {
		return domain.Issue{}, domain.ErrIssueNotFound
	}

	return issue, nil
}

// List returns all issues ordered by creation time, oldest first.
func (r *IssueRepository) List(_ context.Context) ([]domain.Issue, error) {
	r.mu.RLock()
	issues := slices.Collect(maps.Values(r.issues))
	r.mu.RUnlock()

	slices.SortFunc(issues, func(a, b domain.Issue) int {
		return cmp.Or(a.CreatedAt.Compare(b.CreatedAt), cmp.Compare(a.ID, b.ID))
	})

	return issues, nil
}
