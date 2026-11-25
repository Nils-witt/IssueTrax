package memory

import (
	"cmp"
	"context"
	"slices"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that CommentRepository satisfies the driven port.
var _ port.CommentRepository = (*CommentRepository)(nil)

// CommentRepository implements port.CommentRepository on a Store.
type CommentRepository struct{ s *Store }

// NewCommentRepository returns a CommentRepository backed by s.
func NewCommentRepository(s *Store) *CommentRepository { return &CommentRepository{s: s} }

// Create inserts comment.
func (r *CommentRepository) Create(_ context.Context, comment domain.Comment) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	r.s.data.comments[comment.ID] = comment

	return nil
}

// Get returns the comment with id on issue.
func (r *CommentRepository) Get(_ context.Context, issue domain.IssueID, id domain.CommentID) (domain.Comment, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	c, ok := r.s.data.comments[id]
	if !ok || c.IssueID != issue {
		return domain.Comment{}, domain.ErrCommentNotFound
	}

	return c, nil
}

// ListByIssue returns the issue's comments, oldest first.
func (r *CommentRepository) ListByIssue(_ context.Context, issue domain.IssueID) ([]domain.Comment, error) {
	r.s.mu.RLock()

	comments := []domain.Comment{}

	for _, c := range r.s.data.comments {
		if c.IssueID == issue {
			comments = append(comments, c)
		}
	}

	r.s.mu.RUnlock()

	slices.SortFunc(comments, func(a, b domain.Comment) int {
		return cmp.Or(a.CreatedAt.Compare(b.CreatedAt), cmp.Compare(a.ID.String(), b.ID.String()))
	})

	return comments, nil
}

// Update stores comment's body and update time.
func (r *CommentRepository) Update(_ context.Context, comment domain.Comment) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	c, ok := r.s.data.comments[comment.ID]
	if !ok {
		return domain.ErrCommentNotFound
	}

	c.Body, c.UpdatedAt = comment.Body, comment.UpdatedAt
	r.s.data.comments[comment.ID] = c

	return nil
}

// Delete removes the comment with id.
func (r *CommentRepository) Delete(_ context.Context, id domain.CommentID) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	if _, ok := r.s.data.comments[id]; !ok {
		return domain.ErrCommentNotFound
	}

	delete(r.s.data.comments, id)

	return nil
}
