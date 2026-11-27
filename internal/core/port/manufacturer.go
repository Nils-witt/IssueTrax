package port

import (
	"context"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// CatalogFilter narrows a manufacturer or hardware model list.
type CatalogFilter struct {
	// Query keeps entries whose name contains it, ignoring case.
	Query string
	// IncludeDeleted lists deleted entries too; otherwise only live ones.
	IncludeDeleted bool
}

// ManufacturerService is the driving port for the central manufacturer
// list. Everyone may read and change it. Delete only marks a manufacturer
// deleted: items keep it, but it cannot be picked for items or get new
// models.
type ManufacturerService interface {
	// Get returns the manufacturer, also when it is deleted.
	Get(ctx context.Context, id domain.ManufacturerID) (domain.ManufacturerListItem, error)
	// List returns one page of manufacturers ordered by name.
	List(ctx context.Context, filter CatalogFilter, page domain.PageRequest) (domain.Page[domain.ManufacturerListItem], error)
	Create(ctx context.Context, actor domain.UserID, name string) (domain.ManufacturerListItem, error)
	Rename(ctx context.Context, actor domain.UserID, id domain.ManufacturerID, name string) (domain.ManufacturerListItem, error)
	Delete(ctx context.Context, actor domain.UserID, id domain.ManufacturerID) error
	// Restore fails with domain.ErrManufacturerNameTaken when a live
	// manufacturer has taken the name meanwhile.
	Restore(ctx context.Context, actor domain.UserID, id domain.ManufacturerID) (domain.ManufacturerListItem, error)
}

// HardwareModelService is the driving port for the models of the central
// manufacturer list. Everyone may read and change them. Delete only marks
// a model deleted: items keep it, but it cannot be picked for items.
type HardwareModelService interface {
	// Get returns the model, also when it is deleted.
	Get(ctx context.Context, id domain.HardwareModelID) (domain.HardwareModelListItem, error)
	// List returns one page of the manufacturer's models ordered by name.
	List(
		ctx context.Context, manufacturer domain.ManufacturerID, filter CatalogFilter, page domain.PageRequest,
	) (domain.Page[domain.HardwareModelListItem], error)
	// Create adds a model to a live manufacturer.
	Create(ctx context.Context, actor domain.UserID, manufacturer domain.ManufacturerID, name string) (domain.HardwareModelListItem, error)
	Rename(ctx context.Context, actor domain.UserID, id domain.HardwareModelID, name string) (domain.HardwareModelListItem, error)
	Delete(ctx context.Context, actor domain.UserID, id domain.HardwareModelID) error
	// Restore fails with domain.ErrHardwareModelNameTaken when a live model
	// of the manufacturer has taken the name meanwhile.
	Restore(ctx context.Context, actor domain.UserID, id domain.HardwareModelID) (domain.HardwareModelListItem, error)
}

// ManufacturerRepository is the driven port for persisting manufacturers.
//
// Create and Update return domain.ErrManufacturerNameTaken when another
// live manufacturer has the name, compared case-insensitively; deleted
// ones do not hold their name. Update stores every mutable field including
// DeletedAt. Get returns domain.ErrManufacturerNotFound. Get and List fill
// ModelCount (live models) and ItemCount (live items whose hardware names
// the manufacturer); List orders by lower(name), then ID. GetMany skips
// unknown IDs.
type ManufacturerRepository interface {
	Create(ctx context.Context, m domain.Manufacturer) error
	Update(ctx context.Context, m domain.Manufacturer) error
	Get(ctx context.Context, id domain.ManufacturerID) (domain.ManufacturerListItem, error)
	GetMany(ctx context.Context, ids []domain.ManufacturerID) ([]domain.Manufacturer, error)
	List(ctx context.Context, filter CatalogFilter, page domain.PageRequest) (domain.Page[domain.ManufacturerListItem], error)
}

// HardwareModelRepository is the driven port for persisting hardware
// models.
//
// Create and Update return domain.ErrHardwareModelNameTaken when another
// live model of the same manufacturer has the name, compared
// case-insensitively; deleted ones do not hold their name. Update stores
// the name, UpdatedAt and DeletedAt. Get returns
// domain.ErrHardwareModelNotFound. Get and List fill ItemCount (live items
// whose hardware names the model); List orders by lower(name), then ID.
// GetMany skips unknown IDs.
type HardwareModelRepository interface {
	Create(ctx context.Context, m domain.HardwareModel) error
	Update(ctx context.Context, m domain.HardwareModel) error
	Get(ctx context.Context, id domain.HardwareModelID) (domain.HardwareModelListItem, error)
	GetMany(ctx context.Context, ids []domain.HardwareModelID) ([]domain.HardwareModel, error)
	List(
		ctx context.Context, manufacturer domain.ManufacturerID, filter CatalogFilter, page domain.PageRequest,
	) (domain.Page[domain.HardwareModelListItem], error)
}
