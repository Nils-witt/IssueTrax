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

// Compile-time checks that the repositories satisfy the driven ports.
var (
	_ port.ItemTypeRepository        = (*ItemTypeRepository)(nil)
	_ port.AssetTagCounterRepository = (*AssetTagCounterRepository)(nil)
)

// itemTypeNameKey is the unique index on live sibling names.
const itemTypeNameKey = "item_types_parent_name_key"

// ItemTypeRepository implements port.ItemTypeRepository.
type ItemTypeRepository struct{ db *DB }

// NewItemTypeRepository returns an ItemTypeRepository on db.
func NewItemTypeRepository(db *DB) *ItemTypeRepository { return &ItemTypeRepository{db: db} }

// Create inserts t with its defaults.
func (r *ItemTypeRepository) Create(ctx context.Context, t domain.ItemType) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		err := q.CreateItemType(ctx, sqlc.CreateItemTypeParams{
			ID: uuid.UUID(t.ID), ParentID: toPgItemTypeID(t.ParentID), Name: t.Name, TagPrefix: t.TagPrefix,
			CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, DeletedAt: t.DeletedAt,
		})

		switch {
		case isUniqueViolation(err, itemTypeNameKey):
			return domain.ErrItemTypeNameTaken
		case err != nil:
			return fmt.Errorf("insert item type: %w", err)
		}

		return insertDefaults(ctx, q, t)
	})
}

// Update stores every mutable field of t and replaces its defaults.
func (r *ItemTypeRepository) Update(ctx context.Context, t domain.ItemType) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		n, err := q.UpdateItemType(ctx, sqlc.UpdateItemTypeParams{
			ID: uuid.UUID(t.ID), ParentID: toPgItemTypeID(t.ParentID), Name: t.Name, TagPrefix: t.TagPrefix,
			UpdatedAt: t.UpdatedAt, DeletedAt: t.DeletedAt,
		})

		switch {
		case isUniqueViolation(err, itemTypeNameKey):
			return domain.ErrItemTypeNameTaken
		case err != nil:
			return fmt.Errorf("update item type: %w", err)
		case n == 0:
			return domain.ErrItemTypeNotFound
		}

		if err := q.DeleteItemTypeDefaults(ctx, uuid.UUID(t.ID)); err != nil {
			return fmt.Errorf("delete item type defaults: %w", err)
		}

		return insertDefaults(ctx, q, t)
	})
}

func insertDefaults(ctx context.Context, q *sqlc.Queries, t domain.ItemType) error {
	if len(t.Defaults) == 0 {
		return nil
	}

	modules := make([]string, 0, len(t.Defaults))
	counts := make([]int32, 0, len(t.Defaults))

	for _, d := range t.Defaults {
		modules = append(modules, string(d.Module))
		counts = append(counts, int32(d.Count)) //nolint:gosec // counts are validated to at most MaxDefaultCount
	}

	err := q.InsertItemTypeDefaults(ctx, sqlc.InsertItemTypeDefaultsParams{ItemTypeID: uuid.UUID(t.ID), Modules: modules, Counts: counts})
	if err != nil {
		return fmt.Errorf("insert item type defaults: %w", err)
	}

	return nil
}

// Get returns the type with id and its counts.
func (r *ItemTypeRepository) Get(ctx context.Context, id domain.ItemTypeID) (domain.ItemTypeListItem, error) {
	var item domain.ItemTypeListItem

	err := r.db.readTx(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		row, err := q.GetItemType(ctx, uuid.UUID(id))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrItemTypeNotFound
		}

		if err != nil {
			return fmt.Errorf("select item type: %w", err)
		}

		items, err := withDefaults(ctx, q, []sqlc.GetItemTypeRow{row})
		if err != nil {
			return err
		}

		item = items[0]

		return nil
	})

	return item, err
}

// GetMany returns the known types among ids.
func (r *ItemTypeRepository) GetMany(ctx context.Context, ids []domain.ItemTypeID) ([]domain.ItemType, error) {
	return readMany(ctx, r.db, "item types", func(q *sqlc.Queries) ([]sqlc.ItemType, error) {
		return q.GetItemTypesByIDs(ctx, uuids(ids))
	}, typesWithDefaults)
}

