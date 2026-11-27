package port

import (
	"context"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// CreateStorageItemInput carries the data for a new storage item. Usable is
// required.
type CreateStorageItemInput struct {
	AssetTag              string
	Name                  string
	Description           string
	StorageID             domain.StorageID
	TypeID                domain.ItemTypeID
	Usable                *bool
	OrderDate             *time.Time
	CommissionedAt        *time.Time
	DecommissionedAt      *time.Time
	DecommissioningReason string
	ResponsibleID         *domain.UserID
	Modules               []domain.ModuleParams
}

// UpdateStorageItemInput is a partial update; nil fields stay unchanged.
// StorageID moves the item; Modules replaces all module instances (pass an
// instance's ID to keep it). The asset tag cannot change.
type UpdateStorageItemInput struct {
	Name                  *string
	Description           *string
	StorageID             *domain.StorageID
	TypeID                *domain.ItemTypeID
	Usable                *bool
	OrderDate             *domain.StorageItemDateChange
	CommissionedAt        *domain.StorageItemDateChange
	DecommissionedAt      *domain.StorageItemDateChange
	DecommissioningReason *string
	Responsible           *domain.StorageItemResponsibleChange
	Modules               *[]domain.ModuleParams
}

// StorageItemFilter narrows a storage item list. Zero values do not filter.
type StorageItemFilter struct {
	// Recursive includes items kept anywhere in the storage's subtree, not
	// only directly in it. Only used when listing one storage.
	Recursive bool
	// Deleted lists only deleted items instead of only live ones.
	Deleted bool
	// Query keeps items whose name, description, asset tag, manufacturer,
	// model, serial number, host name or interface MAC contains it,
	// ignoring case. A query that parses as a MAC also matches it in any
	// notation.
	Query string
	// TypeID keeps items of the type, and with IncludeSubtypes also those
	// of its descendants.
	TypeID          *domain.ItemTypeID
	IncludeSubtypes bool
	Usable          *bool
	ResponsibleID   *domain.UserID
	// Decommissioned keeps items with (true) or without (false) a
	// decommissioning date.
	Decommissioned *bool
}

// StorageItemView is a storage item with its type, responsible person and
// hardware manufacturer and model resolved. Responsible is nil when nobody
// is responsible; Manufacturer and Model are nil unless the hardware module
// names them. An unknown entity has only the ID set.
type StorageItemView struct {
	domain.StorageItem

	Type         domain.ItemType
	Responsible  *domain.User
	Manufacturer *domain.Manufacturer
	Model        *domain.HardwareModel
}

// StorageItemService is the driving port for storage item use cases.
// Delete only marks an item deleted; Restore undoes it. Deleted items
// cannot be updated.
type StorageItemService interface {
	// Get returns the item, also when it is deleted.
	Get(ctx context.Context, id domain.StorageItemID) (StorageItemView, error)
	// ListInStorage returns one page of the items in storage, Search one
	// page across all storages; both order by name.
	ListInStorage(
		ctx context.Context, storage domain.StorageID, filter StorageItemFilter, page domain.PageRequest,
	) (domain.Page[StorageItemView], error)
	Search(ctx context.Context, filter StorageItemFilter, page domain.PageRequest) (domain.Page[StorageItemView], error)
	Create(ctx context.Context, actor domain.UserID, in CreateStorageItemInput) (StorageItemView, error)
	Update(ctx context.Context, actor domain.UserID, id domain.StorageItemID, in UpdateStorageItemInput) (StorageItemView, error)
	Delete(ctx context.Context, actor domain.UserID, id domain.StorageItemID) error
	Restore(ctx context.Context, actor domain.UserID, id domain.StorageItemID) (StorageItemView, error)
}

// StorageItemRepository is the driven port for persisting storage items
// with their modules.
//
// Create returns domain.ErrAssetTagTaken when any item, deleted ones
// included, has the asset tag, compared case-insensitively. Update stores
// every mutable field including DeletedAt and the modules (by ID) and
// returns domain.ErrStorageItemNotFound for an unknown item. Get returns
// the item, also when deleted, or domain.ErrStorageItemNotFound. GetMany
// returns the known items among ids, deleted ones included, in no
// particular order. ListInStorage (Recursive applies) and Search return
// either only live or only deleted items (Deleted), apply the other
// filters and order by lower(name), then ID. Modules are always in
// domain.SortModules order. AssetTagTaken reports whether any item has tag.
type StorageItemRepository interface {
	Create(ctx context.Context, item domain.StorageItem) error
	Update(ctx context.Context, item domain.StorageItem) error
	Get(ctx context.Context, id domain.StorageItemID) (domain.StorageItem, error)
	GetMany(ctx context.Context, ids []domain.StorageItemID) ([]domain.StorageItem, error)
	ListInStorage(
		ctx context.Context, storage domain.StorageID, filter StorageItemFilter, page domain.PageRequest,
	) (domain.Page[domain.StorageItem], error)
	Search(ctx context.Context, filter StorageItemFilter, page domain.PageRequest) (domain.Page[domain.StorageItem], error)
	AssetTagTaken(ctx context.Context, tag string) (bool, error)
}
