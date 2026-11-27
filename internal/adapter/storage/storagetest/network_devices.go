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

// Network device names several tests use.
const (
	laptop = "Laptop"
	router = "Router"
)

// fullInterface uses every interface field, both IP families included.
func fullInterface() domain.NetworkInterface {
	return domain.NetworkInterface{
		Name: "eth0", MAC: "aa:bb:cc:00:11:22", Type: domain.NetworkInterfaceWired,
		NativeVLAN: 10, TaggedVLANs: []int{20, 30},
		IPv4: domain.IPConfig{
			Mode:       domain.IPModeStatic,
			Address:    netip.MustParsePrefix("192.168.1.10/24"),
			Gateway:    netip.MustParseAddr("192.168.1.1"),
			DNSServers: []netip.Addr{netip.MustParseAddr("192.168.1.1"), netip.MustParseAddr("9.9.9.9")},
		},
		IPv6: domain.IPConfig{
			Mode:       domain.IPModeStatic,
			Address:    netip.MustParsePrefix("2001:db8::10/64"),
			Gateway:    netip.MustParseAddr("fe80::1"),
			DNSServers: []netip.Addr{netip.MustParseAddr("2001:db8::53")},
		},
	}
}

// plainInterface is a wireless interface without static settings.
func plainInterface(name, mac string) domain.NetworkInterface {
	return domain.NetworkInterface{
		Name: name, MAC: mac, Type: domain.NetworkInterfaceWireless, SSID: "Office",
		TaggedVLANs: []int{},
		IPv4:        domain.IPConfig{Mode: domain.IPModeDHCP, DNSServers: []netip.Addr{}},
		IPv6:        domain.IPConfig{Mode: domain.IPModeSLAAC, DNSServers: []netip.Addr{}},
	}
}

func networkDevice(t *testing.T, r service.Repositories, st domain.Storage, name string) domain.NetworkDevice {
	t.Helper()

	device := domain.NetworkDevice{
		ID: domain.StorageItemID(newID()), StorageID: st.ID, Kind: domain.StorageItemKindNetworkDevice,
		Name: name, CreatedAt: now, UpdatedAt: now,
		Role:      domain.NetworkRoleClient,
		Primary:   plainInterface("wlan0", "02:00:00:00:00:01"),
		Secondary: []domain.NetworkInterface{},
	}

	must(t, r.NetworkDevices.Create(t.Context(), device))

	return device
}

func equalDevices(a, b domain.NetworkDevice) bool {
	return equalItemBase(a.StorageItem, b.StorageItem) && equalDeviceFields(a, b)
}

func equalDeviceFields(a, b domain.NetworkDevice) bool {
	return a.Role == b.Role && a.Hostname == b.Hostname && a.Manufacturer == b.Manufacturer && a.Model == b.Model &&
		a.SerialNumber == b.SerialNumber && a.Primary.Equal(b.Primary) &&
		slices.EqualFunc(a.Secondary, b.Secondary, domain.NetworkInterface.Equal)
}

func deviceNames(devices []domain.NetworkDevice) []string {
	out := make([]string, 0, len(devices))
	for _, d := range devices {
		out = append(out, d.Name)
	}

	return out
}