// ListChildren returns one page of parent's children ordered by
// lower(name), then ID.
func (r *ItemTypeRepository) ListChildren(
	ctx context.Context, parent *domain.ItemTypeID, filter port.ItemTypeFilter, page domain.PageRequest,
) (domain.Page[domain.ItemTypeListItem], error) {
	return r.list(ctx, sqlc.ListItemTypesParams{ParentID: toPgItemTypeID(parent), IncludeDeleted: filter.IncludeDeleted}, page)
}

// Search returns one page of the types whose name contains query, ignoring
// case, ordered by lower(name), then ID.
func (r *ItemTypeRepository) Search(
	ctx context.Context, query string, filter port.ItemTypeFilter, page domain.PageRequest,
) (domain.Page[domain.ItemTypeListItem], error) {
	return r.list(ctx, sqlc.ListItemTypesParams{
		AnyParent: true, Pattern: containsPattern(query), IncludeDeleted: filter.IncludeDeleted,
	}, page)
}

// list returns one page of the types params selects, in one read
// transaction.
func (r *ItemTypeRepository) list(
	ctx context.Context, params sqlc.ListItemTypesParams, page domain.PageRequest,
) (domain.Page[domain.ItemTypeListItem], error) {
	cursor, err := decodeNameCursor(page)
	if err != nil {
		return domain.Page[domain.ItemTypeListItem]{}, err
	}

	params.HasCursor, params.CursorName, params.CursorID, params.RowLimit = cursor.has, cursor.name, cursor.id, pageLimit(page.Limit)

	var result domain.Page[domain.ItemTypeListItem]

	err = r.db.readTx(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		rows, err := q.ListItemTypes(ctx, params)
		if err != nil {
			return fmt.Errorf("select item types: %w", err)
		}

		result.Total, err = q.CountItemTypes(ctx, sqlc.CountItemTypesParams{
			AnyParent: params.AnyParent, ParentID: params.ParentID, Pattern: params.Pattern, IncludeDeleted: params.IncludeDeleted,
		})
		if err != nil {
			return fmt.Errorf("count item types: %w", err)
		}

		more := len(rows) > page.Limit

		converted := make([]sqlc.GetItemTypeRow, 0, len(rows))
		for _, row := range rows[:min(len(rows), page.Limit)] {
			converted = append(converted, sqlc.GetItemTypeRow(row))
		}

		if result.Items, err = withDefaults(ctx, q, converted); err != nil {
			return err
		}

		if more {
			last := result.Items[len(result.Items)-1]
			result.NextCursor = domain.EncodeCursor(domain.StorageCursor{Name: strings.ToLower(last.Name), ID: last.ID.String()})
		}

		return nil
	})
	if err != nil {
		return domain.Page[domain.ItemTypeListItem]{}, err
	}

	return result, nil
}

// Chain returns the ancestors of id from the root down to id itself.
func (r *ItemTypeRepository) Chain(ctx context.Context, id domain.ItemTypeID) ([]domain.ItemType, error) {
	var chain []domain.ItemType

	err := r.db.readTx(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		rows, err := q.ItemTypeChain(ctx, uuid.UUID(id))
		if err != nil {
			return fmt.Errorf("select item type chain: %w", err)
		}

		if len(rows) == 0 {
			return domain.ErrItemTypeNotFound
		}

		chain, err = typesWithDefaults(ctx, q, rows)

		return err
	})

	return chain, err
}

// IsDescendant reports whether candidate lies in ancestor's subtree.
func (r *ItemTypeRepository) IsDescendant(ctx context.Context, ancestor, candidate domain.ItemTypeID) (bool, error) {
	ok, err := r.db.queries(ctx).ItemTypeIsDescendant(ctx, sqlc.ItemTypeIsDescendantParams{
		Ancestor: uuid.UUID(ancestor), Candidate: uuid.UUID(candidate),
	})
	if err != nil {
		return false, fmt.Errorf("check item type ancestry: %w", err)
	}

	return ok, nil
}

// LockTree takes a transaction-scoped advisory lock; call it inside a
// transaction, or the lock is released right away.
func (r *ItemTypeRepository) LockTree(ctx context.Context) error {
	if err := r.db.queries(ctx).LockItemTypeTree(ctx); err != nil {
		return fmt.Errorf("lock item type tree: %w", err)
	}

	return nil
}

