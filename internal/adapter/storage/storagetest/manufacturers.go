package storagetest

import (
	"slices"
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
	"nilswitt.dev/issuetrax/internal/core/service"
)

// manufacturer stores a live manufacturer called name.
func manufacturer(t *testing.T, r service.Repositories, name string) domain.Manufacturer {
	t.Helper()

	m := domain.Manufacturer{ID: domain.ManufacturerID(newID()), Name: name, CreatedAt: now, UpdatedAt: now}
	must(t, r.Manufacturers.Create(t.Context(), m))

	return m
}

// hardwareModel stores a live model of maker called name.
func hardwareModel(t *testing.T, r service.Repositories, maker domain.Manufacturer, name string) domain.HardwareModel {
	t.Helper()

	m := domain.HardwareModel{ID: domain.HardwareModelID(newID()), ManufacturerID: maker.ID, Name: name, CreatedAt: now, UpdatedAt: now}
	must(t, r.HardwareModels.Create(t.Context(), m))

	return m
}

// hardwareItem stores a live item whose hardware names maker and model.
func hardwareItem(
	t *testing.T, r service.Repositories, st domain.Storage, typ domain.ItemType, name string,
	maker domain.Manufacturer, model *domain.HardwareModel,
) domain.StorageItem {
	t.Helper()

	item := storageItem(t, r, st, typ, name)
	hw := &domain.Hardware{ManufacturerID: &maker.ID}

	if model != nil {
		hw.ModelID = &model.ID
	}

	item.Modules = []domain.Module{{ID: moduleID(), Kind: domain.ModuleHardware, Hardware: hw}}
	must(t, r.StorageItems.Update(t.Context(), item))

	return item
}

func sameDeletedAt(a, b *time.Time) bool {
	return (a == nil) == (b == nil) && (a == nil || a.Equal(*b))
}

func equalManufacturers(a, b domain.Manufacturer) bool {
	return a.ID == b.ID && a.Name == b.Name && a.CreatedAt.Equal(b.CreatedAt) && a.UpdatedAt.Equal(b.UpdatedAt) &&
		sameDeletedAt(a.DeletedAt, b.DeletedAt)
}

func equalHardwareModels(a, b domain.HardwareModel) bool {
	return a.ID == b.ID && a.ManufacturerID == b.ManufacturerID && a.Name == b.Name &&
		a.CreatedAt.Equal(b.CreatedAt) && a.UpdatedAt.Equal(b.UpdatedAt) && sameDeletedAt(a.DeletedAt, b.DeletedAt)
}

func manufacturerNames(items []domain.ManufacturerListItem) []string {
	out := make([]string, 0, len(items))
	for _, i := range items {
		out = append(out, i.Name)
	}

	return out
}

func modelNames(items []domain.HardwareModelListItem) []string {
	out := make([]string, 0, len(items))
	for _, i := range items {
		out = append(out, i.Name)
	}

	return out
}

func testManufacturersCreateGetAndUpdate(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	acme := manufacturer(t, r, "ACME")

	got, err := r.Manufacturers.Get(t.Context(), acme.ID)
	must(t, err)

	if !equalManufacturers(got.Manufacturer, acme) || got.ModelCount != 0 || got.ItemCount != 0 {
		t.Fatalf("Get() = %+v, want %+v", got, acme)
	}

	s24 := hardwareModel(t, r, acme, "S24")
	gone := hardwareModel(t, r, acme, "S12")
	later := now.Add(time.Hour)
	gone.DeletedAt, gone.UpdatedAt = &later, later
	must(t, r.HardwareModels.Update(t.Context(), gone))

	st := storage(t, r, nil, "Shelf")
	typ := itemType(t, r, switchName)
	hardwareItem(t, r, st, typ, "sw1", acme, &s24)
	deleted := hardwareItem(t, r, st, typ, "sw2", acme, nil)
	deleted.DeletedAt = &later
	must(t, r.StorageItems.Update(t.Context(), deleted))

	if got, err := r.Manufacturers.Get(t.Context(), acme.ID); err != nil || got.ModelCount != 1 || got.ItemCount != 1 {
		t.Fatalf("counts = %+v, %v; want 1 live model and 1 live item", got, err)
	}

	acme.Name, acme.UpdatedAt, acme.DeletedAt = "Acme Inc.", later, &later
	must(t, r.Manufacturers.Update(t.Context(), acme))

	got, err = r.Manufacturers.Get(t.Context(), acme.ID)
	must(t, err)

	if !equalManufacturers(got.Manufacturer, acme) {
		t.Fatalf("after Update() = %+v, want %+v", got.Manufacturer, acme)
	}

	_, err = r.Manufacturers.Get(t.Context(), domain.ManufacturerID(newID()))
	wantErr(t, err, domain.ErrManufacturerNotFound)

	unknown := acme
	unknown.ID = domain.ManufacturerID(newID())
	wantErr(t, r.Manufacturers.Update(t.Context(), unknown), domain.ErrManufacturerNotFound)
}

