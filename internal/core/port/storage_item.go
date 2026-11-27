package port

import (
	"context"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// CreateGenericStorageItemInput carries the data for a new generic item.
type CreateGenericStorageItemInput struct {
	StorageID   domain.StorageID
	Name        string
	Description string
}

// UpdateGenericStorageItemInput is a partial update; nil fields stay
// unchanged. StorageID moves the item to another storage.
type UpdateGenericStorageItemInput struct {
	Name        *string
	Description *string
	StorageID   *domain.StorageID
}

// StorageItemFilter narrows a storage item list.
type StorageItemFilter struct {
	// Recursive includes items kept anywhere in the storage's subtree, not
	// only directly in it. Only used when listing one storage.
	Recursive bool
	// Deleted lists only deleted items instead of only live ones.
	Deleted bool
	// Query keeps items whose name or description contains it, ignoring
	// case. Only used when searching; empty matches everything.
	Query string
}

// StorageItemReader reads the storage items of one kind T. Both the
// driving and the driven port of every item kind include it.
//
// Get returns the item, also when it is deleted, or
// domain.ErrStorageItemNotFound (also for items of another kind).
// ListInStorage returns one page of the items in storage (Recursive and
// Deleted apply); Search one page across all storages (Query and Deleted
// apply). Both order by lower(name), then ID, and return either only live
// or only deleted items.
type StorageItemReader[T any] interface {
	Get(ctx context.Context, id domain.StorageItemID) (T, error)
	ListInStorage(
		ctx context.Context, storage domain.StorageID, filter StorageItemFilter, page domain.PageRequest,
	) (domain.Page[T], error)
	Search(ctx context.Context, filter StorageItemFilter, page domain.PageRequest) (domain.Page[T], error)
}

// StorageItemService holds the driving-port use cases every item kind T
// offers besides its own Create and Update. Delete only marks an item
// deleted; Restore undoes it. Deleted items cannot be updated.
type StorageItemService[T any] interface {
	StorageItemReader[T]
	Delete(ctx context.Context, actor domain.UserID, id domain.StorageItemID) error
	Restore(ctx context.Context, actor domain.UserID, id domain.StorageItemID) (T, error)
}

// GenericStorageItemService is the driving port for generic storage item
// use cases. Search matches name and description.
type GenericStorageItemService interface {
	StorageItemService[domain.GenericStorageItem]
	Create(ctx context.Context, actor domain.UserID, in CreateGenericStorageItemInput) (domain.GenericStorageItem, error)
	Update(
		ctx context.Context, actor domain.UserID, id domain.StorageItemID, in UpdateGenericStorageItemInput,
	) (domain.GenericStorageItem, error)
}

// GenericStorageItemRepository is the driven port for persisting generic
// storage items. It only sees items of kind domain.StorageItemKindGeneric;
// Search matches name and description.
//
// Update stores every mutable field including DeletedAt and returns
// domain.ErrStorageItemNotFound for an unknown item.
type GenericStorageItemRepository interface {
	StorageItemReader[domain.GenericStorageItem]
	Create(ctx context.Context, item domain.GenericStorageItem) error
	Update(ctx context.Context, item domain.GenericStorageItem) error
}

// StorageItemLookupService is the driving port for finding storage items
// of any kind, e.g. to link them to issues. Search matches name and
// description (Query and Deleted apply) and orders by lower(name), then ID.
type StorageItemLookupService interface {
	Search(ctx context.Context, filter StorageItemFilter, page domain.PageRequest) (domain.Page[domain.StorageItem], error)
}

// StorageItemRepository is the kind-agnostic driven port for reading the
// shared fields of storage items of every kind.
//
// GetMany returns the known items among ids, deleted ones included, in no
// particular order and skips unknown ones. Search behaves like
// StorageItemReader.Search across all kinds.
type StorageItemRepository interface {
	GetMany(ctx context.Context, ids []domain.StorageItemID) ([]domain.StorageItem, error)
	Search(ctx context.Context, filter StorageItemFilter, page domain.PageRequest) (domain.Page[domain.StorageItem], error)
}
