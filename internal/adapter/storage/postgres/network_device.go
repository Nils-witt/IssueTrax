package postgres

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nilswitt.dev/issuetrax/internal/adapter/storage/postgres/sqlc"
	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that NetworkDeviceRepository satisfies the driven port.
var _ port.NetworkDeviceRepository = (*NetworkDeviceRepository)(nil)

// networkDeviceKind is the kind column value of network devices.
const networkDeviceKind = string(domain.StorageItemKindNetworkDevice)

// NetworkDeviceRepository implements port.NetworkDeviceRepository on the
// shared storage_items table plus network_devices and network_interfaces.
type NetworkDeviceRepository struct{ db *DB }

// NewNetworkDeviceRepository returns a NetworkDeviceRepository on db.
func NewNetworkDeviceRepository(db *DB) *NetworkDeviceRepository {
	return &NetworkDeviceRepository{db: db}
}

// Create inserts device with its interfaces.
func (r *NetworkDeviceRepository) Create(ctx context.Context, device domain.NetworkDevice) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		err := q.CreateStorageItem(ctx, sqlc.CreateStorageItemParams{
			ID:          uuid.UUID(device.ID),
			StorageID:   uuid.UUID(device.StorageID),
			Kind:        networkDeviceKind,
			Name:        device.Name,
			Description: device.Description,
			CreatedAt:   device.CreatedAt,
			UpdatedAt:   device.UpdatedAt,
			DeletedAt:   device.DeletedAt,
		})
		if err != nil {
			return fmt.Errorf("insert storage item: %w", err)
		}

		err = q.CreateNetworkDevice(ctx, sqlc.CreateNetworkDeviceParams{
			ItemID:       uuid.UUID(device.ID),
			Role:         string(device.Role),
			Hostname:     device.Hostname,
			Manufacturer: device.Manufacturer,
			Model:        device.Model,
			SerialNumber: device.SerialNumber,
		})
		if err != nil {
			return fmt.Errorf("insert network device: %w", err)
		}

		return insertInterfaces(ctx, q, device)
	})
}

// Update stores every mutable field of device and replaces its interfaces.
func (r *NetworkDeviceRepository) Update(ctx context.Context, device domain.NetworkDevice) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		n, err := q.UpdateStorageItem(ctx, sqlc.UpdateStorageItemParams{
			ID:          uuid.UUID(device.ID),
			Kind:        networkDeviceKind,
			StorageID:   uuid.UUID(device.StorageID),
			Name:        device.Name,
			Description: device.Description,
			UpdatedAt:   device.UpdatedAt,
			DeletedAt:   device.DeletedAt,
		})

		switch {
		case err != nil:
			return fmt.Errorf("update storage item: %w", err)
		case n == 0:
			return domain.ErrStorageItemNotFound
		}

		err = q.UpdateNetworkDevice(ctx, sqlc.UpdateNetworkDeviceParams{
			ItemID:       uuid.UUID(device.ID),
			Role:         string(device.Role),
			Hostname:     device.Hostname,
			Manufacturer: device.Manufacturer,
			Model:        device.Model,
			SerialNumber: device.SerialNumber,
		})
		if err != nil {
			return fmt.Errorf("update network device: %w", err)
		}

		if err := q.DeleteNetworkInterfaces(ctx, uuid.UUID(device.ID)); err != nil {
			return fmt.Errorf("delete network interfaces: %w", err)
		}

		return insertInterfaces(ctx, q, device)
	})
}

