package postgres

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"nilswitt.dev/issuetrax/internal/adapter/storage/postgres/sqlc"
	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that StorageItemRepository satisfies the driven port.
var _ port.StorageItemRepository = (*StorageItemRepository)(nil)

// assetTagKey is the unique index on lower-case asset tags.
const assetTagKey = "storage_items_asset_tag_key"

// StorageItemRepository implements port.StorageItemRepository on
// storage_items, storage_item_modules and one table per module kind.
type StorageItemRepository struct{ db *DB }

// NewStorageItemRepository returns a StorageItemRepository on db.
func NewStorageItemRepository(db *DB) *StorageItemRepository { return &StorageItemRepository{db: db} }

// Create inserts item with its modules.
func (r *StorageItemRepository) Create(ctx context.Context, item domain.StorageItem) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		err := q.CreateStorageItem(ctx, sqlc.CreateStorageItemParams{
			ID: uuid.UUID(item.ID), AssetTag: item.AssetTag, StorageID: uuid.UUID(item.StorageID),
			ItemTypeID: uuid.UUID(item.TypeID), Name: item.Name, Description: item.Description, Usable: item.Usable,
			OrderDate: item.OrderDate, CommissionedAt: item.CommissionedAt, DecommissionedAt: item.DecommissionedAt,
			DecommissioningReason: item.DecommissioningReason, ResponsibleID: toPgUserID(item.ResponsibleID),
			CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, DeletedAt: item.DeletedAt,
		})

		switch {
		case isUniqueViolation(err, assetTagKey):
			return domain.ErrAssetTagTaken
		case err != nil:
			return fmt.Errorf("insert storage item: %w", err)
		}

		return storeModules(ctx, q, item)
	})
}

// Update stores every mutable field of item and its modules by ID.
func (r *StorageItemRepository) Update(ctx context.Context, item domain.StorageItem) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		n, err := q.UpdateStorageItem(ctx, sqlc.UpdateStorageItemParams{
			ID: uuid.UUID(item.ID), StorageID: uuid.UUID(item.StorageID), ItemTypeID: uuid.UUID(item.TypeID),
			Name: item.Name, Description: item.Description, Usable: item.Usable, OrderDate: item.OrderDate,
			CommissionedAt: item.CommissionedAt, DecommissionedAt: item.DecommissionedAt,
			DecommissioningReason: item.DecommissioningReason, ResponsibleID: toPgUserID(item.ResponsibleID),
			UpdatedAt: item.UpdatedAt, DeletedAt: item.DeletedAt,
		})

		switch {
		case err != nil:
			return fmt.Errorf("update storage item: %w", err)
		case n == 0:
			return domain.ErrStorageItemNotFound
		}

		keep := make([]uuid.UUID, 0, len(item.Modules))
		for _, m := range item.Modules {
			keep = append(keep, uuid.UUID(m.ID))
		}

		err = q.DeleteStorageItemModulesExcept(ctx, sqlc.DeleteStorageItemModulesExceptParams{ItemID: uuid.UUID(item.ID), KeepIds: keep})
		if err != nil {
			return fmt.Errorf("delete storage item modules: %w", err)
		}

		// Primary flags move between instances, so clear them all before
		// setting the new ones.
		if err := q.ClearStorageItemPrimaries(ctx, uuid.UUID(item.ID)); err != nil {
			return fmt.Errorf("clear primary modules: %w", err)
		}

		return storeModules(ctx, q, item)
	})
}

// storeModules inserts or updates every module of item and its payload.
func storeModules(ctx context.Context, q *sqlc.Queries, item domain.StorageItem) error {
	for _, m := range item.Modules {
		n, err := q.UpsertStorageItemModule(ctx, sqlc.UpsertStorageItemModuleParams{
			ID: uuid.UUID(m.ID), ItemID: uuid.UUID(item.ID), Module: string(m.Kind), IsPrimary: m.Primary,
		})
		if err != nil {
			return fmt.Errorf("upsert module %s: %w", m.ID, err)
		}

		if n == 0 {
			return fmt.Errorf("module %s belongs to another item or kind", m.ID)
		}

		if err := storePayload(ctx, q, m); err != nil {
			return fmt.Errorf("store %s module %s: %w", m.Kind, m.ID, err)
		}
	}

	return nil
}

