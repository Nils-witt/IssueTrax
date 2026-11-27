package storagetest

import (
	"net/netip"
	"slices"
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
	"nilswitt.dev/issuetrax/internal/core/service"
)

// Item names several tests use.
const (
	drill      = "Drill"
	bits       = "bits"
	routerName = "Router"
	chairName  = "Chair"
	coreSwitch = "Core switch"
)

// itemType stores a root item type called name.
func itemType(t *testing.T, r service.Repositories, name string) domain.ItemType {
	t.Helper()

	typ := domain.ItemType{ID: domain.ItemTypeID(newID()), Name: name, Defaults: []domain.ModuleDefault{}, CreatedAt: now, UpdatedAt: now}
	must(t, r.ItemTypes.Create(t.Context(), typ))

	return typ
}

// storageItem stores a usable item of typ called name in st; its asset tag
// is the name.
func storageItem(t *testing.T, r service.Repositories, st domain.Storage, typ domain.ItemType, name string) domain.StorageItem {
	t.Helper()

	item := domain.StorageItem{
		ID: domain.StorageItemID(newID()), AssetTag: name, StorageID: st.ID, TypeID: typ.ID, Name: name, Usable: true,
		Modules: []domain.Module{}, CreatedAt: now, UpdatedAt: now,
	}
	must(t, r.StorageItems.Create(t.Context(), item))

	return item
}

func pageOf(t *testing.T, limit int, cursor string) domain.PageRequest {
	t.Helper()

	p, err := domain.NewPageRequest(limit, cursor)
	must(t, err)

	return p
}

func equalStorageItems(a, b domain.StorageItem) bool {
	sameDeleted := (a.DeletedAt == nil) == (b.DeletedAt == nil) && (a.DeletedAt == nil || a.DeletedAt.Equal(*b.DeletedAt))

	return a.ID == b.ID && a.AssetTag == b.AssetTag && a.SameContent(b) &&
		a.CreatedAt.Equal(b.CreatedAt) && a.UpdatedAt.Equal(b.UpdatedAt) && sameDeleted
}

func itemNames(items []domain.StorageItem) []string {
	out := make([]string, 0, len(items))
	for _, i := range items {
		out = append(out, i.Name)
	}

	return out
}

func moduleID() domain.ModuleID { return domain.ModuleID(newID()) }

// fullModules returns one module of every kind plus a second interface, in
// display order. The hardware is an "ACME S24", stored in r's catalog.
func fullModules(t *testing.T, r service.Repositories) []domain.Module {
	t.Helper()

	warranty := now.Add(24 * time.Hour)
	price := int64(129900)
	acme := manufacturer(t, r, "ACME")
	s24 := hardwareModel(t, r, acme, "S24")

	modules := []domain.Module{
		{ID: moduleID(), Kind: domain.ModuleHardware, Hardware: &domain.Hardware{
			ManufacturerID: &acme.ID, ModelID: &s24.ID, SerialNumber: "SN-42",
		}},
		{ID: moduleID(), Kind: domain.ModuleNetworkHost, NetworkHost: &domain.NetworkHost{Role: domain.NetworkRoleSwitch, Hostname: "sw1.lan"}},
		{ID: moduleID(), Kind: domain.ModulePurchase, Purchase: &domain.Purchase{
			Supplier: "Shop", InvoiceNumber: "R-1", PriceCents: &price, Currency: "EUR", WarrantyEnd: &warranty,
		}},
		{ID: moduleID(), Kind: domain.ModuleSimCard, SimCard: &domain.SimCard{
			Provider: "Telekom", PhoneNumber: "+491711234567", ICCID: "89490200001234567890",
		}},
		{ID: moduleID(), Kind: domain.ModuleWWAN, WWAN: &domain.WWAN{
			IMEI: "490154203237518", APN: "internet.telekom", Technology: domain.WWAN5G,
		}},
		{ID: moduleID(), Kind: domain.ModuleOperatingSystem, OperatingSystem: &domain.OperatingSystem{
			Name: "Cisco IOS XE", Version: "17.9.4a",
		}},
		{ID: moduleID(), Kind: domain.ModuleNetworkInterface, Primary: true, NetworkInterface: &domain.NetworkInterface{
			Name: "eth0", MAC: "aa:bb:cc:00:11:22", Type: domain.NetworkInterfaceWired, NativeVLAN: 10, TaggedVLANs: []int{20, 30},
			IPv4: domain.IPConfig{
				Mode: domain.IPModeStatic, Address: netip.MustParsePrefix("10.0.0.2/24"), Gateway: netip.MustParseAddr("10.0.0.1"),
				DNSServers: []netip.Addr{netip.MustParseAddr("10.0.0.1")},
			},
			IPv6: domain.IPConfig{Mode: domain.IPModeSLAAC, DNSServers: []netip.Addr{}},
		}},
		{ID: moduleID(), Kind: domain.ModuleNetworkInterface, NetworkInterface: &domain.NetworkInterface{
			Name: "wlan0", MAC: "02:00:00:00:00:01", Type: domain.NetworkInterfaceWireless, SSID: "Office", TaggedVLANs: []int{},
			IPv4: domain.IPConfig{Mode: domain.IPModeDHCP, DNSServers: []netip.Addr{}},
			IPv6: domain.IPConfig{Mode: domain.IPModeOff, DNSServers: []netip.Addr{}},
		}},
	}
	domain.SortModules(modules)

	return modules
}

