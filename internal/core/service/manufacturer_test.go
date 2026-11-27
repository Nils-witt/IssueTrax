package service_test

import (
	"testing"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

func (f *fixture) addManufacturer(t *testing.T, name string) domain.ManufacturerListItem {
	t.Helper()

	m, err := f.makers.Create(t.Context(), testActor, name)
	if err != nil {
		t.Fatalf("create manufacturer %q: %v", name, err)
	}

	return m
}

func (f *fixture) addModel(t *testing.T, maker domain.ManufacturerListItem, name string) domain.HardwareModelListItem {
	t.Helper()

	m, err := f.models.Create(t.Context(), testActor, maker.ID, name)
	if err != nil {
		t.Fatalf("create model %q: %v", name, err)
	}

	return m
}

// hardware returns modules holding one hardware module naming maker and
// model (either may be nil).
func hardware(maker *domain.ManufacturerID, model *domain.HardwareModelID) []domain.ModuleParams {
	return []domain.ModuleParams{{Kind: domain.ModuleHardware, Hardware: &domain.HardwareParams{
		ManufacturerID: maker, ModelID: model, SerialNumber: "SN-1",
	}}}
}

func TestManufacturerService_Lifecycle(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	acme := f.addManufacturer(t, " ACME ")

	if acme.Name != "ACME" {
		t.Fatalf("Create() name = %q", acme.Name)
	}

	if changes := f.changes.take(); len(changes) != 1 || changes[0].Kind != domain.ChangeManufacturers {
		t.Fatalf("changes = %v", changes)
	}

	_, err := f.makers.Create(t.Context(), testActor, "acme")
	wantCode(t, err, domain.CodeManufacturerNameTaken)

	_, err = f.makers.Create(t.Context(), testActor, "  ")
	wantField(t, err, "name", domain.FieldRequired)

	renamed, err := f.makers.Rename(t.Context(), testActor, acme.ID, "Acme Inc.")
	if err != nil || renamed.Name != "Acme Inc." {
		t.Fatalf("Rename() = %+v, %v", renamed, err)
	}

	entry := f.auditLog(t)[0]
	if entry.Action != domain.AuditManufacturerUpdate || entry.TargetType != domain.AuditTargetManufacturer {
		t.Fatalf("audit = %+v", entry)
	}

	wantChange(t, entry, "name", "ACME", "Acme Inc.")

	if err := f.makers.Delete(t.Context(), testActor, acme.ID); err != nil {
		t.Fatal(err)
	}

	wantCode(t, f.makers.Delete(t.Context(), testActor, acme.ID), domain.CodeManufacturerDeleted)

	_, err = f.makers.Rename(t.Context(), testActor, acme.ID, "Other")
	wantCode(t, err, domain.CodeManufacturerDeleted)

	_, err = f.models.Create(t.Context(), testActor, acme.ID, "S24")
	wantCode(t, err, domain.CodeManufacturerDeleted)

	// The deleted manufacturer freed its name.
	f.addManufacturer(t, "Acme Inc.")

	_, err = f.makers.Restore(t.Context(), testActor, acme.ID)
	wantCode(t, err, domain.CodeManufacturerNameTaken)

	page, err := f.makers.List(t.Context(), port.CatalogFilter{IncludeDeleted: true}, firstPage(t, 0))
	if err != nil || page.Total != 2 {
		t.Fatalf("List() = %+v, %v", page, err)
	}
}

func TestHardwareModelService_Lifecycle(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	acme := f.addManufacturer(t, "ACME")
	s24 := f.addModel(t, acme, "S24")

	if s24.ManufacturerID != acme.ID {
		t.Fatalf("Create() = %+v", s24)
	}

	_, err := f.models.Create(t.Context(), testActor, acme.ID, "s24")
	wantCode(t, err, domain.CodeHardwareModelNameTaken)

	_, err = f.models.Create(t.Context(), testActor, domain.ManufacturerID(uuid.New()), "X")
	wantCode(t, err, domain.CodeManufacturerNotFound)

	if _, err := f.models.Rename(t.Context(), testActor, s24.ID, "S24 Pro"); err != nil {
		t.Fatal(err)
	}

	if err := f.models.Delete(t.Context(), testActor, s24.ID); err != nil {
		t.Fatal(err)
	}

	restored, err := f.models.Restore(t.Context(), testActor, s24.ID)
	if err != nil || restored.IsDeleted() || restored.Name != "S24 Pro" {
		t.Fatalf("Restore() = %+v, %v", restored, err)
	}

	_, err = f.models.List(t.Context(), domain.ManufacturerID(uuid.New()), port.CatalogFilter{}, firstPage(t, 0))
	wantCode(t, err, domain.CodeManufacturerNotFound)

	page, err := f.models.List(t.Context(), acme.ID, port.CatalogFilter{}, firstPage(t, 0))
	if err != nil || len(page.Items) != 1 || page.Items[0].Name != "S24 Pro" {
		t.Fatalf("List() = %+v, %v", page, err)
	}
}

func TestStorageItemService_HardwareRefs(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	shelf := f.addStorage(t, nil, "Shelf")
	sw := f.addType(t, nil, port.CreateItemTypeInput{Name: "Switch"})
	acme := f.addManufacturer(t, "ACME")
	other := f.addManufacturer(t, "Other")
	s24 := f.addModel(t, acme, "S24")
	foreign := f.addModel(t, other, "X1")

	item := f.addItem(t, shelf, sw, "Core", func(in *port.CreateStorageItemInput) {
		in.Modules = hardware(&acme.ID, &s24.ID)
	})

	if item.Manufacturer == nil || item.Manufacturer.Name != "ACME" || item.Model == nil || item.Model.Name != "S24" {
		t.Fatalf("Create() view = %+v / %+v", item.Manufacturer, item.Model)
	}

	create := func(modules []domain.ModuleParams) error {
		t.Helper()

		_, err := f.items.Create(t.Context(), testActor, port.CreateStorageItemInput{
			AssetTag: "T-" + uuid.NewString()[:8], Name: "x", StorageID: shelf.ID, TypeID: sw.ID, Usable: new(true), Modules: modules,
		})

		return err
	}

	wantField(t, create(hardware(new(domain.ManufacturerID(uuid.New())), nil)), "modules[0].hardware.manufacturerId", domain.FieldInvalid)
	wantField(t, create(hardware(&acme.ID, new(domain.HardwareModelID(uuid.New())))), "modules[0].hardware.modelId", domain.FieldInvalid)
	wantField(t, create(hardware(&acme.ID, &foreign.ID)), "modules[0].hardware.modelId", domain.FieldInvalid)
}

// Deleted entries cannot be picked for new items, but items that have them
// keep them through updates.
func TestStorageItemService_DeletedHardwareRefs(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	shelf := f.addStorage(t, nil, "Shelf")
	sw := f.addType(t, nil, port.CreateItemTypeInput{Name: "Switch"})
	acme := f.addManufacturer(t, "ACME")
	other := f.addManufacturer(t, "Other")
	s24 := f.addModel(t, acme, "S24")

	item := f.addItem(t, shelf, sw, "Core", func(in *port.CreateStorageItemInput) {
		in.Modules = hardware(&acme.ID, &s24.ID)
	})

	create := func(modules []domain.ModuleParams) error {
		t.Helper()

		_, err := f.items.Create(t.Context(), testActor, port.CreateStorageItemInput{
			AssetTag: "T-" + uuid.NewString()[:8], Name: "x", StorageID: shelf.ID, TypeID: sw.ID, Usable: new(true), Modules: modules,
		})

		return err
	}

	if err := f.models.Delete(t.Context(), testActor, s24.ID); err != nil {
		t.Fatal(err)
	}

	if err := f.makers.Delete(t.Context(), testActor, acme.ID); err != nil {
		t.Fatal(err)
	}

	wantCode(t, create(hardware(&acme.ID, nil)), domain.CodeManufacturerDeleted)

	if err := create(hardware(&other.ID, nil)); err != nil {
		t.Fatalf("live manufacturer: %v", err)
	}

	modules := hardware(&acme.ID, &s24.ID)
	modules[0].ID = &item.Modules[0].ID
	modules[0].Hardware.SerialNumber = "SN-2"

	updated, err := f.items.Update(t.Context(), testActor, item.ID, port.UpdateStorageItemInput{Name: new("Core 2"), Modules: &modules})
	if err != nil || updated.Model == nil || !updated.Model.IsDeleted() || !updated.Manufacturer.IsDeleted() {
		t.Fatalf("Update() = %+v, %v", updated, err)
	}

	search, err := f.items.Search(t.Context(), port.StorageItemFilter{Query: "s24"}, firstPage(t, 0))
	if err != nil || len(search.Items) != 1 || search.Items[0].ID != item.ID {
		t.Fatalf("Search(s24) = %+v, %v", search, err)
	}
}