func testNetworkDevicesCreateGetAndUpdate(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	a := storage(t, r, nil, "A")
	b := storage(t, r, nil, "B")

	device := domain.NetworkDevice{
		ID: domain.StorageItemID(newID()), StorageID: a.ID, Kind: domain.StorageItemKindNetworkDevice,
		Name: "Core router", Description: "rack 1", CreatedAt: now, UpdatedAt: now,
		Role: domain.NetworkRoleRouter, Hostname: "gw.example.org", Manufacturer: "ACME", Model: "R1", SerialNumber: "SN-1",
		Primary:   fullInterface(),
		Secondary: []domain.NetworkInterface{plainInterface("wlan0", "02:00:00:00:00:01"), plainInterface("wlan1", "02:00:00:00:00:02")},
	}
	must(t, r.NetworkDevices.Create(t.Context(), device))

	got, err := r.NetworkDevices.Get(t.Context(), device.ID)
	must(t, err)

	if !equalDevices(got, device) {
		t.Fatalf("Get() = %+v, want %+v", got, device)
	}

	// Swap primary and a secondary, drop the other secondary, move and
	// delete.
	later := now.Add(time.Hour)
	device.StorageID = b.ID
	device.Name = "Edge router"
	device.Role = domain.NetworkRoleSwitch
	device.Hostname = ""
	device.Primary, device.Secondary = device.Secondary[1], []domain.NetworkInterface{device.Primary}
	device.UpdatedAt = later
	device.DeletedAt = &later
	must(t, r.NetworkDevices.Update(t.Context(), device))

	got, err = r.NetworkDevices.Get(t.Context(), device.ID)
	must(t, err)

	if !equalDevices(got, device) {
		t.Fatalf("Get() after update = %+v, want %+v", got, device)
	}

	device.DeletedAt = nil
	must(t, r.NetworkDevices.Update(t.Context(), device))

	if got, err = r.NetworkDevices.Get(t.Context(), device.ID); err != nil || got.IsDeleted() {
		t.Fatalf("Get() after restore = %+v, %v", got, err)
	}

	// Stored values do not alias the caller's.
	device.Primary.TaggedVLANs = append(device.Primary.TaggedVLANs, 99)
	if got, err = r.NetworkDevices.Get(t.Context(), device.ID); err != nil || slices.Contains(got.Primary.TaggedVLANs, 99) {
		t.Fatalf("Get() after changing the caller's copy = %+v, %v", got, err)
	}

	missing := domain.NetworkDevice{
		ID: domain.StorageItemID(newID()), StorageID: a.ID, Name: "x",
		Role: domain.NetworkRoleClient, Primary: plainInterface("x", "02:00:00:00:00:09"),
	}
	_, err = r.NetworkDevices.Get(t.Context(), missing.ID)
	wantErr(t, err, domain.ErrStorageItemNotFound)
	wantErr(t, r.NetworkDevices.Update(t.Context(), missing), domain.ErrStorageItemNotFound)
}

func testNetworkDevicesKindsAreSeparate(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	a := storage(t, r, nil, "A")
	item := genericItem(t, r, a, drill, "")
	device := networkDevice(t, r, a, laptop)

	_, err := r.NetworkDevices.Get(t.Context(), item.ID)
	wantErr(t, err, domain.ErrStorageItemNotFound)

	_, err = r.GenericStorageItems.Get(t.Context(), device.ID)
	wantErr(t, err, domain.ErrStorageItemNotFound)

	devices, err := r.NetworkDevices.ListInStorage(t.Context(), a.ID, port.StorageItemFilter{}, pageOf(t, 0, ""))
	must(t, err)

	items, err := r.GenericStorageItems.ListInStorage(t.Context(), a.ID, port.StorageItemFilter{}, pageOf(t, 0, ""))
	must(t, err)

	if !slices.Equal(deviceNames(devices.Items), []string{laptop}) || !slices.Equal(names(items.Items), []string{drill}) {
		t.Fatalf("lists = %v, %v", deviceNames(devices.Items), names(items.Items))
	}

	// Storages count items of every kind.
	st, err := r.Storages.Get(t.Context(), a.ID)
	must(t, err)

	if st.ItemCount != 2 {
		t.Fatalf("item count = %d, want 2", st.ItemCount)
	}
}