func testManufacturersLiveNames(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	acme := manufacturer(t, r, "ACME")

	dup := domain.Manufacturer{ID: domain.ManufacturerID(newID()), Name: "acme", CreatedAt: now, UpdatedAt: now}
	wantErr(t, r.Manufacturers.Create(t.Context(), dup), domain.ErrManufacturerNameTaken)

	other := manufacturer(t, r, "Other")
	other.Name = "Acme"
	wantErr(t, r.Manufacturers.Update(t.Context(), other), domain.ErrManufacturerNameTaken)

	// A deleted manufacturer frees its name; restoring it then clashes.
	later := now.Add(time.Hour)
	acme.DeletedAt = &later
	must(t, r.Manufacturers.Update(t.Context(), acme))
	must(t, r.Manufacturers.Create(t.Context(), dup))

	acme.DeletedAt = nil
	wantErr(t, r.Manufacturers.Update(t.Context(), acme), domain.ErrManufacturerNameTaken)
}

func testManufacturersList(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)

	var ids []domain.ManufacturerID
	for _, name := range []string{"b", "A", "c", "D"} {
		ids = append(ids, manufacturer(t, r, name).ID)
	}

	gone := manufacturer(t, r, "100%_gone")
	later := now.Add(time.Hour)
	gone.DeletedAt = &later
	must(t, r.Manufacturers.Update(t.Context(), gone))

	var got []string

	page := pageOf(t, 3, "")

	for {
		res, err := r.Manufacturers.List(t.Context(), port.CatalogFilter{}, page)
		must(t, err)

		if res.Total != 4 {
			t.Fatalf("Total = %d", res.Total)
		}

		got = append(got, manufacturerNames(res.Items)...)
		if res.NextCursor == "" {
			break
		}

		page = pageOf(t, 3, res.NextCursor)
	}

	if !slices.Equal(got, []string{"A", "b", "c", "D"}) {
		t.Fatalf("pages = %v", got)
	}

	for _, tt := range []struct {
		filter port.CatalogFilter
		want   []string
	}{
		{filter: port.CatalogFilter{Query: "a"}, want: []string{"A"}},
		{filter: port.CatalogFilter{Query: "%_"}, want: []string{}},
		{filter: port.CatalogFilter{Query: "%_", IncludeDeleted: true}, want: []string{"100%_gone"}},
		{filter: port.CatalogFilter{IncludeDeleted: true}, want: []string{"100%_gone", "A", "b", "c", "D"}},
	} {
		res, err := r.Manufacturers.List(t.Context(), tt.filter, firstPage(t, 0))
		must(t, err)

		if names := manufacturerNames(res.Items); !slices.Equal(names, tt.want) || res.Total != int64(len(tt.want)) {
			t.Errorf("List(%+v) = %v (total %d), want %v", tt.filter, names, res.Total, tt.want)
		}
	}

	many, err := r.Manufacturers.GetMany(t.Context(), []domain.ManufacturerID{ids[2], gone.ID, domain.ManufacturerID(newID())})
	must(t, err)

	if len(many) != 2 || !slices.ContainsFunc(many, func(m domain.Manufacturer) bool { return equalManufacturers(m, gone) }) {
		t.Fatalf("GetMany() = %+v", many)
	}
}

