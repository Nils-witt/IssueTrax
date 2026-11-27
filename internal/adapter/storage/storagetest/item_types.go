package storagetest

import (
	"errors"
	"slices"
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
	"nilswitt.dev/issuetrax/internal/core/service"
)

// childType stores a type called name under parent.
func childType(t *testing.T, r service.Repositories, parent domain.ItemType, name string) domain.ItemType {
	t.Helper()

	typ := domain.ItemType{
		ID: domain.ItemTypeID(newID()), ParentID: &parent.ID, Name: name, Defaults: []domain.ModuleDefault{},
		CreatedAt: now, UpdatedAt: now,
	}
	must(t, r.ItemTypes.Create(t.Context(), typ))

	return typ
}

func equalTypes(a, b domain.ItemType) bool {
	sameDeleted := (a.DeletedAt == nil) == (b.DeletedAt == nil) && (a.DeletedAt == nil || a.DeletedAt.Equal(*b.DeletedAt))
	sameParent := (a.ParentID == nil) == (b.ParentID == nil) && (a.ParentID == nil || *a.ParentID == *b.ParentID)

	return a.ID == b.ID && sameParent && a.Name == b.Name && a.TagPrefix == b.TagPrefix &&
		slices.Equal(a.Defaults, b.Defaults) && a.CreatedAt.Equal(b.CreatedAt) && a.UpdatedAt.Equal(b.UpdatedAt) && sameDeleted
}

func typeNames(items []domain.ItemTypeListItem) []string {
	out := make([]string, 0, len(items))
	for _, i := range items {
		out = append(out, i.Name)
	}

	return out
}

func testItemTypesCreateGetAndUpdate(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	root := itemType(t, r, "Network")

	sw := domain.ItemType{
		ID: domain.ItemTypeID(newID()), ParentID: &root.ID, Name: switchName, TagPrefix: "SW-",
		Defaults:  []domain.ModuleDefault{{Module: domain.ModuleHardware, Count: 1}, {Module: domain.ModuleNetworkInterface, Count: 24}},
		CreatedAt: now, UpdatedAt: now,
	}
	must(t, r.ItemTypes.Create(t.Context(), sw))

	got, err := r.ItemTypes.Get(t.Context(), sw.ID)
	must(t, err)

	if !equalTypes(got.ItemType, sw) || got.ChildCount != 0 || got.ItemCount != 0 {
		t.Fatalf("Get() = %+v, want %+v", got, sw)
	}

	st := storage(t, r, nil, "Shelf")
	storageItem(t, r, st, sw, "sw1")

	if got, err := r.ItemTypes.Get(t.Context(), root.ID); err != nil || got.ChildCount != 1 || got.ItemCount != 0 {
		t.Fatalf("root counts = %+v, %v", got, err)
	}

	if got, err := r.ItemTypes.Get(t.Context(), sw.ID); err != nil || got.ItemCount != 1 {
		t.Fatalf("switch counts = %+v, %v", got, err)
	}

	later := now.Add(time.Hour)
	sw.ParentID, sw.Name, sw.TagPrefix, sw.UpdatedAt, sw.DeletedAt = nil, "Switches", "", later, &later
	sw.Defaults = []domain.ModuleDefault{{Module: domain.ModulePurchase, Count: 1}}
	must(t, r.ItemTypes.Update(t.Context(), sw))

	got, err = r.ItemTypes.Get(t.Context(), sw.ID)
	must(t, err)

	if !equalTypes(got.ItemType, sw) {
		t.Fatalf("after Update() = %+v, want %+v", got.ItemType, sw)
	}

	many, err := r.ItemTypes.GetMany(t.Context(), []domain.ItemTypeID{sw.ID, root.ID, domain.ItemTypeID(newID())})
	must(t, err)

	if len(many) != 2 {
		t.Fatalf("GetMany() = %+v", many)
	}

	_, err = r.ItemTypes.Get(t.Context(), domain.ItemTypeID(newID()))
	wantErr(t, err, domain.ErrItemTypeNotFound)
	wantErr(t, r.ItemTypes.Update(t.Context(), domain.ItemType{ID: domain.ItemTypeID(newID()), Name: "x"}), domain.ErrItemTypeNotFound)
}

func testItemTypesSiblingNames(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	root := itemType(t, r, "Network")
	sw := childType(t, r, root, switchName)

	dup := domain.ItemType{ID: domain.ItemTypeID(newID()), Name: "SWITCH", ParentID: &root.ID, CreatedAt: now, UpdatedAt: now}
	wantErr(t, r.ItemTypes.Create(t.Context(), dup), domain.ErrItemTypeNameTaken)

	// The same name elsewhere in the tree is fine, and so are roots of
	// other names.
	itemType(t, r, switchName)

	rootDup := domain.ItemType{ID: domain.ItemTypeID(newID()), Name: "network", CreatedAt: now, UpdatedAt: now}
	wantErr(t, r.ItemTypes.Create(t.Context(), rootDup), domain.ErrItemTypeNameTaken)

	// A deleted type frees its name; restoring it then clashes.
	later := now.Add(time.Hour)
	sw.DeletedAt, sw.UpdatedAt = &later, later
	must(t, r.ItemTypes.Update(t.Context(), sw))
	must(t, r.ItemTypes.Create(t.Context(), dup))

	sw.DeletedAt = nil
	wantErr(t, r.ItemTypes.Update(t.Context(), sw), domain.ErrItemTypeNameTaken)
}

