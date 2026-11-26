package memory

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that StorageRepository satisfies the driven port.
var _ port.StorageRepository = (*StorageRepository)(nil)

// StorageRepository implements port.StorageRepository on a Store.
type StorageRepository struct{ s *Store }

// NewStorageRepository returns a StorageRepository backed by s.
func NewStorageRepository(s *Store) *StorageRepository { return &StorageRepository{s: s} }

// Create inserts storage, rejecting a sibling with the same name.
func (r *StorageRepository) Create(_ context.Context, storage domain.Storage) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	if r.siblingNameTaken(storage) {
		return domain.ErrStorageNameTaken
	}

	r.s.data.storages[storage.ID] = cloneStorage(storage)

	return nil
}

// Update replaces an existing storage, rejecting a sibling with the same name.
func (r *StorageRepository) Update(_ context.Context, storage domain.Storage) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	if _, ok := r.s.data.storages[storage.ID]; !ok {
		return domain.ErrStorageNotFound
	}

	if r.siblingNameTaken(storage) {
		return domain.ErrStorageNameTaken
	}

	r.s.data.storages[storage.ID] = cloneStorage(storage)

	return nil
}

// Get returns the storage with id and its child count.
func (r *StorageRepository) Get(_ context.Context, id domain.StorageID) (domain.StorageListItem, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	st, ok := r.s.data.storages[id]
	if !ok {
		return domain.StorageListItem{}, domain.ErrStorageNotFound
	}

	return r.item(st), nil
}

// ListChildren returns one page of parent's children ordered by
// lower(name), then ID.
func (r *StorageRepository) ListChildren(
	_ context.Context, parent *domain.StorageID, page domain.PageRequest,
) (domain.Page[domain.StorageListItem], error) {
	cursor, hasCursor, err := domain.DecodeCursor[domain.StorageCursor](page.Cursor)
	if err != nil {
		return domain.Page[domain.StorageListItem]{}, err
	}

	r.s.mu.RLock()

	var all []domain.StorageListItem

	for _, st := range r.s.data.storages {
		if sameParentID(st.ParentID, parent) {
			all = append(all, r.item(st))
		}
	}

	r.s.mu.RUnlock()

	slices.SortFunc(all, func(a, b domain.StorageListItem) int {
		return cmp.Or(
			cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)),
			cmp.Compare(a.ID.String(), b.ID.String()),
		)
	})

	after := all
	if hasCursor {
		after = slices.DeleteFunc(slices.Clone(all), func(s domain.StorageListItem) bool {
			return cmp.Or(
				cmp.Compare(strings.ToLower(s.Name), cursor.Name),
				cmp.Compare(s.ID.String(), cursor.ID),
			) <= 0
		})
	}

	items, more := paginate(after, page.Limit)
	result := domain.Page[domain.StorageListItem]{Items: append([]domain.StorageListItem{}, items...), Total: int64(len(all))}

	if more {
		last := items[len(items)-1]
		result.NextCursor = domain.EncodeCursor(domain.StorageCursor{Name: strings.ToLower(last.Name), ID: last.ID.String()})
	}

	return result, nil
}

// IsDescendant walks candidate's ancestors looking for ancestor.
func (r *StorageRepository) IsDescendant(_ context.Context, ancestor, candidate domain.StorageID) (bool, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	seen := map[domain.StorageID]bool{}

	for st, ok := r.s.data.storages[candidate]; ok && st.ParentID != nil && !seen[st.ID]; st, ok = r.s.data.storages[*st.ParentID] {
		seen[st.ID] = true

		if *st.ParentID == ancestor {
			return true, nil
		}
	}

	return false, nil
}

// LockTree is a no-op: Store already serializes transactions.
func (r *StorageRepository) LockTree(context.Context) error { return nil }

// siblingNameTaken reports whether another storage with the same parent has
// storage's name. Callers hold r.s.mu.
func (r *StorageRepository) siblingNameTaken(storage domain.Storage) bool {
	for _, st := range r.s.data.storages {
		if st.ID != storage.ID && sameParentID(st.ParentID, storage.ParentID) && sameName(st.Name, storage.Name) {
			return true
		}
	}

	return false
}

// item adds the child and item counts to st. Callers hold r.s.mu.
func (r *StorageRepository) item(st domain.Storage) domain.StorageListItem {
	var n int64

	for _, c := range r.s.data.storages {
		if c.ParentID != nil && *c.ParentID == st.ID {
			n++
		}
	}

	var items int64

	for _, item := range r.s.data.storageItems {
		if item.StorageID == st.ID && !item.IsDeleted() {
			items++
		}
	}

	return domain.StorageListItem{Storage: cloneStorage(st), ChildCount: n, ItemCount: items}
}

// cloneStorage copies the parent pointer so stored storages never alias
// caller memory.
func cloneStorage(st domain.Storage) domain.Storage {
	if st.ParentID != nil {
		p := *st.ParentID
		st.ParentID = &p
	}

	return st
}

func sameParentID(a, b *domain.StorageID) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}
