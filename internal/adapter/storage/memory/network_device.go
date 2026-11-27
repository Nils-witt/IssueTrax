package memory

import (
	"context"
	"slices"
	"strings"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that NetworkDeviceRepository satisfies the driven port.
var _ port.NetworkDeviceRepository = (*NetworkDeviceRepository)(nil)

// NetworkDeviceRepository implements port.NetworkDeviceRepository on a Store.
type NetworkDeviceRepository struct{ s *Store }

// NewNetworkDeviceRepository returns a NetworkDeviceRepository backed by s.
func NewNetworkDeviceRepository(s *Store) *NetworkDeviceRepository {
	return &NetworkDeviceRepository{s: s}
}

// Create inserts device.
func (r *NetworkDeviceRepository) Create(_ context.Context, device domain.NetworkDevice) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	r.put(device)

	return nil
}

// Update replaces an existing network device.
func (r *NetworkDeviceRepository) Update(_ context.Context, device domain.NetworkDevice) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	current, ok := r.s.data.storageItems[device.ID]
	if !ok || current.Kind != domain.StorageItemKindNetworkDevice {
		return domain.ErrStorageItemNotFound
	}

	device.CreatedAt = current.CreatedAt
	r.put(device)

	return nil
}

// put stores a copy of device. Callers hold r.s.mu for writing.
func (r *NetworkDeviceRepository) put(device domain.NetworkDevice) {
	device.Kind = domain.StorageItemKindNetworkDevice
	device = cloneNetworkDevice(device)
	r.s.data.storageItems[device.ID] = device.StorageItem
	r.s.data.networkDevices[device.ID] = device
}

// Get returns the network device with id, deleted or not.
func (r *NetworkDeviceRepository) Get(_ context.Context, id domain.StorageItemID) (domain.NetworkDevice, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	device, ok := r.get(id)
	if !ok {
		return domain.NetworkDevice{}, domain.ErrStorageItemNotFound
	}

	return device, nil
}

// get returns a copy of the device with id. Callers hold r.s.mu.
func (r *NetworkDeviceRepository) get(id domain.StorageItemID) (domain.NetworkDevice, bool) {
	item, ok := r.s.data.storageItems[id]
	if !ok || item.Kind != domain.StorageItemKindNetworkDevice {
		return domain.NetworkDevice{}, false
	}

	device := cloneNetworkDevice(r.s.data.networkDevices[id])
	device.StorageItem = cloneStorageItem(item)

	return device, true
}

// ListInStorage returns one page of the network devices in storage (or its
// subtree), ordered by lower(name), then ID.
func (r *NetworkDeviceRepository) ListInStorage(
	_ context.Context, storage domain.StorageID, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.NetworkDevice], error) {
	r.s.mu.RLock()

	storages := map[domain.StorageID]bool{storage: true}
	if filter.Recursive {
		storages = r.s.subtree(storage)
	}

	devices := r.matching(func(d domain.NetworkDevice) bool {
		return storages[d.StorageID] && d.IsDeleted() == filter.Deleted
	})

	r.s.mu.RUnlock()

	return pageNetworkDevices(devices, page)
}

// Search returns one page of the network devices whose name, description,
// hostname, serial number or any interface MAC contains filter.Query,
// ignoring case, ordered by lower(name), then ID.
func (r *NetworkDeviceRepository) Search(
	_ context.Context, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.NetworkDevice], error) {
	query := strings.ToLower(filter.Query)
	contains := func(s string) bool { return strings.Contains(strings.ToLower(s), query) }

	r.s.mu.RLock()

	devices := r.matching(func(d domain.NetworkDevice) bool {
		if d.IsDeleted() != filter.Deleted {
			return false
		}

		if contains(d.Name) || contains(d.Description) || contains(d.Hostname) || contains(d.SerialNumber) ||
			contains(d.Primary.MAC) {
			return true
		}

		return slices.ContainsFunc(d.Secondary, func(n domain.NetworkInterface) bool { return contains(n.MAC) })
	})

	r.s.mu.RUnlock()

	return pageNetworkDevices(devices, page)
}

// matching returns the network devices keep accepts. Callers hold r.s.mu.
func (r *NetworkDeviceRepository) matching(keep func(domain.NetworkDevice) bool) []domain.NetworkDevice {
	var devices []domain.NetworkDevice

	for id := range r.s.data.networkDevices {
		if d, ok := r.get(id); ok && keep(d) {
			devices = append(devices, d)
		}
	}

	return devices
}

func pageNetworkDevices(all []domain.NetworkDevice, page domain.PageRequest) (domain.Page[domain.NetworkDevice], error) {
	return pageItems(all, func(d domain.NetworkDevice) domain.StorageItem { return d.StorageItem }, page)
}

// cloneNetworkDevice deep-copies d so stored devices never alias caller
// memory.
func cloneNetworkDevice(d domain.NetworkDevice) domain.NetworkDevice {
	d.StorageItem = cloneStorageItem(d.StorageItem)
	d.Primary = cloneNetworkInterface(d.Primary)

	secondary := make([]domain.NetworkInterface, 0, len(d.Secondary))
	for _, n := range d.Secondary {
		secondary = append(secondary, cloneNetworkInterface(n))
	}

	d.Secondary = secondary

	return d
}

func cloneNetworkInterface(n domain.NetworkInterface) domain.NetworkInterface {
	n.TaggedVLANs = slices.Clone(n.TaggedVLANs)
	n.IPv4.DNSServers = slices.Clone(n.IPv4.DNSServers)
	n.IPv6.DNSServers = slices.Clone(n.IPv6.DNSServers)

	return n
}
