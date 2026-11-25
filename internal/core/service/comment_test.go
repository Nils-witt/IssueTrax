package service_test

import (
	"testing"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// commentFixture has project TRAX with issue 1, created by ada.
func commentFixture(t *testing.T) (f *fixture, ada, bob domain.User) {
	t.Helper()

	f = newFixture(t)
	ada, bob = f.addUser(t, "ada"), f.addUser(t, "bob")
	f.addProject(t, "TRAX", "IssueTrax")
	f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "x"})

	return f, ada, bob
}

func (f *fixture) addComment(t *testing.T, actor domain.UserID, body string) port.CommentView {
	t.Helper()

	v, err := f.comments.Create(t.Context(), actor, "TRAX", 1, body)
	if err != nil {
		t.Fatalf("create comment %q: %v", body, err)
	}

	return v
}

func (f *fixture) archive(t *testing.T) {
	t.Helper()

	if _, err := f.issues.Update(t.Context(), "TRAX", 1, port.UpdateIssueInput{Status: new(domain.StatusArchived)}); err != nil {
		t.Fatal(err)
	}
}

func TestCommentService_CreateAndList(t *testing.T) {
	t.Parallel()

	f, ada, bob := commentFixture(t)

	empty, err := f.comments.List(t.Context(), "TRAX", 1)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("List() = %#v, %v; want empty non-nil", empty, err)
	}

	first := f.addComment(t, ada.ID, "  first  ")
	f.addComment(t, bob.ID, "second")

	if first.Comment.Body != "first" || first.Author.DisplayName != "ada" || !first.Comment.CreatedAt.Equal(testNow) {
		t.Fatalf("Create() = %+v", first)
	}

	list, err := f.comments.List(t.Context(), "trax", 1)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 2 || list[0].Author.ID != ada.ID || list[1].Author.ID != bob.ID {
		t.Fatalf("List() = %+v", list)
	}

	_, err = f.comments.Create(t.Context(), ada.ID, "TRAX", 1, " ")
	wantField(t, err, "body", domain.FieldRequired)

	_, err = f.comments.Create(t.Context(), ada.ID, "TRAX", 2, "x")
	wantCode(t, err, domain.CodeIssueNotFound)

	_, err = f.comments.List(t.Context(), "NOPE", 1)
	wantCode(t, err, domain.CodeProjectNotFound)
}

func TestCommentService_Update(t *testing.T) {
	t.Parallel()

	f, ada, bob := commentFixture(t)
	c := f.addComment(t, ada.ID, "first")

	got, err := f.comments.Update(t.Context(), ada.ID, "TRAX", 1, c.Comment.ID, "edited")
	if err != nil || got.Comment.Body != "edited" || got.Author.ID != ada.ID {
		t.Fatalf("Update() = %+v, %v", got, err)
	}

	_, err = f.comments.Update(t.Context(), bob.ID, "TRAX", 1, c.Comment.ID, "hijack")
	wantCode(t, err, domain.CodeNotCommentAuthor)

	_, err = f.comments.Update(t.Context(), ada.ID, "TRAX", 1, c.Comment.ID, "")
	wantField(t, err, "body", domain.FieldRequired)

	_, err = f.comments.Update(t.Context(), ada.ID, "TRAX", 1, domain.CommentID(uuid.New()), "x")
	wantCode(t, err, domain.CodeCommentNotFound)

	list, err := f.comments.List(t.Context(), "TRAX", 1)
	if err != nil || len(list) != 1 || list[0].Comment.Body != "edited" {
		t.Fatalf("List() after updates = %+v, %v", list, err)
	}
}

func TestCommentService_Delete(t *testing.T) {
	t.Parallel()

	f, ada, bob := commentFixture(t)
	c := f.addComment(t, ada.ID, "first")

	wantCode(t, f.comments.Delete(t.Context(), bob.ID, "TRAX", 1, c.Comment.ID), domain.CodeNotCommentAuthor)

	if err := f.comments.Delete(t.Context(), ada.ID, "TRAX", 1, c.Comment.ID); err != nil {
		t.Fatal(err)
	}

	wantCode(t, f.comments.Delete(t.Context(), ada.ID, "TRAX", 1, c.Comment.ID), domain.CodeCommentNotFound)
}

func TestCommentService_CommentOnOtherIssue(t *testing.T) {
	t.Parallel()

	f, ada, _ := commentFixture(t)
	f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "y"})
	c := f.addComment(t, ada.ID, "on issue 1")

	_, err := f.comments.Update(t.Context(), ada.ID, "TRAX", 2, c.Comment.ID, "moved?")
	wantCode(t, err, domain.CodeCommentNotFound)

	wantCode(t, f.comments.Delete(t.Context(), ada.ID, "TRAX", 2, c.Comment.ID), domain.CodeCommentNotFound)
}

func TestCommentService_ArchivedIssue(t *testing.T) {
	t.Parallel()

	f, ada, _ := commentFixture(t)
	c := f.addComment(t, ada.ID, "before archiving")
	f.archive(t)

	_, err := f.comments.Create(t.Context(), ada.ID, "TRAX", 1, "x")
	wantCode(t, err, domain.CodeIssueArchived)

	_, err = f.comments.Update(t.Context(), ada.ID, "TRAX", 1, c.Comment.ID, "x")
	wantCode(t, err, domain.CodeIssueArchived)

	wantCode(t, f.comments.Delete(t.Context(), ada.ID, "TRAX", 1, c.Comment.ID), domain.CodeIssueArchived)

	list, err := f.comments.List(t.Context(), "TRAX", 1)
	if err != nil || len(list) != 1 {
		t.Fatalf("List() on archived issue = %+v, %v", list, err)
	}
}
