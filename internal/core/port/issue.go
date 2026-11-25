// Package port declares the boundaries of the IssueTrax core: driving ports
// (use cases the core offers to inbound adapters such as the HTTP API) and
// driven ports (capabilities the core needs from outbound adapters such as
// storage).
package port

import (
	"context"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// CreateIssueInput carries the data needed to open a new issue.
type CreateIssueInput struct {
	Title       string
	Description string
}

// IssueService is the driving port for issue use cases.
type IssueService interface {
	Create(ctx context.Context, in CreateIssueInput) (domain.Issue, error)
	Get(ctx context.Context, id domain.IssueID) (domain.Issue, error)
	List(ctx context.Context) ([]domain.Issue, error)
}

// IssueRepository is the driven port for persisting issues.
// Get returns domain.ErrIssueNotFound when the issue does not exist.
type IssueRepository interface {
	Save(ctx context.Context, issue domain.Issue) error
	Get(ctx context.Context, id domain.IssueID) (domain.Issue, error)
	List(ctx context.Context) ([]domain.Issue, error)
}
