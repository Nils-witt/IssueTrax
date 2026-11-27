package service_test

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// addItem creates a usable item of typ in storage, with the proposed asset
// tag when tag is empty.
func (f *fixture) addItem(
	t *testing.T, storage domain.StorageListItem, typ port.ItemTypeView, name string, edit func(*port.CreateStorageItemInput),
) port.StorageItemView {
	t.Helper()

	tag, err := f.itemTypes.NextAssetTag(t.Context(), typ.ID)
	if err != nil {
		t.Fatal(err)
	}

	in := port.CreateStorageItemInput{AssetTag: tag, Name: name, StorageID: storage.ID, TypeID: typ.ID, Usable: new(true)}
	if edit != nil {
		edit(&in)
	}

	item, err := f.items.Create(t.Context(), testActor, in)
	if err != nil {
		t.Fatalf("create item %s: %v", name, err)
	}

	return item
}

func itemNames(items []port.StorageItemView) []string {
	names := make([]string, 0, len(items))
	for _, i := range items {
		names = append(names, i.Name)
	}

	return names
}

func ethParams(name string) *domain.NetworkInterfaceParams {
	return &domain.NetworkInterfaceParams{
		Name: name, MAC: "02:00:00:00:00:01", Type: domain.NetworkInterfaceWired,
		IPv4: domain.IPConfigParams{Mode: domain.IPModeDHCP}, IPv6: domain.IPConfigParams{Mode: domain.IPModeOff},
	}
}

func TestStorageItemService_Create(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "ada")
	shelf := f.addStorage(t, nil, "Shelf")
	sw := f.addType(t, nil, port.CreateItemTypeInput{Name: "Switch", TagPrefix: "NET-"})
	f.changes.take()

	item := f.addItem(t, shelf, sw, "Core switch", func(in *port.CreateStorageItemInput) {
		in.ResponsibleID = &ada.ID
		in.OrderDate = new(testNow.Add(-time.Hour))
		in.Modules = []domain.ModuleParams{
			{Kind: domain.ModuleNetworkInterface, NetworkInterface: ethParams("eth1")},
			{Kind: domain.ModuleHardware, Hardware: &domain.HardwareParams{Model: "X1"}},
		}
	})

	if item.AssetTag != "NET-00001" || item.Type.Name != "Switch" || item.Responsible == nil ||
		item.Responsible.DisplayName != "ada" || len(item.Modules) != 2 || item.Modules[0].Kind != domain.ModuleHardware {
		t.Fatalf("Create() = %+v", item)
	}

	entry := f.auditLog(t)[0]
	if entry.Action != domain.AuditStorageItemCreate || entry.TargetLabel != "NET-00001 Core switch" {
		t.Fatalf("audit = %+v", entry)
	}

	wantChange(t, entry, "usable", "", "true")

	if changes := f.changes.take(); len(changes) != 1 || changes[0].Kind != domain.ChangeStorageItems {
		t.Fatalf("changes = %v", changes)
	}

	got, err := f.items.Get(t.Context(), item.ID)
	if err != nil || got.Name != "Core switch" {
		t.Fatalf("Get() = %+v, %v", got, err)
	}
}

func TestStorageItemService_AssetTags(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	shelf := f.addStorage(t, nil, "Shelf")
	network := f.addType(t, nil, port.CreateItemTypeInput{Name: "Network", TagPrefix: "NET-"})
	sw := f.addType(t, &network, port.CreateItemTypeInput{Name: "Switch"})
	chair := f.addType(t, nil, port.CreateItemTypeInput{Name: "Chair"})

	next := func(typ port.ItemTypeView) string {
		t.Helper()

		tag, err := f.itemTypes.NextAssetTag(t.Context(), typ.ID)
		if err != nil {
			t.Fatal(err)
		}

		return tag
	}

	if got := next(sw); got != "NET-00001" {
		t.Fatalf("first proposal = %q", got)
	}

	f.addItem(t, shelf, sw, "a", nil)

	// A custom tag that the counter will reach later is skipped.
	f.addItem(t, shelf, sw, "b", func(in *port.CreateStorageItemInput) { in.AssetTag = "net-00002" })
	f.addItem(t, shelf, sw, "c", func(in *port.CreateStorageItemInput) { in.AssetTag = "CUSTOM-1" })

	if got := next(network); got != "NET-00003" {
		t.Fatalf("proposal after custom tags = %q", got)
	}

	if got := next(chair); got != "INV-00001" {
		t.Fatalf("fallback proposal = %q", got)
	}

	// Taken tags conflict ignoring case, deleted items included.
	gone := f.addItem(t, shelf, chair, "gone", nil)
	if err := f.items.Delete(t.Context(), testActor, gone.ID); err != nil {
		t.Fatal(err)
	}

	_, err := f.items.Create(t.Context(), testActor, port.CreateStorageItemInput{
		AssetTag: "inv-00001", Name: "x", StorageID: shelf.ID, TypeID: chair.ID, Usable: new(true),
	})
	wantCode(t, err, domain.CodeAssetTagTaken)

	if got := next(chair); got != "INV-00002" {
		t.Fatalf("proposal after delete = %q", got)
	}
}

