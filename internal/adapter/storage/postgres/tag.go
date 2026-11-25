package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nilswitt.dev/issuetrax/internal/adapter/storage/postgres/sqlc"
	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that TagRepository satisfies the driven port.
var _ port.TagRepository = (*TagRepository)(nil)

// TagRepository implements port.TagRepository.
type TagRepository struct{ db *DB }

// NewTagRepository returns a TagRepository on db.
func NewTagRepository(db *DB) *TagRepository { return &TagRepository{db: db} }

// ListByProject returns the project's tags ordered by name.
func (r *TagRepository) ListByProject(ctx context.Context, project domain.ProjectID) ([]domain.Tag, error) {
	rows, err := r.db.queries(ctx).ListTagsByProject(ctx, uuid.UUID(project))
	if err != nil {
		return nil, fmt.Errorf("select tags: %w", err)
	}

	return toTags(rows), nil
}

// Get returns the tag with id in project.
func (r *TagRepository) Get(ctx context.Context, project domain.ProjectID, id domain.TagID) (domain.Tag, error) {
	row, err := r.db.queries(ctx).GetTag(ctx, sqlc.GetTagParams{ProjectID: uuid.UUID(project), ID: uuid.UUID(id)})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Tag{}, domain.ErrTagNotFound
	}

	if err != nil {
		return domain.Tag{}, fmt.Errorf("select tag: %w", err)
	}

	return toTag(row), nil
}

// GetMany returns the tags with the given IDs, skipping unknown ones.
func (r *TagRepository) GetMany(ctx context.Context, ids []domain.TagID) ([]domain.Tag, error) {
	if len(ids) == 0 {
		return []domain.Tag{}, nil
	}

	rows, err := r.db.queries(ctx).GetTags(ctx, uuids(ids))
	if err != nil {
		return nil, fmt.Errorf("select tags: %w", err)
	}

	return toTags(rows), nil
}

// FindByNames returns the project's tags with the given names, skipping unknown ones.
func (r *TagRepository) FindByNames(ctx context.Context, project domain.ProjectID, names []domain.TagName) ([]domain.Tag, error) {
	raw := make([]string, 0, len(names))
	for _, n := range names {
		raw = append(raw, string(n))
	}

	rows, err := r.db.queries(ctx).FindTagsByNames(ctx, sqlc.FindTagsByNamesParams{ProjectID: uuid.UUID(project), Names: raw})
	if err != nil {
		return nil, fmt.Errorf("select tags by name: %w", err)
	}

	return toTags(rows), nil
}

// Ensure inserts tag unless its project already has that name and returns
// the stored tag. Concurrent inserts of the same name converge on one row.
func (r *TagRepository) Ensure(ctx context.Context, tag domain.Tag) (domain.Tag, error) {
	q := r.db.queries(ctx)

	rows, err := q.InsertTagIfAbsent(ctx, sqlc.InsertTagIfAbsentParams{
		ID:        uuid.UUID(tag.ID),
		ProjectID: uuid.UUID(tag.ProjectID),
		Name:      string(tag.Name),
		CreatedAt: tag.CreatedAt,
	})
	if err != nil {
		return domain.Tag{}, fmt.Errorf("insert tag: %w", err)
	}

	if len(rows) == 1 {
		return toTag(rows[0]), nil
	}

	row, err := q.GetTagByName(ctx, sqlc.GetTagByNameParams{ProjectID: uuid.UUID(tag.ProjectID), Name: string(tag.Name)})
	if err != nil {
		return domain.Tag{}, fmt.Errorf("select existing tag: %w", err)
	}

	return toTag(row), nil
}

// Update stores tag's name.
func (r *TagRepository) Update(ctx context.Context, tag domain.Tag) error {
	n, err := r.db.queries(ctx).UpdateTag(ctx, sqlc.UpdateTagParams{ID: uuid.UUID(tag.ID), Name: string(tag.Name)})

	switch {
	case isUniqueViolation(err, "tags_project_name_key"):
		return domain.ErrTagNameTaken
	case err != nil:
		return fmt.Errorf("update tag: %w", err)
	case n == 0:
		return domain.ErrTagNotFound
	}

	return nil
}

func toTag(row sqlc.Tag) domain.Tag {
	return domain.Tag{
		ID:        domain.TagID(row.ID),
		ProjectID: domain.ProjectID(row.ProjectID),
		Name:      domain.TagName(row.Name),
		CreatedAt: row.CreatedAt.UTC(),
	}
}

func toTags(rows []sqlc.Tag) []domain.Tag {
	tags := make([]domain.Tag, 0, len(rows))
	for _, row := range rows {
		tags = append(tags, toTag(row))
	}

	return tags
}