func testHardwareModelsCreateGetAndUpdate(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	acme := manufacturer(t, r, "ACME")
	s24 := hardwareModel(t, r, acme, "S24")

	got, err := r.HardwareModels.Get(t.Context(), s24.ID)
	must(t, err)

	if !equalHardwareModels(got.HardwareModel, s24) || got.ItemCount != 0 {
		t.Fatalf("Get() = %+v, want %+v", got, s24)
	}

	st := storage(t, r, nil, "Shelf")
	typ := itemType(t, r, switchName)
	hardwareItem(t, r, st, typ, "sw1", acme, &s24)
	hardwareItem(t, r, st, typ, "sw2", acme, &s24)
	hardwareItem(t, r, st, typ, "sw3", acme, nil)

	if got, err := r.HardwareModels.Get(t.Context(), s24.ID); err != nil || got.ItemCount != 2 {
		t.Fatalf("counts = %+v, %v; want 2 items", got, err)
	}

	later := now.Add(time.Hour)
	s24.Name, s24.UpdatedAt, s24.DeletedAt = "S24 Pro", later, &later
	must(t, r.HardwareModels.Update(t.Context(), s24))

	got, err = r.HardwareModels.Get(t.Context(), s24.ID)
	must(t, err)

	if !equalHardwareModels(got.HardwareModel, s24) {
		t.Fatalf("after Update() = %+v, want %+v", got.HardwareModel, s24)
	}

	_, err = r.HardwareModels.Get(t.Context(), domain.HardwareModelID(newID()))
	wantErr(t, err, domain.ErrHardwareModelNotFound)

	unknown := s24
	unknown.ID = domain.HardwareModelID(newID())
	wantErr(t, r.HardwareModels.Update(t.Context(), unknown), domain.ErrHardwareModelNotFound)
}

func testHardwareModelsLiveNames(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	acme := manufacturer(t, r, "ACME")
	other := manufacturer(t, r, "Other")
	s24 := hardwareModel(t, r, acme, "S24")

	// The same name is fine at another manufacturer.
	hardwareModel(t, r, other, "S24")

	dup := domain.HardwareModel{ID: domain.HardwareModelID(newID()), ManufacturerID: acme.ID, Name: "s24", CreatedAt: now, UpdatedAt: now}
	wantErr(t, r.HardwareModels.Create(t.Context(), dup), domain.ErrHardwareModelNameTaken)

	s48 := hardwareModel(t, r, acme, "S48")
	s48.Name = "S24"
	wantErr(t, r.HardwareModels.Update(t.Context(), s48), domain.ErrHardwareModelNameTaken)

	later := now.Add(time.Hour)
	s24.DeletedAt = &later
	must(t, r.HardwareModels.Update(t.Context(), s24))
	must(t, r.HardwareModels.Create(t.Context(), dup))

	s24.DeletedAt = nil
	wantErr(t, r.HardwareModels.Update(t.Context(), s24), domain.ErrHardwareModelNameTaken)
}

func testHardwareModelsList(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	acme := manufacturer(t, r, "ACME")
	other := manufacturer(t, r, "Other")
	hardwareModel(t, r, other, "elsewhere")

	var ids []domain.HardwareModelID
	for _, name := range []string{"b", "A", "c", "D"} {
		ids = append(ids, hardwareModel(t, r, acme, name).ID)
	}

	gone := hardwareModel(t, r, acme, "gone")
	later := now.Add(time.Hour)
	gone.DeletedAt = &later
	must(t, r.HardwareModels.Update(t.Context(), gone))

	var got []string

	page := pageOf(t, 3, "")

	for {
		res, err := r.HardwareModels.List(t.Context(), acme.ID, port.CatalogFilter{}, page)
		must(t, err)

		if res.Total != 4 {
			t.Fatalf("Total = %d", res.Total)
		}

		got = append(got, modelNames(res.Items)...)
		if res.NextCursor == "" {
			break
		}

		page = pageOf(t, 3, res.NextCursor)
	}

	if !slices.Equal(got, []string{"A", "b", "c", "D"}) {
		t.Fatalf("pages = %v", got)
	}

	for _, tt := range []struct {
		filter port.CatalogFilter
		want   []string
	}{
		{filter: port.CatalogFilter{Query: "D"}, want: []string{"D"}},
		{filter: port.CatalogFilter{Query: "gon"}, want: []string{}},
		{filter: port.CatalogFilter{IncludeDeleted: true}, want: []string{"A", "b", "c", "D", "gone"}},
	} {
		res, err := r.HardwareModels.List(t.Context(), acme.ID, tt.filter, firstPage(t, 0))
		must(t, err)

		if names := modelNames(res.Items); !slices.Equal(names, tt.want) || res.Total != int64(len(tt.want)) {
			t.Errorf("List(%+v) = %v (total %d), want %v", tt.filter, names, res.Total, tt.want)
		}
	}

	many, err := r.HardwareModels.GetMany(t.Context(), []domain.HardwareModelID{ids[1], gone.ID, domain.HardwareModelID(newID())})
	must(t, err)

	if len(many) != 2 || !slices.ContainsFunc(many, func(m domain.HardwareModel) bool { return equalHardwareModels(m, gone) }) {
		t.Fatalf("GetMany() = %+v", many)
	}
}
