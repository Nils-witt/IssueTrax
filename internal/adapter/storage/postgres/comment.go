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

// Compile-time check that CommentRepository satisfies the driven port.
var _ port.CommentRepository = (*CommentRepository)(nil)

// CommentRepository implements port.CommentRepository.
type CommentRepository struct{ db *DB }

// NewCommentRepository returns a CommentRepository on db.
func NewCommentRepository(db *DB) *CommentRepository { return &CommentRepository{db: db} }

// Create inserts comment.
func (r *CommentRepository) Create(ctx context.Context, comment domain.Comment) error {
	err := r.db.queries(ctx).CreateComment(ctx, sqlc.CreateCommentParams{
		ID:        uuid.UUID(comment.ID),
		IssueID:   uuid.UUID(comment.IssueID),
		AuthorID:  uuid.UUID(comment.Author),
		Body:      comment.Body,
		CreatedAt: comment.CreatedAt,
		UpdatedAt: comment.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("insert comment: %w", err)
	}

	return nil
}

// Get returns the comment with id on issue.
func (r *CommentRepository) Get(ctx context.Context, issue domain.IssueID, id domain.CommentID) (domain.Comment, error) {
	return oneComment(r.db.queries(ctx).GetComment(ctx, sqlc.GetCommentParams{IssueID: uuid.UUID(issue), ID: uuid.UUID(id)}))
}

// ListByIssue returns the issue's comments, oldest first.
func (r *CommentRepository) ListByIssue(ctx context.Context, issue domain.IssueID) ([]domain.Comment, error) {
	rows, err := r.db.queries(ctx).ListCommentsByIssue(ctx, uuid.UUID(issue))
	if err != nil {
		return nil, fmt.Errorf("select comments: %w", err)
	}

	comments := make([]domain.Comment, 0, len(rows))
	for _, row := range rows {
		comments = append(comments, toComment(row))
	}

	return comments, nil
}

// Update stores comment's body and update time.
func (r *CommentRepository) Update(ctx context.Context, comment domain.Comment) error {
	n, err := r.db.queries(ctx).UpdateComment(ctx, sqlc.UpdateCommentParams{
		ID:        uuid.UUID(comment.ID),
		Body:      comment.Body,
		UpdatedAt: comment.UpdatedAt,
	})

	switch {
	case err != nil:
		return fmt.Errorf("update comment: %w", err)
	case n == 0:
		return domain.ErrCommentNotFound
	}

	return nil
}

// Delete removes the comment with id.
func (r *CommentRepository) Delete(ctx context.Context, id domain.CommentID) error {
	n, err := r.db.queries(ctx).DeleteComment(ctx, uuid.UUID(id))

	switch {
	case err != nil:
		return fmt.Errorf("delete comment: %w", err)
	case n == 0:
		return domain.ErrCommentNotFound
	}

	return nil
}

func oneComment(row sqlc.Comment, err error) (domain.Comment, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Comment{}, domain.ErrCommentNotFound
	}

	if err != nil {
		return domain.Comment{}, fmt.Errorf("select comment: %w", err)
	}

	return toComment(row), nil
}

func toComment(row sqlc.Comment) domain.Comment {
	return domain.Comment{
		ID:        domain.CommentID(row.ID),
		IssueID:   domain.IssueID(row.IssueID),
		Body:      row.Body,
		Author:    domain.UserID(row.AuthorID),
		CreatedAt: row.CreatedAt.UTC(),
		UpdatedAt: row.UpdatedAt.UTC(),
	}
}
