package service_test

import (
	"slices"
	"testing"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

func storageItemNames(v port.IssueView) []string {
	names := make([]string, 0, len(v.StorageItems))
	for _, i := range v.StorageItems {
		names = append(names, i.Name)
	}

	return names
}

func TestIssueService_StorageItemLinks(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "ada")
	f.addProject(t, "TRAX", "IssueTrax")
	shelf := f.addStorage(t, nil, "Shelf")
	tool := f.addType(t, nil, port.CreateItemTypeInput{Name: "Tool"})
	drill := f.addItem(t, shelf, tool, "drill", nil)
	sw := f.addItem(t, shelf, tool, "Switch", nil)

	created := f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{
		Title: "Broken", StorageItems: []domain.StorageItemID{sw.ID, drill.ID, sw.ID},
	})

	// Sorted by name ignoring case; duplicates dropped.
	if got := storageItemNames(created); !slices.Equal(got, []string{"drill", "Switch"}) {
		t.Fatalf("created storage items = %v", got)
	}

	if created.StorageItems[1].AssetTag != sw.AssetTag {
		t.Fatalf("switch asset tag = %q", created.StorageItems[1].AssetTag)
	}

	wantChange(t, f.auditLog(t)[0], "storageItems", "", "Switch, drill")

	// A linked item that is deleted later stays linked and may be kept.
	if err := f.items.Delete(t.Context(), testActor, drill.ID); err != nil {
		t.Fatal(err)
	}

	got, err := f.issues.Get(t.Context(), "TRAX", 1)
	if err != nil {
		t.Fatal(err)
	}

	if len(got.StorageItems) != 2 || !got.StorageItems[0].IsDeleted() {
		t.Fatalf("after delete = %+v", got.StorageItems)
	}

	updated, err := f.issues.Update(t.Context(), testActor, "TRAX", 1, port.UpdateIssueInput{
		Title: new("Still broken"), StorageItems: &[]domain.StorageItemID{drill.ID},
	})
	if err != nil {
		t.Fatalf("keep deleted link: %v", err)
	}

	if names := storageItemNames(updated); !slices.Equal(names, []string{"drill"}) {
		t.Fatalf("updated storage items = %v", names)
	}

	wantChange(t, f.auditLog(t)[0], "storageItems", "Switch, drill", "drill")

	// Once unlinked, a deleted item cannot be linked again.
	_, err = f.issues.Update(t.Context(), testActor, "TRAX", 1, port.UpdateIssueInput{StorageItems: &[]domain.StorageItemID{}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = f.issues.Update(t.Context(), testActor, "TRAX", 1, port.UpdateIssueInput{StorageItems: &[]domain.StorageItemID{drill.ID}})
	wantField(t, err, "storageItems", domain.FieldUnknown)

	_, err = f.issues.Create(t.Context(), ada.ID, "TRAX", port.CreateIssueInput{Title: "x", StorageItems: []domain.StorageItemID{drill.ID}})
	wantField(t, err, "storageItems", domain.FieldUnknown)

	_, err = f.issues.Create(t.Context(), ada.ID, "TRAX", port.CreateIssueInput{
		Title: "x", StorageItems: []domain.StorageItemID{domain.StorageItemID(uuid.New())},
	})
	wantField(t, err, "storageItems", domain.FieldUnknown)
}

func TestIssueService_StorageItemLinksArchived(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "ada")
	f.addProject(t, "TRAX", "IssueTrax")
	drill := f.addItem(t, f.addStorage(t, nil, "Shelf"), f.addType(t, nil, port.CreateItemTypeInput{Name: "Tool"}), "drill", nil)
	f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "Old", Status: domain.StatusArchived})

	_, err := f.issues.Update(t.Context(), testActor, "TRAX", 1, port.UpdateIssueInput{
		Status: new(domain.StatusArchived), StorageItems: &[]domain.StorageItemID{drill.ID},
	})
	wantCode(t, err, domain.CodeIssueArchived)
}

func TestIssueService_ListByStorageItemFilter(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "ada")
	f.addProject(t, "TRAX", "IssueTrax")
	shelf := f.addStorage(t, nil, "Shelf")
	tool := f.addType(t, nil, port.CreateItemTypeInput{Name: "Tool"})
	drill := f.addItem(t, shelf, tool, "drill", nil)
	saw := f.addItem(t, shelf, tool, "saw", nil)

	f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "a", StorageItems: []domain.StorageItemID{drill.ID}})
	f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "b", StorageItems: []domain.StorageItemID{drill.ID, saw.ID}})
	f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "c"})

	list := func(filter port.IssueFilter) []port.IssueView {
		t.Helper()

		page, err := f.issues.List(t.Context(), "TRAX", filter, firstPage(t, 0))
		if err != nil {
			t.Fatal(err)
		}

		return page.Items
	}

	wantKeys(t, list(port.IssueFilter{StorageItemIDs: []domain.StorageItemID{drill.ID, saw.ID}}), "TRAX-2")
	wantKeys(t, list(port.IssueFilter{StorageItemIDs: []domain.StorageItemID{drill.ID, saw.ID}, StorageItemMode: port.MatchAny}),
		"TRAX-2", "TRAX-1")

	_, err := f.issues.List(t.Context(), "TRAX", port.IssueFilter{StorageItemMode: "some"}, firstPage(t, 0))
	wantField(t, err, "storageItemMode", domain.FieldInvalid)
}

func TestIssueService_ListByStorageItem(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "ada")
	f.addProject(t, "TRAX", "IssueTrax")
	f.addProject(t, "OPS", "Operations")
	drill := f.addItem(t, f.addStorage(t, nil, "Shelf"), f.addType(t, nil, port.CreateItemTypeInput{Name: "Tool"}), "drill", nil)
	links := []domain.StorageItemID{drill.ID}

	f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "a", StorageItems: links})
	f.addIssue(t, ada.ID, "OPS", port.CreateIssueInput{Title: "b", StorageItems: links, Status: domain.StatusArchived})
	f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "unrelated"})

	page, err := f.issues.ListByStorageItem(t.Context(), drill.ID, firstPage(t, 0))
	if err != nil {
		t.Fatal(err)
	}

	got := make([]string, 0, len(page.Items))
	for _, v := range page.Items {
		got = append(got, v.Issue.Key(v.ProjectKey))
	}

	slices.Sort(got)

	if !slices.Equal(got, []string{"OPS-1", "TRAX-1"}) || page.Total != 2 {
		t.Fatalf("ListByStorageItem() = %v (total %d)", got, page.Total)
	}

	if names := storageItemNames(page.Items[0]); !slices.Equal(names, []string{"drill"}) {
		t.Fatalf("view storage items = %v", names)
	}

	_, err = f.issues.ListByStorageItem(t.Context(), domain.StorageItemID(uuid.New()), firstPage(t, 0))
	wantCode(t, err, domain.CodeStorageItemNotFound)
}
