package domain

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ProjectID is a project's internal identifier.
type ProjectID uuid.UUID

func (id ProjectID) String() string { return uuid.UUID(id).String() }

// ProjectKey is a project's public, immutable short key, e.g. "TRAX".
type ProjectKey string

var projectKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)

// NormalizeProjectKey trims and upper-cases s. Use Valid to check the result.
func NormalizeProjectKey(s string) ProjectKey {
	return ProjectKey(strings.ToUpper(strings.TrimSpace(s)))
}

// Valid reports whether k has the required format (2–10 chars, A-Z0-9,
// starting with a letter).
func (k ProjectKey) Valid() bool { return projectKeyPattern.MatchString(string(k)) }

// Project groups issues and tags.
type Project struct {
	ID          ProjectID
	Key         ProjectKey
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewProjectParams are the inputs for NewProject.
type NewProjectParams struct {
	ID          ProjectID
	Key         string
	Name        string
	Description string
	Now         time.Time
}

// NewProject validates p and builds a Project.
func NewProject(p NewProjectParams) (Project, error) {
	f := fieldErrors{}

	key := NormalizeProjectKey(p.Key)

	switch {
	case key == "":
		f.add("key", FieldRequired)
	case !key.Valid():
		f.add("key", FieldInvalid)
	}

	project := Project{
		ID:          p.ID,
		Key:         key,
		Name:        checkRequiredText(f, "name", p.Name, maxProjectNameRunes),
		Description: checkDescription(f, "description", p.Description),
		CreatedAt:   p.Now,
		UpdatedAt:   p.Now,
	}

	if err := f.err(); err != nil {
		return Project{}, err
	}

	return project, nil
}

// ProjectChanges lists the mutable project fields; nil means unchanged.
type ProjectChanges struct {
	Name        *string
	Description *string
}

// Apply validates and applies c. On error p is left unchanged.
func (p *Project) Apply(c ProjectChanges, now time.Time) error {
	f := fieldErrors{}
	next := *p

	if c.Name != nil {
		next.Name = checkRequiredText(f, "name", *c.Name, maxProjectNameRunes)
	}

	if c.Description != nil {
		next.Description = checkDescription(f, "description", *c.Description)
	}

	if err := f.err(); err != nil {
		return err
	}

	if c.Name != nil || c.Description != nil {
		next.UpdatedAt = now
	}

	*p = next

	return nil
}
