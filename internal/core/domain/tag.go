package domain

import (
	"hash/fnv"
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

// TagColor is a tag's display color as a lower-case "#rrggbb" hex string.
type TagColor string

var tagColorPattern = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// tagPalette holds the colors new tags are given, chosen so that white or
// black text stays readable on each.
var tagPalette = [...]TagColor{
	"#d73a4a", "#e99695", "#f9a03f", "#fbca04", "#0e8a16", "#5fbf7f",
	"#006b75", "#1d76db", "#5319e7", "#b60205", "#c5def5", "#d876e3",
}

// ParseTagColor trims and lower-cases s ("#1D76DB" → "#1d76db") and reports
// false when the result is not a "#rrggbb" hex color.
func ParseTagColor(s string) (TagColor, bool) {
	c := TagColor(strings.ToLower(strings.TrimSpace(s)))

	return c, tagColorPattern.MatchString(string(c))
}

// DefaultTagColor picks a palette color for name. The choice is stable, so
// a tag gets the same color whenever it is created under that name.
func DefaultTagColor(name TagName) TagColor {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))

	return tagPalette[h.Sum32()%uint32(len(tagPalette))]
}

// Tag labels issues within one project. Tags are created on first use and
// can be renamed, but never deleted.
type Tag struct {
	ID        TagID
	ProjectID ProjectID
	Name      TagName
	Color     TagColor
	CreatedAt time.Time
}

// NewTag builds a Tag from an already normalized name, colored with
// DefaultTagColor.
func NewTag(id TagID, project ProjectID, name TagName, now time.Time) Tag {
	return Tag{ID: id, ProjectID: project, Name: name, Color: DefaultTagColor(name), CreatedAt: now}
}

// TagChanges lists the mutable tag fields; nil means unchanged.
type TagChanges struct {
	Name  *string
	Color *string
}

// Apply normalizes and validates c and applies it to t. On error t is left
// unchanged; the validation error's fields are "name" and "color".
func (t *Tag) Apply(c TagChanges) error {
	fields := map[string]string{}
	next := *t

	if c.Name != nil {
		n, ok := NormalizeTagName(*c.Name)

		switch {
		case n == "":
			fields["name"] = FieldRequired
		case !ok:
			fields["name"] = FieldInvalid
		}

		next.Name = n
	}

	if c.Color != nil {
		col, ok := ParseTagColor(*c.Color)

		switch {
		case col == "":
			fields["color"] = FieldRequired
		case !ok:
			fields["color"] = FieldInvalid
		}

		next.Color = col
	}

	if len(fields) > 0 {
		return NewValidationError(fields)
	}

	*t = next

	return nil
}
