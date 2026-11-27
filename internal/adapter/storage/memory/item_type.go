package memory

import (
	"context"
	"slices"
	"strings"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time checks that the repositories satisfy the driven ports.
var (
	_ port.ItemTypeRepository        = (*ItemTypeRepository)(nil)
	_ port.AssetTagCounterRepository = (*AssetTagCounterRepository)(nil)
)

// ItemTypeRepository implements port.ItemTypeRepository on a Store.
type ItemTypeRepository struct{ s *Store }

// NewItemTypeRepository returns an ItemTypeRepository backed by s.
func NewItemTypeRepository(s *Store) *ItemTypeRepository { return &ItemTypeRepository{s: s} }

// Create inserts t, rejecting a live sibling with the same name.
func (r *ItemTypeRepository) Create(_ context.Context, t domain.ItemType) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	if r.siblingNameTaken(t) {
		return domain.ErrItemTypeNameTaken
	}

	r.s.data.itemTypes[t.ID] = cloneItemType(t)

	return nil
}

// Update replaces an existing type, rejecting a live sibling with the same
// name while t is live.
func (r *ItemTypeRepository) Update(_ context.Context, t domain.ItemType) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	current, ok := r.s.data.itemTypes[t.ID]
	if !ok {
		return domain.ErrItemTypeNotFound
	}

	if r.siblingNameTaken(t) {
		return domain.ErrItemTypeNameTaken
	}

	t.CreatedAt = current.CreatedAt
	r.s.data.itemTypes[t.ID] = cloneItemType(t)

	return nil
}

// Get returns the type with id and its counts.
func (r *ItemTypeRepository) Get(_ context.Context, id domain.ItemTypeID) (domain.ItemTypeListItem, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	t, ok := r.s.data.itemTypes[id]
	if !ok {
		return domain.ItemTypeListItem{}, domain.ErrItemTypeNotFound
	}

	return r.listItem(t), nil
}

// GetMany returns the known types among ids.
func (r *ItemTypeRepository) GetMany(_ context.Context, ids []domain.ItemTypeID) ([]domain.ItemType, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	out := make([]domain.ItemType, 0, len(ids))

	for _, id := range ids {
		if t, ok := r.s.data.itemTypes[id]; ok {
			out = append(out, cloneItemType(t))
		}
	}

	return out, nil
}

// ListChildren returns one page of parent's children ordered by
// lower(name), then ID.
func (r *ItemTypeRepository) ListChildren(
	_ context.Context, parent *domain.ItemTypeID, filter port.ItemTypeFilter, page domain.PageRequest,
) (domain.Page[domain.ItemTypeListItem], error) {
	return r.page(page, func(t domain.ItemType) bool {
		return sameTypeParent(t.ParentID, parent) && (filter.IncludeDeleted || !t.IsDeleted())
	})
}

// Search returns one page of the types whose name contains query, ignoring
// case, ordered by lower(name), then ID.
func (r *ItemTypeRepository) Search(
	_ context.Context, query string, filter port.ItemTypeFilter, page domain.PageRequest,
) (domain.Page[domain.ItemTypeListItem], error) {
	query = strings.ToLower(query)

	return r.page(page, func(t domain.ItemType) bool {
		return strings.Contains(strings.ToLower(t.Name), query) && (filter.IncludeDeleted || !t.IsDeleted())
	})
}

// Chain returns the ancestors of id from the root down to id itself.
func (r *ItemTypeRepository) Chain(_ context.Context, id domain.ItemTypeID) ([]domain.ItemType, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	t, ok := r.s.data.itemTypes[id]
	if !ok {
		return nil, domain.ErrItemTypeNotFound
	}

	chain := []domain.ItemType{cloneItemType(t)}
	seen := map[domain.ItemTypeID]bool{t.ID: true}

	for t.ParentID != nil && !seen[*t.ParentID] {
		if t, ok = r.s.data.itemTypes[*t.ParentID]; !ok {
			break
		}

		seen[t.ID] = true
		chain = append(chain, cloneItemType(t))
	}

	slices.Reverse(chain)

	return chain, nil
}

