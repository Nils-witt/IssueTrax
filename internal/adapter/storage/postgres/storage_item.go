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

// Compile-time check that GenericStorageItemRepository satisfies the driven port.
var _ port.GenericStorageItemRepository = (*GenericStorageItemRepository)(nil)

// genericKind is the kind column value of generic items.
const genericKind = string(domain.StorageItemKindGeneric)

// GenericStorageItemRepository implements port.GenericStorageItemRepository
// on the shared storage_items table.
type GenericStorageItemRepository struct{ db *DB }

// NewGenericStorageItemRepository returns a GenericStorageItemRepository on db.
func NewGenericStorageItemRepository(db *DB) *GenericStorageItemRepository {
	return &GenericStorageItemRepository{db: db}
}

// Create inserts item.
func (r *GenericStorageItemRepository) Create(ctx context.Context, item domain.GenericStorageItem) error {
	err := r.db.queries(ctx).CreateStorageItem(ctx, sqlc.CreateStorageItemParams{
		ID:          uuid.UUID(item.ID),
		StorageID:   uuid.UUID(item.StorageID),
		Kind:        genericKind,
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
		DeletedAt:   item.DeletedAt,
	})
	if err != nil {
		return fmt.Errorf("insert storage item: %w", err)
	}

	return nil
}

// Update stores the storage, name, description, update and deletion times
// of item.
func (r *GenericStorageItemRepository) Update(ctx context.Context, item domain.GenericStorageItem) error {
	n, err := r.db.queries(ctx).UpdateStorageItem(ctx, sqlc.UpdateStorageItemParams{
		ID:          uuid.UUID(item.ID),
		Kind:        genericKind,
		StorageID:   uuid.UUID(item.StorageID),
		Name:        item.Name,
		Description: item.Description,
		UpdatedAt:   item.UpdatedAt,
		DeletedAt:   item.DeletedAt,
	})

	switch {
	case err != nil:
		return fmt.Errorf("update storage item: %w", err)
	case n == 0:
		return domain.ErrStorageItemNotFound
	}

	return nil
}

// Get returns the generic item with id, deleted or not.
func (r *GenericStorageItemRepository) Get(ctx context.Context, id domain.StorageItemID) (domain.GenericStorageItem, error) {
	row, err := r.db.queries(ctx).GetStorageItem(ctx, sqlc.GetStorageItemParams{ID: uuid.UUID(id), Kind: genericKind})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GenericStorageItem{}, domain.ErrStorageItemNotFound
	}

	if err != nil {
		return domain.GenericStorageItem{}, fmt.Errorf("select storage item: %w", err)
	}

	return toGenericStorageItem(row), nil
}

// ListInStorage returns one page of the generic items in storage (or its
// subtree), ordered by lower(name), then ID.
func (r *GenericStorageItemRepository) ListInStorage(
	ctx context.Context, storage domain.StorageID, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.GenericStorageItem], error) {
	load, err := itemsInStorage(ctx, genericKind, storage, filter, page)
	if err != nil {
		return domain.Page[domain.GenericStorageItem]{}, err
	}

	return r.page(ctx, page, load)
}

// itemLoader loads one page of storage item rows (up to one more than the
// limit) and the total.
type itemLoader func(q *sqlc.Queries) ([]sqlc.StorageItem, int64, error)

// itemsInStorage returns the loader for one page of the items of kind in
// storage (or its subtree), ordered by lower(name), then ID.
func itemsInStorage(
	ctx context.Context, kind string, storage domain.StorageID, filter port.StorageItemFilter, page domain.PageRequest,
) (itemLoader, error) {
	cursor, err := decodeStorageItemCursor(page)
	if err != nil {
		return nil, err
	}

	params := sqlc.ListStorageItemsInStorageParams{
		Kind: kind, Deleted: filter.Deleted, StorageID: uuid.UUID(storage), Recursive: filter.Recursive,
		HasCursor: cursor.has, CursorName: cursor.name, CursorID: cursor.id, RowLimit: pageLimit(page.Limit),
	}

	return func(q *sqlc.Queries) ([]sqlc.StorageItem, int64, error) {
		rows, err := q.ListStorageItemsInStorage(ctx, params)
		if err != nil {
			return nil, 0, fmt.Errorf("select storage items: %w", err)
		}

		total, err := q.CountStorageItemsInStorage(ctx, sqlc.CountStorageItemsInStorageParams{
			Kind: params.Kind, Deleted: params.Deleted, StorageID: params.StorageID, Recursive: params.Recursive,
		})
		if err != nil {
			return nil, 0, fmt.Errorf("count storage items: %w", err)
		}

		return rows, total, nil
	}, nil
}

