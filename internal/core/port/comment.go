package port

import (
	"context"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// CommentView is a comment with its author resolved for presentation.
type CommentView struct {
	Comment domain.Comment
	Author  domain.User
}

// CommentService is the driving port for comment use cases. Comments are
// addressed through their issue; only the author may edit or delete one,
// and comments on archived issues cannot be added, edited or deleted.
type CommentService interface {
	List(ctx context.Context, project domain.ProjectKey, number int64) ([]CommentView, error)
	Create(ctx context.Context, actor domain.UserID, project domain.ProjectKey, number int64, body string) (CommentView, error)
	Update(ctx context.Context, actor domain.UserID, project domain.ProjectKey, number int64, id domain.CommentID, body string) (CommentView, error)
	Delete(ctx context.Context, actor domain.UserID, project domain.ProjectKey, number int64, id domain.CommentID) error
}

// CommentRepository is the driven port for persisting comments.
//
// Get returns domain.ErrCommentNotFound, also when the comment belongs to
// another issue; Update and Delete return it for unknown IDs.
// ListByIssue orders oldest first.
type CommentRepository interface {
	Create(ctx context.Context, comment domain.Comment) error
	Get(ctx context.Context, issue domain.IssueID, id domain.CommentID) (domain.Comment, error)
	ListByIssue(ctx context.Context, issue domain.IssueID) ([]domain.Comment, error)
	Update(ctx context.Context, comment domain.Comment) error
	Delete(ctx context.Context, id domain.CommentID) error
}