func testStorageItemsCreateGetAndUpdate(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	st := storage(t, r, nil, "Shelf")
	other := storage(t, r, nil, "Box")
	sw := itemType(t, r, switchName)
	router := itemType(t, r, routerName)
	ada := user(t, r, "ada")
	ordered := now.Add(-48 * time.Hour)

	item := domain.StorageItem{
		ID: domain.StorageItemID(newID()), AssetTag: "NET-00001", StorageID: st.ID, TypeID: sw.ID, Name: coreSwitch,
		Description: "rack 1", Usable: true, OrderDate: &ordered, CommissionedAt: &now, ResponsibleID: &ada.ID,
		Modules: fullModules(t, r), CreatedAt: now, UpdatedAt: now,
	}
	must(t, r.StorageItems.Create(t.Context(), item))

	got, err := r.StorageItems.Get(t.Context(), item.ID)
	must(t, err)

	if !equalStorageItems(got, item) {
		t.Fatalf("Get() = %+v\nwant %+v", got, item)
	}

	later := now.Add(time.Hour)
	item.StorageID, item.TypeID, item.Name, item.Description, item.Usable = other.ID, router.ID, "Old switch", "", false
	item.OrderDate, item.DecommissionedAt, item.DecommissioningReason, item.ResponsibleID = nil, &later, "broken", nil
	item.Modules = item.Modules[:1]
	item.UpdatedAt, item.DeletedAt = later, &later
	must(t, r.StorageItems.Update(t.Context(), item))

	got, err = r.StorageItems.Get(t.Context(), item.ID)
	must(t, err)

	if !equalStorageItems(got, item) {
		t.Fatalf("after Update() = %+v\nwant %+v", got, item)
	}

	_, err = r.StorageItems.Get(t.Context(), domain.StorageItemID(newID()))
	wantErr(t, err, domain.ErrStorageItemNotFound)

	unknown := item
	unknown.ID = domain.StorageItemID(newID())
	wantErr(t, r.StorageItems.Update(t.Context(), unknown), domain.ErrStorageItemNotFound)
}

