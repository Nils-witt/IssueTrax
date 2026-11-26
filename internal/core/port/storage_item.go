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

// GenericStorageItemService is the driving port for generic storage item
// use cases. Delete only marks an item deleted; Restore undoes it. Deleted
// items cannot be updated.
type GenericStorageItemService interface {
	Create(ctx context.Context, actor domain.UserID, in CreateGenericStorageItemInput) (domain.GenericStorageItem, error)
	// Get returns the item, also when it is deleted.
	Get(ctx context.Context, id domain.StorageItemID) (domain.GenericStorageItem, error)
	// ListInStorage returns one page of the items in storage (Recursive and
	// Deleted apply), ordered by name.
	ListInStorage(
		ctx context.Context, storage domain.StorageID, filter StorageItemFilter, page domain.PageRequest,
	) (domain.Page[domain.GenericStorageItem], error)
	// Search returns one page of items across all storages (Query and
	// Deleted apply), ordered by name.
	Search(ctx context.Context, filter StorageItemFilter, page domain.PageRequest) (domain.Page[domain.GenericStorageItem], error)
	Update(
		ctx context.Context, actor domain.UserID, id domain.StorageItemID, in UpdateGenericStorageItemInput,
	) (domain.GenericStorageItem, error)
	Delete(ctx context.Context, actor domain.UserID, id domain.StorageItemID) error
	Restore(ctx context.Context, actor domain.UserID, id domain.StorageItemID) (domain.GenericStorageItem, error)
}

// GenericStorageItemRepository is the driven port for persisting generic
// storage items. It only sees items of kind domain.StorageItemKindGeneric.
//
// Get returns domain.ErrStorageItemNotFound, deleted items included. Update
// stores every mutable field including DeletedAt and returns
// domain.ErrStorageItemNotFound for an unknown item. ListInStorage and
// Search order by lower(name), then ID, and return either only live or only
// deleted items (filter.Deleted).
type GenericStorageItemRepository interface {
	Create(ctx context.Context, item domain.GenericStorageItem) error
	Update(ctx context.Context, item domain.GenericStorageItem) error
	Get(ctx context.Context, id domain.StorageItemID) (domain.GenericStorageItem, error)
	ListInStorage(
		ctx context.Context, storage domain.StorageID, filter StorageItemFilter, page domain.PageRequest,
	) (domain.Page[domain.GenericStorageItem], error)
	Search(ctx context.Context, filter StorageItemFilter, page domain.PageRequest) (domain.Page[domain.GenericStorageItem], error)
}
