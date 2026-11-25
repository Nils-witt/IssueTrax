package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestNewComment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		body   string
		want   string
		fields map[string]string
	}{
		{name: "trims", body: "  **looks good**\n", want: "**looks good**"},
		{name: "empty", body: "", fields: map[string]string{"body": domain.FieldRequired}},
		{name: "blank", body: " \n\t", fields: map[string]string{"body": domain.FieldRequired}},
		{name: "max size", body: strings.Repeat("a", domain.MaxDescriptionBytes), want: strings.Repeat("a", domain.MaxDescriptionBytes)},
		{name: "too long", body: strings.Repeat("a", domain.MaxDescriptionBytes+1), fields: map[string]string{"body": domain.FieldTooLong}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
			author := domain.UserID(uuid.New())

			c, err := domain.NewComment(domain.CommentID(uuid.New()), domain.IssueID(uuid.New()), author, tt.body, now)
			wantFields(t, err, tt.fields)

			if err != nil {
				return
			}

			if c.Body != tt.want || c.Author != author || !c.CreatedAt.Equal(now) || !c.UpdatedAt.Equal(now) {
				t.Fatalf("NewComment() = %+v", c)
			}
		})
	}
}

func TestComment_Edit(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	later := created.Add(time.Hour)
	author := domain.UserID(uuid.New())

	newComment := func(t *testing.T) domain.Comment {
		t.Helper()

		c, err := domain.NewComment(domain.CommentID(uuid.New()), domain.IssueID(uuid.New()), author, "first", created)
		if err != nil {
			t.Fatal(err)
		}

		return c
	}

	t.Run("author edits", func(t *testing.T) {
		t.Parallel()

		c := newComment(t)
		if err := c.Edit(author, " second ", later); err != nil {
			t.Fatal(err)
		}

		if c.Body != "second" || !c.UpdatedAt.Equal(later) || !c.CreatedAt.Equal(created) {
			t.Fatalf("after Edit() = %+v", c)
		}
	})

	t.Run("unchanged body keeps timestamp", func(t *testing.T) {
		t.Parallel()

		c := newComment(t)
		if err := c.Edit(author, "first ", later); err != nil {
			t.Fatal(err)
		}

		if !c.UpdatedAt.Equal(created) {
			t.Fatalf("UpdatedAt = %v, want %v", c.UpdatedAt, created)
		}
	})

	t.Run("someone else", func(t *testing.T) {
		t.Parallel()

		c := newComment(t)
		before := c

		if err := c.Edit(domain.UserID(uuid.New()), "hijack", later); !errors.Is(err, domain.ErrNotCommentAuthor) {
			t.Fatalf("Edit() = %v, want ErrNotCommentAuthor", err)
		}

		if c != before {
			t.Fatalf("comment changed on error: %+v", c)
		}
	})

	t.Run("invalid body", func(t *testing.T) {
		t.Parallel()

		c := newComment(t)
		before := c

		wantFields(t, c.Edit(author, "  ", later), map[string]string{"body": domain.FieldRequired})

		if c != before {
			t.Fatalf("comment changed on error: %+v", c)
		}
	})
}
