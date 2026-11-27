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
	// StorageItemIDs restricts by linked storage item; StorageItemMode says all or any.
	StorageItemIDs  []domain.StorageItemID
	StorageItemMode MatchMode
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
	// StorageItems are linked storage items of any kind; each must exist
	// and not be deleted.
	StorageItems []domain.StorageItemID
}

// UpdateIssueInput is a partial update; nil fields stay unchanged. Tags,
// Assignees and StorageItems replace the whole set when non-nil. Newly
// linked storage items must not be deleted; already linked ones may stay.
type UpdateIssueInput struct {
	Title        *string
	Description  *string
	Status       *domain.IssueStatus
	Tags         *[]string
	Assignees    *[]domain.UserID
	StorageItems *[]domain.StorageItemID
}

// IssueView is an issue with its references resolved for presentation.
type IssueView struct {
	Issue      domain.Issue
	ProjectKey domain.ProjectKey
	Tags       []domain.Tag
	Assignees  []domain.User
	// StorageItems are the linked items, deleted ones included.
	StorageItems []domain.StorageItem
	CreatedBy    domain.User
}

// IssueService is the driving port for issue use cases.
type IssueService interface {
	Create(ctx context.Context, actor domain.UserID, project domain.ProjectKey, in CreateIssueInput) (IssueView, error)
	Get(ctx context.Context, project domain.ProjectKey, number int64) (IssueView, error)
	List(ctx context.Context, project domain.ProjectKey, filter IssueFilter, page domain.PageRequest) (domain.Page[IssueView], error)
	Update(ctx context.Context, actor domain.UserID, project domain.ProjectKey, number int64, in UpdateIssueInput) (IssueView, error)
	// ListByStorageItem returns one page of the issues of any project and
	// status linking item, newest first. An unknown item is
	// domain.ErrStorageItemNotFound.
	ListByStorageItem(ctx context.Context, item domain.StorageItemID, page domain.PageRequest) (domain.Page[IssueView], error)
}

// IssueRepository is the driven port for persisting issues.
//
// Get returns domain.ErrIssueNotFound. List receives a fully defaulted
// filter (explicit statuses, explicit match modes) and orders by number
// descending. ListByStorageItem spans all projects and statuses and orders
// by creation time descending, then ID.
type IssueRepository interface {
	Create(ctx context.Context, issue domain.Issue) error
	Update(ctx context.Context, issue domain.Issue) error
	Get(ctx context.Context, project domain.ProjectID, number int64) (domain.Issue, error)
	List(ctx context.Context, project domain.ProjectID, filter IssueFilter, page domain.PageRequest) (domain.Page[domain.Issue], error)
	ListByStorageItem(ctx context.Context, item domain.StorageItemID, page domain.PageRequest) (domain.Page[domain.Issue], error)
}
