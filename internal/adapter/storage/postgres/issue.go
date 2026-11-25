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

// Compile-time check that IssueRepository satisfies the driven port.
var _ port.IssueRepository = (*IssueRepository)(nil)

// IssueRepository implements port.IssueRepository.
type IssueRepository struct{ db *DB }

// NewIssueRepository returns an IssueRepository on db.
func NewIssueRepository(db *DB) *IssueRepository { return &IssueRepository{db: db} }

// Create inserts issue with its tags and assignees.
func (r *IssueRepository) Create(ctx context.Context, issue domain.Issue) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		err := r.db.queries(ctx).CreateIssue(ctx, sqlc.CreateIssueParams{
			ID:          uuid.UUID(issue.ID),
			ProjectID:   uuid.UUID(issue.ProjectID),
			Number:      issue.Number,
			Title:       issue.Title,
			Description: issue.Description,
			Status:      sqlc.IssueStatus(issue.Status),
			CreatedBy:   uuid.UUID(issue.CreatedBy),
			CreatedAt:   issue.CreatedAt,
			UpdatedAt:   issue.UpdatedAt,
		})
		if err != nil {
			return fmt.Errorf("insert issue: %w", err)
		}

		return r.insertRelations(ctx, issue)
	})
}

// Update stores the mutable fields of issue and replaces its tags and assignees.
func (r *IssueRepository) Update(ctx context.Context, issue domain.Issue) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		n, err := q.UpdateIssue(ctx, sqlc.UpdateIssueParams{
			ID:          uuid.UUID(issue.ID),
			Title:       issue.Title,
			Description: issue.Description,
			Status:      sqlc.IssueStatus(issue.Status),
			UpdatedAt:   issue.UpdatedAt,
		})
		if err != nil {
			return fmt.Errorf("update issue: %w", err)
		}

		if n == 0 {
			return domain.ErrIssueNotFound
		}

		if err := q.DeleteIssueTags(ctx, uuid.UUID(issue.ID)); err != nil {
			return fmt.Errorf("delete issue tags: %w", err)
		}

		if err := q.DeleteIssueAssignees(ctx, uuid.UUID(issue.ID)); err != nil {
			return fmt.Errorf("delete issue assignees: %w", err)
		}

		return r.insertRelations(ctx, issue)
	})
}

func (r *IssueRepository) insertRelations(ctx context.Context, issue domain.Issue) error {
	q := r.db.queries(ctx)

	if len(issue.Tags) > 0 {
		err := q.InsertIssueTags(ctx, sqlc.InsertIssueTagsParams{IssueID: uuid.UUID(issue.ID), TagIds: uuids(issue.Tags)})
		if err != nil {
			return fmt.Errorf("insert issue tags: %w", err)
		}
	}

	if len(issue.Assignees) > 0 {
		err := q.InsertIssueAssignees(ctx, sqlc.InsertIssueAssigneesParams{IssueID: uuid.UUID(issue.ID), UserIds: uuids(issue.Assignees)})
		if err != nil {
			return fmt.Errorf("insert issue assignees: %w", err)
		}
	}

	return nil
}

// Get returns the issue with the given project-scoped number. Inside a
// caller's transaction the row is locked until that transaction ends, so a
// following Update cannot overwrite a concurrent change.
func (r *IssueRepository) Get(ctx context.Context, project domain.ProjectID, number int64) (domain.Issue, error) {
	lock := inTx(ctx)
	run := r.db.readTx

	if lock {
		run = r.db.WithinTx // joins the caller's transaction
	}

	var issue domain.Issue

	err := run(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		var (
			row sqlc.Issue
			err error
		)

		if lock {
			row, err = q.GetIssueForUpdate(ctx, sqlc.GetIssueForUpdateParams{ProjectID: uuid.UUID(project), Number: number})
		} else {
			row, err = q.GetIssue(ctx, sqlc.GetIssueParams{ProjectID: uuid.UUID(project), Number: number})
		}

		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrIssueNotFound
		}

		if err != nil {
			return fmt.Errorf("select issue: %w", err)
		}

		issues, err := r.withRelations(ctx, []sqlc.Issue{row})
		if err != nil {
			return err
		}

		issue = issues[0]

		return nil
	})

	return issue, err
}

