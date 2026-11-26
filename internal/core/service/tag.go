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
	opts  options
}

// NewTagService returns a TagService backed by repos.
func NewTagService(repos Repositories, opts ...Option) *TagService {
	return &TagService{repos: repos, opts: newOptions(opts)}
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
func (s *TagService) Update(
	ctx context.Context, actor domain.UserID, key domain.ProjectKey, id domain.TagID, in port.UpdateTagInput,
) (domain.Tag, error) {
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

		before := tag

		if err := tag.Apply(domain.TagChanges{Name: in.Name, Color: in.Color}); err != nil {
			return err
		}

		if err := s.repos.Tags.Update(ctx, tag); err != nil {
			return fmt.Errorf("update tag %s: %w", id, err)
		}

		changes := domain.AuditChanges{}
		changes.Set("name", string(before.Name), string(tag.Name))
		changes.Set("color", string(before.Color), string(tag.Color))

		if len(changes) == 0 {
			return nil
		}

		return record(ctx, s.repos, s.opts, auditEvent{
			actor: &actor, action: domain.AuditTagUpdate, changes: changes,
			targetType: domain.AuditTargetTag, targetID: tag.ID.String(), targetLabel: string(project.Key) + ": " + string(tag.Name),
		})
	})
	if err != nil {
		return domain.Tag{}, err
	}

	return tag, nil
}