// withDefaults converts rows with their counts and loads their defaults.
func withDefaults(ctx context.Context, q *sqlc.Queries, rows []sqlc.GetItemTypeRow) ([]domain.ItemTypeListItem, error) {
	types := make([]sqlc.ItemType, 0, len(rows))
	for _, row := range rows {
		types = append(types, sqlc.ItemType{
			ID: row.ID, ParentID: row.ParentID, Name: row.Name, TagPrefix: row.TagPrefix,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt,
		})
	}

	converted, err := typesWithDefaults(ctx, q, types)
	if err != nil {
		return nil, err
	}

	items := make([]domain.ItemTypeListItem, 0, len(rows))
	for i, row := range rows {
		items = append(items, domain.ItemTypeListItem{ItemType: converted[i], ChildCount: row.ChildCount, ItemCount: row.ItemCount})
	}

	return items, nil
}

// typesWithDefaults converts rows and loads their defaults with one query,
// keeping the order of rows.
func typesWithDefaults(ctx context.Context, q *sqlc.Queries, rows []sqlc.ItemType) ([]domain.ItemType, error) {
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}

	defaultRows, err := q.ListItemTypeDefaults(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("select item type defaults: %w", err)
	}

	defaults := map[uuid.UUID][]domain.ModuleDefault{}
	for _, d := range defaultRows {
		defaults[d.ItemTypeID] = append(defaults[d.ItemTypeID], domain.ModuleDefault{
			Module: domain.ModuleKind(d.Module), Count: int(d.DefaultCount),
		})
	}

	types := make([]domain.ItemType, 0, len(rows))

	for _, row := range rows {
		t := domain.ItemType{
			ID: domain.ItemTypeID(row.ID), Name: row.Name, TagPrefix: row.TagPrefix,
			Defaults:  sortedDefaults(defaults[row.ID]),
			CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(), DeletedAt: utcPtr(row.DeletedAt),
		}

		if row.ParentID.Valid {
			parent := domain.ItemTypeID(row.ParentID.Bytes)
			t.ParentID = &parent
		}

		types = append(types, t)
	}

	return types, nil
}

// sortedDefaults puts defaults into display order; it never returns nil.
func sortedDefaults(defaults []domain.ModuleDefault) []domain.ModuleDefault {
	out := make([]domain.ModuleDefault, 0, len(defaults))

	for _, kind := range domain.ModuleKinds {
		for _, d := range defaults {
			if d.Module == kind {
				out = append(out, d)
			}
		}
	}

	return out
}

func toPgItemTypeID(id *domain.ItemTypeID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}

	return pgtype.UUID{Bytes: *id, Valid: true}
}

// AssetTagCounterRepository implements port.AssetTagCounterRepository.
type AssetTagCounterRepository struct{ db *DB }

// NewAssetTagCounterRepository returns an AssetTagCounterRepository on db.
func NewAssetTagCounterRepository(db *DB) *AssetTagCounterRepository {
	return &AssetTagCounterRepository{db: db}
}

// Get returns the counter of prefix, 0 when there is none.
func (r *AssetTagCounterRepository) Get(ctx context.Context, prefix string) (int64, error) {
	n, err := r.db.queries(ctx).GetAssetTagCounter(ctx, prefix)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}

	if err != nil {
		return 0, fmt.Errorf("select asset tag counter: %w", err)
	}

	return n, nil
}

// Set stores the counter of prefix.
func (r *AssetTagCounterRepository) Set(ctx context.Context, prefix string, value int64) error {
	if err := r.db.queries(ctx).SetAssetTagCounter(ctx, sqlc.SetAssetTagCounterParams{Prefix: prefix, Value: value}); err != nil {
		return fmt.Errorf("upsert asset tag counter: %w", err)
	}

	return nil
}

// nameCursor is a decoded StorageCursor.
type nameCursor struct {
	has  bool
	name string
	id   uuid.UUID
}

func decodeNameCursor(page domain.PageRequest) (nameCursor, error) {
	cursor, has, err := domain.DecodeCursor[domain.StorageCursor](page.Cursor)
	if err != nil || !has {
		return nameCursor{}, err
	}

	id, err := uuid.Parse(cursor.ID)
	if err != nil {
		return nameCursor{}, domain.NewValidationError(map[string]string{cursorField: domain.FieldInvalid})
	}

	return nameCursor{has: true, name: cursor.Name, id: id}, nil
}
