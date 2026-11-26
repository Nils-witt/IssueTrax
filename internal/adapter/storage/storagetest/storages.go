package storagetest

import (
	"slices"
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/service"
)

func storage(t *testing.T, r service.Repositories, parent *domain.Storage, name string) domain.Storage {
	t.Helper()

	st := domain.Storage{ID: domain.StorageID(newID()), Name: name, CreatedAt: now, UpdatedAt: now}
	if parent != nil {
		id := parent.ID
		st.ParentID = &id
	}

	must(t, r.Storages.Create(t.Context(), st))

	return st
}

func equalStorages(a, b domain.Storage) bool {
	return a.ID == b.ID && a.ParentString() == b.ParentString() && a.Name == b.Name &&
		a.CreatedAt.Equal(b.CreatedAt) && a.UpdatedAt.Equal(b.UpdatedAt)
}

func testStoragesCreateAndGet(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	root := storage(t, r, nil, "Warehouse")
	shelf := storage(t, r, &root, "Shelf")
	storage(t, r, &shelf, "Box")

	for _, tt := range []struct {
		want     domain.Storage
		children int64
	}{{root, 1}, {shelf, 1}} {
		got, err := r.Storages.Get(t.Context(), tt.want.ID)
		must(t, err)

		if !equalStorages(got.Storage, tt.want) || got.ChildCount != tt.children {
			t.Fatalf("Get() = %+v, want %+v with %d children", got, tt.want, tt.children)
		}
	}

	_, err := r.Storages.Get(t.Context(), domain.StorageID(newID()))
	wantErr(t, err, domain.ErrStorageNotFound)
}

func testStoragesUniqueSiblingNames(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	a := storage(t, r, nil, "Room A")
	b := storage(t, r, nil, "Room B")
	storage(t, r, &a, "Shelf")

	clash := func(parent *domain.StorageID, name string) error {
		return r.Storages.Create(t.Context(), domain.Storage{
			ID: domain.StorageID(newID()), ParentID: parent, Name: name, CreatedAt: now, UpdatedAt: now,
		})
	}

	wantErr(t, clash(nil, "ROOM a"), domain.ErrStorageNameTaken)
	wantErr(t, clash(&a.ID, "shelf"), domain.ErrStorageNameTaken)

	// The same name is fine under another parent, and for a child of a
	// root with that name.
	storage(t, r, &b, "Shelf")
	storage(t, r, &a, "Room A")

	// Renaming or moving onto a sibling's name clashes too.
	renamed := b
	renamed.Name = "room a"
	wantErr(t, r.Storages.Update(t.Context(), renamed), domain.ErrStorageNameTaken)

	moved := storage(t, r, nil, "Shelf")
	moved.ParentID = &a.ID
	wantErr(t, r.Storages.Update(t.Context(), moved), domain.ErrStorageNameTaken)
}

func testStoragesUpdate(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	a := storage(t, r, nil, "A")
	b := storage(t, r, nil, "B")
	child := storage(t, r, &a, "Child")

	child.Name, child.ParentID, child.UpdatedAt = "Moved", &b.ID, now.Add(time.Hour)
	must(t, r.Storages.Update(t.Context(), child))

	got, err := r.Storages.Get(t.Context(), child.ID)
	must(t, err)

	if !equalStorages(got.Storage, child) {
		t.Fatalf("after Update() = %+v, want %+v", got.Storage, child)
	}

	child.ParentID = nil
	must(t, r.Storages.Update(t.Context(), child))

	got, err = r.Storages.Get(t.Context(), child.ID)
	must(t, err)

	if got.ParentID != nil {
		t.Fatalf("after move to root ParentID = %v, want nil", got.ParentID)
	}

	missing := child
	missing.ID = domain.StorageID(newID())
	wantErr(t, r.Storages.Update(t.Context(), missing), domain.ErrStorageNotFound)
}

func testStoragesListChildren(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)

	var roots []domain.Storage
	for _, name := range []string{"e", "D", "c", "B", "a"} {
		roots = append(roots, storage(t, r, nil, name))
	}

	parent := roots[0] // "e"
	storage(t, r, &parent, "z")
	storage(t, r, &parent, "Y")
	storage(t, r, &roots[1], "only")

	names, counts, total := listAllChildren(t, r, nil, 2)
	if want := []string{"a", "B", "c", "D", "e"}; !slices.Equal(names, want) || total != 5 {
		t.Fatalf("roots = %v (total %d), want %v (total 5)", names, total, want)
	}

	if want := []int64{0, 0, 0, 1, 2}; !slices.Equal(counts, want) {
		t.Fatalf("child counts = %v, want %v", counts, want)
	}

	names, _, total = listAllChildren(t, r, &parent.ID, 1)
	if want := []string{"Y", "z"}; !slices.Equal(names, want) || total != 2 {
		t.Fatalf("children = %v (total %d), want %v (total 2)", names, total, want)
	}

	empty, err := r.Storages.ListChildren(t.Context(), &roots[4].ID, firstPage(t, 0))
	must(t, err)

	if empty.Items == nil || len(empty.Items) != 0 || empty.Total != 0 || empty.NextCursor != "" {
		t.Fatalf("empty ListChildren() = %#v", empty)
	}

	_, err = r.Storages.ListChildren(t.Context(), nil, domain.PageRequest{Limit: 1, Cursor: "garbage!"})
	wantErr(t, err, domain.ErrInvalid)
}

// listAllChildren pages through parent's children limit at a time and
// returns their names, child counts and the reported total.
func listAllChildren(t *testing.T, r service.Repositories, parent *domain.StorageID, limit int) (names []string, counts []int64, total int64) {
	t.Helper()

	page := firstPage(t, limit)

	for {
		got, err := r.Storages.ListChildren(t.Context(), parent, page)
		must(t, err)

		total = got.Total
		for _, s := range got.Items {
			names = append(names, s.Name)
			counts = append(counts, s.ChildCount)
		}

		if got.NextCursor == "" {
			return names, counts, total
		}

		page.Cursor = got.NextCursor
	}
}

func testStoragesIsDescendant(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	root := storage(t, r, nil, "Root")
	mid := storage(t, r, &root, "Mid")
	leaf := storage(t, r, &mid, "Leaf")
	other := storage(t, r, nil, "Other")

	for _, tt := range []struct {
		name                string
		ancestor, candidate domain.Storage
		want                bool
	}{
		{"child", root, mid, true},
		{"grandchild", root, leaf, true},
		{"self", root, root, false},
		{"parent", leaf, root, false},
		{"unrelated", other, leaf, false},
	} {
		got, err := r.Storages.IsDescendant(t.Context(), tt.ancestor.ID, tt.candidate.ID)
		must(t, err)

		if got != tt.want {
			t.Fatalf("%s: IsDescendant() = %v, want %v", tt.name, got, tt.want)
		}
	}

	must(t, r.Tx.WithinTx(t.Context(), r.Storages.LockTree))
}
