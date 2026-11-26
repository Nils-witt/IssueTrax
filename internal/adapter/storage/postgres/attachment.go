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

// Compile-time check that AttachmentRepository satisfies the driven port.
var _ port.AttachmentRepository = (*AttachmentRepository)(nil)

// AttachmentRepository implements port.AttachmentRepository. Content is
// stored in the attachments row itself.
type AttachmentRepository struct{ db *DB }

// NewAttachmentRepository returns an AttachmentRepository on db.
func NewAttachmentRepository(db *DB) *AttachmentRepository { return &AttachmentRepository{db: db} }

// Create inserts attachment with its content.
func (r *AttachmentRepository) Create(ctx context.Context, attachment domain.Attachment, content []byte) error {
	err := r.db.queries(ctx).CreateAttachment(ctx, sqlc.CreateAttachmentParams{
		ID:         uuid.UUID(attachment.ID),
		IssueID:    uuid.UUID(attachment.IssueID),
		UploaderID: uuid.UUID(attachment.Uploader),
		FileName:   attachment.FileName,
		MediaType:  attachment.MediaType,
		Size:       attachment.Size,
		Content:    content,
		CreatedAt:  attachment.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("insert attachment: %w", err)
	}

	return nil
}

// Get returns the attachment with id on issue, without its content.
func (r *AttachmentRepository) Get(ctx context.Context, issue domain.IssueID, id domain.AttachmentID) (domain.Attachment, error) {
	row, err := r.db.queries(ctx).GetAttachment(ctx, sqlc.GetAttachmentParams{IssueID: uuid.UUID(issue), ID: uuid.UUID(id)})
	if err != nil {
		return domain.Attachment{}, attachmentError(err)
	}

	return toAttachment(row), nil
}

// Content returns the attachment with id on issue and its content.
func (r *AttachmentRepository) Content(ctx context.Context, issue domain.IssueID, id domain.AttachmentID) (domain.Attachment, []byte, error) {
	row, err := r.db.queries(ctx).GetAttachmentContent(ctx, sqlc.GetAttachmentContentParams{IssueID: uuid.UUID(issue), ID: uuid.UUID(id)})
	if err != nil {
		return domain.Attachment{}, nil, attachmentError(err)
	}

	return toAttachment(sqlc.GetAttachmentRow{
		ID: row.ID, IssueID: row.IssueID, UploaderID: row.UploaderID, FileName: row.FileName,
		MediaType: row.MediaType, Size: row.Size, CreatedAt: row.CreatedAt,
	}), row.Content, nil
}

// ListByIssue returns the issue's attachments, oldest first.
func (r *AttachmentRepository) ListByIssue(ctx context.Context, issue domain.IssueID) ([]domain.Attachment, error) {
	rows, err := r.db.queries(ctx).ListAttachmentsByIssue(ctx, uuid.UUID(issue))
	if err != nil {
		return nil, fmt.Errorf("select attachments: %w", err)
	}

	attachments := make([]domain.Attachment, 0, len(rows))
	for _, row := range rows {
		attachments = append(attachments, toAttachment(sqlc.GetAttachmentRow(row)))
	}

	return attachments, nil
}

// Delete removes the attachment with id and its content.
func (r *AttachmentRepository) Delete(ctx context.Context, id domain.AttachmentID) error {
	n, err := r.db.queries(ctx).DeleteAttachment(ctx, uuid.UUID(id))

	switch {
	case err != nil:
		return fmt.Errorf("delete attachment: %w", err)
	case n == 0:
		return domain.ErrAttachmentNotFound
	}

	return nil
}

func attachmentError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrAttachmentNotFound
	}

	return fmt.Errorf("select attachment: %w", err)
}

func toAttachment(row sqlc.GetAttachmentRow) domain.Attachment {
	return domain.Attachment{
		ID:        domain.AttachmentID(row.ID),
		IssueID:   domain.IssueID(row.IssueID),
		FileName:  row.FileName,
		MediaType: row.MediaType,
		Size:      row.Size,
		Uploader:  domain.UserID(row.UploaderID),
		CreatedAt: row.CreatedAt.UTC(),
	}
}