func TestStorageItemService_ConcurrentProposals(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	shelf := f.addStorage(t, nil, "Shelf")
	typ := f.addType(t, nil, port.CreateItemTypeInput{Name: "Chair"})

	tag, err := f.itemTypes.NextAssetTag(t.Context(), typ.ID)
	if err != nil {
		t.Fatal(err)
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	for range 2 {
		wg.Go(func() {
			_, err := f.items.Create(t.Context(), testActor, port.CreateStorageItemInput{
				AssetTag: tag, Name: "chair", StorageID: shelf.ID, TypeID: typ.ID, Usable: new(true),
			})

			mu.Lock()

			errs = append(errs, err)
			mu.Unlock()
		})
	}

	wg.Wait()

	if (errs[0] == nil) == (errs[1] == nil) {
		t.Fatalf("errors = %v, want exactly one conflict", errs)
	}
}

func TestStorageItemService_CreateRejects(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	shelf := f.addStorage(t, nil, "Shelf")
	typ := f.addType(t, nil, port.CreateItemTypeInput{Name: "Chair"})
	gone := f.addType(t, nil, port.CreateItemTypeInput{Name: "Gone"})
	bot := f.addUser(t, "bot")
	bot.Issuer = domain.ServiceAccountIssuer

	if err := f.repos.Users.Update(t.Context(), bot); err != nil {
		t.Fatal(err)
	}

	if err := f.itemTypes.Delete(t.Context(), f.admin(t), gone.ID); err != nil {
		t.Fatal(err)
	}

	valid := func() port.CreateStorageItemInput {
		return port.CreateStorageItemInput{AssetTag: "A-1", Name: "x", StorageID: shelf.ID, TypeID: typ.ID, Usable: new(true)}
	}

	tests := []struct {
		name      string
		edit      func(*port.CreateStorageItemInput)
		field     string
		fieldCode string
		code      string
	}{
		{name: "unknown storage", edit: func(in *port.CreateStorageItemInput) { in.StorageID = domain.StorageID(uuid.New()) }, field: "storageId", fieldCode: domain.FieldInvalid},
		{name: "unknown type", edit: func(in *port.CreateStorageItemInput) { in.TypeID = domain.ItemTypeID(uuid.New()) }, field: "typeId", fieldCode: domain.FieldInvalid},
		{name: "deleted type", edit: func(in *port.CreateStorageItemInput) { in.TypeID = gone.ID }, code: domain.CodeItemTypeDeleted},
		{name: "unknown responsible", edit: func(in *port.CreateStorageItemInput) { in.ResponsibleID = new(domain.UserID(uuid.New())) }, field: "personResponsibleId", fieldCode: domain.FieldInvalid},
		{name: "service account responsible", edit: func(in *port.CreateStorageItemInput) { in.ResponsibleID = &bot.ID }, field: "personResponsibleId", fieldCode: domain.FieldInvalid},
		{name: "missing usable", edit: func(in *port.CreateStorageItemInput) { in.Usable = nil }, field: "usable", fieldCode: domain.FieldRequired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			in := valid()
			tt.edit(&in)

			_, err := f.items.Create(t.Context(), testActor, in)
			if tt.code != "" {
				wantCode(t, err, tt.code)
			} else {
				wantField(t, err, tt.field, tt.fieldCode)
			}
		})
	}
}

