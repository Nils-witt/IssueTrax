package port

import (
	"context"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// AttachmentView is an attachment with its uploader resolved for presentation.
type AttachmentView struct {
	Attachment domain.Attachment
	Uploader   domain.User
}

// UploadAttachmentInput is a file to attach. Content holds the whole file,
// at most domain.MaxAttachmentBytes.
type UploadAttachmentInput struct {
	FileName  string
	MediaType string
	Content   []byte
}

// AttachmentService is the driving port for file attachments on issues.
// Attachments are addressed through their issue; only the uploader may
// delete one, and attachments of archived issues cannot be added or deleted.
type AttachmentService interface {
	List(ctx context.Context, project domain.ProjectKey, number int64) ([]AttachmentView, error)
	Upload(ctx context.Context, actor domain.UserID, project domain.ProjectKey, number int64, in UploadAttachmentInput) (AttachmentView, error)
	// Content returns the attachment's metadata and file content.
	Content(ctx context.Context, project domain.ProjectKey, number int64, id domain.AttachmentID) (domain.Attachment, []byte, error)
	Delete(ctx context.Context, actor domain.UserID, project domain.ProjectKey, number int64, id domain.AttachmentID) error
}

// AttachmentRepository is the driven port for persisting attachments and
// their content.
//
// Get and Content return domain.ErrAttachmentNotFound, also when the
// attachment belongs to another issue; Delete returns it for unknown IDs.
// ListByIssue orders oldest first and never loads content.
type AttachmentRepository interface {
	Create(ctx context.Context, attachment domain.Attachment, content []byte) error
	Get(ctx context.Context, issue domain.IssueID, id domain.AttachmentID) (domain.Attachment, error)
	Content(ctx context.Context, issue domain.IssueID, id domain.AttachmentID) (domain.Attachment, []byte, error)
	ListByIssue(ctx context.Context, issue domain.IssueID) ([]domain.Attachment, error)
	Delete(ctx context.Context, id domain.AttachmentID) error
}