func storePayload(ctx context.Context, q *sqlc.Queries, m domain.Module) error {
	id := uuid.UUID(m.ID)

	switch {
	case m.Hardware != nil:
		return q.UpsertHardwareModule(ctx, sqlc.UpsertHardwareModuleParams{
			ModuleID: id, ManufacturerID: toPgUUID(m.Hardware.ManufacturerID), ModelID: toPgUUID(m.Hardware.ModelID),
			SerialNumber: m.Hardware.SerialNumber,
		})
	case m.NetworkHost != nil:
		return q.UpsertNetworkHostModule(ctx, sqlc.UpsertNetworkHostModuleParams{
			ModuleID: id, Role: string(m.NetworkHost.Role), Hostname: m.NetworkHost.Hostname,
		})
	case m.Purchase != nil:
		p := m.Purchase

		price := pgtype.Int8{}
		if p.PriceCents != nil {
			price = pgtype.Int8{Int64: *p.PriceCents, Valid: true}
		}

		return q.UpsertPurchaseModule(ctx, sqlc.UpsertPurchaseModuleParams{
			ModuleID: id, Supplier: p.Supplier, InvoiceNumber: p.InvoiceNumber, PriceCents: price, Currency: p.Currency,
			WarrantyEnd: p.WarrantyEnd,
		})
	case m.SimCard != nil:
		return q.UpsertSimCardModule(ctx, sqlc.UpsertSimCardModuleParams{
			ModuleID: id, Provider: m.SimCard.Provider, PhoneNumber: m.SimCard.PhoneNumber, Iccid: m.SimCard.ICCID,
			Imsi: m.SimCard.IMSI,
		})
	case m.WWAN != nil:
		return q.UpsertWwanModule(ctx, sqlc.UpsertWwanModuleParams{
			ModuleID: id, Imei: m.WWAN.IMEI, Apn: m.WWAN.APN, Technology: string(m.WWAN.Technology),
			SimCardItemID: toPgUUID(m.WWAN.SimCardItemID),
		})
	case m.OperatingSystem != nil:
		return q.UpsertOperatingSystemModule(ctx, sqlc.UpsertOperatingSystemModuleParams{
			ModuleID: id, Name: m.OperatingSystem.Name, Version: m.OperatingSystem.Version,
		})
	case m.SIPClient != nil:
		c := m.SIPClient

		return q.UpsertSipClientModule(ctx, sqlc.UpsertSipClientModuleParams{
			ModuleID: id, Server: c.Server, Port: int32(c.Port), Transport: string(c.Transport), //nolint:gosec // domain bounds Port to 0..65535
			Username: c.Username, Extension: c.Extension,
		})
	case m.NetworkInterface != nil:
		return storeInterface(ctx, q, id, *m.NetworkInterface)
	}

	return fmt.Errorf("module %s has no payload", m.ID)
}

