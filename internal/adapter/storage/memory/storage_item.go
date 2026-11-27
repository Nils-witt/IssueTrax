package memory

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that StorageItemRepository satisfies the driven port.
var _ port.StorageItemRepository = (*StorageItemRepository)(nil)

// StorageItemRepository implements port.StorageItemRepository on a Store.
type StorageItemRepository struct{ s *Store }

// NewStorageItemRepository returns a StorageItemRepository backed by s.
func NewStorageItemRepository(s *Store) *StorageItemRepository { return &StorageItemRepository{s: s} }

// Create inserts item, rejecting an asset tag in use.
func (r *StorageItemRepository) Create(_ context.Context, item domain.StorageItem) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	if r.assetTagTaken(item.AssetTag) {
		return domain.ErrAssetTagTaken
	}

	r.s.data.storageItems[item.ID] = cloneStorageItem(item)

	return nil
}

// Update replaces an existing item. The asset tag and creation time stay.
func (r *StorageItemRepository) Update(_ context.Context, item domain.StorageItem) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	current, ok := r.s.data.storageItems[item.ID]
	if !ok {
		return domain.ErrStorageItemNotFound
	}

	item.AssetTag = current.AssetTag
	item.CreatedAt = current.CreatedAt
	r.s.data.storageItems[item.ID] = cloneStorageItem(item)

	return nil
}

// Get returns the item with id, deleted or not.
func (r *StorageItemRepository) Get(_ context.Context, id domain.StorageItemID) (domain.StorageItem, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	item, ok := r.s.data.storageItems[id]
	if !ok {
		return domain.StorageItem{}, domain.ErrStorageItemNotFound
	}

	return cloneStorageItem(item), nil
}

// GetMany returns the known items among ids, deleted ones included.
func (r *StorageItemRepository) GetMany(_ context.Context, ids []domain.StorageItemID) ([]domain.StorageItem, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	items := make([]domain.StorageItem, 0, len(ids))

	for _, id := range ids {
		if item, ok := r.s.data.storageItems[id]; ok {
			items = append(items, cloneStorageItem(item))
		}
	}

	return items, nil
}

// ListInStorage returns one page of the items in storage (or its subtree)
// that match filter, ordered by lower(name), then ID.
func (r *StorageItemRepository) ListInStorage(
	_ context.Context, storage domain.StorageID, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.StorageItem], error) {
	r.s.mu.RLock()

	storages := map[domain.StorageID]bool{storage: true}
	if filter.Recursive {
		storages = r.s.subtree(storage)
	}

	items := r.matching(filter, func(item domain.StorageItem) bool { return storages[item.StorageID] })

	r.s.mu.RUnlock()

	return pageItems(items, page)
}

// Search returns one page of the items across all storages that match
// filter, ordered by lower(name), then ID.
func (r *StorageItemRepository) Search(
	_ context.Context, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.StorageItem], error) {
	r.s.mu.RLock()
	items := r.matching(filter, func(domain.StorageItem) bool { return true })
	r.s.mu.RUnlock()

	return pageItems(items, page)
}

// AssetTagTaken reports whether any item has tag, ignoring case.
func (r *StorageItemRepository) AssetTagTaken(_ context.Context, tag string) (bool, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	return r.assetTagTaken(tag), nil
}

// assetTagTaken reports whether any item has tag. Callers hold r.s.mu.
func (r *StorageItemRepository) assetTagTaken(tag string) bool {
	for _, item := range r.s.data.storageItems {
		if strings.EqualFold(item.AssetTag, tag) {
			return true
		}
	}

	return false
}

// matching returns the items in scope that pass filter. Callers hold r.s.mu.
func (r *StorageItemRepository) matching(filter port.StorageItemFilter, inScope func(domain.StorageItem) bool) []domain.StorageItem {
	var types map[domain.ItemTypeID]bool

	if filter.TypeID != nil {
		types = map[domain.ItemTypeID]bool{*filter.TypeID: true}
		if filter.IncludeSubtypes {
			types = r.s.typeSubtree(*filter.TypeID)
		}
	}

	query := strings.ToLower(filter.Query)
	mac, isMAC := domain.NormalizeMAC(filter.Query)

	var items []domain.StorageItem

	for _, item := range r.s.data.storageItems {
		if inScope(item) && item.IsDeleted() == filter.Deleted && (types == nil || types[item.TypeID]) &&
			matchesFields(item, filter) && r.s.matchesQuery(item, query, mac, isMAC) {
			items = append(items, cloneStorageItem(item))
		}
	}

	return items
}

// matchesFields reports whether item passes the usable, responsible and
// decommissioned filters.
func matchesFields(item domain.StorageItem, filter port.StorageItemFilter) bool {
	switch {
	case filter.Usable != nil && item.Usable != *filter.Usable:
		return false
	case filter.ResponsibleID != nil && (item.ResponsibleID == nil || *item.ResponsibleID != *filter.ResponsibleID):
		return false
	case filter.Decommissioned != nil && (item.DecommissionedAt != nil) != *filter.Decommissioned:
		return false
	}

	return true
}

