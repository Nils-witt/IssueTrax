package service_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// addAdmin stores an admin person called name.
func (f *fixture) addAdmin(t *testing.T, name string) domain.User {
	t.Helper()

	u := f.addUser(t, name)
	u.Admin = true

	if err := f.repos.Users.Update(t.Context(), u); err != nil {
		t.Fatal(err)
	}

	return u
}

// addType creates an item type as a fresh admin.
func (f *fixture) addType(t *testing.T, parent *port.ItemTypeView, in port.CreateItemTypeInput) port.ItemTypeView {
	t.Helper()

	if parent != nil {
		in.ParentID = &parent.ID
	}

	v, err := f.itemTypes.Create(t.Context(), f.admin(t), in)
	if err != nil {
		t.Fatalf("create item type %s: %v", in.Name, err)
	}

	return v
}

// admin returns the ID of the fixture's admin, creating it on first use.
func (f *fixture) admin(t *testing.T) domain.UserID {
	t.Helper()

	if u, err := f.repos.Users.GetByIdentity(t.Context(), "iss", "root"); err == nil {
		return u.ID
	}

	return f.addAdmin(t, "root").ID
}

func TestItemTypeService_AdminOnly(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "ada")
	typ := f.addType(t, nil, port.CreateItemTypeInput{Name: "Switch"})

	_, err := f.itemTypes.Create(t.Context(), ada.ID, port.CreateItemTypeInput{Name: "Router"})
	wantCode(t, err, domain.CodeAdminRequired)

	_, err = f.itemTypes.Update(t.Context(), ada.ID, typ.ID, port.UpdateItemTypeInput{Name: new("x")})
	wantCode(t, err, domain.CodeAdminRequired)

	wantCode(t, f.itemTypes.Delete(t.Context(), ada.ID, typ.ID), domain.CodeAdminRequired)

	_, err = f.itemTypes.Create(t.Context(), testActor, port.CreateItemTypeInput{Name: "Router"})
	wantCode(t, err, domain.CodeAdminRequired)

	if _, err := f.itemTypes.Get(t.Context(), typ.ID); err != nil {
		t.Fatalf("anyone may read: %v", err)
	}
}

func TestItemTypeService_TreeAndInheritance(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	device := f.addType(t, nil, port.CreateItemTypeInput{Name: "Device", TagPrefix: "DEV-", Defaults: []domain.ModuleDefault{
		{Module: domain.ModuleHardware, Count: 1}, {Module: domain.ModuleNetworkInterface, Count: 1},
	}})
	network := f.addType(t, &device, port.CreateItemTypeInput{Name: "Network", Defaults: []domain.ModuleDefault{
		{Module: domain.ModuleHardware, Count: 1}, {Module: domain.ModuleNetworkHost, Count: 1},
	}})
	sw := f.addType(t, &network, port.CreateItemTypeInput{Name: "Switch", Defaults: []domain.ModuleDefault{
		{Module: domain.ModuleNetworkInterface, Count: 24},
	}})

	want := []domain.ModuleDefault{
		{Module: domain.ModuleHardware, Count: 1},
		{Module: domain.ModuleNetworkHost, Count: 1},
		{Module: domain.ModuleNetworkInterface, Count: 24},
	}
	if !slices.Equal(sw.EffectiveDefaults, want) || sw.EffectiveTagPrefix != "DEV-" || len(sw.Path) != 2 ||
		sw.Path[0].ID != device.ID || sw.Path[1].ID != network.ID {
		t.Fatalf("switch view = %+v", sw)
	}

	if root := f.addType(t, nil, port.CreateItemTypeInput{Name: "Furniture"}); root.EffectiveTagPrefix != "INV-" {
		t.Fatalf("fallback prefix = %q", root.EffectiveTagPrefix)
	}

	_, err := f.itemTypes.Create(t.Context(), f.admin(t), port.CreateItemTypeInput{Name: "switch", ParentID: &network.ID})
	wantCode(t, err, domain.CodeItemTypeNameTaken)

	_, err = f.itemTypes.Create(t.Context(), f.admin(t), port.CreateItemTypeInput{Name: "x", ParentID: new(domain.ItemTypeID(uuid.New()))})
	wantField(t, err, "parentId", domain.FieldInvalid)

	// Moving a type into its own subtree is a cycle.
	_, err = f.itemTypes.Update(t.Context(), f.admin(t), device.ID, port.UpdateItemTypeInput{
		Parent: &domain.ItemTypeParentChange{ID: &sw.ID},
	})
	wantCode(t, err, domain.CodeItemTypeCycle)

	children, err := f.itemTypes.ListChildren(t.Context(), &device.ID, port.ItemTypeFilter{}, firstPage(t, 0))
	if err != nil || len(children.Items) != 1 || children.Items[0].ChildCount != 1 {
		t.Fatalf("ListChildren() = %+v, %v", children, err)
	}

	found, err := f.itemTypes.Search(t.Context(), "WITCH", port.ItemTypeFilter{}, firstPage(t, 0))
	if err != nil || len(found.Items) != 1 || len(found.Items[0].Path) != 2 {
		t.Fatalf("Search() = %+v, %v", found, err)
	}
}