func storeInterface(ctx context.Context, q *sqlc.Queries, id uuid.UUID, n domain.NetworkInterface) error {
	mac, err := net.ParseMAC(n.MAC)
	if err != nil {
		return fmt.Errorf("parse mac %q: %w", n.MAC, err)
	}

	return q.UpsertNetworkInterfaceModule(ctx, sqlc.UpsertNetworkInterfaceModuleParams{
		ModuleID:    id,
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
}

// Get returns the item with id, deleted or not.
func (r *StorageItemRepository) Get(ctx context.Context, id domain.StorageItemID) (domain.StorageItem, error) {
	items, err := r.GetMany(ctx, []domain.StorageItemID{id})
	if err != nil {
		return domain.StorageItem{}, err
	}

	if len(items) == 0 {
		return domain.StorageItem{}, domain.ErrStorageItemNotFound
	}

	return items[0], nil
}

// GetMany returns the known items among ids, deleted ones included.
func (r *StorageItemRepository) GetMany(ctx context.Context, ids []domain.StorageItemID) ([]domain.StorageItem, error) {
	return readMany(ctx, r.db, "storage items", func(q *sqlc.Queries) ([]sqlc.StorageItem, error) {
		return q.GetStorageItemsByIDs(ctx, uuids(ids))
	}, withModules)
}

// readMany loads rows and completes them into values in one read
// transaction; what names the rows in errors.
func readMany[R, T any](
	ctx context.Context, db *DB, what string, load func(*sqlc.Queries) ([]R, error),
	complete func(context.Context, *sqlc.Queries, []R) ([]T, error),
) ([]T, error) {
	var values []T

	err := db.readTx(ctx, func(ctx context.Context) error {
		q := db.queries(ctx)

		rows, err := load(q)
		if err != nil {
			return fmt.Errorf("select %s: %w", what, err)
		}

		values, err = complete(ctx, q, rows)

		return err
	})

	return values, err
}

// ListInStorage returns one page of the items in storage (or its subtree)
// matching filter, ordered by lower(name), then ID.
func (r *StorageItemRepository) ListInStorage(
	ctx context.Context, storage domain.StorageID, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.StorageItem], error) {
	return r.list(ctx, &storage, filter, page)
}

// Search returns one page of the items across all storages matching
// filter, ordered by lower(name), then ID.
func (r *StorageItemRepository) Search(
	ctx context.Context, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.StorageItem], error) {
	return r.list(ctx, nil, filter, page)
}

// AssetTagTaken reports whether any item has tag, ignoring case.
func (r *StorageItemRepository) AssetTagTaken(ctx context.Context, tag string) (bool, error) {
	taken, err := r.db.queries(ctx).StorageItemAssetTagTaken(ctx, tag)
	if err != nil {
		return false, fmt.Errorf("check asset tag: %w", err)
	}

	return taken, nil
}

func (r *StorageItemRepository) list(
	ctx context.Context, storage *domain.StorageID, filter port.StorageItemFilter, page domain.PageRequest,
) (domain.Page[domain.StorageItem], error) {
	cursor, err := decodeNameCursor(page)
	if err != nil {
		return domain.Page[domain.StorageItem]{}, err
	}

	params := listParams(storage, filter)
	params.HasCursor, params.CursorName, params.CursorID, params.RowLimit = cursor.has, cursor.name, cursor.id, pageLimit(page.Limit)

	var result domain.Page[domain.StorageItem]

	err = r.db.readTx(ctx, func(ctx context.Context) error {
		q := r.db.queries(ctx)

		rows, err := q.ListStorageItems(ctx, params)
		if err != nil {
			return fmt.Errorf("select storage items: %w", err)
		}

		result.Total, err = q.CountStorageItems(ctx, sqlc.CountStorageItemsParams{
			StorageID: params.StorageID, TypeID: params.TypeID, Deleted: params.Deleted, Usable: params.Usable,
			ResponsibleID: params.ResponsibleID, Decommissioned: params.Decommissioned, Pattern: params.Pattern,
			Mac: params.Mac, Recursive: params.Recursive, IncludeSubtypes: params.IncludeSubtypes,
		})
		if err != nil {
			return fmt.Errorf("count storage items: %w", err)
		}

		more := len(rows) > page.Limit

		if result.Items, err = withModules(ctx, q, rows[:min(len(rows), page.Limit)]); err != nil {
			return err
		}

		if more {
			last := result.Items[len(result.Items)-1]
			result.NextCursor = domain.EncodeCursor(domain.StorageCursor{Name: strings.ToLower(last.Name), ID: last.ID.String()})
		}

		return nil
	})
	if err != nil {
		return domain.Page[domain.StorageItem]{}, err
	}

	return result, nil
}

