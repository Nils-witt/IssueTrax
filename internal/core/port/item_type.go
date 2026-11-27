package port

import (
	"context"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

// CreateItemTypeInput carries the data for a new item type. A nil ParentID
// creates a root type.
type CreateItemTypeInput struct {
	Name      string
	ParentID  *domain.ItemTypeID
	TagPrefix string
	Defaults  []domain.ModuleDefault
}

// UpdateItemTypeInput is a partial update; nil fields stay unchanged.
// Parent moves the type (a nil Parent.ID makes it a root); Defaults
// replaces the type's own defaults.
type UpdateItemTypeInput struct {
	Name      *string
	Parent    *domain.ItemTypeParentChange
	TagPrefix *string
	Defaults  *[]domain.ModuleDefault
}

// ItemTypeFilter narrows an item type list.
type ItemTypeFilter struct {
	// IncludeDeleted lists deleted types too; otherwise only live ones.
	IncludeDeleted bool
}

// ItemTypeView is an item type with what it inherits.
type ItemTypeView struct {
	domain.ItemTypeListItem

	// Path lists the type's ancestors from the root down to its parent.
	Path []domain.ItemType
	// EffectiveDefaults merges the defaults of the type and its ancestors.
	EffectiveDefaults []domain.ModuleDefault
	// EffectiveTagPrefix is the prefix proposed asset tags start with.
	EffectiveTagPrefix string
}

// ItemTypeService is the driving port for item type use cases. Everyone
// may read types; only admins (not service accounts) may change them,
// others get domain.ErrAdminRequired. Delete only marks a type deleted:
// its children stay live, but no type can be created under or moved into
// it and no item can be given it.
type ItemTypeService interface {
	Get(ctx context.Context, id domain.ItemTypeID) (ItemTypeView, error)
	// ListChildren returns one page of parent's direct children ordered by
	// name; a nil parent lists the roots.
	ListChildren(
		ctx context.Context, parent *domain.ItemTypeID, filter ItemTypeFilter, page domain.PageRequest,
	) (domain.Page[domain.ItemTypeListItem], error)
	// Search returns one page of the types whose name contains query,
	// ignoring case, across the whole tree, ordered by name.
	Search(ctx context.Context, query string, filter ItemTypeFilter, page domain.PageRequest) (domain.Page[ItemTypeView], error)
	// NextAssetTag proposes the asset tag for a new item of the type. It
	// reserves nothing.
	NextAssetTag(ctx context.Context, id domain.ItemTypeID) (string, error)
	Create(ctx context.Context, actor domain.UserID, in CreateItemTypeInput) (ItemTypeView, error)
	Update(ctx context.Context, actor domain.UserID, id domain.ItemTypeID, in UpdateItemTypeInput) (ItemTypeView, error)
	Delete(ctx context.Context, actor domain.UserID, id domain.ItemTypeID) error
	Restore(ctx context.Context, actor domain.UserID, id domain.ItemTypeID) (ItemTypeView, error)
}

// ItemTypeRepository is the driven port for persisting item types.
//
// Create and Update return domain.ErrItemTypeNameTaken when a live sibling
// (same parent, roots included) has the name, compared case-insensitively;
// deleted types do not hold their name. Update stores every mutable field
// including DeletedAt. Get returns domain.ErrItemTypeNotFound. Get,
// ListChildren and Search fill ChildCount (all direct children) and
// ItemCount (live items of exactly the type) and order by lower(name),
// then ID. GetMany skips unknown IDs. Chain returns the type's ancestors
// from the root down to the type itself. IsDescendant reports whether
// candidate lies in ancestor's subtree, excluding ancestor itself. LockTree
// serializes tree moves until the surrounding transaction ends.
type ItemTypeRepository interface {
	Create(ctx context.Context, t domain.ItemType) error
	Update(ctx context.Context, t domain.ItemType) error
	Get(ctx context.Context, id domain.ItemTypeID) (domain.ItemTypeListItem, error)
	GetMany(ctx context.Context, ids []domain.ItemTypeID) ([]domain.ItemType, error)
	ListChildren(
		ctx context.Context, parent *domain.ItemTypeID, filter ItemTypeFilter, page domain.PageRequest,
	) (domain.Page[domain.ItemTypeListItem], error)
	Search(ctx context.Context, query string, filter ItemTypeFilter, page domain.PageRequest) (domain.Page[domain.ItemTypeListItem], error)
	Chain(ctx context.Context, id domain.ItemTypeID) ([]domain.ItemType, error)
	IsDescendant(ctx context.Context, ancestor, candidate domain.ItemTypeID) (bool, error)
	LockTree(ctx context.Context) error
}

// AssetTagCounterRepository is the driven port for the per-prefix counters
// of proposed asset tags. Prefixes are compared case-insensitively. Get
// returns 0 for a prefix without a counter.
type AssetTagCounterRepository interface {
	Get(ctx context.Context, prefix string) (int64, error)
	Set(ctx context.Context, prefix string, value int64) error
}
