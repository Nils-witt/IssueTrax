package memory

import (
	"cmp"
	"context"
	"slices"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that TagRepository satisfies the driven port.
var _ port.TagRepository = (*TagRepository)(nil)

// TagRepository implements port.TagRepository on a Store.
type TagRepository struct{ s *Store }

// NewTagRepository returns a TagRepository backed by s.
func NewTagRepository(s *Store) *TagRepository { return &TagRepository{s: s} }

// ListByProject returns the project's tags ordered by name.
func (r *TagRepository) ListByProject(_ context.Context, project domain.ProjectID) ([]domain.Tag, error) {
	r.s.mu.RLock()

	tags := []domain.Tag{}

	for _, t := range r.s.data.tags {
		if t.ProjectID == project {
			tags = append(tags, t)
		}
	}

	r.s.mu.RUnlock()

	slices.SortFunc(tags, func(a, b domain.Tag) int { return cmp.Compare(a.Name, b.Name) })

	return tags, nil
}

// Get returns the tag with id in project.
func (r *TagRepository) Get(_ context.Context, project domain.ProjectID, id domain.TagID) (domain.Tag, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	t, ok := r.s.data.tags[id]
	if !ok || t.ProjectID != project {
		return domain.Tag{}, domain.ErrTagNotFound
	}

	return t, nil
}

// GetMany returns the tags with the given IDs, skipping unknown ones.
func (r *TagRepository) GetMany(_ context.Context, ids []domain.TagID) ([]domain.Tag, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	tags := make([]domain.Tag, 0, len(ids))

	for _, id := range ids {
		if t, ok := r.s.data.tags[id]; ok {
			tags = append(tags, t)
		}
	}

	return tags, nil
}

// FindByNames returns the project's tags with the given names, skipping unknown ones.
func (r *TagRepository) FindByNames(_ context.Context, project domain.ProjectID, names []domain.TagName) ([]domain.Tag, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	var tags []domain.Tag

	for _, t := range r.s.data.tags {
		if t.ProjectID == project && slices.Contains(names, t.Name) {
			tags = append(tags, t)
		}
	}

	return tags, nil
}

// Ensure inserts tag unless its project already has that name.
func (r *TagRepository) Ensure(_ context.Context, tag domain.Tag) (domain.Tag, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	for _, t := range r.s.data.tags {
		if t.ProjectID == tag.ProjectID && t.Name == tag.Name {
			return t, nil
		}
	}

	r.s.data.tags[tag.ID] = tag

	return tag, nil
}

// Update replaces an existing tag, rejecting duplicate names in its project.
func (r *TagRepository) Update(_ context.Context, tag domain.Tag) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	if _, ok := r.s.data.tags[tag.ID]; !ok {
		return domain.ErrTagNotFound
	}

	for _, t := range r.s.data.tags {
		if t.ID != tag.ID && t.ProjectID == tag.ProjectID && t.Name == tag.Name {
			return domain.ErrTagNameTaken
		}
	}

	r.s.data.tags[tag.ID] = tag

	return nil
}