// listParams turns filter into query parameters without the page.
func listParams(storage *domain.StorageID, filter port.StorageItemFilter) sqlc.ListStorageItemsParams {
	params := sqlc.ListStorageItemsParams{
		Deleted: filter.Deleted, Recursive: filter.Recursive, IncludeSubtypes: filter.IncludeSubtypes,
		Pattern: containsPattern(filter.Query), ResponsibleID: toPgUserID(filter.ResponsibleID),
	}

	if storage != nil {
		params.StorageID = pgtype.UUID{Bytes: *storage, Valid: true}
	}

	if filter.TypeID != nil {
		params.TypeID = pgtype.UUID{Bytes: *filter.TypeID, Valid: true}
	}

	if filter.Usable != nil {
		params.Usable = pgtype.Bool{Bool: *filter.Usable, Valid: true}
	}

	if filter.Decommissioned != nil {
		params.Decommissioned = pgtype.Bool{Bool: *filter.Decommissioned, Valid: true}
	}

	if mac, ok := domain.NormalizeMAC(filter.Query); ok {
		params.Mac, _ = net.ParseMAC(mac)
	}

	return params
}

// withModules converts rows and loads their modules, keeping the order of
// rows.
func withModules(ctx context.Context, q *sqlc.Queries, rows []sqlc.StorageItem) ([]domain.StorageItem, error) {
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}

	modules, err := loadModules(ctx, q, ids)
	if err != nil {
		return nil, err
	}

	items := make([]domain.StorageItem, 0, len(rows))

	for _, row := range rows {
		item := domain.StorageItem{
			ID: domain.StorageItemID(row.ID), AssetTag: row.AssetTag, StorageID: domain.StorageID(row.StorageID),
			TypeID: domain.ItemTypeID(row.ItemTypeID), Name: row.Name, Description: row.Description, Usable: row.Usable,
			OrderDate: utcPtr(row.OrderDate), CommissionedAt: utcPtr(row.CommissionedAt),
			DecommissionedAt: utcPtr(row.DecommissionedAt), DecommissioningReason: row.DecommissioningReason,
			Modules:   append([]domain.Module{}, modules[row.ID]...),
			CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(), DeletedAt: utcPtr(row.DeletedAt),
		}

		if row.ResponsibleID.Valid {
			user := domain.UserID(row.ResponsibleID.Bytes)
			item.ResponsibleID = &user
		}

		domain.SortModules(item.Modules)
		items = append(items, item)
	}

	return items, nil
}

// loadModules returns the modules of the items with ids, by item, loading
// the instances and then each kind's payloads with one query each.
func loadModules(ctx context.Context, q *sqlc.Queries, ids []uuid.UUID) (map[uuid.UUID][]domain.Module, error) {
	rows, err := q.ListStorageItemModules(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("select storage item modules: %w", err)
	}

	moduleIDs := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		moduleIDs = append(moduleIDs, row.ID)
	}

	payloads, err := loadPayloads(ctx, q, moduleIDs)
	if err != nil {
		return nil, err
	}

	out := make(map[uuid.UUID][]domain.Module, len(ids))

	for _, row := range rows {
		m, ok := payloads[row.ID]
		if !ok {
			return nil, fmt.Errorf("module %s has no payload", row.ID)
		}

		m.ID, m.Kind, m.Primary = domain.ModuleID(row.ID), domain.ModuleKind(row.Module), row.IsPrimary
		out[row.ItemID] = append(out[row.ItemID], m)
	}

	return out, nil
}

