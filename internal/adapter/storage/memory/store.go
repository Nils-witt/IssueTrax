// Package memory is an in-memory implementation of the storage ports, used
// by service unit tests. Data is lost when the process exits.
package memory

import (
	"context"
	"maps"
	"slices"
	"sync"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that Store satisfies the driven port.
var _ port.TxManager = (*Store)(nil)

// Store holds all in-memory data. Create one per test and build the
// repositories from it; it doubles as the TxManager.
//
// Transactions are serialized and roll back by restoring a snapshot taken at
// their start. Writes made outside a transaction while another transaction
// rolls back can be lost, which is acceptable for tests.
type Store struct {
	txMu sync.Mutex

	mu   sync.RWMutex
	data data
}

type data struct {
	projects   map[domain.ProjectID]domain.Project
	nextNumber map[domain.ProjectID]int64
	issues     map[domain.IssueID]domain.Issue
	tags       map[domain.TagID]domain.Tag
	users      map[domain.UserID]domain.User
	sessions   map[string]domain.Session
}

// NewStore returns an empty Store.
func NewStore() *Store {
	return &Store{data: data{
		projects:   map[domain.ProjectID]domain.Project{},
		nextNumber: map[domain.ProjectID]int64{},
		issues:     map[domain.IssueID]domain.Issue{},
		tags:       map[domain.TagID]domain.Tag{},
		users:      map[domain.UserID]domain.User{},
		sessions:   map[string]domain.Session{},
	}}
}

type txKey struct{}

// WithinTx runs fn atomically; see port.TxManager. Nested calls join the
// outer transaction.
func (s *Store) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if ctx.Value(txKey{}) != nil {
		return fn(ctx)
	}

	s.txMu.Lock()
	defer s.txMu.Unlock()

	s.mu.RLock()
	snapshot := s.data.clone()
	s.mu.RUnlock()

	if err := fn(context.WithValue(ctx, txKey{}, true)); err != nil {
		s.mu.Lock()
		s.data = snapshot
		s.mu.Unlock()

		return err
	}

	return nil
}

func (d data) clone() data {
	issues := make(map[domain.IssueID]domain.Issue, len(d.issues))
	for id, i := range d.issues {
		issues[id] = cloneIssue(i)
	}

	return data{
		projects:   maps.Clone(d.projects),
		nextNumber: maps.Clone(d.nextNumber),
		issues:     issues,
		tags:       maps.Clone(d.tags),
		users:      maps.Clone(d.users),
		sessions:   maps.Clone(d.sessions),
	}
}

// cloneIssue copies i's slices so stored issues never alias caller memory.
func cloneIssue(i domain.Issue) domain.Issue {
	i.Tags = slices.Clone(i.Tags)
	i.Assignees = slices.Clone(i.Assignees)

	return i
}

// paginate cuts a sorted list that already starts after the cursor down to
// limit items and reports whether more items remain.
func paginate[T any](items []T, limit int) (page []T, more bool) {
	if len(items) > limit {
		return items[:limit], true
	}

	return items, false
}
