package service_test

import (
	"testing"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

func wwanWithSim(sim *domain.StorageItemID) []domain.ModuleParams {
	return []domain.ModuleParams{{Kind: domain.ModuleWWAN, WWAN: &domain.WWANParams{APN: "internet", SimCardItemID: sim}}}
}

func TestStorageItemService_SimCardItemRef(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	shelf := f.addStorage(t, nil, "Shelf")
	typ := f.addType(t, nil, port.CreateItemTypeInput{Name: "Device"})

	sim := f.addItem(t, shelf, typ, "SIM 1", func(in *port.CreateStorageItemInput) {
		in.Modules = []domain.ModuleParams{{Kind: domain.ModuleSimCard, SimCard: &domain.SimCardParams{Provider: "Telekom"}}}
	})
	plain := f.addItem(t, shelf, typ, "Cable", nil)

	router := f.addItem(t, shelf, typ, "LTE router", func(in *port.CreateStorageItemInput) {
		in.Modules = wwanWithSim(&sim.ID)
	})

	if router.SimCardItem == nil || router.SimCardItem.ID != sim.ID || router.SimCardItem.Name != "SIM 1" {
		t.Fatalf("Create() sim card item = %+v", router.SimCardItem)
	}

	create := func(modules []domain.ModuleParams) error {
		t.Helper()

		_, err := f.items.Create(t.Context(), testActor, port.CreateStorageItemInput{
			AssetTag: "T-" + uuid.NewString()[:8], Name: "x", StorageID: shelf.ID, TypeID: typ.ID, Usable: new(true), Modules: modules,
		})

		return err
	}

	const field = "modules[0].wwan.simCardItemId"

	wantField(t, create(wwanWithSim(new(domain.StorageItemID(uuid.New())))), field, domain.FieldInvalid)
	wantField(t, create(wwanWithSim(&plain.ID)), field, domain.FieldInvalid)

	self := wwanWithSim(&router.ID)
	self[0].ID = &router.Modules[0].ID

	_, err := f.items.Update(t.Context(), testActor, router.ID, port.UpdateStorageItemInput{Modules: &self})
	wantField(t, err, field, domain.FieldInvalid)
}

// A deleted SIM card item cannot be picked, but a WWAN module naming it
// keeps it through updates.
func TestStorageItemService_DeletedSimCardItemRef(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	shelf := f.addStorage(t, nil, "Shelf")
	typ := f.addType(t, nil, port.CreateItemTypeInput{Name: "Device"})

	sim := f.addItem(t, shelf, typ, "SIM 1", func(in *port.CreateStorageItemInput) {
		in.Modules = []domain.ModuleParams{{Kind: domain.ModuleSimCard, SimCard: &domain.SimCardParams{}}}
	})
	router := f.addItem(t, shelf, typ, "LTE router", func(in *port.CreateStorageItemInput) {
		in.Modules = wwanWithSim(&sim.ID)
	})

	if err := f.items.Delete(t.Context(), testActor, sim.ID); err != nil {
		t.Fatal(err)
	}

	_, err := f.items.Create(t.Context(), testActor, port.CreateStorageItemInput{
		AssetTag: "T-2", Name: "x", StorageID: shelf.ID, TypeID: typ.ID, Usable: new(true), Modules: wwanWithSim(&sim.ID),
	})
	wantField(t, err, "modules[0].wwan.simCardItemId", domain.FieldInvalid)

	modules := wwanWithSim(&sim.ID)
	modules[0].ID = &router.Modules[0].ID
	modules[0].WWAN.APN = "iot"

	updated, err := f.items.Update(t.Context(), testActor, router.ID, port.UpdateStorageItemInput{Modules: &modules})
	if err != nil || updated.SimCardItem == nil || !updated.SimCardItem.IsDeleted() || updated.Modules[0].WWAN.APN != "iot" {
		t.Fatalf("Update() = %+v, %v", updated, err)
	}
}