func testItemTypesListSearchAndChain(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	network := itemType(t, r, "Network")
	itemType(t, r, "chairs")
	gone := itemType(t, r, "Boxes")
	sw := childType(t, r, network, switchName)
	childType(t, r, sw, "PoE switch")
	childType(t, r, network, routerName)

	later := now.Add(time.Hour)
	gone.DeletedAt, gone.UpdatedAt = &later, later
	must(t, r.ItemTypes.Update(t.Context(), gone))

	list := func(parent *domain.ItemTypeID, filter port.ItemTypeFilter, limit int, cursor string) domain.Page[domain.ItemTypeListItem] {
		t.Helper()

		page, err := r.ItemTypes.ListChildren(t.Context(), parent, filter, pageOf(t, limit, cursor))
		must(t, err)

		return page
	}

	if got := typeNames(list(nil, port.ItemTypeFilter{}, 0, "").Items); !slices.Equal(got, []string{"chairs", "Network"}) {
		t.Fatalf("live roots = %v", got)
	}

	first := list(nil, port.ItemTypeFilter{IncludeDeleted: true}, 2, "")
	rest := list(nil, port.ItemTypeFilter{IncludeDeleted: true}, 2, first.NextCursor)

	if got := append(typeNames(first.Items), typeNames(rest.Items)...); !slices.Equal(got, []string{"Boxes", "chairs", "Network"}) ||
		first.Total != 3 || rest.NextCursor != "" {
		t.Fatalf("all roots = %v (total %d)", got, first.Total)
	}

	if got := typeNames(list(&network.ID, port.ItemTypeFilter{}, 0, "").Items); !slices.Equal(got, []string{routerName, switchName}) {
		t.Fatalf("children = %v", got)
	}

	found, err := r.ItemTypes.Search(t.Context(), "SWITCH", port.ItemTypeFilter{}, firstPage(t, 0))
	must(t, err)

	if got := typeNames(found.Items); !slices.Equal(got, []string{"PoE switch", switchName}) || found.Total != 2 {
		t.Fatalf("Search() = %v", got)
	}
}

func testItemTypesChain(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	network := itemType(t, r, "Network")
	sw := childType(t, r, network, switchName)
	poe := childType(t, r, sw, "PoE switch")

	chain, err := r.ItemTypes.Chain(t.Context(), poe.ID)
	must(t, err)

	if len(chain) != 3 || chain[0].ID != network.ID || chain[1].ID != sw.ID || chain[2].ID != poe.ID {
		t.Fatalf("Chain() = %+v", chain)
	}

	if _, err := r.ItemTypes.Chain(t.Context(), domain.ItemTypeID(newID())); !errors.Is(err, domain.ErrItemTypeNotFound) {
		t.Fatalf("Chain(unknown) = %v", err)
	}

	for _, tt := range []struct {
		ancestor, candidate domain.ItemType
		want                bool
	}{
		{network, poe, true}, {sw, poe, true}, {poe, network, false}, {network, network, false},
	} {
		got, err := r.ItemTypes.IsDescendant(t.Context(), tt.ancestor.ID, tt.candidate.ID)
		if err != nil || got != tt.want {
			t.Errorf("IsDescendant(%s, %s) = %v, %v", tt.ancestor.Name, tt.candidate.Name, got, err)
		}
	}

	must(t, r.Tx.WithinTx(t.Context(), r.ItemTypes.LockTree))
}

func testAssetTagCounters(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)

	if n, err := r.AssetTagCounters.Get(t.Context(), "NET-"); err != nil || n != 0 {
		t.Fatalf("Get(new) = %d, %v", n, err)
	}

	must(t, r.AssetTagCounters.Set(t.Context(), "NET-", 4))
	must(t, r.AssetTagCounters.Set(t.Context(), "net-", 7))
	must(t, r.AssetTagCounters.Set(t.Context(), "", 2))

	if n, err := r.AssetTagCounters.Get(t.Context(), "Net-"); err != nil || n != 7 {
		t.Fatalf("Get(NET-) = %d, %v", n, err)
	}

	if n, err := r.AssetTagCounters.Get(t.Context(), ""); err != nil || n != 2 {
		t.Fatalf("Get(empty) = %d, %v", n, err)
	}
}
