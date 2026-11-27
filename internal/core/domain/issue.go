// Package domain holds IssueTrax's core business types and rules. It has no
// dependencies on transport, storage or any other adapter.
package domain

import (
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// IssueID is an issue's internal identifier.
type IssueID uuid.UUID

func (id IssueID) String() string { return uuid.UUID(id).String() }

// IssueStatus is the lifecycle state of an Issue.
type IssueStatus string

// Known issue statuses. Any status may change to any other.
const (
	StatusDraft    IssueStatus = "draft"
	StatusOpen     IssueStatus = "open"
	StatusClosed   IssueStatus = "closed"
	StatusArchived IssueStatus = "archived"
)

// IssueStatuses lists every valid status in lifecycle order.
func IssueStatuses() []IssueStatus {
	return []IssueStatus{StatusDraft, StatusOpen, StatusClosed, StatusArchived}
}

// Valid reports whether s is a known status.
func (s IssueStatus) Valid() bool { return slices.Contains(IssueStatuses(), s) }

// Issue is a single tracked work item inside a project.
type Issue struct {
	ID          IssueID
	ProjectID   ProjectID
	Number      int64
	Title       string
	Description string
	Status      IssueStatus
	Assignees   []UserID
	Tags        []TagID
	// StorageItems are the storage items (of any kind) the issue concerns.
	StorageItems []StorageItemID
	CreatedBy    UserID
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Key returns the readable identifier, e.g. "TRAX-42".
func (i Issue) Key(project ProjectKey) string {
	return string(project) + "-" + strconv.FormatInt(i.Number, 10)
}

// NewIssueParams are the inputs for NewIssue.
type NewIssueParams struct {
	ID          IssueID
	ProjectID   ProjectID
	Number      int64
	Title       string
	Description string
	// Status defaults to StatusOpen when empty.
	Status       IssueStatus
	Assignees    []UserID
	Tags         []TagID
	StorageItems []StorageItemID
	CreatedBy    UserID
	Now          time.Time
}

// NewIssue validates p and builds an Issue.
func NewIssue(p NewIssueParams) (Issue, error) {
	f := fieldErrors{}

	status := p.Status
	if status == "" {
		status = StatusOpen
	}

	if !status.Valid() {
		f.add("status", FieldInvalid)
	}

	issue := Issue{
		ID:           p.ID,
		ProjectID:    p.ProjectID,
		Number:       p.Number,
		Title:        checkRequiredText(f, "title", p.Title, maxTitleRunes),
		Description:  checkDescription(f, "description", p.Description),
		Status:       status,
		Assignees:    dedupe(p.Assignees),
		Tags:         dedupe(p.Tags),
		StorageItems: dedupe(p.StorageItems),
		CreatedBy:    p.CreatedBy,
		CreatedAt:    p.Now,
		UpdatedAt:    p.Now,
	}

	if err := f.err(); err != nil {
		return Issue{}, err
	}

	return issue, nil
}

// IssueChanges lists the mutable issue fields; nil means unchanged. Tags,
// Assignees and StorageItems replace the whole set when non-nil.
type IssueChanges struct {
	Title        *string
	Description  *string
	Status       *IssueStatus
	Tags         *[]TagID
	Assignees    *[]UserID
	StorageItems *[]StorageItemID
}

// StatusOnly reports whether c changes nothing but (possibly) the status.
func (c IssueChanges) StatusOnly() bool {
	return c.Title == nil && c.Description == nil && c.Tags == nil && c.Assignees == nil && c.StorageItems == nil
}

// CheckEditable returns ErrArchived when i is archived and a change would
// touch more than the status. An archived issue must be unarchived on its own
// before anything else changes.
func (i Issue) CheckEditable(statusOnly bool) error {
	if i.Status == StatusArchived && !statusOnly {
		return ErrArchived
	}

	return nil
}

// Apply validates and applies c. On error i is left unchanged.
func (i *Issue) Apply(c IssueChanges, now time.Time) error {
	if err := i.CheckEditable(c.StatusOnly()); err != nil {
		return err
	}

	f := fieldErrors{}
	next := *i
	changed := false

	if c.Title != nil {
		next.Title = checkRequiredText(f, "title", *c.Title, maxTitleRunes)
		changed = true
	}

	if c.Description != nil {
		next.Description = checkDescription(f, "description", *c.Description)
		changed = true
	}

	if c.Status != nil {
		if !c.Status.Valid() {
			f.add("status", FieldInvalid)
		}

		next.Status = *c.Status
		changed = true
	}

	if c.Tags != nil {
		next.Tags = dedupe(*c.Tags)
		changed = true
	}

	if c.Assignees != nil {
		next.Assignees = dedupe(*c.Assignees)
		changed = true
	}

	if c.StorageItems != nil {
		next.StorageItems = dedupe(*c.StorageItems)
		changed = true
	}

	if err := f.err(); err != nil {
		return err
	}

	if changed {
		next.UpdatedAt = now
	}

	*i = next

	return nil
}