// matchesQuery reports whether a searchable field of item contains query
// (lower case), or one of its MACs is mac. Callers hold s.mu.
func (s *Store) matchesQuery(item domain.StorageItem, query, mac string, isMAC bool) bool {
	fields := []string{item.Name, item.Description, item.AssetTag}

	for _, m := range item.Modules {
		switch {
		case m.Hardware != nil:
			manufacturer, model := s.hardwareNames(*m.Hardware)
			fields = append(fields, manufacturer, model, m.Hardware.SerialNumber)
		case m.NetworkHost != nil:
			fields = append(fields, m.NetworkHost.Hostname)
		case m.SimCard != nil:
			fields = append(fields, m.SimCard.Provider, m.SimCard.PhoneNumber, m.SimCard.ICCID)
		case m.WWAN != nil:
			fields = append(fields, m.WWAN.IMEI, m.WWAN.APN)
		case m.OperatingSystem != nil:
			fields = append(fields, m.OperatingSystem.Name, m.OperatingSystem.Version)
		case m.NetworkInterface != nil:
			if isMAC && m.NetworkInterface.MAC == mac {
				return true
			}

			fields = append(fields, m.NetworkInterface.MAC)
		}
	}

	return slices.ContainsFunc(fields, func(f string) bool { return strings.Contains(strings.ToLower(f), query) })
}

// subtree returns root and all storages below it. Callers hold s.mu.
func (s *Store) subtree(root domain.StorageID) map[domain.StorageID]bool {
	in := map[domain.StorageID]bool{root: true}

	for grew := true; grew; {
		grew = false

		for _, st := range s.data.storages {
			if st.ParentID != nil && in[*st.ParentID] && !in[st.ID] {
				in[st.ID] = true
				grew = true
			}
		}
	}

	return in
}

func pageItems(all []domain.StorageItem, page domain.PageRequest) (domain.Page[domain.StorageItem], error) {
	return pageByName(all, func(i domain.StorageItem) (string, string) { return i.Name, i.ID.String() }, page)
}

// pageByName sorts values by lower(name), then ID, and cuts out the page
// after page.Cursor.
func pageByName[T any](all []T, key func(T) (name, id string), page domain.PageRequest) (domain.Page[T], error) {
	cursor, hasCursor, err := domain.DecodeCursor[domain.StorageCursor](page.Cursor)
	if err != nil {
		return domain.Page[T]{}, err
	}

	compare := func(v T, name, id string) int {
		n, i := key(v)

		return cmp.Or(cmp.Compare(strings.ToLower(n), name), cmp.Compare(i, id))
	}

	slices.SortFunc(all, func(a, b T) int {
		n, i := key(b)

		return compare(a, strings.ToLower(n), i)
	})

	after := all
	if hasCursor {
		after = slices.DeleteFunc(slices.Clone(all), func(v T) bool { return compare(v, cursor.Name, cursor.ID) <= 0 })
	}

	items, more := paginate(after, page.Limit)
	result := domain.Page[T]{Items: append([]T{}, items...), Total: int64(len(all))}

	if more {
		n, i := key(items[len(items)-1])
		result.NextCursor = domain.EncodeCursor(domain.StorageCursor{Name: strings.ToLower(n), ID: i})
	}

	return result, nil
}

// cloneStorageItem deep-copies item so stored items never alias caller
// memory.
func cloneStorageItem(item domain.StorageItem) domain.StorageItem {
	item.OrderDate = clonePtr(item.OrderDate)
	item.CommissionedAt = clonePtr(item.CommissionedAt)
	item.DecommissionedAt = clonePtr(item.DecommissionedAt)
	item.ResponsibleID = clonePtr(item.ResponsibleID)
	item.DeletedAt = clonePtr(item.DeletedAt)

	modules := make([]domain.Module, 0, len(item.Modules))
	for _, m := range item.Modules {
		modules = append(modules, cloneModule(m))
	}

	item.Modules = modules

	return item
}

func cloneModule(m domain.Module) domain.Module {
	if m.Hardware = clonePtr(m.Hardware); m.Hardware != nil {
		m.Hardware.ManufacturerID, m.Hardware.ModelID = clonePtr(m.Hardware.ManufacturerID), clonePtr(m.Hardware.ModelID)
	}

	m.NetworkHost = clonePtr(m.NetworkHost)
	m.SimCard = clonePtr(m.SimCard)

	if m.WWAN = clonePtr(m.WWAN); m.WWAN != nil {
		m.WWAN.SimCardItemID = clonePtr(m.WWAN.SimCardItemID)
	}

	m.OperatingSystem = clonePtr(m.OperatingSystem)

	if m.Purchase != nil {
		p := *m.Purchase
		p.PriceCents = clonePtr(p.PriceCents)
		p.WarrantyEnd = clonePtr(p.WarrantyEnd)
		m.Purchase = &p
	}

	if m.NetworkInterface != nil {
		n := *m.NetworkInterface
		n.TaggedVLANs = slices.Clone(n.TaggedVLANs)
		n.IPv4.DNSServers = slices.Clone(n.IPv4.DNSServers)
		n.IPv6.DNSServers = slices.Clone(n.IPv6.DNSServers)
		m.NetworkInterface = &n
	}

	return m
}
