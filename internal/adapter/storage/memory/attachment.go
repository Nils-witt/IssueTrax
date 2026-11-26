package memory

import (
	"cmp"
	"context"
	"slices"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that AttachmentRepository satisfies the driven port.
var _ port.AttachmentRepository = (*AttachmentRepository)(nil)

// AttachmentRepository implements port.AttachmentRepository on a Store.
type AttachmentRepository struct{ s *Store }

// NewAttachmentRepository returns an AttachmentRepository backed by s.
func NewAttachmentRepository(s *Store) *AttachmentRepository { return &AttachmentRepository{s: s} }

// storedAttachment is an attachment with its content.
type storedAttachment struct {
	attachment domain.Attachment
	content    []byte
}

// Create inserts attachment with a copy of content.
func (r *AttachmentRepository) Create(_ context.Context, attachment domain.Attachment, content []byte) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	r.s.data.attachments[attachment.ID] = storedAttachment{attachment: attachment, content: slices.Clone(content)}

	return nil
}

// Get returns the attachment with id on issue.
func (r *AttachmentRepository) Get(ctx context.Context, issue domain.IssueID, id domain.AttachmentID) (domain.Attachment, error) {
	a, _, err := r.Content(ctx, issue, id)

	return a, err
}

// Content returns the attachment with id on issue and a copy of its content.
func (r *AttachmentRepository) Content(_ context.Context, issue domain.IssueID, id domain.AttachmentID) (domain.Attachment, []byte, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	a, ok := r.s.data.attachments[id]
	if !ok || a.attachment.IssueID != issue {
		return domain.Attachment{}, nil, domain.ErrAttachmentNotFound
	}

	return a.attachment, slices.Clone(a.content), nil
}

// ListByIssue returns the issue's attachments, oldest first.
func (r *AttachmentRepository) ListByIssue(_ context.Context, issue domain.IssueID) ([]domain.Attachment, error) {
	r.s.mu.RLock()

	attachments := []domain.Attachment{}

	for _, a := range r.s.data.attachments {
		if a.attachment.IssueID == issue {
			attachments = append(attachments, a.attachment)
		}
	}

	r.s.mu.RUnlock()

	slices.SortFunc(attachments, func(a, b domain.Attachment) int {
		return cmp.Or(a.CreatedAt.Compare(b.CreatedAt), cmp.Compare(a.ID.String(), b.ID.String()))
	})

	return attachments, nil
}

// Delete removes the attachment with id.
func (r *AttachmentRepository) Delete(_ context.Context, id domain.AttachmentID) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	if _, ok := r.s.data.attachments[id]; !ok {
		return domain.ErrAttachmentNotFound
	}

	delete(r.s.data.attachments, id)

	return nil
}
