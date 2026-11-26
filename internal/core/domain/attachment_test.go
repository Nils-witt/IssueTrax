package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestNewAttachment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fileName  string
		mediaType string
		size      int64
		wantName  string
		wantType  string
		fields    map[string]string
	}{
		{name: "valid", fileName: "report.pdf", mediaType: "application/pdf", size: 3, wantName: "report.pdf", wantType: "application/pdf"},
		{name: "trims name", fileName: "  notes.txt ", mediaType: "text/plain", size: 1, wantName: "notes.txt", wantType: "text/plain"},
		{name: "strips unix path", fileName: "/home/ada/a.png", mediaType: "image/png", size: 1, wantName: "a.png", wantType: "image/png"},
		{name: "strips windows path", fileName: `C:\Users\ada\a.png`, mediaType: "image/png", size: 1, wantName: "a.png", wantType: "image/png"},
		{name: "unicode name", fileName: "Übersicht 📎.md", size: 1, wantName: "Übersicht 📎.md", wantType: domain.DefaultMediaType},
		{name: "normalizes type", fileName: "a.txt", mediaType: "Text/Plain; Charset=UTF-8", size: 1, wantName: "a.txt", wantType: "text/plain; charset=UTF-8"},
		{name: "malformed type", fileName: "a", mediaType: "not a type", size: 1, wantName: "a", wantType: domain.DefaultMediaType},
		{name: "max size", fileName: "a", size: domain.MaxAttachmentBytes, wantName: "a", wantType: domain.DefaultMediaType},
		{name: "empty file", fileName: "a", size: 0, fields: map[string]string{"file": domain.FieldRequired}},
		{name: "too large", fileName: "a", size: domain.MaxAttachmentBytes + 1, fields: map[string]string{"file": domain.FieldTooLong}},
		{name: "no name", fileName: " ", size: 1, fields: map[string]string{"fileName": domain.FieldRequired}},
		{name: "only a directory", fileName: "dir/", size: 1, fields: map[string]string{"fileName": domain.FieldRequired}},
		{name: "parent directory", fileName: "..", size: 1, fields: map[string]string{"fileName": domain.FieldRequired}},
		{name: "long name", fileName: strings.Repeat("a", 256), size: 1, fields: map[string]string{"fileName": domain.FieldTooLong}},
		{name: "control character", fileName: "a\x00b", size: 1, fields: map[string]string{"fileName": domain.FieldInvalid}},
		{name: "newline", fileName: "a\nb", size: 1, fields: map[string]string{"fileName": domain.FieldInvalid}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
			uploader := domain.UserID(uuid.New())

			a, err := domain.NewAttachment(domain.AttachmentID(uuid.New()), domain.IssueID(uuid.New()), uploader, tt.fileName, tt.mediaType, tt.size, now)
			wantFields(t, err, tt.fields)

			if err != nil {
				return
			}

			if a.FileName != tt.wantName || a.MediaType != tt.wantType || a.Size != tt.size || a.Uploader != uploader || !a.CreatedAt.Equal(now) {
				t.Fatalf("NewAttachment() = %+v, want name %q type %q", a, tt.wantName, tt.wantType)
			}
		})
	}
}

func TestAttachment_CheckUploader(t *testing.T) {
	t.Parallel()

	ada, bob := domain.UserID(uuid.New()), domain.UserID(uuid.New())
	a := domain.Attachment{Uploader: ada}

	if err := a.CheckUploader(ada); err != nil {
		t.Fatalf("CheckUploader(uploader) = %v", err)
	}

	if err := a.CheckUploader(bob); !errors.Is(err, domain.ErrNotAttachmentUploader) || !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("CheckUploader(other) = %v, want ErrNotAttachmentUploader", err)
	}
}
