package memory

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"strings"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that ProjectRepository satisfies the driven port.
var _ port.ProjectRepository = (*ProjectRepository)(nil)

// ProjectRepository implements port.ProjectRepository on a Store.
type ProjectRepository struct{ s *Store }

// NewProjectRepository returns a ProjectRepository backed by s.
func NewProjectRepository(s *Store) *ProjectRepository { return &ProjectRepository{s: s} }

// Create inserts project, rejecting duplicate keys and names.
func (r *ProjectRepository) Create(_ context.Context, project domain.Project) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	for _, p := range r.s.data.projects {
		if p.Key == project.Key {
			return domain.ErrProjectKeyTaken
		}

		if sameName(p.Name, project.Name) {
			return domain.ErrProjectNameTaken
		}
	}

	r.s.data.projects[project.ID] = project
	r.s.data.nextNumber[project.ID] = 1

	return nil
}

// Update replaces an existing project, rejecting duplicate names.
func (r *ProjectRepository) Update(_ context.Context, project domain.Project) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	if _, ok := r.s.data.projects[project.ID]; !ok {
		return domain.ErrProjectNotFound
	}

	for _, p := range r.s.data.projects {
		if p.ID != project.ID && sameName(p.Name, project.Name) {
			return domain.ErrProjectNameTaken
		}
	}

	r.s.data.projects[project.ID] = project

	return nil
}

// GetByKey returns the project with key.
func (r *ProjectRepository) GetByKey(_ context.Context, key domain.ProjectKey) (domain.Project, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	for _, p := range r.s.data.projects {
		if p.Key == key {
			return p, nil
		}
	}

	return domain.Project{}, domain.ErrProjectNotFound
}

// List returns one page of projects ordered by lower(name), then ID.
func (r *ProjectRepository) List(_ context.Context, page domain.PageRequest) (domain.Page[domain.Project], error) {
	cursor, hasCursor, err := domain.DecodeCursor[domain.ProjectCursor](page.Cursor)
	if err != nil {
		return domain.Page[domain.Project]{}, err
	}

	r.s.mu.RLock()
	all := slices.Collect(maps.Values(r.s.data.projects))
	r.s.mu.RUnlock()

	slices.SortFunc(all, func(a, b domain.Project) int {
		return cmp.Or(
			cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)),
			cmp.Compare(a.ID.String(), b.ID.String()),
		)
	})

	after := all
	if hasCursor {
		after = slices.DeleteFunc(slices.Clone(all), func(p domain.Project) bool {
			return cmp.Or(
				cmp.Compare(strings.ToLower(p.Name), cursor.Name),
				cmp.Compare(p.ID.String(), cursor.ID),
			) <= 0
		})
	}

	items, more := paginate(after, page.Limit)
	result := domain.Page[domain.Project]{Items: append([]domain.Project{}, items...), Total: int64(len(all))}

	if more {
		last := items[len(items)-1]
		result.NextCursor = domain.EncodeCursor(domain.ProjectCursor{Name: strings.ToLower(last.Name), ID: last.ID.String()})
	}

	return result, nil
}

// AllocateIssueNumber returns and advances the project's issue counter.
func (r *ProjectRepository) AllocateIssueNumber(_ context.Context, project domain.ProjectID) (int64, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	n, ok := r.s.data.nextNumber[project]
	if !ok {
		return 0, domain.ErrProjectNotFound
	}

	r.s.data.nextNumber[project] = n + 1

	return n, nil
}

// GetMany returns the known projects among ids.
func (r *ProjectRepository) GetMany(_ context.Context, ids []domain.ProjectID) ([]domain.Project, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	projects := make([]domain.Project, 0, len(ids))

	for _, id := range ids {
		if p, ok := r.s.data.projects[id]; ok {
			projects = append(projects, p)
		}
	}

	return projects, nil
}

// sameName mirrors the Postgres unique index on lower(name); Unicode case
// folding (strings.EqualFold) would treat more names as equal.
func sameName(a, b string) bool {
	return strings.ToLower(a) == strings.ToLower(b) //nolint:staticcheck // must match Postgres lower() semantics
}