func testStorageItemsModuleIDs(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	st := storage(t, r, nil, "Shelf")
	typ := itemType(t, r, switchName)

	item := storageItem(t, r, st, typ, "sw1")
	item.Modules = fullModules(t, r)
	must(t, r.StorageItems.Update(t.Context(), item))

	// Move the primary flag, edit a payload, drop a module and add one.
	sim, wwan, opsys := item.Modules[3], item.Modules[4], item.Modules[5]
	eth0, wlan0 := item.Modules[6], item.Modules[7]
	eth0.Primary, wlan0.Primary = false, true
	wwan.WWAN = &domain.WWAN{IMEI: wwan.WWAN.IMEI, APN: "iot.1nce.net", Technology: domain.WWAN4G}
	wlan0.NetworkInterface = new(*wlan0.NetworkInterface)
	wlan0.NetworkInterface.SSID = "Guests"
	added := domain.Module{ID: moduleID(), Kind: domain.ModuleHardware, Hardware: &domain.Hardware{SerialNumber: "S48"}}

	item.Modules = []domain.Module{added, item.Modules[1], sim, wwan, opsys, eth0, wlan0}
	domain.SortModules(item.Modules)
	must(t, r.StorageItems.Update(t.Context(), item))

	got, err := r.StorageItems.Get(t.Context(), item.ID)
	must(t, err)

	if !equalStorageItems(got, item) {
		t.Fatalf("after module update = %+v\nwant %+v", got.Modules, item.Modules)
	}

	if got.Modules[5].ID != wlan0.ID || !got.Modules[5].Primary || got.Modules[5].NetworkInterface.SSID != "Guests" {
		t.Fatalf("primary interface = %+v", got.Modules[5])
	}
}

func testStorageItemsAssetTags(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	st := storage(t, r, nil, "Shelf")
	typ := itemType(t, r, chairName)
	item := storageItem(t, r, st, typ, "INV-00001")

	later := now.Add(time.Hour)
	item.DeletedAt, item.UpdatedAt = &later, later
	must(t, r.StorageItems.Update(t.Context(), item))

	dup := domain.StorageItem{
		ID: domain.StorageItemID(newID()), AssetTag: "inv-00001", StorageID: st.ID, TypeID: typ.ID, Name: "x",
		CreatedAt: now, UpdatedAt: now,
	}
	wantErr(t, r.StorageItems.Create(t.Context(), dup), domain.ErrAssetTagTaken)

	for tag, want := range map[string]bool{"INV-00001": true, "Inv-00001": true, "INV-00002": false} {
		if got, err := r.StorageItems.AssetTagTaken(t.Context(), tag); err != nil || got != want {
			t.Errorf("AssetTagTaken(%q) = %v, %v", tag, got, err)
		}
	}
}

func testStorageItemsListAndFilters(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	room := storage(t, r, nil, "Room")
	shelf := storage(t, r, &room, "Shelf")
	network := itemType(t, r, "Network")
	sw := childType(t, r, network, switchName)
	chair := itemType(t, r, chairName)
	ada := user(t, r, "ada")

	storageItem(t, r, room, network, routerName)

	swItem := storageItem(t, r, shelf, sw, switchName)
	swItem.ResponsibleID = &ada.ID
	must(t, r.StorageItems.Update(t.Context(), swItem))

	chairItem := storageItem(t, r, shelf, chair, chairName)
	chairItem.Usable, chairItem.DecommissionedAt = false, &now
	must(t, r.StorageItems.Update(t.Context(), chairItem))

	gone := storageItem(t, r, room, chair, "Gone")
	gone.DeletedAt = &now
	must(t, r.StorageItems.Update(t.Context(), gone))

	tests := []struct {
		name    string
		storage *domain.Storage
		filter  port.StorageItemFilter
		want    []string
	}{
		{name: "direct", storage: &room, want: []string{routerName}},
		{name: "recursive", storage: &room, filter: port.StorageItemFilter{Recursive: true}, want: []string{chairName, routerName, switchName}},
		{name: "deleted", storage: &room, filter: port.StorageItemFilter{Deleted: true}, want: []string{"Gone"}},
		{name: "all", want: []string{chairName, routerName, switchName}},
		{name: "type", filter: port.StorageItemFilter{TypeID: &network.ID}, want: []string{routerName}},
		{name: "subtypes", filter: port.StorageItemFilter{TypeID: &network.ID, IncludeSubtypes: true}, want: []string{routerName, switchName}},
		{name: "usable", filter: port.StorageItemFilter{Usable: new(true)}, want: []string{routerName, switchName}},
		{name: "not usable", filter: port.StorageItemFilter{Usable: new(false)}, want: []string{chairName}},
		{name: "responsible", filter: port.StorageItemFilter{ResponsibleID: &ada.ID}, want: []string{switchName}},
		{name: "decommissioned", filter: port.StorageItemFilter{Decommissioned: new(true)}, want: []string{chairName}},
		{name: "in service", filter: port.StorageItemFilter{Decommissioned: new(false)}, want: []string{routerName, switchName}},
		{
			name: "combined", storage: &room,
			filter: port.StorageItemFilter{Recursive: true, TypeID: &network.ID, IncludeSubtypes: true, ResponsibleID: &ada.ID},
			want:   []string{switchName},
		},
	}

	for _, tt := range tests {
		var (
			page domain.Page[domain.StorageItem]
			err  error
		)

		if tt.storage != nil {
			page, err = r.StorageItems.ListInStorage(t.Context(), tt.storage.ID, tt.filter, firstPage(t, 0))
		} else {
			page, err = r.StorageItems.Search(t.Context(), tt.filter, firstPage(t, 0))
		}

		must(t, err)

		if got := itemNames(page.Items); !slices.Equal(got, tt.want) || page.Total != int64(len(tt.want)) {
			t.Errorf("%s = %v (total %d), want %v", tt.name, got, page.Total, tt.want)
		}
	}
}

