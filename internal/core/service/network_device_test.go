package service_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

func wiredDHCP(name, mac string) domain.NetworkInterfaceParams {
	return domain.NetworkInterfaceParams{
		Name: name, MAC: mac, Type: domain.NetworkInterfaceWired,
		IPv4: domain.IPConfigParams{Mode: domain.IPModeDHCP},
		IPv6: domain.IPConfigParams{Mode: domain.IPModeOff},
	}
}

func (f *fixture) addDevice(t *testing.T, storage domain.StorageListItem, name string) domain.NetworkDevice {
	t.Helper()

	device, err := f.devices.Create(t.Context(), testActor, port.CreateNetworkDeviceInput{
		StorageID: storage.ID, Name: name, Role: domain.NetworkRoleClient,
		Primary:   wiredDHCP("eth0", "02:00:00:00:00:01"),
		Secondary: []domain.NetworkInterfaceParams{wiredDHCP("eth1", "02:00:00:00:00:02")},
	})
	if err != nil {
		t.Fatalf("create device %s: %v", name, err)
	}

	return device
}

func TestNetworkDeviceService_Create(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	rack := f.addStorage(t, nil, "Rack")
	f.changes.take()

	device, err := f.devices.Create(t.Context(), testActor, port.CreateNetworkDeviceInput{
		StorageID: rack.ID, Name: " Router ", Role: domain.NetworkRoleRouter, Hostname: "gw", SerialNumber: "SN",
		Primary: wiredDHCP("eth0", "AA:BB:CC:00:11:22"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if device.Name != "Router" || device.Kind != domain.StorageItemKindNetworkDevice || device.Primary.MAC != "aa:bb:cc:00:11:22" ||
		len(device.Secondary) != 0 {
		t.Fatalf("Create() = %+v", device)
	}

	got, err := f.devices.Get(t.Context(), device.ID)
	if err != nil || got.Hostname != "gw" || got.Primary.Name != "eth0" {
		t.Fatalf("Get() = %+v, %v", got, err)
	}

	if got := f.changes.take(); !slices.Equal(got, []domain.Change{{Kind: domain.ChangeStorageItems}}) {
		t.Fatalf("changes = %v", got)
	}

	wantCreateAudit(t, f, device)
}

// wantCreateAudit checks the audit entry of creating the router in
// TestNetworkDeviceService_Create.
func wantCreateAudit(t *testing.T, f *fixture, device domain.NetworkDevice) {
	t.Helper()

	e := f.auditLog(t)[0]
	if e.Action != domain.AuditNetworkDeviceCreate || e.TargetType != domain.AuditTargetNetworkDevice ||
		e.TargetID != device.ID.String() || e.TargetLabel != "Router" {
		t.Fatalf("audit entry = %+v", e)
	}

	wantChange(t, e, "role", "", "router")
	wantChange(t, e, "serialNumber", "", "SN")
	wantChange(t, e, "primaryInterface", "", "eth0 aa:bb:cc:00:11:22 wired; ipv4 dhcp; ipv6 off")

	if _, ok := e.Changes["secondaryInterfaces"]; ok {
		t.Fatalf("empty secondary interfaces recorded: %+v", e.Changes)
	}
}

func TestNetworkDeviceService_CreateRejects(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	_, err := f.devices.Create(t.Context(), testActor, port.CreateNetworkDeviceInput{
		StorageID: domain.StorageID(uuid.New()), Name: "Router", Role: domain.NetworkRoleRouter,
		Primary: wiredDHCP("eth0", "02:00:00:00:00:01"),
	})
	wantField(t, err, "storageId", domain.FieldInvalid)

	rack := f.addStorage(t, nil, "Rack")

	_, err = f.devices.Create(t.Context(), testActor, port.CreateNetworkDeviceInput{StorageID: rack.ID, Name: "Router"})
	wantField(t, err, "role", domain.FieldRequired)
	wantField(t, err, "primaryInterface.mac", domain.FieldRequired)

	if entries := f.auditLog(t); len(entries) != 1 { // only the storage
		t.Fatalf("audit entries = %+v", entries)
	}
}

func TestNetworkDeviceService_UpdateSwapsPrimaryAndMoves(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	a := f.addStorage(t, nil, "A")
	b := f.addStorage(t, nil, "B")
	device := f.addDevice(t, a, "Laptop")
	f.changes.take()

	primary := wiredDHCP("eth1", "02:00:00:00:00:02")
	secondary := []domain.NetworkInterfaceParams{wiredDHCP("eth0", "02:00:00:00:00:01")}
	role := domain.NetworkRoleSwitch

	got, err := f.devices.Update(t.Context(), testActor, device.ID, port.UpdateNetworkDeviceInput{
		StorageID: &b.ID, Role: &role, Primary: &primary, Secondary: &secondary,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got.Primary.Name != "eth1" || got.Secondary[0].Name != "eth0" || got.StorageID != b.ID || got.Role != role {
		t.Fatalf("Update() = %+v", got)
	}

	if got := f.changes.take(); !slices.Equal(got, []domain.Change{{Kind: domain.ChangeStorageItems}}) {
		t.Fatalf("changes = %v", got)
	}

	e := f.auditLog(t)[0]
	if e.Action != domain.AuditNetworkDeviceUpdate {
		t.Fatalf("audit action = %s", e.Action)
	}

	wantChange(t, e, "role", "client", "switch")
	wantChange(t, e, "storage", a.ID.String(), b.ID.String())
	wantChange(t, e, "primaryInterface",
		"eth0 02:00:00:00:00:01 wired; ipv4 dhcp; ipv6 off", "eth1 02:00:00:00:00:02 wired; ipv4 dhcp; ipv6 off")

	// Nothing changed: no audit entry, no announcement.
	before := len(f.auditLog(t))

	if _, err := f.devices.Update(t.Context(), testActor, device.ID, port.UpdateNetworkDeviceInput{Primary: &primary}); err != nil {
		t.Fatal(err)
	}

	if len(f.auditLog(t)) != before || len(f.changes.take()) != 0 {
		t.Fatal("no-op update was recorded")
	}

	// Moving to a missing storage fails.
	missing := domain.StorageID(uuid.New())
	_, err = f.devices.Update(t.Context(), testActor, device.ID, port.UpdateNetworkDeviceInput{StorageID: &missing})
	wantField(t, err, "storageId", domain.FieldInvalid)
}

func TestNetworkDeviceService_DeleteAndRestore(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	a := f.addStorage(t, nil, "A")
	device := f.addDevice(t, a, "Laptop")

	if err := f.devices.Delete(t.Context(), testActor, device.ID); err != nil {
		t.Fatal(err)
	}

	if err := f.devices.Delete(t.Context(), testActor, device.ID); !errors.Is(err, domain.ErrStorageItemDeleted) {
		t.Fatalf("second Delete() = %v", err)
	}

	name := "x"
	if _, err := f.devices.Update(t.Context(), testActor, device.ID, port.UpdateNetworkDeviceInput{Name: &name}); !errors.Is(
		err, domain.ErrStorageItemDeleted) {
		t.Fatalf("Update(deleted) = %v", err)
	}

	deleted, err := f.devices.ListInStorage(t.Context(), a.ID, port.StorageItemFilter{Deleted: true}, firstPage(t, 50))
	if err != nil || len(deleted.Items) != 1 {
		t.Fatalf("deleted list = %+v, %v", deleted, err)
	}

	restored, err := f.devices.Restore(t.Context(), testActor, device.ID)
	if err != nil || restored.IsDeleted() || restored.Primary.Name != "eth0" {
		t.Fatalf("Restore() = %+v, %v", restored, err)
	}

	if _, err := f.devices.Restore(t.Context(), testActor, device.ID); !errors.Is(err, domain.ErrStorageItemNotDeleted) {
		t.Fatalf("second Restore() = %v", err)
	}

	actions := []domain.AuditAction{}
	for _, e := range f.auditLog(t) {
		actions = append(actions, e.Action)
	}

	if !slices.Contains(actions, domain.AuditNetworkDeviceDelete) || !slices.Contains(actions, domain.AuditNetworkDeviceRestore) {
		t.Fatalf("audit actions = %v", actions)
	}
}

func TestNetworkDeviceService_KindsAndMissingStorage(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	a := f.addStorage(t, nil, "A")
	device := f.addDevice(t, a, "Laptop")

	// Generic items do not see devices.
	if _, err := f.items.Get(t.Context(), device.ID); !errors.Is(err, domain.ErrStorageItemNotFound) {
		t.Fatalf("generic Get(device) = %v", err)
	}

	if _, err := f.devices.ListInStorage(t.Context(), domain.StorageID(uuid.New()), port.StorageItemFilter{}, firstPage(t, 50)); !errors.Is(
		err, domain.ErrStorageNotFound) {
		t.Fatalf("ListInStorage(missing) = %v", err)
	}
}