// insertInterfaces stores the primary interface at position 0 and the
// secondary ones after it.
func insertInterfaces(ctx context.Context, q *sqlc.Queries, device domain.NetworkDevice) error {
	all := append([]domain.NetworkInterface{device.Primary}, device.Secondary...)

	for i, n := range all {
		mac, err := net.ParseMAC(n.MAC)
		if err != nil {
			return fmt.Errorf("parse mac %q: %w", n.MAC, err)
		}

		err = q.CreateNetworkInterface(ctx, sqlc.CreateNetworkInterfaceParams{
			ItemID:      uuid.UUID(device.ID),
			Position:    int32(i),
			Name:        n.Name,
			Mac:         mac,
			Type:        string(n.Type),
			Ssid:        n.SSID,
			NativeVlan:  nativeVLANParam(n.NativeVLAN),
			TaggedVlans: vlanParams(n.TaggedVLANs),
			Ipv4Mode:    string(n.IPv4.Mode),
			Ipv4Address: prefixParam(n.IPv4.Address),
			Ipv4Gateway: addrParam(n.IPv4.Gateway),
			Ipv4Dns:     dnsParam(n.IPv4.DNSServers),
			Ipv6Mode:    string(n.IPv6.Mode),
			Ipv6Address: prefixParam(n.IPv6.Address),
			Ipv6Gateway: addrParam(n.IPv6.Gateway),
			Ipv6Dns:     dnsParam(n.IPv6.DNSServers),
		})
		if err != nil {
			return fmt.Errorf("insert network interface %d: %w", i, err)
		}
	}

	return nil
}

// Get returns the network device with id, deleted or not.
func (r *NetworkDeviceRepository) Get(ctx context.Context, id domain.StorageItemID) (domain.NetworkDevice, error) {
	var device domain.NetworkDevice

	err := r.db.readTx(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		row, err := q.GetStorageItem(ctx, sqlc.GetStorageItemParams{ID: uuid.UUID(id), Kind: networkDeviceKind})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrStorageItemNotFound
		}

		if err != nil {
			return fmt.Errorf("select storage item: %w", err)
		}

		devices, err := r.complete(ctx, q, []sqlc.StorageItem{row})
		if err != nil {
			return err
		}

		device = devices[0]

		return nil
	})
	if err != nil {
		return domain.NetworkDevice{}, err
	}

	return device, nil
}

// ListInStorage returns one page of the network devices in storage (or its
// subtree), ordered by lower(name), then ID.
func (r *NetworkDeviceRepository) ListInStorage(
	ctx context.Context, storage domain.StorageID, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.NetworkDevice], error) {
	load, err := itemsInStorage(ctx, networkDeviceKind, storage, filter, page)
	if err != nil {
		return domain.Page[domain.NetworkDevice]{}, err
	}

	return r.page(ctx, page, load)
}

// Search returns one page of the network devices whose name, description,
// hostname, serial number or any interface MAC contains filter.Query,
// ignoring case, ordered by lower(name), then ID.
func (r *NetworkDeviceRepository) Search(
	ctx context.Context, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.NetworkDevice], error) {
	cursor, err := decodeStorageItemCursor(page)
	if err != nil {
		return domain.Page[domain.NetworkDevice]{}, err
	}

	params := sqlc.SearchNetworkDevicesParams{
		Deleted: filter.Deleted, Pattern: containsPattern(filter.Query),
		HasCursor: cursor.has, CursorName: cursor.name, CursorID: cursor.id, RowLimit: pageLimit(page.Limit),
	}

	return r.page(ctx, page, func(q *sqlc.Queries) ([]sqlc.StorageItem, int64, error) {
		rows, err := q.SearchNetworkDevices(ctx, params)
		if err != nil {
			return nil, 0, fmt.Errorf("search network devices: %w", err)
		}

		total, err := q.CountSearchNetworkDevices(ctx, sqlc.CountSearchNetworkDevicesParams{
			Deleted: params.Deleted, Pattern: params.Pattern,
		})
		if err != nil {
			return nil, 0, fmt.Errorf("count network devices: %w", err)
		}

		return rows, total, nil
	})
}

// page runs load in one read transaction, completes its rows (up to one
// more than the limit) with their device fields and interfaces, and turns
// them into a page.
func (r *NetworkDeviceRepository) page(
	ctx context.Context, page domain.PageRequest, load itemLoader,
) (domain.Page[domain.NetworkDevice], error) {
	var result domain.Page[domain.NetworkDevice]

	err := r.db.readTx(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		rows, total, err := load(q)
		if err != nil {
			return err
		}

		result.Total = total
		more := len(rows) > page.Limit
		rows = rows[:min(len(rows), page.Limit)]

		if result.Items, err = r.complete(ctx, q, rows); err != nil {
			return err
		}

		if more {
			last := result.Items[len(result.Items)-1]
			result.NextCursor = domain.EncodeCursor(domain.StorageCursor{Name: strings.ToLower(last.Name), ID: last.ID.String()})
		}

		return nil
	})
	if err != nil {
		return domain.Page[domain.NetworkDevice]{}, err
	}

	return result, nil
}

