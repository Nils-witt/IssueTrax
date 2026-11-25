package service

import (
	"context"
	"fmt"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that TagService satisfies the driving port.
var _ port.TagService = (*TagService)(nil)

// TagService implements port.TagService.
type TagService struct {
	repos Repositories
}

// NewTagService returns a TagService backed by repos.
func NewTagService(repos Repositories) *TagService {
	return &TagService{repos: repos}
}

// List returns the project's tags ordered by name.
func (s *TagService) List(ctx context.Context, key domain.ProjectKey) ([]domain.Tag, error) {
	project, err := projectByKey(ctx, s.repos.Projects, key)
	if err != nil {
		return nil, err
	}

	tags, err := s.repos.Tags.ListByProject(ctx, project.ID)
	if err != nil {
		return nil, fmt.Errorf("list tags in %q: %w", project.Key, err)
	}

	return tags, nil
}

// Update renames and/or recolors a tag. Issues keep the tag.
func (s *TagService) Update(ctx context.Context, key domain.ProjectKey, id domain.TagID, in port.UpdateTagInput) (domain.Tag, error) {
	var tag domain.Tag

	err := s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		project, err := projectByKey(ctx, s.repos.Projects, key)
		if err != nil {
			return err
		}

		tag, err = s.repos.Tags.Get(ctx, project.ID, id)
		if err != nil {
			return fmt.Errorf("get tag %s: %w", id, err)
		}

		if err := tag.Apply(domain.TagChanges{Name: in.Name, Color: in.Color}); err != nil {
			return err
		}

		if err := s.repos.Tags.Update(ctx, tag); err != nil {
			return fmt.Errorf("update tag %s: %w", id, err)
		}

		return nil
	})
	if err != nil {
		return domain.Tag{}, err
	}

	return tag, nil
}
