package service

import (
	"context"
	"fmt"
	"strconv"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that AttachmentService satisfies the driving port.
var _ port.AttachmentService = (*AttachmentService)(nil)

// AttachmentService implements port.AttachmentService.
type AttachmentService struct {
	repos Repositories
	opts  options
}

// NewAttachmentService returns an AttachmentService backed by repos.
func NewAttachmentService(repos Repositories, opts ...Option) *AttachmentService {
	return &AttachmentService{repos: repos, opts: newOptions(opts)}
}

// List returns the issue's attachments, oldest first.
func (s *AttachmentService) List(ctx context.Context, key domain.ProjectKey, number int64) ([]port.AttachmentView, error) {
	issue, err := issueByNumber(ctx, s.repos, key, number)
	if err != nil {
		return nil, err
	}

	attachments, err := s.repos.Attachments.ListByIssue(ctx, issue.ID)
	if err != nil {
		return nil, fmt.Errorf("list attachments of issue %s: %w", issue.ID, err)
	}

	return s.views(ctx, attachments)
}

// Upload attaches a file by actor to an issue that is not archived.
func (s *AttachmentService) Upload(
	ctx context.Context, actor domain.UserID, key domain.ProjectKey, number int64, in port.UploadAttachmentInput,
) (port.AttachmentView, error) {
	var attachment domain.Attachment

	err := s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		issue, err := editableIssue(ctx, s.repos, key, number)
		if err != nil {
			return err
		}

		attachment, err = domain.NewAttachment(
			domain.AttachmentID(s.opts.newID()), issue.ID, actor, in.FileName, in.MediaType, int64(len(in.Content)), s.opts.now(),
		)
		if err != nil {
			return err
		}

		if err := s.repos.Attachments.Create(ctx, attachment, in.Content); err != nil {
			return fmt.Errorf("create attachment on issue %s: %w", issue.ID, err)
		}

		changes := domain.AuditChanges{}
		changes.Initial("fileName", attachment.FileName)
		changes.Initial("mediaType", attachment.MediaType)
		changes.Initial("size", strconv.FormatInt(attachment.Size, 10))

		return s.record(ctx, actor, domain.AuditAttachmentCreate, key, issue, attachment.ID, changes)
	})
	if err != nil {
		return port.AttachmentView{}, err
	}

	s.notify(ctx, key, number)

	views, err := s.views(ctx, []domain.Attachment{attachment})
	if err != nil {
		return port.AttachmentView{}, err
	}

	return views[0], nil
}

// Content returns an attachment of the issue with its file content.
// Attachments of archived issues stay readable.
func (s *AttachmentService) Content(
	ctx context.Context, key domain.ProjectKey, number int64, id domain.AttachmentID,
) (domain.Attachment, []byte, error) {
	issue, err := issueByNumber(ctx, s.repos, key, number)
	if err != nil {
		return domain.Attachment{}, nil, err
	}

	attachment, content, err := s.repos.Attachments.Content(ctx, issue.ID, id)
	if err != nil {
		return domain.Attachment{}, nil, fmt.Errorf("get attachment %s: %w", id, err)
	}

	return attachment, content, nil
}

// Delete removes actor's own attachment.
func (s *AttachmentService) Delete(ctx context.Context, actor domain.UserID, key domain.ProjectKey, number int64, id domain.AttachmentID) error {
	err := s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		issue, err := editableIssue(ctx, s.repos, key, number)
		if err != nil {
			return err
		}

		attachment, err := s.repos.Attachments.Get(ctx, issue.ID, id)
		if err != nil {
			return fmt.Errorf("get attachment %s: %w", id, err)
		}

		if err := attachment.CheckUploader(actor); err != nil {
			return err
		}

		if err := s.repos.Attachments.Delete(ctx, id); err != nil {
			return fmt.Errorf("delete attachment %s: %w", id, err)
		}

		changes := domain.AuditChanges{}
		changes.Set("fileName", attachment.FileName, "")

		return s.record(ctx, actor, domain.AuditAttachmentDelete, key, issue, id, changes)
	})
	if err != nil {
		return err
	}

	s.notify(ctx, key, number)

	return nil
}

// notify announces that the attachments of an issue changed.
func (s *AttachmentService) notify(ctx context.Context, key domain.ProjectKey, number int64) {
	s.opts.notify(ctx, domain.Change{Kind: domain.ChangeAttachments, Project: domain.NormalizeProjectKey(string(key)), Issue: number})
}

// record writes the audit entry for a change to attachment id on issue.
func (s *AttachmentService) record(
	ctx context.Context, actor domain.UserID, action domain.AuditAction, key domain.ProjectKey, issue domain.Issue,
	id domain.AttachmentID, changes domain.AuditChanges,
) error {
	return record(ctx, s.repos, s.opts, auditEvent{
		actor: &actor, action: action, changes: changes,
		targetType: domain.AuditTargetAttachment, targetID: id.String(),
		targetLabel: issue.Key(domain.NormalizeProjectKey(string(key))),
	})
}

// views resolves uploaders with one lookup.
func (s *AttachmentService) views(ctx context.Context, attachments []domain.Attachment) ([]port.AttachmentView, error) {
	return withUsers(ctx, s.repos, "attachment uploaders", attachments, attachmentUploader, newAttachmentView)
}

func attachmentUploader(a domain.Attachment) domain.UserID { return a.Uploader }

func newAttachmentView(a domain.Attachment, uploader domain.User) port.AttachmentView {
	return port.AttachmentView{Attachment: a, Uploader: uploader}
}