// complete loads the device fields and interfaces of rows, in two queries,
// and returns the devices in the order of rows.
func (r *NetworkDeviceRepository) complete(
	ctx context.Context, q *sqlc.Queries, rows []sqlc.StorageItem,
) ([]domain.NetworkDevice, error) {
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}

	deviceRows, err := q.ListNetworkDevices(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("select network devices: %w", err)
	}

	ifaceRows, err := q.ListNetworkInterfaces(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("select network interfaces: %w", err)
	}

	byID := make(map[uuid.UUID]sqlc.NetworkDevice, len(deviceRows))
	for _, d := range deviceRows {
		byID[d.ItemID] = d
	}

	// Rows come primary first, so the first interface of a device is its
	// primary one.
	ifaces := make(map[uuid.UUID][]domain.NetworkInterface, len(rows))
	for _, n := range ifaceRows {
		ifaces[n.ItemID] = append(ifaces[n.ItemID], toNetworkInterface(n))
	}

	devices := make([]domain.NetworkDevice, 0, len(rows))

	for _, row := range rows {
		d, ok := byID[row.ID]
		all := ifaces[row.ID]

		if !ok || len(all) == 0 {
			return nil, fmt.Errorf("network device %s is incomplete", row.ID)
		}

		devices = append(devices, domain.NetworkDevice{
			StorageItem:  toGenericStorageItem(row).StorageItem,
			Role:         domain.NetworkRole(d.Role),
			Hostname:     d.Hostname,
			Manufacturer: d.Manufacturer,
			Model:        d.Model,
			SerialNumber: d.SerialNumber,
			Primary:      all[0],
			Secondary:    append([]domain.NetworkInterface{}, all[1:]...),
		})
	}

	return devices, nil
}

func toNetworkInterface(row sqlc.NetworkInterface) domain.NetworkInterface {
	n := domain.NetworkInterface{
		Name:        row.Name,
		MAC:         row.Mac.String(),
		Type:        domain.NetworkInterfaceType(row.Type),
		SSID:        row.Ssid,
		TaggedVLANs: make([]int, 0, len(row.TaggedVlans)),
		IPv4:        toIPConfig(row.Ipv4Mode, row.Ipv4Address, row.Ipv4Gateway, row.Ipv4Dns),
		IPv6:        toIPConfig(row.Ipv6Mode, row.Ipv6Address, row.Ipv6Gateway, row.Ipv6Dns),
	}

	if row.NativeVlan != nil {
		n.NativeVLAN = int(*row.NativeVlan)
	}

	for _, v := range row.TaggedVlans {
		n.TaggedVLANs = append(n.TaggedVLANs, int(v))
	}

	return n
}

func toIPConfig(mode string, address *netip.Prefix, gateway *netip.Addr, dns []netip.Addr) domain.IPConfig {
	c := domain.IPConfig{Mode: domain.IPMode(mode), DNSServers: make([]netip.Addr, 0, len(dns))}

	if address != nil {
		c.Address = *address
	}

	if gateway != nil {
		c.Gateway = *gateway
	}

	c.DNSServers = append(c.DNSServers, dns...)

	return c
}

func nativeVLANParam(v int) *int32 {
	if v == 0 {
		return nil
	}

	v32 := int32(v) //nolint:gosec // VLAN IDs are validated to 1–4094

	return &v32
}

func vlanParams(vlans []int) []int32 {
	out := make([]int32, 0, len(vlans))
	for _, v := range vlans {
		out = append(out, int32(v)) //nolint:gosec // VLAN IDs are validated to 1–4094
	}

	return out
}

func prefixParam(p netip.Prefix) *netip.Prefix {
	if !p.IsValid() {
		return nil
	}

	return &p
}

func addrParam(a netip.Addr) *netip.Addr {
	if !a.IsValid() {
		return nil
	}

	return &a
}

func dnsParam(dns []netip.Addr) []netip.Addr {
	if dns == nil {
		return []netip.Addr{}
	}

	return dns
}
