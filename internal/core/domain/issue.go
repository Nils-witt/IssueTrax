// Package domain holds IssueTrax's core business types and rules. It has no
// dependencies on transport, storage or any other adapter.
package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// IssueID uniquely identifies an Issue.
type IssueID string

// IssueStatus is the lifecycle state of an Issue.
type IssueStatus string

// Known issue statuses.
const (
	StatusOpen   IssueStatus = "open"
	StatusClosed IssueStatus = "closed"
)

// maxTitleLength bounds an issue title, in runes.
const maxTitleLength = 200

var (
	// ErrIssueNotFound is returned when no issue exists for a given ID.
	ErrIssueNotFound = errors.New("issue not found")
	// ErrInvalidIssue is returned when issue data violates a domain rule.
	ErrInvalidIssue = errors.New("invalid issue")
)

// Issue is a single tracked work item.
type Issue struct {
	ID          IssueID
	Title       string
	Description string
	Status      IssueStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewIssue builds a new open Issue, validating its fields.
func NewIssue(id IssueID, title, description string, now time.Time) (Issue, error) {
	title = strings.TrimSpace(title)

	if title == "" {
		return Issue{}, fmt.Errorf("%w: title is required", ErrInvalidIssue)
	}

	if len([]rune(title)) > maxTitleLength {
		return Issue{}, fmt.Errorf("%w: title exceeds %d characters", ErrInvalidIssue, maxTitleLength)
	}

	return Issue{
		ID:          id,
		Title:       title,
		Description: strings.TrimSpace(description),
		Status:      StatusOpen,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}
