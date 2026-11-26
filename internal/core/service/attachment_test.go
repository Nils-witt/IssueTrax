package service_test

import (
	"bytes"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

func (f *fixture) addAttachment(t *testing.T, actor domain.UserID, name, content string) port.AttachmentView {
	t.Helper()

	v, err := f.attachments.Upload(t.Context(), actor, "TRAX", 1, port.UploadAttachmentInput{
		FileName: name, MediaType: "text/plain", Content: []byte(content),
	})
	if err != nil {
		t.Fatalf("upload %q: %v", name, err)
	}

	return v
}

func TestAttachmentService_UploadListAndContent(t *testing.T) {
	t.Parallel()

	f, ada, bob := commentFixture(t)

	empty, err := f.attachments.List(t.Context(), "TRAX", 1)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("List() = %#v, %v; want empty non-nil", empty, err)
	}

	first := f.addAttachment(t, ada.ID, " notes.txt ", "hello")
	f.addAttachment(t, bob.ID, "b.txt", "bye")

	a := first.Attachment

	type summary struct {
		name, mediaType, uploader string
		size                      int64
		createdAt                 time.Time
	}

	if got, want := (summary{a.FileName, a.MediaType, first.Uploader.DisplayName, a.Size, a.CreatedAt}),
		(summary{"notes.txt", "text/plain", "ada", 5, testNow}); got != want {
		t.Fatalf("Upload() = %+v, want %+v", got, want)
	}

	list, err := f.attachments.List(t.Context(), "trax", 1)
	if err != nil || len(list) != 2 || list[0].Uploader.ID != ada.ID || list[1].Uploader.ID != bob.ID {
		t.Fatalf("List() = %+v, %v", list, err)
	}

	got, content, err := f.attachments.Content(t.Context(), "trax", 1, a.ID)
	if err != nil || got.ID != a.ID || !bytes.Equal(content, []byte("hello")) {
		t.Fatalf("Content() = %+v, %q, %v", got, content, err)
	}
}

func TestAttachmentService_Errors(t *testing.T) {
	t.Parallel()

	f, ada, _ := commentFixture(t)

	_, err := f.attachments.Upload(t.Context(), ada.ID, "TRAX", 1, port.UploadAttachmentInput{FileName: "empty.txt"})
	wantField(t, err, "file", domain.FieldRequired)

	_, err = f.attachments.Upload(t.Context(), ada.ID, "TRAX", 1, port.UploadAttachmentInput{
		FileName: "big.bin", Content: make([]byte, domain.MaxAttachmentBytes+1),
	})
	wantField(t, err, "file", domain.FieldTooLong)

	_, err = f.attachments.Upload(t.Context(), ada.ID, "TRAX", 2, port.UploadAttachmentInput{FileName: "a", Content: []byte("x")})
	wantCode(t, err, domain.CodeIssueNotFound)

	_, _, err = f.attachments.Content(t.Context(), "TRAX", 1, domain.AttachmentID(uuid.New()))
	wantCode(t, err, domain.CodeAttachmentNotFound)

	_, err = f.attachments.List(t.Context(), "NOPE", 1)
	wantCode(t, err, domain.CodeProjectNotFound)
}

func TestAttachmentService_Delete(t *testing.T) {
	t.Parallel()

	f, ada, bob := commentFixture(t)
	a := f.addAttachment(t, ada.ID, "a.txt", "x")

	wantCode(t, f.attachments.Delete(t.Context(), bob.ID, "TRAX", 1, a.Attachment.ID), domain.CodeNotAttachmentUploader)

	if err := f.attachments.Delete(t.Context(), ada.ID, "TRAX", 1, a.Attachment.ID); err != nil {
		t.Fatal(err)
	}

	wantCode(t, f.attachments.Delete(t.Context(), ada.ID, "TRAX", 1, a.Attachment.ID), domain.CodeAttachmentNotFound)

	_, _, err := f.attachments.Content(t.Context(), "TRAX", 1, a.Attachment.ID)
	wantCode(t, err, domain.CodeAttachmentNotFound)
}

func TestAttachmentService_AttachmentOfOtherIssue(t *testing.T) {
	t.Parallel()

	f, ada, _ := commentFixture(t)
	f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "y"})
	a := f.addAttachment(t, ada.ID, "a.txt", "on issue 1")

	_, _, err := f.attachments.Content(t.Context(), "TRAX", 2, a.Attachment.ID)
	wantCode(t, err, domain.CodeAttachmentNotFound)

	wantCode(t, f.attachments.Delete(t.Context(), ada.ID, "TRAX", 2, a.Attachment.ID), domain.CodeAttachmentNotFound)
}

func TestAttachmentService_ArchivedIssue(t *testing.T) {
	t.Parallel()

	f, ada, _ := commentFixture(t)
	a := f.addAttachment(t, ada.ID, "a.txt", "before archiving")
	f.archive(t)

	_, err := f.attachments.Upload(t.Context(), ada.ID, "TRAX", 1, port.UploadAttachmentInput{FileName: "b", Content: []byte("x")})
	wantCode(t, err, domain.CodeIssueArchived)

	wantCode(t, f.attachments.Delete(t.Context(), ada.ID, "TRAX", 1, a.Attachment.ID), domain.CodeIssueArchived)

	if _, _, err := f.attachments.Content(t.Context(), "TRAX", 1, a.Attachment.ID); err != nil {
		t.Fatalf("Content() on archived issue: %v", err)
	}
}

func TestAttachmentService_AuditAndChanges(t *testing.T) {
	t.Parallel()

	f, ada, _ := commentFixture(t)
	f.changes.take()

	a := f.addAttachment(t, ada.ID, "a.txt", "abc")

	if err := f.attachments.Delete(t.Context(), ada.ID, "trax", 1, a.Attachment.ID); err != nil {
		t.Fatal(err)
	}

	want := domain.Change{Kind: domain.ChangeAttachments, Project: "TRAX", Issue: 1}
	if got := f.changes.take(); !slices.Equal(got, []domain.Change{want, want}) {
		t.Fatalf("changes = %+v, want two %+v", got, want)
	}

	// A rejected upload is neither recorded nor announced.
	if _, err := f.attachments.Upload(t.Context(), ada.ID, "TRAX", 1, port.UploadAttachmentInput{FileName: "x"}); err == nil {
		t.Fatal("empty upload accepted")
	}

	if got := f.changes.take(); len(got) != 0 {
		t.Fatalf("changes after failed upload = %+v", got)
	}

	log := f.auditLog(t)
	wantActions(t, log[:2], domain.AuditAttachmentDelete, domain.AuditAttachmentCreate)

	created, deleted := log[1], log[0]
	if created.TargetType != domain.AuditTargetAttachment || created.TargetID != a.Attachment.ID.String() ||
		created.TargetLabel != "TRAX-1" || *created.ActorID != ada.ID {
		t.Fatalf("create entry = %+v", created)
	}

	wantChange(t, created, "fileName", "", "a.txt")
	wantChange(t, created, "mediaType", "", "text/plain")
	wantChange(t, created, "size", "", "3")
	wantChange(t, deleted, "fileName", "a.txt", "")
}
