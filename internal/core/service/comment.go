package service

import (
	"context"
	"fmt"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that CommentService satisfies the driving port.
var _ port.CommentService = (*CommentService)(nil)

// CommentService implements port.CommentService.
type CommentService struct {
	repos Repositories
	opts  options
}

// NewCommentService returns a CommentService backed by repos.
func NewCommentService(repos Repositories, opts ...Option) *CommentService {
	return &CommentService{repos: repos, opts: newOptions(opts)}
}

// List returns the issue's comments, oldest first.
func (s *CommentService) List(ctx context.Context, key domain.ProjectKey, number int64) ([]port.CommentView, error) {
	issue, err := s.issue(ctx, key, number)
	if err != nil {
		return nil, err
	}

	comments, err := s.repos.Comments.ListByIssue(ctx, issue.ID)
	if err != nil {
		return nil, fmt.Errorf("list comments on issue %s: %w", issue.ID, err)
	}

	return s.views(ctx, comments)
}

// Create adds a comment by actor to an issue that is not archived.
func (s *CommentService) Create(
	ctx context.Context, actor domain.UserID, key domain.ProjectKey, number int64, body string,
) (port.CommentView, error) {
	var comment domain.Comment

	err := s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		issue, err := s.editableIssue(ctx, key, number)
		if err != nil {
			return err
		}

		comment, err = domain.NewComment(domain.CommentID(s.opts.newID()), issue.ID, actor, body, s.opts.now())
		if err != nil {
			return err
		}

		if err := s.repos.Comments.Create(ctx, comment); err != nil {
			return fmt.Errorf("create comment on issue %s: %w", issue.ID, err)
		}

		changes := domain.AuditChanges{}
		changes.InitialText("body", comment.Body)

		return s.record(ctx, actor, domain.AuditCommentCreate, key, issue, comment.ID, changes)
	})
	if err != nil {
		return port.CommentView{}, err
	}

	return s.view(ctx, comment)
}

// Update replaces the body of actor's own comment.
func (s *CommentService) Update(
	ctx context.Context, actor domain.UserID, key domain.ProjectKey, number int64, id domain.CommentID, body string,
) (port.CommentView, error) {
	var comment domain.Comment

	err := s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		issue, err := s.editableIssue(ctx, key, number)
		if err != nil {
			return err
		}

		comment, err = s.repos.Comments.Get(ctx, issue.ID, id)
		if err != nil {
			return fmt.Errorf("get comment %s: %w", id, err)
		}

		before := comment.Body

		if err := comment.Edit(actor, body, s.opts.now()); err != nil {
			return err
		}

		if err := s.repos.Comments.Update(ctx, comment); err != nil {
			return fmt.Errorf("update comment %s: %w", id, err)
		}

		changes := domain.AuditChanges{}
		changes.Text("body", before, comment.Body)

		if len(changes) == 0 {
			return nil
		}

		return s.record(ctx, actor, domain.AuditCommentUpdate, key, issue, comment.ID, changes)
	})
	if err != nil {
		return port.CommentView{}, err
	}

	return s.view(ctx, comment)
}

// Delete removes actor's own comment.
func (s *CommentService) Delete(ctx context.Context, actor domain.UserID, key domain.ProjectKey, number int64, id domain.CommentID) error {
	return s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		issue, err := s.editableIssue(ctx, key, number)
		if err != nil {
			return err
		}

		comment, err := s.repos.Comments.Get(ctx, issue.ID, id)
		if err != nil {
			return fmt.Errorf("get comment %s: %w", id, err)
		}

		if err := comment.CheckAuthor(actor); err != nil {
			return err
		}

		if err := s.repos.Comments.Delete(ctx, id); err != nil {
			return fmt.Errorf("delete comment %s: %w", id, err)
		}

		return s.record(ctx, actor, domain.AuditCommentDelete, key, issue, id, nil)
	})
}

// record writes the audit entry for a change to comment id on issue.
func (s *CommentService) record(
	ctx context.Context, actor domain.UserID, action domain.AuditAction, key domain.ProjectKey, issue domain.Issue,
	id domain.CommentID, changes domain.AuditChanges,
) error {
	return record(ctx, s.repos, s.opts, auditEvent{
		actor: &actor, action: action, changes: changes,
		targetType: domain.AuditTargetComment, targetID: id.String(),
		targetLabel: issue.Key(domain.NormalizeProjectKey(string(key))),
	})
}

func (s *CommentService) issue(ctx context.Context, key domain.ProjectKey, number int64) (domain.Issue, error) {
	project, err := projectByKey(ctx, s.repos.Projects, key)
	if err != nil {
		return domain.Issue{}, err
	}

	issue, err := s.repos.Issues.Get(ctx, project.ID, number)
	if err != nil {
		return domain.Issue{}, fmt.Errorf("get issue %s-%d: %w", project.Key, number, err)
	}

	return issue, nil
}

// editableIssue loads the issue and rejects archived ones. Inside a
// transaction the issue row stays locked, so its status cannot change
// before the comment is written.
func (s *CommentService) editableIssue(ctx context.Context, key domain.ProjectKey, number int64) (domain.Issue, error) {
	issue, err := s.issue(ctx, key, number)
	if err != nil {
		return domain.Issue{}, err
	}

	if err := issue.CheckEditable(false); err != nil {
		return domain.Issue{}, err
	}

	return issue, nil
}

func (s *CommentService) view(ctx context.Context, c domain.Comment) (port.CommentView, error) {
	views, err := s.views(ctx, []domain.Comment{c})
	if err != nil {
		return port.CommentView{}, err
	}

	return views[0], nil
}

// views resolves comment authors with one lookup.
func (s *CommentService) views(ctx context.Context, comments []domain.Comment) ([]port.CommentView, error) {
	ids := make([]domain.UserID, 0, len(comments))
	for _, c := range comments {
		ids = append(ids, c.Author)
	}

	users, err := s.repos.Users.GetMany(ctx, unique(ids))
	if err != nil {
		return nil, fmt.Errorf("get comment authors: %w", err)
	}

	byID := make(map[domain.UserID]domain.User, len(users))
	for _, u := range users {
		byID[u.ID] = u
	}

	views := make([]port.CommentView, 0, len(comments))

	for _, c := range comments {
		author, ok := byID[c.Author]
		if !ok {
			author = domain.User{ID: c.Author}
		}

		views = append(views, port.CommentView{Comment: c, Author: author})
	}

	return views, nil
}
