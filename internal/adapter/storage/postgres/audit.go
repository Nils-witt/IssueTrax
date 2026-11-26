package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"nilswitt.dev/issuetrax/internal/adapter/storage/postgres/sqlc"
	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that AuditRepository satisfies the driven port.
var _ port.AuditRepository = (*AuditRepository)(nil)

// AuditRepository implements port.AuditRepository.
type AuditRepository struct{ db *DB }

// NewAuditRepository returns an AuditRepository on db.
func NewAuditRepository(db *DB) *AuditRepository { return &AuditRepository{db: db} }

// auditChange is the stored JSON form of a domain.AuditChange.
type auditChange struct {
	Old      *string `json:"old,omitempty"`
	New      *string `json:"new,omitempty"`
	Redacted bool    `json:"redacted,omitempty"`
}

// Append inserts entry.
func (r *AuditRepository) Append(ctx context.Context, e domain.AuditEntry) error {
	stored := make(map[string]auditChange, len(e.Changes))
	for field, c := range e.Changes {
		stored[field] = auditChange(c)
	}

	changes, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("encode audit changes: %w", err)
	}

	var actor pgtype.UUID
	if e.ActorID != nil {
		actor = pgtype.UUID{Bytes: *e.ActorID, Valid: true}
	}

	err = r.db.queries(ctx).AppendAuditEntry(ctx, sqlc.AppendAuditEntryParams{
		ID:          uuid.UUID(e.ID),
		At:          e.At,
		ActorID:     actor,
		Action:      string(e.Action),
		TargetType:  e.TargetType,
		TargetID:    e.TargetID,
		TargetLabel: e.TargetLabel,
		Changes:     changes,
		Ip:          e.IP,
		UserAgent:   e.UserAgent,
	})
	if err != nil {
		return fmt.Errorf("insert audit entry: %w", err)
	}

	return nil
}

// List returns one page of matching entries, newest first.
func (r *AuditRepository) List(
	ctx context.Context, filter port.AuditFilter, page domain.PageRequest,
) (domain.Page[domain.AuditEntry], error) {
	cursor, hasCursor, err := domain.DecodeCursor[domain.AuditCursor](page.Cursor)
	if err != nil {
		return domain.Page[domain.AuditEntry]{}, err
	}

	actions := make([]string, 0, len(filter.Actions))
	for _, a := range filter.Actions {
		actions = append(actions, string(a))
	}

	count := sqlc.CountAuditEntriesParams{Actions: actions, TargetType: filter.TargetType}
	if filter.ActorID != nil {
		count.FilterActor, count.ActorID = true, uuid.UUID(*filter.ActorID)
	}

	params := sqlc.ListAuditEntriesParams{
		FilterActor: count.FilterActor, ActorID: count.ActorID, Actions: actions, TargetType: filter.TargetType,
		HasCursor: hasCursor, CursorAt: cursor.At, RowLimit: pageLimit(page.Limit),
	}

	if hasCursor {
		if params.CursorID, err = uuid.Parse(cursor.ID); err != nil {
			return domain.Page[domain.AuditEntry]{}, domain.NewValidationError(map[string]string{cursorField: domain.FieldInvalid})
		}
	}

	var result domain.Page[domain.AuditEntry]

	err = r.db.readTx(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		rows, err := q.ListAuditEntries(ctx, params)
		if err != nil {
			return fmt.Errorf("select audit entries: %w", err)
		}

		if result.Total, err = q.CountAuditEntries(ctx, count); err != nil {
			return fmt.Errorf("count audit entries: %w", err)
		}

		more := len(rows) > page.Limit
		rows = rows[:min(len(rows), page.Limit)]

		result.Items = make([]domain.AuditEntry, 0, len(rows))

		for _, row := range rows {
			e, err := toAuditEntry(row)
			if err != nil {
				return err
			}

			result.Items = append(result.Items, e)
		}

		if more {
			last := result.Items[len(result.Items)-1]
			result.NextCursor = domain.EncodeCursor(domain.AuditCursor{At: last.At, ID: last.ID.String()})
		}

		return nil
	})
	if err != nil {
		return domain.Page[domain.AuditEntry]{}, err
	}

	return result, nil
}

// DeleteBefore removes entries older than t.
func (r *AuditRepository) DeleteBefore(ctx context.Context, t time.Time) (int64, error) {
	n, err := r.db.queries(ctx).DeleteAuditEntriesBefore(ctx, t)
	if err != nil {
		return 0, fmt.Errorf("delete audit entries: %w", err)
	}

	return n, nil
}

func toAuditEntry(row sqlc.AuditLog) (domain.AuditEntry, error) {
	var stored map[string]auditChange
	if err := json.Unmarshal(row.Changes, &stored); err != nil {
		return domain.AuditEntry{}, fmt.Errorf("decode audit changes of %s: %w", row.ID, err)
	}

	changes := make(domain.AuditChanges, len(stored))
	for field, c := range stored {
		changes[field] = domain.AuditChange(c)
	}

	e := domain.AuditEntry{
		ID:          domain.AuditEntryID(row.ID),
		At:          row.At.UTC(),
		Action:      domain.AuditAction(row.Action),
		TargetType:  row.TargetType,
		TargetID:    row.TargetID,
		TargetLabel: row.TargetLabel,
		Changes:     changes,
		IP:          row.Ip,
		UserAgent:   row.UserAgent,
	}

	if row.ActorID.Valid {
		actor := domain.UserID(row.ActorID.Bytes)
		e.ActorID = &actor
	}

	return e, nil
}
