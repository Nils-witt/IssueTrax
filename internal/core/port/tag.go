package port

import (
	"context"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// TagService is the driving port for tag use cases. Tags are created
// implicitly when assigned to an issue, so there is no Create.
type TagService interface {
	List(ctx context.Context, project domain.ProjectKey) ([]domain.Tag, error)
	Rename(ctx context.Context, project domain.ProjectKey, id domain.TagID, name string) (domain.Tag, error)
}

// TagRepository is the driven port for persisting tags.
//
// Get returns domain.ErrTagNotFound. Ensure inserts tag unless the project
// already has a tag of that name, and returns the stored tag either way.
// Update returns domain.ErrTagNameTaken on a clash. ListByProject orders by
// name. GetMany and FindByNames ignore unknown entries.
type TagRepository interface {
	ListByProject(ctx context.Context, project domain.ProjectID) ([]domain.Tag, error)
	Get(ctx context.Context, project domain.ProjectID, id domain.TagID) (domain.Tag, error)
	GetMany(ctx context.Context, ids []domain.TagID) ([]domain.Tag, error)
	FindByNames(ctx context.Context, project domain.ProjectID, names []domain.TagName) ([]domain.Tag, error)
	Ensure(ctx context.Context, tag domain.Tag) (domain.Tag, error)
	Update(ctx context.Context, tag domain.Tag) error
}
