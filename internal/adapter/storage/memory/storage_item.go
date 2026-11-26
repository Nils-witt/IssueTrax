package memory

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that GenericStorageItemRepository satisfies the driven port.
var _ port.GenericStorageItemRepository = (*GenericStorageItemRepository)(nil)

// GenericStorageItemRepository implements port.GenericStorageItemRepository
// on a Store.
type GenericStorageItemRepository struct{ s *Store }

// NewGenericStorageItemRepository returns a GenericStorageItemRepository backed by s.
func NewGenericStorageItemRepository(s *Store) *GenericStorageItemRepository {
	return &GenericStorageItemRepository{s: s}
}

// Create inserts item.
func (r *GenericStorageItemRepository) Create(_ context.Context, item domain.GenericStorageItem) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	item.Kind = domain.StorageItemKindGeneric
	r.s.data.storageItems[item.ID] = cloneStorageItem(item.StorageItem)

	return nil
}

// Update replaces an existing generic item.
func (r *GenericStorageItemRepository) Update(_ context.Context, item domain.GenericStorageItem) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	current, ok := r.s.data.storageItems[item.ID]
	if !ok || current.Kind != domain.StorageItemKindGeneric {
		return domain.ErrStorageItemNotFound
	}

	item.Kind = domain.StorageItemKindGeneric
	item.CreatedAt = current.CreatedAt
	r.s.data.storageItems[item.ID] = cloneStorageItem(item.StorageItem)

	return nil
}

// Get returns the generic item with id, deleted or not.
func (r *GenericStorageItemRepository) Get(_ context.Context, id domain.StorageItemID) (domain.GenericStorageItem, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	item, ok := r.s.data.storageItems[id]
	if !ok || item.Kind != domain.StorageItemKindGeneric {
		return domain.GenericStorageItem{}, domain.ErrStorageItemNotFound
	}

	return domain.GenericStorageItem{StorageItem: cloneStorageItem(item)}, nil
}

// ListInStorage returns one page of the generic items in storage (or its
// subtree), ordered by lower(name), then ID.
func (r *GenericStorageItemRepository) ListInStorage(
	_ context.Context, storage domain.StorageID, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.GenericStorageItem], error) {
	r.s.mu.RLock()

	storages := map[domain.StorageID]bool{storage: true}
	if filter.Recursive {
		storages = r.subtree(storage)
	}

	items := r.matching(func(item domain.StorageItem) bool {
		return storages[item.StorageID] && item.IsDeleted() == filter.Deleted
	})

	r.s.mu.RUnlock()

	return pageStorageItems(items, page)
}

// Search returns one page of the generic items whose name or description
// contains filter.Query, ignoring case, ordered by lower(name), then ID.
func (r *GenericStorageItemRepository) Search(
	_ context.Context, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.GenericStorageItem], error) {
	query := strings.ToLower(filter.Query)

	r.s.mu.RLock()

	items := r.matching(func(item domain.StorageItem) bool {
		return item.IsDeleted() == filter.Deleted &&
			(strings.Contains(strings.ToLower(item.Name), query) || strings.Contains(strings.ToLower(item.Description), query))
	})

	r.s.mu.RUnlock()

	return pageStorageItems(items, page)
}

// matching returns the generic items keep accepts. Callers hold r.s.mu.
func (r *GenericStorageItemRepository) matching(keep func(domain.StorageItem) bool) []domain.GenericStorageItem {
	var items []domain.GenericStorageItem

	for _, item := range r.s.data.storageItems {
		if item.Kind == domain.StorageItemKindGeneric && keep(item) {
			items = append(items, domain.GenericStorageItem{StorageItem: cloneStorageItem(item)})
		}
	}

	return items
}

// subtree returns root and all storages below it. Callers hold r.s.mu.
func (r *GenericStorageItemRepository) subtree(root domain.StorageID) map[domain.StorageID]bool {
	in := map[domain.StorageID]bool{root: true}

	for grew := true; grew; {
		grew = false

		for _, st := range r.s.data.storages {
			if st.ParentID != nil && in[*st.ParentID] && !in[st.ID] {
				in[st.ID] = true
				grew = true
			}
		}
	}

	return in
}

func pageStorageItems(all []domain.GenericStorageItem, page domain.PageRequest) (domain.Page[domain.GenericStorageItem], error) {
	cursor, hasCursor, err := domain.DecodeCursor[domain.StorageCursor](page.Cursor)
	if err != nil {
		return domain.Page[domain.GenericStorageItem]{}, err
	}

	slices.SortFunc(all, func(a, b domain.GenericStorageItem) int {
		return cmp.Or(
			cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)),
			cmp.Compare(a.ID.String(), b.ID.String()),
		)
	})

	after := all
	if hasCursor {
		after = slices.DeleteFunc(slices.Clone(all), func(i domain.GenericStorageItem) bool {
			return cmp.Or(
				cmp.Compare(strings.ToLower(i.Name), cursor.Name),
				cmp.Compare(i.ID.String(), cursor.ID),
			) <= 0
		})
	}

	items, more := paginate(after, page.Limit)
	result := domain.Page[domain.GenericStorageItem]{
		Items: append([]domain.GenericStorageItem{}, items...), Total: int64(len(all)),
	}

	if more {
		last := items[len(items)-1]
		result.NextCursor = domain.EncodeCursor(domain.StorageCursor{Name: strings.ToLower(last.Name), ID: last.ID.String()})
	}

	return result, nil
}

// cloneStorageItem copies the deletion time so stored items never alias
// caller memory.
func cloneStorageItem(item domain.StorageItem) domain.StorageItem {
	if item.DeletedAt != nil {
		at := *item.DeletedAt
		item.DeletedAt = &at
	}

	return item
}
