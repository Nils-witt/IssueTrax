package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"nilswitt.dev/issuetrax/internal/adapter/storage/postgres/sqlc"
	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that StorageRepository satisfies the driven port.
var _ port.StorageRepository = (*StorageRepository)(nil)

// storageNameKey is the unique index on sibling names.
const storageNameKey = "storages_parent_name_key"

// StorageRepository implements port.StorageRepository.
type StorageRepository struct{ db *DB }

// NewStorageRepository returns a StorageRepository on db.
func NewStorageRepository(db *DB) *StorageRepository { return &StorageRepository{db: db} }

// Create inserts storage.
func (r *StorageRepository) Create(ctx context.Context, s domain.Storage) error {
	err := r.db.queries(ctx).CreateStorage(ctx, sqlc.CreateStorageParams{
		ID:        uuid.UUID(s.ID),
		ParentID:  toPgStorageID(s.ParentID),
		Name:      s.Name,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	})

	switch {
	case isUniqueViolation(err, storageNameKey):
		return domain.ErrStorageNameTaken
	case err != nil:
		return fmt.Errorf("insert storage: %w", err)
	}

	return nil
}

// Update stores the parent, name and update time of storage.
func (r *StorageRepository) Update(ctx context.Context, s domain.Storage) error {
	n, err := r.db.queries(ctx).UpdateStorage(ctx, sqlc.UpdateStorageParams{
		ID:        uuid.UUID(s.ID),
		ParentID:  toPgStorageID(s.ParentID),
		Name:      s.Name,
		UpdatedAt: s.UpdatedAt,
	})

	switch {
	case isUniqueViolation(err, storageNameKey):
		return domain.ErrStorageNameTaken
	case err != nil:
		return fmt.Errorf("update storage: %w", err)
	case n == 0:
		return domain.ErrStorageNotFound
	}

	return nil
}

// Get returns the storage with id and its child and item counts.
func (r *StorageRepository) Get(ctx context.Context, id domain.StorageID) (domain.StorageListItem, error) {
	row, err := r.db.queries(ctx).GetStorage(ctx, uuid.UUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.StorageListItem{}, domain.ErrStorageNotFound
	}

	if err != nil {
		return domain.StorageListItem{}, fmt.Errorf("select storage: %w", err)
	}

	return toStorageListItem(row), nil
}

// ListChildren returns one page of parent's children ordered by
// lower(name), then ID.
func (r *StorageRepository) ListChildren(
	ctx context.Context, parent *domain.StorageID, page domain.PageRequest,
) (domain.Page[domain.StorageListItem], error) {
	cursor, hasCursor, err := domain.DecodeCursor[domain.StorageCursor](page.Cursor)
	if err != nil {
		return domain.Page[domain.StorageListItem]{}, err
	}

	params := sqlc.ListStorageChildrenParams{
		ParentID: toPgStorageID(parent), HasCursor: hasCursor, CursorName: cursor.Name, RowLimit: pageLimit(page.Limit),
	}

	if hasCursor {
		if params.CursorID, err = uuid.Parse(cursor.ID); err != nil {
			return domain.Page[domain.StorageListItem]{}, domain.NewValidationError(map[string]string{cursorField: domain.FieldInvalid})
		}
	}

	var result domain.Page[domain.StorageListItem]

	err = r.db.readTx(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		rows, err := q.ListStorageChildren(ctx, params)
		if err != nil {
			return fmt.Errorf("select storages: %w", err)
		}

		if result.Total, err = q.CountStorageChildren(ctx, params.ParentID); err != nil {
			return fmt.Errorf("count storages: %w", err)
		}

		more := len(rows) > page.Limit
		rows = rows[:min(len(rows), page.Limit)]

		result.Items = make([]domain.StorageListItem, 0, len(rows))
		for _, row := range rows {
			result.Items = append(result.Items, toStorageListItem(sqlc.GetStorageRow(row)))
		}

		if more {
			last := result.Items[len(result.Items)-1]
			result.NextCursor = domain.EncodeCursor(domain.StorageCursor{Name: strings.ToLower(last.Name), ID: last.ID.String()})
		}

		return nil
	})
	if err != nil {
		return domain.Page[domain.StorageListItem]{}, err
	}

	return result, nil
}

// IsDescendant reports whether candidate lies in ancestor's subtree.
func (r *StorageRepository) IsDescendant(ctx context.Context, ancestor, candidate domain.StorageID) (bool, error) {
	ok, err := r.db.queries(ctx).StorageIsDescendant(ctx, sqlc.StorageIsDescendantParams{
		Ancestor: uuid.UUID(ancestor), Candidate: uuid.UUID(candidate),
	})
	if err != nil {
		return false, fmt.Errorf("check storage ancestry: %w", err)
	}

	return ok, nil
}

// LockTree takes a transaction-scoped advisory lock; call it inside a
// transaction, or the lock is released right away.
func (r *StorageRepository) LockTree(ctx context.Context) error {
	if err := r.db.queries(ctx).LockStorageTree(ctx); err != nil {
		return fmt.Errorf("lock storage tree: %w", err)
	}

	return nil
}

func toPgStorageID(id *domain.StorageID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}

	return pgtype.UUID{Bytes: *id, Valid: true}
}

func toStorageListItem(row sqlc.GetStorageRow) domain.StorageListItem {
	s := domain.Storage{
		ID:        domain.StorageID(row.ID),
		Name:      row.Name,
		CreatedAt: row.CreatedAt.UTC(),
		UpdatedAt: row.UpdatedAt.UTC(),
	}

	if row.ParentID.Valid {
		parent := domain.StorageID(row.ParentID.Bytes)
		s.ParentID = &parent
	}

	return domain.StorageListItem{Storage: s, ChildCount: row.ChildCount, ItemCount: row.ItemCount}
}
