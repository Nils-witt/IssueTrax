package port

import (
	"context"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// CreateProjectInput carries the data for a new project.
type CreateProjectInput struct {
	Key         string
	Name        string
	Description string
}

// UpdateProjectInput is a partial update; nil fields stay unchanged. The key
// is immutable and therefore absent.
type UpdateProjectInput struct {
	Name        *string
	Description *string
}

// ProjectService is the driving port for project use cases.
type ProjectService interface {
	Create(ctx context.Context, actor domain.UserID, in CreateProjectInput) (domain.Project, error)
	Get(ctx context.Context, key domain.ProjectKey) (domain.Project, error)
	List(ctx context.Context, page domain.PageRequest) (domain.Page[domain.Project], error)
	Update(ctx context.Context, actor domain.UserID, key domain.ProjectKey, in UpdateProjectInput) (domain.Project, error)
}

// ProjectRepository is the driven port for persisting projects.
//
// Create returns domain.ErrProjectKeyTaken or domain.ErrProjectNameTaken on
// clashes (names compare case-insensitively); Update returns
// domain.ErrProjectNameTaken. GetByKey returns domain.ErrProjectNotFound.
// List orders by lower(name), then ID. AllocateIssueNumber returns the next
// per-project issue number and must run inside a transaction so the number
// is released again if the issue is not created. GetMany returns the known
// projects among ids in no particular order and skips unknown ones.
type ProjectRepository interface {
	Create(ctx context.Context, project domain.Project) error
	Update(ctx context.Context, project domain.Project) error
	GetByKey(ctx context.Context, key domain.ProjectKey) (domain.Project, error)
	GetMany(ctx context.Context, ids []domain.ProjectID) ([]domain.Project, error)
	List(ctx context.Context, page domain.PageRequest) (domain.Page[domain.Project], error)
	AllocateIssueNumber(ctx context.Context, project domain.ProjectID) (int64, error)
}
