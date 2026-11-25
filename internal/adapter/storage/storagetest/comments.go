package storagetest

import (
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/service"
)

func comment(t *testing.T, r service.Repositories, i domain.Issue, by domain.User, body string, at time.Time) domain.Comment {
	t.Helper()

	c := domain.Comment{ID: domain.CommentID(newID()), IssueID: i.ID, Body: body, Author: by.ID, CreatedAt: at, UpdatedAt: at}
	must(t, r.Comments.Create(t.Context(), c))

	return c
}

func testCommentsLifecycle(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	p := project(t, r, "TRAX", "IssueTrax")
	ada := user(t, r, "ada")
	i, other := issue(t, r, p, ada, nil), issue(t, r, p, ada, nil)
	c := comment(t, r, i, ada, "first", now)

	got, err := r.Comments.Get(t.Context(), i.ID, c.ID)
	if err != nil || !equalComments(got, c) {
		t.Fatalf("Get() = %+v, %v; want %+v", got, err, c)
	}

	_, err = r.Comments.Get(t.Context(), other.ID, c.ID)
	wantErr(t, err, domain.ErrCommentNotFound)

	c.Body, c.UpdatedAt = "edited", now.Add(time.Minute)
	must(t, r.Comments.Update(t.Context(), c))

	got, err = r.Comments.Get(t.Context(), i.ID, c.ID)
	if err != nil || !equalComments(got, c) {
		t.Fatalf("after Update() = %+v, %v; want %+v", got, err, c)
	}

	must(t, r.Comments.Delete(t.Context(), c.ID))

	_, err = r.Comments.Get(t.Context(), i.ID, c.ID)
	wantErr(t, err, domain.ErrCommentNotFound)
	wantErr(t, r.Comments.Delete(t.Context(), c.ID), domain.ErrCommentNotFound)
	wantErr(t, r.Comments.Update(t.Context(), c), domain.ErrCommentNotFound)
}

func testCommentsListByIssue(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	p := project(t, r, "TRAX", "IssueTrax")
	ada, bob := user(t, r, "ada"), user(t, r, "bob")
	i, other := issue(t, r, p, ada, nil), issue(t, r, p, ada, nil)

	second := comment(t, r, i, bob, "second", now.Add(time.Minute))
	first := comment(t, r, i, ada, "first", now)
	comment(t, r, other, ada, "elsewhere", now)

	list, err := r.Comments.ListByIssue(t.Context(), i.ID)
	must(t, err)

	if len(list) != 2 || !equalComments(list[0], first) || !equalComments(list[1], second) {
		t.Fatalf("ListByIssue() = %+v, want [first second]", list)
	}

	none, err := r.Comments.ListByIssue(t.Context(), domain.IssueID(newID()))
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("ListByIssue(unknown) = %#v, %v; want empty non-nil", none, err)
	}
}

func equalComments(a, b domain.Comment) bool {
	return a.ID == b.ID && a.IssueID == b.IssueID && a.Body == b.Body && a.Author == b.Author &&
		a.CreatedAt.Equal(b.CreatedAt) && a.UpdatedAt.Equal(b.UpdatedAt)
}