func testStorageItemsSearch(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	st := storage(t, r, nil, "Shelf")
	typ := itemType(t, r, switchName)

	sw := storageItem(t, r, st, typ, coreSwitch)
	sw.Description, sw.Modules = "rack one", fullModules(t, r)
	must(t, r.StorageItems.Update(t.Context(), sw))

	drillItem := storageItem(t, r, st, typ, drill)
	drillItem.Description = "100%_cordless"
	must(t, r.StorageItems.Update(t.Context(), drillItem))

	for query, want := range map[string][]string{
		"":                  {coreSwitch, drill},
		"CORE":              {coreSwitch},
		"rack":              {coreSwitch},
		"core sw":           {coreSwitch},
		"acme":              {coreSwitch},
		"s24":               {coreSwitch},
		"sn-4":              {coreSwitch},
		"SW1.LAN":           {coreSwitch},
		"telekom":           {coreSwitch},
		"+49171":            {coreSwitch},
		"8949020000":        {coreSwitch},
		"49015420":          {coreSwitch},
		"ios xe":            {coreSwitch},
		"17.9.4":            {coreSwitch},
		"aa:bb":             {coreSwitch},
		"AA-BB-CC-00-11-22": {coreSwitch},
		"0200.0000.0001":    {coreSwitch},
		"%_c":               {drill},
		"drill":             {drill},
		"nothing":           {},
	} {
		page, err := r.StorageItems.Search(t.Context(), port.StorageItemFilter{Query: query}, firstPage(t, 0))
		must(t, err)

		if got := itemNames(page.Items); !slices.Equal(got, want) {
			t.Errorf("Search(%q) = %v, want %v", query, got, want)
		}
	}
}

func testStorageItemsPagination(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	st := storage(t, r, nil, "Shelf")
	typ := itemType(t, r, "Tool")

	var ids []domain.StorageItemID
	for _, name := range []string{"b", "A", "c", "D"} {
		ids = append(ids, storageItem(t, r, st, typ, name).ID)
	}

	var got []string

	page := pageOf(t, 3, "")

	for {
		res, err := r.StorageItems.ListInStorage(t.Context(), st.ID, port.StorageItemFilter{}, page)
		must(t, err)

		if res.Total != 4 {
			t.Fatalf("Total = %d", res.Total)
		}

		got = append(got, itemNames(res.Items)...)
		if res.NextCursor == "" {
			break
		}

		page = pageOf(t, 3, res.NextCursor)
	}

	if !slices.Equal(got, []string{"A", "b", "c", "D"}) {
		t.Fatalf("pages = %v", got)
	}

	many, err := r.StorageItems.GetMany(t.Context(), []domain.StorageItemID{ids[2], ids[0], domain.StorageItemID(newID())})
	must(t, err)

	if names := itemNames(many); len(names) != 2 || !slices.Contains(names, "b") || !slices.Contains(names, "c") {
		t.Fatalf("GetMany() = %v", names)
	}
}