// loadPayloads returns modules with only their payload set, by module ID.
func loadPayloads(ctx context.Context, q *sqlc.Queries, ids []uuid.UUID) (map[uuid.UUID]domain.Module, error) {
	out := make(map[uuid.UUID]domain.Module, len(ids))

	hardware, err := q.ListHardwareModules(ctx, ids)
	if err := collectPayloads(out, "hardware", hardware, err, func(h sqlc.ModuleHardware) (uuid.UUID, domain.Module) {
		return h.ModuleID, domain.Module{Hardware: &domain.Hardware{
			ManufacturerID: fromPgUUID[domain.ManufacturerID](h.ManufacturerID),
			ModelID:        fromPgUUID[domain.HardwareModelID](h.ModelID),
			SerialNumber:   h.SerialNumber,
		}}
	}); err != nil {
		return nil, err
	}

	hosts, err := q.ListNetworkHostModules(ctx, ids)
	if err := collectPayloads(out, "network host", hosts, err, func(h sqlc.ModuleNetworkHost) (uuid.UUID, domain.Module) {
		return h.ModuleID, domain.Module{NetworkHost: &domain.NetworkHost{Role: domain.NetworkRole(h.Role), Hostname: h.Hostname}}
	}); err != nil {
		return nil, err
	}

	purchases, err := q.ListPurchaseModules(ctx, ids)
	if err := collectPayloads(out, "purchase", purchases, err, func(p sqlc.ModulePurchase) (uuid.UUID, domain.Module) {
		purchase := domain.Purchase{
			Supplier: p.Supplier, InvoiceNumber: p.InvoiceNumber, Currency: p.Currency, WarrantyEnd: utcPtr(p.WarrantyEnd),
		}

		if p.PriceCents.Valid {
			purchase.PriceCents = &p.PriceCents.Int64
		}

		return p.ModuleID, domain.Module{Purchase: &purchase}
	}); err != nil {
		return nil, err
	}

	sims, err := q.ListSimCardModules(ctx, ids)
	if err := collectPayloads(out, "sim card", sims, err, func(c sqlc.ModuleSimCard) (uuid.UUID, domain.Module) {
		return c.ModuleID, domain.Module{SimCard: &domain.SimCard{
			Provider: c.Provider, PhoneNumber: c.PhoneNumber, ICCID: c.Iccid, IMSI: c.Imsi,
		}}
	}); err != nil {
		return nil, err
	}

	wwans, err := q.ListWwanModules(ctx, ids)
	if err := collectPayloads(out, "wwan", wwans, err, func(w sqlc.ModuleWwan) (uuid.UUID, domain.Module) {
		return w.ModuleID, domain.Module{WWAN: &domain.WWAN{
			IMEI: w.Imei, APN: w.Apn, Technology: domain.WWANTechnology(w.Technology),
			SimCardItemID: fromPgUUID[domain.StorageItemID](w.SimCardItemID),
		}}
	}); err != nil {
		return nil, err
	}

	systems, err := q.ListOperatingSystemModules(ctx, ids)
	if err := collectPayloads(out, "operating system", systems, err, func(o sqlc.ModuleOperatingSystem) (uuid.UUID, domain.Module) {
		return o.ModuleID, domain.Module{OperatingSystem: &domain.OperatingSystem{Name: o.Name, Version: o.Version}}
	}); err != nil {
		return nil, err
	}

	sips, err := q.ListSipClientModules(ctx, ids)
	if err := collectPayloads(out, "sip client", sips, err, func(c sqlc.ModuleSipClient) (uuid.UUID, domain.Module) {
		return c.ModuleID, domain.Module{SIPClient: &domain.SIPClient{
			Server: c.Server, Port: int(c.Port), Transport: domain.SIPTransport(c.Transport), Username: c.Username,
			Extension: c.Extension,
		}}
	}); err != nil {
		return nil, err
	}

	ifaces, err := q.ListNetworkInterfaceModules(ctx, ids)
	if err := collectPayloads(out, "network interface", ifaces, err, func(n sqlc.ModuleNetworkInterface) (uuid.UUID, domain.Module) {
		return n.ModuleID, domain.Module{NetworkInterface: new(toNetworkInterface(n))}
	}); err != nil {
		return nil, err
	}

	return out, nil
}

// collectPayloads adds the payload of each row to out, or wraps the error of
// the query that selected the rows.
func collectPayloads[R any](
	out map[uuid.UUID]domain.Module, what string, rows []R, err error, toModule func(R) (uuid.UUID, domain.Module),
) error {
	if err != nil {
		return fmt.Errorf("select %s modules: %w", what, err)
	}

	for _, row := range rows {
		id, m := toModule(row)
		out[id] = m
	}

	return nil
}

func toNetworkInterface(row sqlc.ModuleNetworkInterface) domain.NetworkInterface {
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

func toPgUserID(id *domain.UserID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}

	return pgtype.UUID{Bytes: *id, Valid: true}
}
