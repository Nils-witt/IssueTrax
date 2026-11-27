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
	_ port.ManufacturerRepository  = (*ManufacturerRepository)(nil)
	_ port.HardwareModelRepository = (*HardwareModelRepository)(nil)
)

// Unique indexes on live names.
const (
	manufacturerNameKey  = "manufacturers_name_key"
	hardwareModelNameKey = "hardware_models_manufacturer_name_key"
)

// ManufacturerRepository implements port.ManufacturerRepository.
type ManufacturerRepository struct{ db *DB }

// NewManufacturerRepository returns a ManufacturerRepository on db.
func NewManufacturerRepository(db *DB) *ManufacturerRepository {
	return &ManufacturerRepository{db: db}
}

// Create inserts m.
func (r *ManufacturerRepository) Create(ctx context.Context, m domain.Manufacturer) error {
	err := r.db.queries(ctx).CreateManufacturer(ctx, sqlc.CreateManufacturerParams{
		ID: uuid.UUID(m.ID), Name: m.Name, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, DeletedAt: m.DeletedAt,
	})

	switch {
	case isUniqueViolation(err, manufacturerNameKey):
		return domain.ErrManufacturerNameTaken
	case err != nil:
		return fmt.Errorf("insert manufacturer: %w", err)
	}

	return nil
}

// Update stores every mutable field of m.
func (r *ManufacturerRepository) Update(ctx context.Context, m domain.Manufacturer) error {
	n, err := r.db.queries(ctx).UpdateManufacturer(ctx, sqlc.UpdateManufacturerParams{
		ID: uuid.UUID(m.ID), Name: m.Name, UpdatedAt: m.UpdatedAt, DeletedAt: m.DeletedAt,
	})

	switch {
	case isUniqueViolation(err, manufacturerNameKey):
		return domain.ErrManufacturerNameTaken
	case err != nil:
		return fmt.Errorf("update manufacturer: %w", err)
	case n == 0:
		return domain.ErrManufacturerNotFound
	}

	return nil
}

// Get returns the manufacturer with id and its counts.
func (r *ManufacturerRepository) Get(ctx context.Context, id domain.ManufacturerID) (domain.ManufacturerListItem, error) {
	row, err := r.db.queries(ctx).GetManufacturer(ctx, uuid.UUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ManufacturerListItem{}, domain.ErrManufacturerNotFound
	}

	if err != nil {
		return domain.ManufacturerListItem{}, fmt.Errorf("select manufacturer: %w", err)
	}

	return toManufacturerListItem(sqlc.ListManufacturersRow(row)), nil
}

// GetMany returns the known manufacturers among ids.
func (r *ManufacturerRepository) GetMany(ctx context.Context, ids []domain.ManufacturerID) ([]domain.Manufacturer, error) {
	rows, err := r.db.queries(ctx).GetManufacturersByIDs(ctx, uuids(ids))
	if err != nil {
		return nil, fmt.Errorf("select manufacturers: %w", err)
	}

	out := make([]domain.Manufacturer, 0, len(rows))
	for _, row := range rows {
		out = append(out, toManufacturer(row))
	}

	return out, nil
}

// List returns one page of the manufacturers filter keeps, ordered by
// lower(name), then ID.
func (r *ManufacturerRepository) List(
	ctx context.Context, filter port.CatalogFilter, page domain.PageRequest,
) (domain.Page[domain.ManufacturerListItem], error) {
	cursor, err := decodeNameCursor(page)
	if err != nil {
		return domain.Page[domain.ManufacturerListItem]{}, err
	}

	pattern := containsPattern(filter.Query)

	var result domain.Page[domain.ManufacturerListItem]

	err = r.db.readTx(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		rows, err := q.ListManufacturers(ctx, sqlc.ListManufacturersParams{
			Pattern: pattern, IncludeDeleted: filter.IncludeDeleted,
			HasCursor: cursor.has, CursorName: cursor.name, CursorID: cursor.id, RowLimit: pageLimit(page.Limit),
		})
		if err != nil {
			return fmt.Errorf("select manufacturers: %w", err)
		}

		result.Total, err = q.CountManufacturers(ctx, sqlc.CountManufacturersParams{Pattern: pattern, IncludeDeleted: filter.IncludeDeleted})
		if err != nil {
			return fmt.Errorf("count manufacturers: %w", err)
		}

		rows, more := rows[:min(len(rows), page.Limit)], len(rows) > page.Limit

		result.Items = make([]domain.ManufacturerListItem, 0, len(rows))
		for _, row := range rows {
			result.Items = append(result.Items, toManufacturerListItem(row))
		}

		if more {
			last := result.Items[len(result.Items)-1]
			result.NextCursor = domain.EncodeCursor(domain.StorageCursor{Name: strings.ToLower(last.Name), ID: last.ID.String()})
		}

		return nil
	})
	if err != nil {
		return domain.Page[domain.ManufacturerListItem]{}, err
	}

	return result, nil
}

// HardwareModelRepository implements port.HardwareModelRepository.
type HardwareModelRepository struct{ db *DB }

// NewHardwareModelRepository returns a HardwareModelRepository on db.
func NewHardwareModelRepository(db *DB) *HardwareModelRepository {
	return &HardwareModelRepository{db: db}
}

// Create inserts m.
func (r *HardwareModelRepository) Create(ctx context.Context, m domain.HardwareModel) error {
	err := r.db.queries(ctx).CreateHardwareModel(ctx, sqlc.CreateHardwareModelParams{
		ID: uuid.UUID(m.ID), ManufacturerID: uuid.UUID(m.ManufacturerID), Name: m.Name,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, DeletedAt: m.DeletedAt,
	})

	switch {
	case isUniqueViolation(err, hardwareModelNameKey):
		return domain.ErrHardwareModelNameTaken
	case err != nil:
		return fmt.Errorf("insert hardware model: %w", err)
	}

	return nil
}

