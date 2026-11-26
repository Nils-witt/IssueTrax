package domain

import (
	"errors"
	"maps"
	"slices"
	"strings"
)

// Error kinds. Every error produced by the core wraps exactly one of these, so
// adapters can classify failures with errors.Is.
var (
	// ErrNotFound means the requested entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrInvalid means input violates a validation rule.
	ErrInvalid = errors.New("invalid input")
	// ErrConflict means the change clashes with existing state (e.g. a duplicate name).
	ErrConflict = errors.New("conflict")
	// ErrIssueArchived means an archived issue was asked to change more than its status.
	ErrIssueArchived = errors.New("issue is archived")
	// ErrUnauthenticated means no valid session was presented.
	ErrUnauthenticated = errors.New("unauthenticated")
	// ErrForbidden means the caller may not perform the change (e.g. edit someone else's comment).
	ErrForbidden = errors.New("forbidden")
)

// Stable error codes. Clients (e.g. the SPA's translations) key off these.
const (
	CodeValidationFailed = "validation_failed"
	CodeProjectNotFound  = "project_not_found"
	CodeIssueNotFound    = "issue_not_found"
	CodeTagNotFound      = "tag_not_found"
	CodeUserNotFound     = "user_not_found"
	CodeCommentNotFound  = "comment_not_found"
	CodeProjectKeyTaken  = "project_key_taken"
	CodeProjectNameTaken = "project_name_taken"
	CodeTagNameTaken     = "tag_name_taken"
	CodeIssueArchived    = "issue_archived"
	CodeUnauthenticated  = "unauthenticated"
	CodeLoginFailed      = "login_failed"
	CodeNotCommentAuthor = "not_comment_author"

	CodeServiceAccountNotFound  = "service_account_not_found"
	CodeServiceAccountNameTaken = "service_account_name_taken"
	CodeUserIdentityTaken       = "user_identity_taken"
	CodeAPIKeyNotFound          = "api_key_not_found" //nolint:gosec // an error code, not a credential
	CodeInvalidAPIKey           = "invalid_api_key"
	CodeSessionRequired         = "session_required"
	CodeAdminRequired           = "admin_required"
)

// Stable per-field validation codes, used in Error.Fields.
const (
	FieldRequired   = "required"
	FieldTooLong    = "too_long"
	FieldInvalid    = "invalid"
	FieldUnknown    = "unknown"
	FieldOutOfRange = "out_of_range"
	FieldImmutable  = "immutable"
)

// Error is a classified domain error with a stable machine-readable code.
type Error struct {
	kind error
	// Code is a stable identifier such as "issue_archived".
	Code string
	// Message is a human-readable English description.
	Message string
	// Fields maps an input field name to a Field* code. Set only for
	// validation errors.
	Fields map[string]string
}

func (e *Error) Error() string {
	if len(e.Fields) == 0 {
		return e.Message
	}

	parts := make([]string, 0, len(e.Fields))
	for _, k := range slices.Sorted(maps.Keys(e.Fields)) {
		parts = append(parts, k+": "+e.Fields[k])
	}

	return e.Message + " (" + strings.Join(parts, ", ") + ")"
}

// Unwrap returns the error kind (ErrNotFound, ErrInvalid, ...).
func (e *Error) Unwrap() error { return e.kind }

// Predefined errors. Compare with errors.Is; they also match their kind.
var (
	ErrProjectNotFound  = &Error{kind: ErrNotFound, Code: CodeProjectNotFound, Message: "project not found"}
	ErrIssueNotFound    = &Error{kind: ErrNotFound, Code: CodeIssueNotFound, Message: "issue not found"}
	ErrTagNotFound      = &Error{kind: ErrNotFound, Code: CodeTagNotFound, Message: "tag not found"}
	ErrUserNotFound     = &Error{kind: ErrNotFound, Code: CodeUserNotFound, Message: "user not found"}
	ErrCommentNotFound  = &Error{kind: ErrNotFound, Code: CodeCommentNotFound, Message: "comment not found"}
	ErrProjectKeyTaken  = &Error{kind: ErrConflict, Code: CodeProjectKeyTaken, Message: "project key is already taken"}
	ErrProjectNameTaken = &Error{kind: ErrConflict, Code: CodeProjectNameTaken, Message: "project name is already taken"}
	ErrTagNameTaken     = &Error{kind: ErrConflict, Code: CodeTagNameTaken, Message: "tag name is already taken"}
	ErrArchived         = &Error{kind: ErrIssueArchived, Code: CodeIssueArchived, Message: "archived issues can only change their status"}
	ErrNoSession        = &Error{kind: ErrUnauthenticated, Code: CodeUnauthenticated, Message: "authentication required"}
	ErrLoginFailed      = &Error{kind: ErrUnauthenticated, Code: CodeLoginFailed, Message: "sign-in failed"}
	ErrNotCommentAuthor = &Error{kind: ErrForbidden, Code: CodeNotCommentAuthor, Message: "only the author can change a comment"}

	ErrServiceAccountNotFound  = &Error{kind: ErrNotFound, Code: CodeServiceAccountNotFound, Message: "service account not found"}
	ErrServiceAccountNameTaken = &Error{kind: ErrConflict, Code: CodeServiceAccountNameTaken, Message: "service account name is already taken"}
	// ErrUserIdentityTaken is returned by user repositories when another
	// user already has the same issuer and subject.
	ErrUserIdentityTaken = &Error{kind: ErrConflict, Code: CodeUserIdentityTaken, Message: "user identity is already taken"}
	ErrAPIKeyNotFound    = &Error{kind: ErrNotFound, Code: CodeAPIKeyNotFound, Message: "api key not found"}
	ErrInvalidAPIKey     = &Error{kind: ErrUnauthenticated, Code: CodeInvalidAPIKey, Message: "invalid, expired or revoked api key"}
	ErrSessionRequired   = &Error{kind: ErrForbidden, Code: CodeSessionRequired, Message: "this action requires a signed-in person, not an api key"}
	ErrAdminRequired     = &Error{kind: ErrForbidden, Code: CodeAdminRequired, Message: "this action requires an administrator"}
)

// NewValidationError returns an ErrInvalid error for the given field codes.
func NewValidationError(fields map[string]string) error {
	return &Error{kind: ErrInvalid, Code: CodeValidationFailed, Message: "validation failed", Fields: fields}
}

// fieldErrors collects validation failures; the first code per field wins.
type fieldErrors map[string]string

func (f fieldErrors) add(field, code string) {
	if _, ok := f[field]; !ok {
		f[field] = code
	}
}

func (f fieldErrors) err() error {
	if len(f) == 0 {
		return nil
	}

	return NewValidationError(f)
}
