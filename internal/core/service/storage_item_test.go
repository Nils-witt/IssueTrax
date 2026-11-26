package service_test

import (
	"slices"
	"testing"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

func (f *fixture) addItem(t *testing.T, storage domain.StorageListItem, name, description string) domain.GenericStorageItem {
	t.Helper()

	item, err := f.items.Create(t.Context(), testActor, port.CreateGenericStorageItemInput{
		StorageID: storage.ID, Name: name, Description: description,
	})
	if err != nil {
		t.Fatalf("create item %s: %v", name, err)
	}

	return item
}

func itemNames(items []domain.GenericStorageItem) []string {
	names := make([]string, 0, len(items))
	for _, i := range items {
		names = append(names, i.Name)
	}

	return names
}

func TestGenericStorageItemService_Create(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	shelf := f.addStorage(t, nil, "Shelf")
	f.changes.take()

	item, err := f.items.Create(t.Context(), testActor, port.CreateGenericStorageItemInput{
		StorageID: shelf.ID, Name: " Drill ", Description: "cordless",
	})
	if err != nil {
		t.Fatal(err)
	}

	if item.Name != "Drill" || item.StorageID != shelf.ID || item.Kind != domain.StorageItemKindGeneric || !item.CreatedAt.Equal(testNow) {
		t.Fatalf("Create() = %+v", item)
	}

	if got := f.changes.take(); !slices.Equal(got, []domain.Change{{Kind: domain.ChangeStorageItems}}) {
		t.Fatalf("changes = %v", got)
	}

	entries := f.auditLog(t)
	e := entries[0]

	if e.Action != domain.AuditGenericStorageItemCreate || e.TargetType != domain.AuditTargetGenericStorageItem ||
		e.TargetID != item.ID.String() || e.TargetLabel != "Drill" {
		t.Fatalf("audit entry = %+v", e)
	}

	wantChange(t, e, "name", "", "Drill")
	wantChange(t, e, "storage", "", shelf.ID.String())

	if c := e.Changes["description"]; !c.Redacted {
		t.Fatalf("description change = %+v, want redacted", c)
	}
}

func TestGenericStorageItemService_CreateRejects(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	shelf := f.addStorage(t, nil, "Shelf")

	_, err := f.items.Create(t.Context(), testActor, port.CreateGenericStorageItemInput{StorageID: domain.StorageID(uuid.New()), Name: "Saw"})
	wantField(t, err, "storageId", domain.FieldInvalid)

	_, err = f.items.Create(t.Context(), testActor, port.CreateGenericStorageItemInput{StorageID: shelf.ID, Name: " "})
	wantField(t, err, "name", domain.FieldRequired)
}

func TestGenericStorageItemService_UpdateEditsAndMoves(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	a := f.addStorage(t, nil, "A")
	b := f.addStorage(t, nil, "B")
	item := f.addItem(t, a, "Drill", "")
	f.changes.take()

	name, desc := "Hammer", "heavy"

	got, err := f.items.Update(t.Context(), testActor, item.ID, port.UpdateGenericStorageItemInput{
		Name: &name, Description: &desc, StorageID: &b.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got.Name != "Hammer" || got.Description != "heavy" || got.StorageID != b.ID {
		t.Fatalf("Update() = %+v", got)
	}

	if got := f.changes.take(); !slices.Equal(got, []domain.Change{{Kind: domain.ChangeStorageItems}}) {
		t.Fatalf("changes = %v", got)
	}

	e := f.auditLog(t)[0]
	if e.Action != domain.AuditGenericStorageItemUpdate {
		t.Fatalf("audit action = %s", e.Action)
	}

	wantChange(t, e, "name", "Drill", "Hammer")
	wantChange(t, e, "storage", a.ID.String(), b.ID.String())

	// Nothing changed: no audit entry, no announcement.
	if _, err := f.items.Update(t.Context(), testActor, item.ID, port.UpdateGenericStorageItemInput{Name: &name}); err != nil {
		t.Fatal(err)
	}

	if got := f.changes.take(); len(got) != 0 {
		t.Fatalf("changes after no-op = %v", got)
	}

	missing := domain.StorageID(uuid.New())
	_, err = f.items.Update(t.Context(), testActor, item.ID, port.UpdateGenericStorageItemInput{StorageID: &missing})
	wantField(t, err, "storageId", domain.FieldInvalid)

	_, err = f.items.Update(t.Context(), testActor, domain.StorageItemID(uuid.New()), port.UpdateGenericStorageItemInput{Name: &name})
	wantCode(t, err, domain.CodeStorageItemNotFound)
}

func TestGenericStorageItemService_DeleteAndRestore(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	shelf := f.addStorage(t, nil, "Shelf")
	item := f.addItem(t, shelf, "Drill", "")
	f.changes.take()

	if err := f.items.Delete(t.Context(), testActor, item.ID); err != nil {
		t.Fatal(err)
	}

	got, err := f.items.Get(t.Context(), item.ID)
	if err != nil || !got.IsDeleted() || !got.DeletedAt.Equal(testNow) {
		t.Fatalf("Get() after delete = %+v, %v", got, err)
	}

	if st, _ := f.storages.Get(t.Context(), shelf.ID); st.ItemCount != 0 {
		t.Fatalf("item count after delete = %d", st.ItemCount)
	}

	live, _ := f.items.ListInStorage(t.Context(), shelf.ID, port.StorageItemFilter{}, firstPage(t, 0))
	deleted, _ := f.items.ListInStorage(t.Context(), shelf.ID, port.StorageItemFilter{Deleted: true}, firstPage(t, 0))

	if len(live.Items) != 0 || len(deleted.Items) != 1 {
		t.Fatalf("live = %v, deleted = %v", itemNames(live.Items), itemNames(deleted.Items))
	}

	name := "Saw"
	_, err = f.items.Update(t.Context(), testActor, item.ID, port.UpdateGenericStorageItemInput{Name: &name})
	wantCode(t, err, domain.CodeStorageItemDeleted)
	wantCode(t, f.items.Delete(t.Context(), testActor, item.ID), domain.CodeStorageItemDeleted)

	restored, err := f.items.Restore(t.Context(), testActor, item.ID)
	if err != nil || restored.IsDeleted() {
		t.Fatalf("Restore() = %+v, %v", restored, err)
	}

	_, err = f.items.Restore(t.Context(), testActor, item.ID)
	wantCode(t, err, domain.CodeStorageItemNotDeleted)

	wantActions(t, f.auditLog(t), domain.AuditGenericStorageItemRestore, domain.AuditGenericStorageItemDelete,
		domain.AuditGenericStorageItemCreate, domain.AuditStorageCreate)

	want := []domain.Change{{Kind: domain.ChangeStorageItems}, {Kind: domain.ChangeStorageItems}}
	if got := f.changes.take(); !slices.Equal(got, want) {
		t.Fatalf("changes = %v", got)
	}
}

func TestGenericStorageItemService_ListAndSearch(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	room := f.addStorage(t, nil, "Room")
	shelf := f.addStorage(t, &room, "Shelf")
	box := f.addStorage(t, &shelf, "Box")
	other := f.addStorage(t, nil, "Other")
	f.addItem(t, room, "lamp", "")
	f.addItem(t, shelf, "Drill", "cordless")
	f.addItem(t, box, "bits", "for the DRILL")
	f.addItem(t, other, "Chair", "")

	if st, err := f.storages.Get(t.Context(), shelf.ID); err != nil || st.ItemCount != 1 {
		t.Fatalf("shelf item count = %+v, %v", st, err)
	}

	direct, err := f.items.ListInStorage(t.Context(), shelf.ID, port.StorageItemFilter{}, firstPage(t, 0))
	if err != nil || !slices.Equal(itemNames(direct.Items), []string{"Drill"}) {
		t.Fatalf("direct = %v, %v", itemNames(direct.Items), err)
	}

	subtree, err := f.items.ListInStorage(t.Context(), room.ID, port.StorageItemFilter{Recursive: true}, firstPage(t, 0))
	if err != nil || !slices.Equal(itemNames(subtree.Items), []string{"bits", "Drill", "lamp"}) || subtree.Total != 3 {
		t.Fatalf("subtree = %v (total %d), %v", itemNames(subtree.Items), subtree.Total, err)
	}

	_, err = f.items.ListInStorage(t.Context(), domain.StorageID(uuid.New()), port.StorageItemFilter{}, firstPage(t, 0))
	wantCode(t, err, domain.CodeStorageNotFound)

	found, err := f.items.Search(t.Context(), port.StorageItemFilter{Query: "drill"}, firstPage(t, 0))
	if err != nil || !slices.Equal(itemNames(found.Items), []string{"bits", "Drill"}) {
		t.Fatalf("search = %v, %v", itemNames(found.Items), err)
	}

	all, err := f.items.Search(t.Context(), port.StorageItemFilter{}, firstPage(t, 0))
	if err != nil || all.Total != 4 {
		t.Fatalf("search all = %v, %v", itemNames(all.Items), err)
	}
}