// Update stores the name, UpdatedAt and DeletedAt of m.
func (r *HardwareModelRepository) Update(ctx context.Context, m domain.HardwareModel) error {
	n, err := r.db.queries(ctx).UpdateHardwareModel(ctx, sqlc.UpdateHardwareModelParams{
		ID: uuid.UUID(m.ID), Name: m.Name, UpdatedAt: m.UpdatedAt, DeletedAt: m.DeletedAt,
	})

	switch {
	case isUniqueViolation(err, hardwareModelNameKey):
		return domain.ErrHardwareModelNameTaken
	case err != nil:
		return fmt.Errorf("update hardware model: %w", err)
	case n == 0:
		return domain.ErrHardwareModelNotFound
	}

	return nil
}

// Get returns the model with id and its count.
func (r *HardwareModelRepository) Get(ctx context.Context, id domain.HardwareModelID) (domain.HardwareModelListItem, error) {
	row, err := r.db.queries(ctx).GetHardwareModel(ctx, uuid.UUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.HardwareModelListItem{}, domain.ErrHardwareModelNotFound
	}

	if err != nil {
		return domain.HardwareModelListItem{}, fmt.Errorf("select hardware model: %w", err)
	}

	return toHardwareModelListItem(sqlc.ListHardwareModelsRow(row)), nil
}

// GetMany returns the known models among ids.
func (r *HardwareModelRepository) GetMany(ctx context.Context, ids []domain.HardwareModelID) ([]domain.HardwareModel, error) {
	rows, err := r.db.queries(ctx).GetHardwareModelsByIDs(ctx, uuids(ids))
	if err != nil {
		return nil, fmt.Errorf("select hardware models: %w", err)
	}

	out := make([]domain.HardwareModel, 0, len(rows))
	for _, row := range rows {
		out = append(out, toHardwareModel(row))
	}

	return out, nil
}

// List returns one page of the manufacturer's models filter keeps, ordered
// by lower(name), then ID.
func (r *HardwareModelRepository) List(
	ctx context.Context, manufacturer domain.ManufacturerID, filter port.CatalogFilter, page domain.PageRequest,
) (domain.Page[domain.HardwareModelListItem], error) {
	cursor, err := decodeNameCursor(page)
	if err != nil {
		return domain.Page[domain.HardwareModelListItem]{}, err
	}

	pattern := containsPattern(filter.Query)

	var result domain.Page[domain.HardwareModelListItem]

	err = r.db.readTx(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		rows, err := q.ListHardwareModels(ctx, sqlc.ListHardwareModelsParams{
			ManufacturerID: uuid.UUID(manufacturer), Pattern: pattern, IncludeDeleted: filter.IncludeDeleted,
			HasCursor: cursor.has, CursorName: cursor.name, CursorID: cursor.id, RowLimit: pageLimit(page.Limit),
		})
		if err != nil {
			return fmt.Errorf("select hardware models: %w", err)
		}

		result.Total, err = q.CountHardwareModels(ctx, sqlc.CountHardwareModelsParams{
			ManufacturerID: uuid.UUID(manufacturer), Pattern: pattern, IncludeDeleted: filter.IncludeDeleted,
		})
		if err != nil {
			return fmt.Errorf("count hardware models: %w", err)
		}

		rows, more := rows[:min(len(rows), page.Limit)], len(rows) > page.Limit

		result.Items = make([]domain.HardwareModelListItem, 0, len(rows))
		for _, row := range rows {
			result.Items = append(result.Items, toHardwareModelListItem(row))
		}

		if more {
			last := result.Items[len(result.Items)-1]
			result.NextCursor = domain.EncodeCursor(domain.StorageCursor{Name: strings.ToLower(last.Name), ID: last.ID.String()})
		}

		return nil
	})
	if err != nil {
		return domain.Page[domain.HardwareModelListItem]{}, err
	}

	return result, nil
}

func toManufacturer(row sqlc.Manufacturer) domain.Manufacturer {
	return domain.Manufacturer{
		ID: domain.ManufacturerID(row.ID), Name: row.Name,
		CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(), DeletedAt: utcPtr(row.DeletedAt),
	}
}

func toManufacturerListItem(row sqlc.ListManufacturersRow) domain.ManufacturerListItem {
	return domain.ManufacturerListItem{
		Manufacturer: toManufacturer(sqlc.Manufacturer{
			ID: row.ID, Name: row.Name, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt,
		}),
		ModelCount: row.ModelCount, ItemCount: row.ItemCount,
	}
}

func toHardwareModel(row sqlc.HardwareModel) domain.HardwareModel {
	return domain.HardwareModel{
		ID: domain.HardwareModelID(row.ID), ManufacturerID: domain.ManufacturerID(row.ManufacturerID), Name: row.Name,
		CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(), DeletedAt: utcPtr(row.DeletedAt),
	}
}

func toHardwareModelListItem(row sqlc.ListHardwareModelsRow) domain.HardwareModelListItem {
	return domain.HardwareModelListItem{
		HardwareModel: toHardwareModel(sqlc.HardwareModel{
			ID: row.ID, ManufacturerID: row.ManufacturerID, Name: row.Name,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt,
		}),
		ItemCount: row.ItemCount,
	}
}

// toPgUUID converts an optional ID to a nullable UUID.
func toPgUUID[T ~[16]byte](id *T) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}

	return pgtype.UUID{Bytes: *id, Valid: true}
}

// fromPgUUID converts a nullable UUID to an optional ID.
func fromPgUUID[T ~[16]byte](u pgtype.UUID) *T {
	if !u.Valid {
		return nil
	}

	id := T(u.Bytes)

	return &id
}