func testNetworkDevicesListInStorage(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	room := storage(t, r, nil, "Room")
	rack := storage(t, r, &room, "Rack")
	networkDevice(t, r, room, "printer")
	networkDevice(t, r, rack, "Switch")
	networkDevice(t, r, rack, router)

	gone := networkDevice(t, r, rack, "Old AP")
	gone.DeletedAt = &now
	must(t, r.NetworkDevices.Update(t.Context(), gone))

	tests := []struct {
		name    string
		storage domain.Storage
		filter  port.StorageItemFilter
		want    []string
	}{
		{name: "direct", storage: rack, want: []string{router, "Switch"}},
		{name: "recursive", storage: room, filter: port.StorageItemFilter{Recursive: true}, want: []string{"printer", router, "Switch"}},
		{name: deletedCase, storage: rack, filter: port.StorageItemFilter{Deleted: true}, want: []string{"Old AP"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, err := r.NetworkDevices.ListInStorage(t.Context(), tt.storage.ID, tt.filter, pageOf(t, 0, ""))
			must(t, err)

			if !slices.Equal(deviceNames(page.Items), tt.want) || page.Total != int64(len(tt.want)) || page.NextCursor != "" {
				t.Fatalf("ListInStorage() = %v (total %d), want %v", deviceNames(page.Items), page.Total, tt.want)
			}

			for _, d := range page.Items {
				if d.Primary.Name != "wlan0" {
					t.Fatalf("listed device %s lacks its interfaces: %+v", d.Name, d)
				}
			}
		})
	}

	first, err := r.NetworkDevices.ListInStorage(t.Context(), room.ID, port.StorageItemFilter{Recursive: true}, pageOf(t, 2, ""))
	must(t, err)

	rest, err := r.NetworkDevices.ListInStorage(t.Context(), room.ID, port.StorageItemFilter{Recursive: true}, pageOf(t, 2, first.NextCursor))
	must(t, err)

	if got := append(deviceNames(first.Items), deviceNames(rest.Items)...); !slices.Equal(got, []string{"printer", router, "Switch"}) ||
		rest.NextCursor != "" {
		t.Fatalf("paginated = %v", got)
	}
}

func testNetworkDevicesSearch(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	a := storage(t, r, nil, "A")

	gw := networkDevice(t, r, a, router)
	gw.Hostname = "gw.example.org"
	gw.SerialNumber = "XYZ-42"
	gw.Secondary = []domain.NetworkInterface{plainInterface("lan", "de:ad:be:ef:00:01")}
	must(t, r.NetworkDevices.Update(t.Context(), gw))

	notebook := networkDevice(t, r, a, laptop)
	notebook.Description = "Ada's"
	must(t, r.NetworkDevices.Update(t.Context(), notebook))

	gone := networkDevice(t, r, a, "Old router")
	gone.DeletedAt = &now
	must(t, r.NetworkDevices.Update(t.Context(), gone))

	genericItem(t, r, a, "Router manual", "")

	tests := []struct {
		name   string
		filter port.StorageItemFilter
		want   []string
	}{
		{name: "all", want: []string{laptop, router}},
		{name: "name", filter: port.StorageItemFilter{Query: "rOUTER"}, want: []string{router}},
		{name: "description", filter: port.StorageItemFilter{Query: "ada"}, want: []string{laptop}},
		{name: "hostname", filter: port.StorageItemFilter{Query: "EXAMPLE"}, want: []string{router}},
		{name: "serial number", filter: port.StorageItemFilter{Query: "xyz"}, want: []string{router}},
		{name: "secondary mac", filter: port.StorageItemFilter{Query: "BE:EF"}, want: []string{router}},
		{name: "primary mac", filter: port.StorageItemFilter{Query: "02:00:00"}, want: []string{laptop, router}},
		{name: deletedCase, filter: port.StorageItemFilter{Query: "router", Deleted: true}, want: []string{"Old router"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, err := r.NetworkDevices.Search(t.Context(), tt.filter, pageOf(t, 0, ""))
			must(t, err)

			if !slices.Equal(deviceNames(page.Items), tt.want) || page.Total != int64(len(tt.want)) {
				t.Fatalf("Search() = %v (total %d), want %v", deviceNames(page.Items), page.Total, tt.want)
			}
		})
	}
}
