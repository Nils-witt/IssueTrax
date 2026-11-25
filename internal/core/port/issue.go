// Package port declares the boundaries of the IssueTrax core: driving ports
// (use cases the core offers to inbound adapters such as the HTTP API) and
// driven ports (capabilities the core needs from outbound adapters such as
// storage and the identity provider).
package port

import (
	"context"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// MatchMode says how a multi-value filter combines its values.
type MatchMode string

// Match modes. The zero value behaves like MatchAll.
const (
	MatchAll MatchMode = "all"
	MatchAny MatchMode = "any"
)

// IssueFilter narrows an issue list. Different criteria combine with AND.
type IssueFilter struct {
	// Statuses to include. Empty means every status except archived.
	Statuses []domain.IssueStatus
	// TagIDs restricts by tag; TagMode says whether an issue needs all or any of them.
	TagIDs  []domain.TagID
	TagMode MatchMode
	// AssigneeIDs restricts by assignee; AssigneeMode says all or any.
	AssigneeIDs  []domain.UserID
	AssigneeMode MatchMode
	// Query is a case-insensitive substring searched in title and description.
	Query string
}

// CreateIssueInput carries the data for a new issue.
type CreateIssueInput struct {
	Title       string
	Description string
	// Status defaults to open when empty.
	Status domain.IssueStatus
	// Tags are names; unknown names are created in the project.
	Tags      []string
	Assignees []domain.UserID
}

// UpdateIssueInput is a partial update; nil fields stay unchanged. Tags and
// Assignees replace the whole set when non-nil.
type UpdateIssueInput struct {
	Title       *string
	Description *string
	Status      *domain.IssueStatus
	Tags        *[]string
	Assignees   *[]domain.UserID
}

// IssueView is an issue with its references resolved for presentation.
type IssueView struct {
	Issue      domain.Issue
	ProjectKey domain.ProjectKey
	Tags       []domain.Tag
	Assignees  []domain.User
	CreatedBy  domain.User
}

// IssueService is the driving port for issue use cases.
type IssueService interface {
	Create(ctx context.Context, actor domain.UserID, project domain.ProjectKey, in CreateIssueInput) (IssueView, error)
	Get(ctx context.Context, project domain.ProjectKey, number int64) (IssueView, error)
	List(ctx context.Context, project domain.ProjectKey, filter IssueFilter, page domain.PageRequest) (domain.Page[IssueView], error)
	Update(ctx context.Context, project domain.ProjectKey, number int64, in UpdateIssueInput) (IssueView, error)
}

// IssueRepository is the driven port for persisting issues.
//
// Get returns domain.ErrIssueNotFound. List receives a fully defaulted
// filter (explicit statuses, explicit match modes) and orders by number
// descending.
type IssueRepository interface {
	Create(ctx context.Context, issue domain.Issue) error
	Update(ctx context.Context, issue domain.Issue) error
	Get(ctx context.Context, project domain.ProjectID, number int64) (domain.Issue, error)
	List(ctx context.Context, project domain.ProjectID, filter IssueFilter, page domain.PageRequest) (domain.Page[domain.Issue], error)
}
