package storagetest

import (
	"slices"
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
	"nilswitt.dev/issuetrax/internal/core/service"
)

// Item names several tests use.
const (
	drill = "Drill"
	bits  = "bits"
)

func genericItem(t *testing.T, r service.Repositories, st domain.Storage, name, description string) domain.GenericStorageItem {
	t.Helper()

	item := domain.GenericStorageItem{
		ID: domain.StorageItemID(newID()), StorageID: st.ID, Kind: domain.StorageItemKindGeneric,
		Name: name, Description: description, CreatedAt: now, UpdatedAt: now,
	}

	must(t, r.GenericStorageItems.Create(t.Context(), item))

	return item
}

func equalItems(a, b domain.GenericStorageItem) bool {
	sameDeleted := (a.DeletedAt == nil) == (b.DeletedAt == nil) && (a.DeletedAt == nil || a.DeletedAt.Equal(*b.DeletedAt))

	return a.ID == b.ID && a.StorageID == b.StorageID && a.Kind == b.Kind && a.Name == b.Name &&
		a.Description == b.Description && a.CreatedAt.Equal(b.CreatedAt) && a.UpdatedAt.Equal(b.UpdatedAt) && sameDeleted
}

func names(items []domain.GenericStorageItem) []string {
	out := make([]string, 0, len(items))
	for _, i := range items {
		out = append(out, i.Name)
	}

	return out
}

func testGenericStorageItemsCreateGetAndUpdate(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	a := storage(t, r, nil, "A")
	b := storage(t, r, nil, "B")
	item := genericItem(t, r, a, drill, "cordless")

	got, err := r.GenericStorageItems.Get(t.Context(), item.ID)
	must(t, err)

	if !equalItems(got, item) {
		t.Fatalf("Get() = %+v, want %+v", got, item)
	}

	later := now.Add(time.Hour)
	item.StorageID = b.ID
	item.Name = "Hammer"
	item.Description = ""
	item.UpdatedAt = later
	item.DeletedAt = &later
	must(t, r.GenericStorageItems.Update(t.Context(), item))

	got, err = r.GenericStorageItems.Get(t.Context(), item.ID)
	must(t, err)

	if !equalItems(got, item) {
		t.Fatalf("Get() after update = %+v, want %+v", got, item)
	}

	item.DeletedAt = nil
	must(t, r.GenericStorageItems.Update(t.Context(), item))

	if got, err = r.GenericStorageItems.Get(t.Context(), item.ID); err != nil || got.IsDeleted() {
		t.Fatalf("Get() after restore = %+v, %v", got, err)
	}

	missing := domain.GenericStorageItem{ID: domain.StorageItemID(newID()), StorageID: a.ID, Name: "x"}
	_, err = r.GenericStorageItems.Get(t.Context(), missing.ID)
	wantErr(t, err, domain.ErrStorageItemNotFound)
	wantErr(t, r.GenericStorageItems.Update(t.Context(), missing), domain.ErrStorageItemNotFound)
}

