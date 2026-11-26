package storagetest

import (
	"bytes"
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/service"
)

func attachment(t *testing.T, r service.Repositories, i domain.Issue, by domain.User, name string, content []byte, at time.Time) domain.Attachment {
	t.Helper()

	a := domain.Attachment{
		ID: domain.AttachmentID(newID()), IssueID: i.ID, FileName: name, MediaType: "application/octet-stream",
		Size: int64(len(content)), Uploader: by.ID, CreatedAt: at,
	}
	must(t, r.Attachments.Create(t.Context(), a, content))

	return a
}

func testAttachmentsLifecycle(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	p := project(t, r, "TRAX", "IssueTrax")
	ada := user(t, r, "ada")
	i, other := issue(t, r, p, ada, nil), issue(t, r, p, ada, nil)

	// Every byte value survives the round trip.
	content := make([]byte, 256)
	for b := range content {
		content[b] = byte(b)
	}

	a := attachment(t, r, i, ada, "Übersicht.bin", content, now)

	got, err := r.Attachments.Get(t.Context(), i.ID, a.ID)
	if err != nil || !equalAttachments(got, a) {
		t.Fatalf("Get() = %+v, %v; want %+v", got, err, a)
	}

	got, gotContent, err := r.Attachments.Content(t.Context(), i.ID, a.ID)
	if err != nil || !equalAttachments(got, a) || !bytes.Equal(gotContent, content) {
		t.Fatalf("Content() = %+v, %d bytes, %v; want %+v with %d bytes", got, len(gotContent), err, a, len(content))
	}

	_, err = r.Attachments.Get(t.Context(), other.ID, a.ID)
	wantErr(t, err, domain.ErrAttachmentNotFound)

	_, _, err = r.Attachments.Content(t.Context(), other.ID, a.ID)
	wantErr(t, err, domain.ErrAttachmentNotFound)

	must(t, r.Attachments.Delete(t.Context(), a.ID))

	_, _, err = r.Attachments.Content(t.Context(), i.ID, a.ID)
	wantErr(t, err, domain.ErrAttachmentNotFound)
	wantErr(t, r.Attachments.Delete(t.Context(), a.ID), domain.ErrAttachmentNotFound)
}

func testAttachmentsListByIssue(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	p := project(t, r, "TRAX", "IssueTrax")
	ada, bob := user(t, r, "ada"), user(t, r, "bob")
	i, other := issue(t, r, p, ada, nil), issue(t, r, p, ada, nil)

	second := attachment(t, r, i, bob, "second.txt", []byte("2"), now.Add(time.Minute))
	first := attachment(t, r, i, ada, "first.txt", []byte("1"), now)
	attachment(t, r, other, ada, "elsewhere.txt", []byte("x"), now)

	list, err := r.Attachments.ListByIssue(t.Context(), i.ID)
	must(t, err)

	if len(list) != 2 || !equalAttachments(list[0], first) || !equalAttachments(list[1], second) {
		t.Fatalf("ListByIssue() = %+v, want [first second]", list)
	}

	none, err := r.Attachments.ListByIssue(t.Context(), domain.IssueID(newID()))
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("ListByIssue(unknown) = %#v, %v; want empty non-nil", none, err)
	}
}

func equalAttachments(a, b domain.Attachment) bool {
	return a.ID == b.ID && a.IssueID == b.IssueID && a.FileName == b.FileName && a.MediaType == b.MediaType &&
		a.Size == b.Size && a.Uploader == b.Uploader && a.CreatedAt.Equal(b.CreatedAt)
}
