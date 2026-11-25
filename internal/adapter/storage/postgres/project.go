package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nilswitt.dev/issuetrax/internal/adapter/storage/postgres/sqlc"
	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that ProjectRepository satisfies the driven port.
var _ port.ProjectRepository = (*ProjectRepository)(nil)

// ProjectRepository implements port.ProjectRepository.
type ProjectRepository struct{ db *DB }

// NewProjectRepository returns a ProjectRepository on db.
func NewProjectRepository(db *DB) *ProjectRepository { return &ProjectRepository{db: db} }

// Create inserts project.
func (r *ProjectRepository) Create(ctx context.Context, p domain.Project) error {
	err := r.db.queries(ctx).CreateProject(ctx, sqlc.CreateProjectParams{
		ID:          uuid.UUID(p.ID),
		Key:         string(p.Key),
		Name:        p.Name,
		Description: p.Description,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	})

	switch {
	case isUniqueViolation(err, "projects_key_key"):
		return domain.ErrProjectKeyTaken
	case isUniqueViolation(err, "projects_name_lower_key"):
		return domain.ErrProjectNameTaken
	case err != nil:
		return fmt.Errorf("insert project: %w", err)
	}

	return nil
}

// Update stores the mutable fields of project.
func (r *ProjectRepository) Update(ctx context.Context, p domain.Project) error {
	n, err := r.db.queries(ctx).UpdateProject(ctx, sqlc.UpdateProjectParams{
		ID:          uuid.UUID(p.ID),
		Name:        p.Name,
		Description: p.Description,
		UpdatedAt:   p.UpdatedAt,
	})

	switch {
	case isUniqueViolation(err, "projects_name_lower_key"):
		return domain.ErrProjectNameTaken
	case err != nil:
		return fmt.Errorf("update project: %w", err)
	case n == 0:
		return domain.ErrProjectNotFound
	}

	return nil
}

// GetByKey returns the project with key.
func (r *ProjectRepository) GetByKey(ctx context.Context, key domain.ProjectKey) (domain.Project, error) {
	row, err := r.db.queries(ctx).GetProjectByKey(ctx, string(key))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Project{}, domain.ErrProjectNotFound
	}

	if err != nil {
		return domain.Project{}, fmt.Errorf("select project: %w", err)
	}

	return toProject(row), nil
}

// List returns one page of projects ordered by lower(name), then ID.
func (r *ProjectRepository) List(ctx context.Context, page domain.PageRequest) (domain.Page[domain.Project], error) {
	cursor, hasCursor, err := domain.DecodeCursor[domain.ProjectCursor](page.Cursor)
	if err != nil {
		return domain.Page[domain.Project]{}, err
	}

	params := sqlc.ListProjectsParams{HasCursor: hasCursor, CursorName: cursor.Name, RowLimit: pageLimit(page.Limit)}

	if hasCursor {
		if params.CursorID, err = uuid.Parse(cursor.ID); err != nil {
			return domain.Page[domain.Project]{}, domain.NewValidationError(map[string]string{"cursor": domain.FieldInvalid})
		}
	}

	var result domain.Page[domain.Project]

	err = r.db.readTx(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		rows, err := q.ListProjects(ctx, params)
		if err != nil {
			return fmt.Errorf("select projects: %w", err)
		}

		if result.Total, err = q.CountProjects(ctx); err != nil {
			return fmt.Errorf("count projects: %w", err)
		}

		more := len(rows) > page.Limit
		rows = rows[:min(len(rows), page.Limit)]

		result.Items = make([]domain.Project, 0, len(rows))
		for _, row := range rows {
			result.Items = append(result.Items, toProject(sqlc.GetProjectByKeyRow(row)))
		}

		if more {
			last := result.Items[len(result.Items)-1]
			result.NextCursor = domain.EncodeCursor(domain.ProjectCursor{Name: strings.ToLower(last.Name), ID: last.ID.String()})
		}

		return nil
	})
	if err != nil {
		return domain.Page[domain.Project]{}, err
	}

	return result, nil
}

// AllocateIssueNumber returns and advances the project's issue counter. The
// row stays locked until the caller's transaction ends.
func (r *ProjectRepository) AllocateIssueNumber(ctx context.Context, project domain.ProjectID) (int64, error) {
	n, err := r.db.queries(ctx).AllocateIssueNumber(ctx, uuid.UUID(project))
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrProjectNotFound
	}

	if err != nil {
		return 0, fmt.Errorf("allocate issue number: %w", err)
	}

	return n, nil
}

func toProject(row sqlc.GetProjectByKeyRow) domain.Project {
	return domain.Project{
		ID:          domain.ProjectID(row.ID),
		Key:         domain.ProjectKey(row.Key),
		Name:        row.Name,
		Description: row.Description,
		CreatedAt:   row.CreatedAt.UTC(),
		UpdatedAt:   row.UpdatedAt.UTC(),
	}
}
