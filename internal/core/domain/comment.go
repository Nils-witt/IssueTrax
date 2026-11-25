package domain

import (
	"time"

	"github.com/google/uuid"
)

// CommentID is a comment's internal identifier.
type CommentID uuid.UUID

func (id CommentID) String() string { return uuid.UUID(id).String() }

// Comment is a Markdown remark on an issue. Only its author may edit or
// delete it, and comments on archived issues cannot change.
type Comment struct {
	ID        CommentID
	IssueID   IssueID
	Body      string
	Author    UserID
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewComment validates body and builds a Comment by author on issue.
func NewComment(id CommentID, issue IssueID, author UserID, body string, now time.Time) (Comment, error) {
	f := fieldErrors{}
	c := Comment{
		ID:        id,
		IssueID:   issue,
		Body:      checkCommentBody(f, body),
		Author:    author,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := f.err(); err != nil {
		return Comment{}, err
	}

	return c, nil
}

// CheckAuthor returns ErrNotCommentAuthor unless user wrote c.
func (c Comment) CheckAuthor(user UserID) error {
	if c.Author != user {
		return ErrNotCommentAuthor
	}

	return nil
}

// Edit replaces the body on behalf of user. On error c is left unchanged;
// an unchanged body keeps UpdatedAt.
func (c *Comment) Edit(user UserID, body string, now time.Time) error {
	if err := c.CheckAuthor(user); err != nil {
		return err
	}

	f := fieldErrors{}

	body = checkCommentBody(f, body)
	if err := f.err(); err != nil {
		return err
	}

	if body != c.Body {
		c.Body = body
		c.UpdatedAt = now
	}

	return nil
}

// checkCommentBody trims s and records required/too_long for "body".
func checkCommentBody(f fieldErrors, s string) string {
	s = checkDescription(f, "body", s)
	if s == "" {
		f.add("body", FieldRequired)
	}

	return s
}
