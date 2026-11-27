package memory

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that IssueRepository satisfies the driven port.
var _ port.IssueRepository = (*IssueRepository)(nil)

// IssueRepository implements port.IssueRepository on a Store.
type IssueRepository struct{ s *Store }

// NewIssueRepository returns an IssueRepository backed by s.
func NewIssueRepository(s *Store) *IssueRepository { return &IssueRepository{s: s} }

// Create inserts issue.
func (r *IssueRepository) Create(_ context.Context, issue domain.Issue) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	r.s.data.issues[issue.ID] = cloneIssue(issue)

	return nil
}

// Update replaces an existing issue.
func (r *IssueRepository) Update(_ context.Context, issue domain.Issue) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	if _, ok := r.s.data.issues[issue.ID]; !ok {
		return domain.ErrIssueNotFound
	}

	r.s.data.issues[issue.ID] = cloneIssue(issue)

	return nil
}

// Get returns the issue with the given project-scoped number.
func (r *IssueRepository) Get(_ context.Context, project domain.ProjectID, number int64) (domain.Issue, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	for _, i := range r.s.data.issues {
		if i.ProjectID == project && i.Number == number {
			return cloneIssue(i), nil
		}
	}

	return domain.Issue{}, domain.ErrIssueNotFound
}

// List returns one page of matching issues, newest (highest number) first.
func (r *IssueRepository) List(
	_ context.Context, project domain.ProjectID, filter port.IssueFilter, page domain.PageRequest,
) (domain.Page[domain.Issue], error) {
	cursor, hasCursor, err := domain.DecodeCursor[domain.IssueCursor](page.Cursor)
	if err != nil {
		return domain.Page[domain.Issue]{}, err
	}

	r.s.mu.RLock()

	var matching []domain.Issue

	for _, i := range r.s.data.issues {
		if i.ProjectID == project && matches(i, filter) {
			matching = append(matching, cloneIssue(i))
		}
	}

	r.s.mu.RUnlock()

	slices.SortFunc(matching, func(a, b domain.Issue) int { return cmp.Compare(b.Number, a.Number) })

	after := matching
	if hasCursor {
		after = slices.DeleteFunc(slices.Clone(matching), func(i domain.Issue) bool { return i.Number >= cursor.Number })
	}

	items, more := paginate(after, page.Limit)
	result := domain.Page[domain.Issue]{Items: items, Total: int64(len(matching))}

	if more {
		result.NextCursor = domain.EncodeCursor(domain.IssueCursor{Number: items[len(items)-1].Number})
	}

	if result.Items == nil {
		result.Items = []domain.Issue{}
	}

	return result, nil
}

// ListByStorageItem returns one page of the issues of any project linking
// item, newest (created last) first.
func (r *IssueRepository) ListByStorageItem(
	_ context.Context, item domain.StorageItemID, page domain.PageRequest,
) (domain.Page[domain.Issue], error) {
	cursor, hasCursor, err := domain.DecodeCursor[domain.LinkedIssueCursor](page.Cursor)
	if err != nil {
		return domain.Page[domain.Issue]{}, err
	}

	r.s.mu.RLock()

	var matching []domain.Issue

	for _, i := range r.s.data.issues {
		if slices.Contains(i.StorageItems, item) {
			matching = append(matching, cloneIssue(i))
		}
	}

	r.s.mu.RUnlock()

	newestFirst := func(a, b domain.Issue) int {
		return cmp.Or(b.CreatedAt.Compare(a.CreatedAt), cmp.Compare(b.ID.String(), a.ID.String()))
	}
	slices.SortFunc(matching, newestFirst)

	after := matching
	if hasCursor {
		after = slices.DeleteFunc(slices.Clone(matching), func(i domain.Issue) bool {
			return cmp.Or(cursor.At.Compare(i.CreatedAt), cmp.Compare(cursor.ID, i.ID.String())) <= 0
		})
	}

	items, more := paginate(after, page.Limit)
	result := domain.Page[domain.Issue]{Items: append([]domain.Issue{}, items...), Total: int64(len(matching))}

	if more {
		last := items[len(items)-1]
		result.NextCursor = domain.EncodeCursor(domain.LinkedIssueCursor{At: last.CreatedAt, ID: last.ID.String()})
	}

	return result, nil
}

func matches(i domain.Issue, f port.IssueFilter) bool {
	if len(f.Statuses) > 0 && !slices.Contains(f.Statuses, i.Status) {
		return false
	}

	if !matchSet(i.Tags, f.TagIDs, f.TagMode) || !matchSet(i.Assignees, f.AssigneeIDs, f.AssigneeMode) ||
		!matchSet(i.StorageItems, f.StorageItemIDs, f.StorageItemMode) {
		return false
	}

	if f.Query == "" {
		return true
	}

	q := strings.ToLower(f.Query)

	return strings.Contains(strings.ToLower(i.Title), q) || strings.Contains(strings.ToLower(i.Description), q)
}

// matchSet reports whether have contains all (or, for MatchAny, at least one)
// of want. An empty want matches everything.
func matchSet[T comparable](have, want []T, mode port.MatchMode) bool {
	if len(want) == 0 {
		return true
	}

	contains := func(w T) bool { return slices.Contains(have, w) }

	if mode == port.MatchAny {
		return slices.ContainsFunc(want, contains)
	}

	return !slices.ContainsFunc(want, func(w T) bool { return !contains(w) })
}
