package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestError_Error(t *testing.T) {
	t.Parallel()

	err := domain.NewValidationError(map[string]string{"title": domain.FieldRequired, "key": domain.FieldInvalid})

	if got, want := err.Error(), "validation failed (key: invalid, title: required)"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}

	if got, want := domain.ErrProjectNotFound.Error(), "project not found"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestError_Unwrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		kind error
	}{
		{name: "not found", err: domain.ErrIssueNotFound, kind: domain.ErrNotFound},
		{name: "conflict", err: domain.ErrTagNameTaken, kind: domain.ErrConflict},
		{name: "archived", err: domain.ErrArchived, kind: domain.ErrIssueArchived},
		{name: "unauthenticated", err: domain.ErrNoSession, kind: domain.ErrUnauthenticated},
		{name: "forbidden", err: domain.ErrNotCommentAuthor, kind: domain.ErrForbidden},
		{name: "validation", err: domain.NewValidationError(map[string]string{"x": "y"}), kind: domain.ErrInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wrapped := fmt.Errorf("context: %w", tt.err)

			if !errors.Is(wrapped, tt.kind) {
				t.Fatalf("errors.Is(%v, kind) = false", wrapped)
			}

			if !errors.Is(wrapped, tt.err) {
				t.Fatalf("errors.Is(%v, err) = false", wrapped)
			}
		})
	}
}
