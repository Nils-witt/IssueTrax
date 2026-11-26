package domain

import (
	"mime"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// Attachment limits.
const (
	// MaxAttachmentBytes bounds the size of one attached file.
	MaxAttachmentBytes = 10 << 20
	// maxFileNameBytes matches the common file system limit for a name.
	maxFileNameBytes = 255
	// DefaultMediaType is used when a file's media type is missing or malformed.
	DefaultMediaType = "application/octet-stream"
)

// AttachmentID is an attachment's internal identifier.
type AttachmentID uuid.UUID

func (id AttachmentID) String() string { return uuid.UUID(id).String() }

// Attachment is a file attached to an issue. Its content is stored
// separately. Attachments cannot change; only the uploader may delete one,
// and attachments of archived issues cannot be added or deleted.
type Attachment struct {
	ID      AttachmentID
	IssueID IssueID
	// FileName is the base name the file was uploaded with.
	FileName string
	// MediaType is the type the uploader declared, normalized; it is not
	// verified against the content.
	MediaType string
	Size      int64
	Uploader  UserID
	CreatedAt time.Time
}

// NewAttachment validates the file's metadata and builds an Attachment
// uploaded by uploader to issue. Validation failures are reported for the
// fields "fileName" and "file" (the content).
func NewAttachment(
	id AttachmentID, issue IssueID, uploader UserID, fileName, mediaType string, size int64, now time.Time,
) (Attachment, error) {
	f := fieldErrors{}
	a := Attachment{
		ID:        id,
		IssueID:   issue,
		FileName:  checkFileName(f, fileName),
		MediaType: NormalizeMediaType(mediaType),
		Size:      size,
		Uploader:  uploader,
		CreatedAt: now,
	}

	switch {
	case size <= 0:
		f.add("file", FieldRequired)
	case size > MaxAttachmentBytes:
		f.add("file", FieldTooLong)
	}

	if err := f.err(); err != nil {
		return Attachment{}, err
	}

	return a, nil
}

// CheckUploader returns ErrNotAttachmentUploader unless user uploaded a.
func (a Attachment) CheckUploader(user UserID) error {
	if a.Uploader != user {
		return ErrNotAttachmentUploader
	}

	return nil
}

// NormalizeMediaType returns s as a lower-case "type/subtype" with its
// parameters, or DefaultMediaType when s is empty or malformed.
func NormalizeMediaType(s string) string {
	mt, params, err := mime.ParseMediaType(s)
	if err != nil || !strings.Contains(mt, "/") {
		return DefaultMediaType
	}

	if out := mime.FormatMediaType(mt, params); out != "" {
		return out
	}

	return DefaultMediaType
}

// checkFileName reduces s to its base name (browsers on some systems send
// full paths), trims it and records required/too_long/invalid for "fileName".
func checkFileName(f fieldErrors, s string) string {
	if i := strings.LastIndexAny(s, `/\`); i >= 0 {
		s = s[i+1:]
	}

	s = strings.TrimSpace(s)

	switch {
	case s == "" || s == "." || s == "..":
		f.add("fileName", FieldRequired)
	case len(s) > maxFileNameBytes:
		f.add("fileName", FieldTooLong)
	case strings.ContainsFunc(s, unicode.IsControl):
		f.add("fileName", FieldInvalid)
	}

	return s
}
