package port

import (
	"context"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// CreateStorageInput carries the data for a new storage. A nil ParentID
// creates a root storage.
type CreateStorageInput struct {
	Name     string
	ParentID *domain.StorageID
}

// UpdateStorageInput is a partial update; nil fields stay unchanged. Parent
// moves the storage (a nil Parent.ID makes it a root).
type UpdateStorageInput struct {
	Name   *string
	Parent *domain.StorageParentChange
}

// StorageService is the driving port for storage use cases. Storages are
// never deleted, so there is no Delete.
type StorageService interface {
	Create(ctx context.Context, actor domain.UserID, in CreateStorageInput) (domain.StorageListItem, error)
	Get(ctx context.Context, id domain.StorageID) (domain.StorageListItem, error)
	// List returns one page of parent's direct children ordered by name; a
	// nil parent lists the root storages.
	List(ctx context.Context, parent *domain.StorageID, page domain.PageRequest) (domain.Page[domain.StorageListItem], error)
	Update(ctx context.Context, actor domain.UserID, id domain.StorageID, in UpdateStorageInput) (domain.StorageListItem, error)
}

// StorageRepository is the driven port for persisting storages.
//
// Create and Update return domain.ErrStorageNameTaken when a sibling (a
// storage with the same parent, roots included) already has the name,
// compared case-insensitively. Get returns domain.ErrStorageNotFound.
// ListChildren orders by lower(name), then ID. Get and ListChildren fill
// ChildCount and ItemCount (live items of any kind kept directly in it).
// IsDescendant reports whether candidate lies in ancestor's subtree,
// excluding ancestor itself. LockTree serializes tree moves until the
// surrounding transaction ends, so concurrent moves cannot form a cycle.
type StorageRepository interface {
	Create(ctx context.Context, storage domain.Storage) error
	Update(ctx context.Context, storage domain.Storage) error
	Get(ctx context.Context, id domain.StorageID) (domain.StorageListItem, error)
	ListChildren(ctx context.Context, parent *domain.StorageID, page domain.PageRequest) (domain.Page[domain.StorageListItem], error)
	IsDescendant(ctx context.Context, ancestor, candidate domain.StorageID) (bool, error)
	LockTree(ctx context.Context) error
}
