package domain

import (
	"encoding/base64"
	"encoding/json"
	"time"
)

// Page sizes for paginated lists.
const (
	DefaultPageSize = 50
	MaxPageSize     = 100
)

// PageRequest asks for one page of a list. Cursor is opaque to callers; an
// empty cursor means the first page.
type PageRequest struct {
	Limit  int
	Cursor string
}

// NewPageRequest validates limit (0 means DefaultPageSize).
func NewPageRequest(limit int, cursor string) (PageRequest, error) {
	if limit == 0 {
		limit = DefaultPageSize
	}

	if limit < 1 || limit > MaxPageSize {
		return PageRequest{}, NewValidationError(map[string]string{"limit": FieldOutOfRange})
	}

	return PageRequest{Limit: limit, Cursor: cursor}, nil
}

// Page is one page of results. Total counts all matching items across pages;
// NextCursor is empty on the last page.
type Page[T any] struct {
	Items      []T
	Total      int64
	NextCursor string
}

// IssueCursor is the keyset position in an issue list (number descending).
type IssueCursor struct {
	Number int64 `json:"n"`
}

// ProjectCursor is the keyset position in a project list (lower(name), id ascending).
type ProjectCursor struct {
	Name string `json:"n"`
	ID   string `json:"i"`
}

// StorageCursor is the keyset position in a storage, item type or storage
// item list (lower(name), id ascending).
type StorageCursor struct {
	Name string `json:"n"`
	ID   string `json:"i"`
}

// AuditCursor is the keyset position in the audit log (at, id descending).
type AuditCursor struct {
	At time.Time `json:"t"`
	ID string    `json:"i"`
}

// LinkedIssueCursor is the keyset position in the list of issues linking a
// storage item (created at, id descending).
type LinkedIssueCursor struct {
	At time.Time `json:"t"`
	ID string    `json:"i"`
}

// EncodeCursor serializes a keyset position into an opaque cursor.
func EncodeCursor[T IssueCursor | ProjectCursor | StorageCursor | AuditCursor | LinkedIssueCursor](c T) string {
	b, err := json.Marshal(c)
	if err != nil {
		// All cursor types are plain structs of strings, ints and times.
		panic("encode cursor: " + err.Error())
	}

	return base64.RawURLEncoding.EncodeToString(b)
}

// DecodeCursor parses an opaque cursor; ok is false for an empty cursor.
// Malformed cursors yield a validation error on field "cursor".
func DecodeCursor[T IssueCursor | ProjectCursor | StorageCursor | AuditCursor | LinkedIssueCursor](s string) (c T, ok bool, err error) {
	if s == "" {
		return c, false, nil
	}

	b, err := base64.RawURLEncoding.DecodeString(s)
	if err == nil {
		err = json.Unmarshal(b, &c)
	}

	if err != nil {
		return c, false, NewValidationError(map[string]string{"cursor": FieldInvalid})
	}

	return c, true, nil
}