func TestStorageItemService_Update(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	shelf := f.addStorage(t, nil, "Shelf")
	box := f.addStorage(t, nil, "Box")
	sw := f.addType(t, nil, port.CreateItemTypeInput{Name: "Switch"})
	router := f.addType(t, nil, port.CreateItemTypeInput{Name: "Router"})
	item := f.addItem(t, shelf, sw, "sw1", func(in *port.CreateStorageItemInput) {
		in.Modules = []domain.ModuleParams{{Kind: domain.ModuleNetworkInterface, NetworkInterface: ethParams("eth0")}}
	})
	eth := item.Modules[0]

	f.changes.take()

	got, err := f.items.Update(t.Context(), testActor, item.ID, port.UpdateStorageItemInput{
		StorageID: &box.ID, TypeID: &router.ID, Usable: new(false),
		DecommissionedAt:      &domain.StorageItemDateChange{At: &testNow},
		DecommissioningReason: new("broken"),
		Modules: &[]domain.ModuleParams{
			{ID: &eth.ID, Kind: domain.ModuleNetworkInterface, Primary: true, NetworkInterface: ethParams("eth0")},
			{Kind: domain.ModuleNetworkInterface, NetworkInterface: ethParams("eth1")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	wantUpdated(t, got, item, box, eth)

	entry := f.auditLog(t)[0]
	wantChange(t, entry, "usable", "true", "false")
	wantChange(t, entry, "type", sw.ID.String(), router.ID.String())

	if _, ok := entry.Changes["modules"]; !ok {
		t.Fatalf("modules not audited: %v", entry.Changes)
	}

	if changes := f.changes.take(); len(changes) != 1 {
		t.Fatalf("changes = %v", changes)
	}

	// Unchanged values are not recorded.
	if _, err := f.items.Update(t.Context(), testActor, item.ID, port.UpdateStorageItemInput{Name: new("sw1")}); err != nil {
		t.Fatal(err)
	}

	if changes := f.changes.take(); len(changes) != 0 {
		t.Fatalf("no-op changes = %v", changes)
	}
}

// wantUpdated checks the result of TestStorageItemService_Update's edit.
func wantUpdated(t *testing.T, got, before port.StorageItemView, box domain.StorageListItem, eth domain.Module) {
	t.Helper()

	if got.StorageID != box.ID || got.Type.Name != "Router" || got.Usable || got.DecommissionedAt == nil ||
		len(got.Modules) != 2 || got.Modules[0].ID != eth.ID || !got.Modules[0].Primary || got.AssetTag != before.AssetTag {
		t.Fatalf("Update() = %+v", got)
	}
}

func TestStorageItemService_UpdateDeletedType(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	shelf := f.addStorage(t, nil, "Shelf")
	sw := f.addType(t, nil, port.CreateItemTypeInput{Name: "Switch"})
	router := f.addType(t, nil, port.CreateItemTypeInput{Name: "Router"})
	item := f.addItem(t, shelf, router, "r1", nil)

	if err := f.itemTypes.Delete(t.Context(), f.admin(t), sw.ID); err != nil {
		t.Fatal(err)
	}

	_, err := f.items.Update(t.Context(), testActor, item.ID, port.UpdateStorageItemInput{TypeID: &sw.ID})
	wantCode(t, err, domain.CodeItemTypeDeleted)

	// An item keeps a type that is deleted after it was given.
	if err := f.itemTypes.Delete(t.Context(), f.admin(t), router.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := f.items.Update(t.Context(), testActor, item.ID, port.UpdateStorageItemInput{Name: new("r2")}); err != nil {
		t.Fatal(err)
	}
}

func TestStorageItemService_DeleteAndRestore(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	shelf := f.addStorage(t, nil, "Shelf")
	typ := f.addType(t, nil, port.CreateItemTypeInput{Name: "Chair"})
	item := f.addItem(t, shelf, typ, "chair", nil)

	if err := f.items.Delete(t.Context(), testActor, item.ID); err != nil {
		t.Fatal(err)
	}

	wantCode(t, f.items.Delete(t.Context(), testActor, item.ID), domain.CodeStorageItemDeleted)

	_, err := f.items.Update(t.Context(), testActor, item.ID, port.UpdateStorageItemInput{Name: new("x")})
	wantCode(t, err, domain.CodeStorageItemDeleted)

	restored, err := f.items.Restore(t.Context(), testActor, item.ID)
	if err != nil || restored.IsDeleted() {
		t.Fatalf("Restore() = %+v, %v", restored, err)
	}

	_, err = f.items.Restore(t.Context(), testActor, item.ID)
	wantCode(t, err, domain.CodeStorageItemNotDeleted)

	actions := []domain.AuditAction{}
	for _, e := range f.auditLog(t)[:2] {
		actions = append(actions, e.Action)
	}

	if !slices.Equal(actions, []domain.AuditAction{domain.AuditStorageItemRestore, domain.AuditStorageItemDelete}) {
		t.Fatalf("audit = %v", actions)
	}
}

func TestStorageItemService_ListAndSearch(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "ada")
	room := f.addStorage(t, nil, "Room")
	shelf := f.addStorage(t, &room, "Shelf")
	network := f.addType(t, nil, port.CreateItemTypeInput{Name: "Network"})
	sw := f.addType(t, &network, port.CreateItemTypeInput{Name: "Switch"})
	chair := f.addType(t, nil, port.CreateItemTypeInput{Name: "Chair"})

	f.addItem(t, room, network, "Router", func(in *port.CreateStorageItemInput) {
		in.Modules = []domain.ModuleParams{{Kind: domain.ModuleNetworkInterface, NetworkInterface: ethParams("eth0")}}
	})
	f.addItem(t, shelf, sw, "Switch", func(in *port.CreateStorageItemInput) {
		in.ResponsibleID = &ada.ID
		in.Modules = []domain.ModuleParams{{Kind: domain.ModuleHardware, Hardware: &domain.HardwareParams{SerialNumber: "SN-42"}}}
	})
	f.addItem(t, shelf, chair, "Chair", func(in *port.CreateStorageItemInput) {
		in.Usable = new(false)
		in.DecommissionedAt = &testNow
	})

	list := func(filter port.StorageItemFilter) []string {
		t.Helper()

		page, err := f.items.Search(t.Context(), filter, firstPage(t, 0))
		if err != nil {
			t.Fatal(err)
		}

		return itemNames(page.Items)
	}

	tests := []struct {
		name   string
		filter port.StorageItemFilter
		want   []string
	}{
		{name: "all", want: []string{"Chair", "Router", "Switch"}},
		{name: "type", filter: port.StorageItemFilter{TypeID: &network.ID}, want: []string{"Router"}},
		{name: "subtypes", filter: port.StorageItemFilter{TypeID: &network.ID, IncludeSubtypes: true}, want: []string{"Router", "Switch"}},
		{name: "not usable", filter: port.StorageItemFilter{Usable: new(false)}, want: []string{"Chair"}},
		{name: "responsible", filter: port.StorageItemFilter{ResponsibleID: &ada.ID}, want: []string{"Switch"}},
		{name: "decommissioned", filter: port.StorageItemFilter{Decommissioned: new(true)}, want: []string{"Chair"}},
		{name: "in service", filter: port.StorageItemFilter{Decommissioned: new(false)}, want: []string{"Router", "Switch"}},
		{name: "serial", filter: port.StorageItemFilter{Query: "sn-4"}, want: []string{"Switch"}},
		{name: "asset tag", filter: port.StorageItemFilter{Query: "inv-00001"}, want: []string{"Router"}},
		{name: "mac notation", filter: port.StorageItemFilter{Query: "02-00-00-00-00-01"}, want: []string{"Router"}},
	}

	for _, tt := range tests {
		if got := list(tt.filter); !slices.Equal(got, tt.want) {
			t.Errorf("%s: Search() = %v, want %v", tt.name, got, tt.want)
		}
	}

	direct, err := f.items.ListInStorage(t.Context(), room.ID, port.StorageItemFilter{}, firstPage(t, 0))
	if err != nil || !slices.Equal(itemNames(direct.Items), []string{"Router"}) {
		t.Fatalf("ListInStorage(direct) = %v, %v", itemNames(direct.Items), err)
	}

	deep, err := f.items.ListInStorage(t.Context(), room.ID, port.StorageItemFilter{Recursive: true}, firstPage(t, 0))
	if err != nil || len(deep.Items) != 3 {
		t.Fatalf("ListInStorage(recursive) = %v, %v", itemNames(deep.Items), err)
	}

	_, err = f.items.ListInStorage(t.Context(), domain.StorageID(uuid.New()), port.StorageItemFilter{}, firstPage(t, 0))
	wantCode(t, err, domain.CodeStorageNotFound)
}