func testGenericStorageItemsListInStorage(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	room := storage(t, r, nil, "Room")
	shelf := storage(t, r, &room, "Shelf")
	box := storage(t, r, &shelf, "Box")
	other := storage(t, r, nil, "Other")
	genericItem(t, r, room, "lamp", "")
	genericItem(t, r, shelf, drill, "")
	genericItem(t, r, shelf, bits, "")
	genericItem(t, r, box, "Screws", "")
	genericItem(t, r, other, "Chair", "")

	gone := genericItem(t, r, shelf, "Broken", "")
	gone.DeletedAt = &now
	must(t, r.GenericStorageItems.Update(t.Context(), gone))

	tests := []struct {
		name    string
		storage domain.Storage
		filter  port.StorageItemFilter
		want    []string
	}{
		{name: "direct", storage: shelf, want: []string{bits, drill}},
		{name: "recursive", storage: room, filter: port.StorageItemFilter{Recursive: true}, want: []string{bits, drill, "lamp", "Screws"}},
		{name: "recursive leaf", storage: box, filter: port.StorageItemFilter{Recursive: true}, want: []string{"Screws"}},
		{name: "deleted", storage: shelf, filter: port.StorageItemFilter{Deleted: true}, want: []string{"Broken"}},
		{name: "deleted recursive", storage: room, filter: port.StorageItemFilter{Deleted: true, Recursive: true}, want: []string{"Broken"}},
		{name: "empty", storage: storage(t, r, nil, "Empty"), want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, err := r.GenericStorageItems.ListInStorage(t.Context(), tt.storage.ID, tt.filter, pageOf(t, 0, ""))
			must(t, err)

			if !slices.Equal(names(page.Items), tt.want) || page.Total != int64(len(tt.want)) || page.NextCursor != "" {
				t.Fatalf("ListInStorage() = %v (total %d, next %q), want %v", names(page.Items), page.Total, page.NextCursor, tt.want)
			}
		})
	}

	// Pagination walks the same order.
	var (
		walked []string
		cursor string
	)

	for range 10 {
		page, err := r.GenericStorageItems.ListInStorage(t.Context(), room.ID, port.StorageItemFilter{Recursive: true}, pageOf(t, 1, cursor))
		must(t, err)

		if page.Total != 4 {
			t.Fatalf("page total = %d", page.Total)
		}

		walked = append(walked, names(page.Items)...)

		if cursor = page.NextCursor; cursor == "" {
			break
		}
	}

	if !slices.Equal(walked, []string{bits, drill, "lamp", "Screws"}) {
		t.Fatalf("paginated = %v", walked)
	}

	counts := map[domain.StorageID]int64{room.ID: 1, shelf.ID: 2, box.ID: 1}
	for id, want := range counts {
		st, err := r.Storages.Get(t.Context(), id)
		must(t, err)

		if st.ItemCount != want {
			t.Fatalf("storage %s item count = %d, want %d", st.Name, st.ItemCount, want)
		}
	}

	children, err := r.Storages.ListChildren(t.Context(), &room.ID, pageOf(t, 0, ""))
	must(t, err)

	if len(children.Items) != 1 || children.Items[0].ItemCount != 2 {
		t.Fatalf("ListChildren() item counts = %+v", children.Items)
	}
}

func testGenericStorageItemsSearch(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	a := storage(t, r, nil, "A")
	b := storage(t, r, nil, "B")
	genericItem(t, r, a, drill, "cordless")
	genericItem(t, r, b, bits, "for the DRILL")
	genericItem(t, r, b, "Chair", "")
	genericItem(t, r, a, "100% cotton", "")

	gone := genericItem(t, r, a, "Old drill", "")
	gone.DeletedAt = &now
	must(t, r.GenericStorageItems.Update(t.Context(), gone))

	tests := []struct {
		name   string
		filter port.StorageItemFilter
		want   []string
	}{
		{name: "all", want: []string{"100% cotton", bits, "Chair", drill}},
		{name: "name or description ignoring case", filter: port.StorageItemFilter{Query: "dRiLl"}, want: []string{bits, drill}},
		{name: "wildcards are literal", filter: port.StorageItemFilter{Query: "%"}, want: []string{"100% cotton"}},
		{name: "underscore is literal", filter: port.StorageItemFilter{Query: "_"}, want: []string{}},
		{name: "deleted", filter: port.StorageItemFilter{Query: "drill", Deleted: true}, want: []string{"Old drill"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, err := r.GenericStorageItems.Search(t.Context(), tt.filter, pageOf(t, 0, ""))
			must(t, err)

			if !slices.Equal(names(page.Items), tt.want) || page.Total != int64(len(tt.want)) {
				t.Fatalf("Search() = %v (total %d), want %v", names(page.Items), page.Total, tt.want)
			}
		})
	}

	first, err := r.GenericStorageItems.Search(t.Context(), port.StorageItemFilter{}, pageOf(t, 3, ""))
	must(t, err)

	rest, err := r.GenericStorageItems.Search(t.Context(), port.StorageItemFilter{}, pageOf(t, 3, first.NextCursor))
	must(t, err)

	if got := append(names(first.Items), names(rest.Items)...); !slices.Equal(got, []string{"100% cotton", bits, "Chair", drill}) ||
		rest.NextCursor != "" {
		t.Fatalf("paginated search = %v", got)
	}
}

func pageOf(t *testing.T, limit int, cursor string) domain.PageRequest {
	t.Helper()

	p, err := domain.NewPageRequest(limit, cursor)
	must(t, err)

	return p
}