// IsDescendant walks candidate's ancestors looking for ancestor.
func (r *ItemTypeRepository) IsDescendant(_ context.Context, ancestor, candidate domain.ItemTypeID) (bool, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	return r.s.typeSubtree(ancestor)[candidate] && candidate != ancestor, nil
}

// LockTree is a no-op: Store already serializes transactions.
func (r *ItemTypeRepository) LockTree(context.Context) error { return nil }

func (r *ItemTypeRepository) page(
	page domain.PageRequest, keep func(domain.ItemType) bool,
) (domain.Page[domain.ItemTypeListItem], error) {
	r.s.mu.RLock()

	var all []domain.ItemTypeListItem

	for _, t := range r.s.data.itemTypes {
		if keep(t) {
			all = append(all, r.listItem(t))
		}
	}

	r.s.mu.RUnlock()

	return pageByName(all, func(t domain.ItemTypeListItem) (string, string) { return t.Name, t.ID.String() }, page)
}

// siblingNameTaken reports whether another live type with the same parent
// has t's name, while t is live. Callers hold r.s.mu.
func (r *ItemTypeRepository) siblingNameTaken(t domain.ItemType) bool {
	if t.IsDeleted() {
		return false
	}

	for _, o := range r.s.data.itemTypes {
		if o.ID != t.ID && !o.IsDeleted() && sameTypeParent(o.ParentID, t.ParentID) && sameName(o.Name, t.Name) {
			return true
		}
	}

	return false
}

// listItem adds the counts to t. Callers hold r.s.mu.
func (r *ItemTypeRepository) listItem(t domain.ItemType) domain.ItemTypeListItem {
	item := domain.ItemTypeListItem{ItemType: cloneItemType(t)}

	for _, c := range r.s.data.itemTypes {
		if c.ParentID != nil && *c.ParentID == t.ID {
			item.ChildCount++
		}
	}

	for _, i := range r.s.data.storageItems {
		if i.TypeID == t.ID && !i.IsDeleted() {
			item.ItemCount++
		}
	}

	return item
}

// typeSubtree returns root and all types below it. Callers hold s.mu.
func (s *Store) typeSubtree(root domain.ItemTypeID) map[domain.ItemTypeID]bool {
	in := map[domain.ItemTypeID]bool{root: true}

	for grew := true; grew; {
		grew = false

		for _, t := range s.data.itemTypes {
			if t.ParentID != nil && in[*t.ParentID] && !in[t.ID] {
				in[t.ID] = true
				grew = true
			}
		}
	}

	return in
}

func cloneItemType(t domain.ItemType) domain.ItemType {
	t.ParentID = clonePtr(t.ParentID)
	t.DeletedAt = clonePtr(t.DeletedAt)
	t.Defaults = slices.Clone(t.Defaults)

	return t
}

func sameTypeParent(a, b *domain.ItemTypeID) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

// AssetTagCounterRepository implements port.AssetTagCounterRepository on a
// Store.
type AssetTagCounterRepository struct{ s *Store }

// NewAssetTagCounterRepository returns an AssetTagCounterRepository backed by s.
func NewAssetTagCounterRepository(s *Store) *AssetTagCounterRepository {
	return &AssetTagCounterRepository{s: s}
}

// Get returns the counter of prefix, 0 when there is none.
func (r *AssetTagCounterRepository) Get(_ context.Context, prefix string) (int64, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	return r.s.data.assetTagCounters[strings.ToLower(prefix)], nil
}

// Set stores the counter of prefix.
func (r *AssetTagCounterRepository) Set(_ context.Context, prefix string, value int64) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	r.s.data.assetTagCounters[strings.ToLower(prefix)] = value

	return nil
}
