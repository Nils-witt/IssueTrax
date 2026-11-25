package domain

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TagID is a tag's internal identifier.
type TagID uuid.UUID

func (id TagID) String() string { return uuid.UUID(id).String() }

// TagName is a normalized tag slug, unique within a project.
type TagName string

var (
	tagNamePattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,29}$`)
	tagSeparatorRuns = regexp.MustCompile(`[\s_]+`)
)

// NormalizeTagName trims and lower-cases s and turns runs of whitespace or
// underscores into a single "-" ("Needs Review" → "needs-review"). It
// reports false when the result is not a valid slug.
func NormalizeTagName(s string) (TagName, bool) {
	n := strings.ToLower(strings.TrimSpace(s))
	n = tagSeparatorRuns.ReplaceAllString(n, "-")

	return TagName(n), tagNamePattern.MatchString(n)
}

// Tag labels issues within one project. Tags are created on first use and
// can be renamed, but never deleted.
type Tag struct {
	ID        TagID
	ProjectID ProjectID
	Name      TagName
	CreatedAt time.Time
}

// NewTag builds a Tag from an already normalized name.
func NewTag(id TagID, project ProjectID, name TagName, now time.Time) Tag {
	return Tag{ID: id, ProjectID: project, Name: name, CreatedAt: now}
}

// Rename normalizes and sets a new name. The error's field is "name".
func (t *Tag) Rename(name string) error {
	n, ok := NormalizeTagName(name)

	switch {
	case n == "":
		return NewValidationError(map[string]string{"name": FieldRequired})
	case !ok:
		return NewValidationError(map[string]string{"name": FieldInvalid})
	}

	t.Name = n

	return nil
}