func TestItemTypeService_UpdateAudits(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	a := f.addType(t, nil, port.CreateItemTypeInput{Name: "A"})
	b := f.addType(t, nil, port.CreateItemTypeInput{Name: "B"})
	f.changes.take()

	defaults := []domain.ModuleDefault{{Module: domain.ModuleNetworkInterface, Count: 4}}

	got, err := f.itemTypes.Update(t.Context(), f.admin(t), b.ID, port.UpdateItemTypeInput{
		Name: new("Switch"), Parent: &domain.ItemTypeParentChange{ID: &a.ID}, TagPrefix: new("SW-"), Defaults: &defaults,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got.Name != "Switch" || *got.ParentID != a.ID || got.EffectiveTagPrefix != "SW-" || len(got.Defaults) != 1 {
		t.Fatalf("Update() = %+v", got)
	}

	entry := f.auditLog(t)[0]
	if entry.Action != domain.AuditItemTypeUpdate {
		t.Fatalf("audit = %+v", entry)
	}

	wantChange(t, entry, "defaults", "", "network_interface×4")
	wantChange(t, entry, "tagPrefix", "", "SW-")

	if changes := f.changes.take(); len(changes) != 1 || changes[0].Kind != domain.ChangeItemTypes {
		t.Fatalf("changes = %v", changes)
	}

	// A no-op update records nothing.
	if _, err := f.itemTypes.Update(t.Context(), f.admin(t), b.ID, port.UpdateItemTypeInput{Name: new("Switch")}); err != nil {
		t.Fatal(err)
	}

	if changes := f.changes.take(); len(changes) != 0 {
		t.Fatalf("no-op changes = %v", changes)
	}
}

func TestItemTypeService_DeleteAndRestore(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	parent := f.addType(t, nil, port.CreateItemTypeInput{Name: "Network"})
	child := f.addType(t, &parent, port.CreateItemTypeInput{Name: "Switch"})
	other := f.addType(t, nil, port.CreateItemTypeInput{Name: "Other"})

	if err := f.itemTypes.Delete(t.Context(), f.admin(t), parent.ID); err != nil {
		t.Fatal(err)
	}

	wantCode(t, f.itemTypes.Delete(t.Context(), f.admin(t), parent.ID), domain.CodeItemTypeDeleted)

	// Children stay live, but nothing can be put under a deleted type.
	if got, err := f.itemTypes.Get(t.Context(), child.ID); err != nil || got.IsDeleted() {
		t.Fatalf("child = %+v, %v", got, err)
	}

	_, err := f.itemTypes.Create(t.Context(), f.admin(t), port.CreateItemTypeInput{Name: "Router", ParentID: &parent.ID})
	wantCode(t, err, domain.CodeItemTypeDeleted)

	_, err = f.itemTypes.Update(t.Context(), f.admin(t), other.ID, port.UpdateItemTypeInput{
		Parent: &domain.ItemTypeParentChange{ID: &parent.ID},
	})
	wantCode(t, err, domain.CodeItemTypeDeleted)

	_, err = f.itemTypes.NextAssetTag(t.Context(), parent.ID)
	wantCode(t, err, domain.CodeItemTypeDeleted)

	live, err := f.itemTypes.ListChildren(t.Context(), nil, port.ItemTypeFilter{}, firstPage(t, 0))
	if err != nil || len(live.Items) != 1 {
		t.Fatalf("live roots = %+v, %v", live.Items, err)
	}

	// The deleted name is free; restoring then clashes.
	f.addType(t, nil, port.CreateItemTypeInput{Name: "network"})

	_, err = f.itemTypes.Restore(t.Context(), f.admin(t), parent.ID)
	wantCode(t, err, domain.CodeItemTypeNameTaken)

	all, err := f.itemTypes.ListChildren(t.Context(), nil, port.ItemTypeFilter{IncludeDeleted: true}, firstPage(t, 0))
	if err != nil || len(all.Items) != 3 {
		t.Fatalf("all roots = %+v, %v", all.Items, err)
	}

	if _, err := f.itemTypes.Get(t.Context(), domain.ItemTypeID(uuid.New())); !errors.Is(err, domain.ErrItemTypeNotFound) {
		t.Fatalf("Get(unknown) = %v", err)
	}
}