// Search returns one page of the generic items whose name or description
// contains filter.Query, ignoring case, ordered by lower(name), then ID.
func (r *GenericStorageItemRepository) Search(
	ctx context.Context, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.GenericStorageItem], error) {
	cursor, err := decodeStorageItemCursor(page)
	if err != nil {
		return domain.Page[domain.GenericStorageItem]{}, err
	}

	params := sqlc.SearchStorageItemsParams{
		Kind: genericKind, Deleted: filter.Deleted, Pattern: containsPattern(filter.Query),
		HasCursor: cursor.has, CursorName: cursor.name, CursorID: cursor.id, RowLimit: pageLimit(page.Limit),
	}

	return r.page(ctx, page, func(q *sqlc.Queries) ([]sqlc.StorageItem, int64, error) {
		rows, err := q.SearchStorageItems(ctx, params)
		if err != nil {
			return nil, 0, fmt.Errorf("search storage items: %w", err)
		}

		total, err := q.CountSearchStorageItems(ctx, sqlc.CountSearchStorageItemsParams{
			Kind: params.Kind, Deleted: params.Deleted, Pattern: params.Pattern,
		})
		if err != nil {
			return nil, 0, fmt.Errorf("count storage items: %w", err)
		}

		return rows, total, nil
	})
}

// page runs load in one read transaction and turns its rows (up to one more
// than the limit) into a page.
func (r *GenericStorageItemRepository) page(
	ctx context.Context, page domain.PageRequest, load itemLoader,
) (domain.Page[domain.GenericStorageItem], error) {
	var result domain.Page[domain.GenericStorageItem]

	err := r.db.readTx(ctx, func(ctx context.Context) error {
		rows, total, err := load(r.db.queries(ctx))
		if err != nil {
			return err
		}

		result.Total = total
		more := len(rows) > page.Limit
		rows = rows[:min(len(rows), page.Limit)]

		result.Items = make([]domain.GenericStorageItem, 0, len(rows))
		for _, row := range rows {
			result.Items = append(result.Items, toGenericStorageItem(row))
		}

		if more {
			last := result.Items[len(result.Items)-1]
			result.NextCursor = domain.EncodeCursor(domain.StorageCursor{Name: strings.ToLower(last.Name), ID: last.ID.String()})
		}

		return nil
	})
	if err != nil {
		return domain.Page[domain.GenericStorageItem]{}, err
	}

	return result, nil
}

type storageItemCursor struct {
	has  bool
	name string
	id   uuid.UUID
}

func decodeStorageItemCursor(page domain.PageRequest) (storageItemCursor, error) {
	cursor, has, err := domain.DecodeCursor[domain.StorageCursor](page.Cursor)
	if err != nil || !has {
		return storageItemCursor{}, err
	}

	id, err := uuid.Parse(cursor.ID)
	if err != nil {
		return storageItemCursor{}, domain.NewValidationError(map[string]string{cursorField: domain.FieldInvalid})
	}

	return storageItemCursor{has: true, name: cursor.Name, id: id}, nil
}

func toGenericStorageItem(row sqlc.StorageItem) domain.GenericStorageItem {
	item := domain.StorageItem{
		ID:          domain.StorageItemID(row.ID),
		StorageID:   domain.StorageID(row.StorageID),
		Kind:        domain.StorageItemKind(row.Kind),
		Name:        row.Name,
		Description: row.Description,
		CreatedAt:   row.CreatedAt.UTC(),
		UpdatedAt:   row.UpdatedAt.UTC(),
	}

	if row.DeletedAt != nil {
		at := row.DeletedAt.UTC()
		item.DeletedAt = &at
	}

	return domain.GenericStorageItem{StorageItem: item}
}