// List returns one page of matching issues, newest (highest number) first.
func (r *IssueRepository) List(
	ctx context.Context, project domain.ProjectID, filter port.IssueFilter, page domain.PageRequest,
) (domain.Page[domain.Issue], error) {
	cursor, _, err := domain.DecodeCursor[domain.IssueCursor](page.Cursor)
	if err != nil {
		return domain.Page[domain.Issue]{}, err
	}

	statuses := make([]sqlc.IssueStatus, 0, len(filter.Statuses))
	for _, s := range filter.Statuses {
		statuses = append(statuses, sqlc.IssueStatus(s))
	}

	params := sqlc.ListIssuesParams{
		ProjectID:    uuid.UUID(project),
		Statuses:     statuses,
		TagIds:       uuids(filter.TagIDs),
		TagsAll:      filter.TagMode != port.MatchAny,
		AssigneeIds:  uuids(filter.AssigneeIDs),
		AssigneesAll: filter.AssigneeMode != port.MatchAny,
		Pattern:      containsPattern(filter.Query),
		BeforeNumber: cursor.Number,
		RowLimit:     pageLimit(page.Limit),
	}

	var result domain.Page[domain.Issue]

	err = r.db.readTx(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		rows, err := q.ListIssues(ctx, params)
		if err != nil {
			return fmt.Errorf("select issues: %w", err)
		}

		result.Total, err = q.CountIssues(ctx, sqlc.CountIssuesParams{
			ProjectID:    params.ProjectID,
			Statuses:     params.Statuses,
			TagIds:       params.TagIds,
			TagsAll:      params.TagsAll,
			AssigneeIds:  params.AssigneeIds,
			AssigneesAll: params.AssigneesAll,
			Pattern:      params.Pattern,
		})
		if err != nil {
			return fmt.Errorf("count issues: %w", err)
		}

		more := len(rows) > page.Limit
		rows = rows[:min(len(rows), page.Limit)]

		if result.Items, err = r.withRelations(ctx, rows); err != nil {
			return err
		}

		if more {
			result.NextCursor = domain.EncodeCursor(domain.IssueCursor{Number: rows[len(rows)-1].Number})
		}

		return nil
	})
	if err != nil {
		return domain.Page[domain.Issue]{}, err
	}

	return result, nil
}

// withRelations converts rows and loads their tags and assignees with one
// query each.
func (r *IssueRepository) withRelations(ctx context.Context, rows []sqlc.Issue) ([]domain.Issue, error) {
	issues := make([]domain.Issue, 0, len(rows))
	if len(rows) == 0 {
		return issues, nil
	}

	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}

	q := r.db.queries(ctx)

	tagRows, err := q.ListIssueTags(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("select issue tags: %w", err)
	}

	assigneeRows, err := q.ListIssueAssignees(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("select issue assignees: %w", err)
	}

	tags := map[uuid.UUID][]domain.TagID{}
	for _, t := range tagRows {
		tags[t.IssueID] = append(tags[t.IssueID], domain.TagID(t.TagID))
	}

	assignees := map[uuid.UUID][]domain.UserID{}
	for _, a := range assigneeRows {
		assignees[a.IssueID] = append(assignees[a.IssueID], domain.UserID(a.UserID))
	}

	for _, row := range rows {
		issues = append(issues, domain.Issue{
			ID:          domain.IssueID(row.ID),
			ProjectID:   domain.ProjectID(row.ProjectID),
			Number:      row.Number,
			Title:       row.Title,
			Description: row.Description,
			Status:      domain.IssueStatus(row.Status),
			Tags:        nonNil(tags[row.ID]),
			Assignees:   nonNil(assignees[row.ID]),
			CreatedBy:   domain.UserID(row.CreatedBy),
			CreatedAt:   row.CreatedAt.UTC(),
			UpdatedAt:   row.UpdatedAt.UTC(),
		})
	}

	return issues, nil
}

// likeEscaper escapes ILIKE wildcards; backslash is Postgres's default
// LIKE escape character.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// containsPattern turns a search term into an ILIKE "contains" pattern, or
// "" for no search.
func containsPattern(query string) string {
	if query == "" {
		return ""
	}

	return "%" + likeEscaper.Replace(query) + "%"
}

func uuids[T ~[16]byte](ids []T) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		out = append(out, uuid.UUID(id))
	}

	return out
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}

	return s
}
