package service_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
	"nilswitt.dev/issuetrax/internal/core/service"
)

func (f *fixture) addStorage(t *testing.T, parent *domain.StorageListItem, name string) domain.StorageListItem {
	t.Helper()

	in := port.CreateStorageInput{Name: name}
	if parent != nil {
		in.ParentID = &parent.ID
	}

	s, err := f.storages.Create(t.Context(), testActor, in)
	if err != nil {
		t.Fatalf("create storage %s: %v", name, err)
	}

	return s
}

func moveTo(parent *domain.StorageListItem) *domain.StorageParentChange {
	if parent == nil {
		return &domain.StorageParentChange{}
	}

	return &domain.StorageParentChange{ID: &parent.ID}
}

func TestStorageService_Create(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	root := f.addStorage(t, nil, "Warehouse")
	f.changes.take()

	shelf, err := f.storages.Create(t.Context(), testActor, port.CreateStorageInput{Name: " Shelf ", ParentID: &root.ID})
	if err != nil {
		t.Fatal(err)
	}

	if shelf.Name != "Shelf" || shelf.ParentID == nil || *shelf.ParentID != root.ID || !shelf.CreatedAt.Equal(testNow) {
		t.Fatalf("Create() = %+v", shelf)
	}

	if got := f.changes.take(); !slices.Equal(got, []domain.Change{{Kind: domain.ChangeStorages}}) {
		t.Fatalf("changes = %v", got)
	}

	entries := f.auditLog(t)
	wantActions(t, entries, domain.AuditStorageCreate, domain.AuditStorageCreate)

	e := entries[0]
	if e.TargetType != domain.AuditTargetStorage || e.TargetID != shelf.ID.String() || e.TargetLabel != "Shelf" {
		t.Fatalf("audit target = %s %s %s", e.TargetType, e.TargetID, e.TargetLabel)
	}

	wantChange(t, e, "name", "", "Shelf")
	wantChange(t, e, "parent", "", root.ID.String())

	missing := domain.StorageID(uuid.New())

	tests := []struct {
		name      string
		in        port.CreateStorageInput
		wantCode  string
		wantField string
	}{
		{name: "duplicate root ignoring case", in: port.CreateStorageInput{Name: "WAREHOUSE"}, wantCode: domain.CodeStorageNameTaken},
		{name: "duplicate sibling", in: port.CreateStorageInput{Name: "shelf", ParentID: &root.ID}, wantCode: domain.CodeStorageNameTaken},
		{name: "empty name", in: port.CreateStorageInput{Name: " "}, wantField: "name"},
		{name: "unknown parent", in: port.CreateStorageInput{Name: "Box", ParentID: &missing}, wantField: "parentId"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := f.storages.Create(t.Context(), testActor, tt.in)
			if tt.wantField != "" {
				wantField(t, err, tt.wantField, map[string]string{"name": domain.FieldRequired, "parentId": domain.FieldInvalid}[tt.wantField])
				return
			}

			wantCode(t, err, tt.wantCode)
		})
	}
}

func TestStorageService_GetAndList(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	b := f.addStorage(t, nil, "b")
	a := f.addStorage(t, nil, "A")
	f.addStorage(t, &b, "child")

	got, err := f.storages.Get(t.Context(), b.ID)
	if err != nil || got.ChildCount != 1 || got.Name != "b" {
		t.Fatalf("Get() = %+v, %v", got, err)
	}

	_, err = f.storages.Get(t.Context(), domain.StorageID(uuid.New()))
	wantCode(t, err, domain.CodeStorageNotFound)

	roots, err := f.storages.List(t.Context(), nil, firstPage(t, 0))
	if err != nil {
		t.Fatal(err)
	}

	if len(roots.Items) != 2 || roots.Items[0].ID != a.ID || roots.Items[1].ID != b.ID || roots.Total != 2 {
		t.Fatalf("roots = %+v", roots)
	}

	children, err := f.storages.List(t.Context(), &b.ID, firstPage(t, 0))
	if err != nil || len(children.Items) != 1 || children.Items[0].Name != "child" {
		t.Fatalf("children = %+v, %v", children, err)
	}

	missing := domain.StorageID(uuid.New())
	_, err = f.storages.List(t.Context(), &missing, firstPage(t, 0))
	wantCode(t, err, domain.CodeStorageNotFound)
}

func TestStorageService_UpdateRenamesAndMoves(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	a := f.addStorage(t, nil, "A")
	b := f.addStorage(t, nil, "B")
	box := f.addStorage(t, &a, "Box")
	f.addStorage(t, &box, "Inner")
	f.changes.take()

	got, err := f.storages.Update(t.Context(), testActor, box.ID, port.UpdateStorageInput{Name: new("Crate"), Parent: moveTo(&b)})
	if err != nil {
		t.Fatal(err)
	}

	if got.Name != "Crate" || got.ParentID == nil || *got.ParentID != b.ID || got.ChildCount != 1 {
		t.Fatalf("Update() = %+v", got)
	}

	if got := f.changes.take(); len(got) != 1 || got[0].Kind != domain.ChangeStorages {
		t.Fatalf("changes = %v", got)
	}

	e := f.auditLog(t)[0]
	if e.Action != domain.AuditStorageUpdate || e.TargetLabel != "Crate" {
		t.Fatalf("audit entry = %+v", e)
	}

	wantChange(t, e, "name", "Box", "Crate")
	wantChange(t, e, "parent", a.ID.String(), b.ID.String())

	got, err = f.storages.Update(t.Context(), testActor, box.ID, port.UpdateStorageInput{Parent: moveTo(nil)})
	if err != nil || got.ParentID != nil {
		t.Fatalf("move to root = %+v, %v", got, err)
	}

	wantChange(t, f.auditLog(t)[0], "parent", b.ID.String(), "")
}

func TestStorageService_UpdateWithoutChange(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	a := f.addStorage(t, nil, "A")
	box := f.addStorage(t, &a, "Box")
	f.changes.take()

	got, err := f.storages.Update(t.Context(), testActor, box.ID, port.UpdateStorageInput{Name: new("Box"), Parent: moveTo(&a)})
	if err != nil || !got.UpdatedAt.Equal(testNow) {
		t.Fatalf("Update() = %+v, %v", got, err)
	}

	if got := f.changes.take(); len(got) != 0 {
		t.Fatalf("changes = %v, want none", got)
	}

	wantActions(t, f.auditLog(t), domain.AuditStorageCreate, domain.AuditStorageCreate)
}

func TestStorageService_UpdateRejects(t *testing.T) {
	t.Parallel()

	missing := domain.StorageID(uuid.New())

	// Each case gets its own tree Root > Mid > Leaf, plus Root > Taken.
	type tree struct{ root, mid, leaf domain.StorageListItem }

	tests := []struct {
		name string
		in   func(tr tree) (domain.StorageID, port.UpdateStorageInput)
		// want is a code, or "field:<name>" for a validation error.
		want string
	}{
		{
			name: "into own child", want: domain.CodeStorageCycle,
			in: func(tr tree) (domain.StorageID, port.UpdateStorageInput) {
				return tr.root.ID, port.UpdateStorageInput{Parent: moveTo(&tr.mid)}
			},
		},
		{
			name: "into own grandchild", want: domain.CodeStorageCycle,
			in: func(tr tree) (domain.StorageID, port.UpdateStorageInput) {
				return tr.root.ID, port.UpdateStorageInput{Parent: moveTo(&tr.leaf)}
			},
		},
		{
			name: "under itself", want: "field:parentId",
			in: func(tr tree) (domain.StorageID, port.UpdateStorageInput) {
				return tr.mid.ID, port.UpdateStorageInput{Parent: moveTo(&tr.mid)}
			},
		},
		{
			name: "unknown parent", want: "field:parentId",
			in: func(tr tree) (domain.StorageID, port.UpdateStorageInput) {
				return tr.mid.ID, port.UpdateStorageInput{Parent: &domain.StorageParentChange{ID: &missing}}
			},
		},
		{
			name: "sibling name", want: domain.CodeStorageNameTaken,
			in: func(tr tree) (domain.StorageID, port.UpdateStorageInput) {
				return tr.mid.ID, port.UpdateStorageInput{Name: new("taken")}
			},
		},
		{
			name: "move onto sibling name", want: domain.CodeStorageNameTaken,
			in: func(tr tree) (domain.StorageID, port.UpdateStorageInput) {
				return tr.leaf.ID, port.UpdateStorageInput{Name: new("Mid"), Parent: moveTo(&tr.root)}
			},
		},
		{
			name: "empty name", want: "field:name",
			in: func(tr tree) (domain.StorageID, port.UpdateStorageInput) {
				return tr.mid.ID, port.UpdateStorageInput{Name: new("")}
			},
		},
		{
			name: "unknown storage", want: domain.CodeStorageNotFound,
			in: func(tree) (domain.StorageID, port.UpdateStorageInput) {
				return missing, port.UpdateStorageInput{Name: new("x")}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)

			var tr tree

			tr.root = f.addStorage(t, nil, "Root")
			tr.mid = f.addStorage(t, &tr.root, "Mid")
			tr.leaf = f.addStorage(t, &tr.mid, "Leaf")
			f.addStorage(t, &tr.root, "Taken")

			id, in := tt.in(tr)
			_, err := f.storages.Update(t.Context(), testActor, id, in)

			switch tt.want {
			case "field:parentId":
				wantField(t, err, "parentId", domain.FieldInvalid)
			case "field:name":
				wantField(t, err, "name", domain.FieldRequired)
			default:
				wantCode(t, err, tt.want)
			}

			got, err := f.storages.Get(t.Context(), tr.root.ID)
			if err != nil || got.ParentID != nil || got.ChildCount != 2 {
				t.Fatalf("root after rejected update = %+v, %v", got, err)
			}
		})
	}
}

func TestStorageService_UpdateRollsBackOnAuditFailure(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	a := f.addStorage(t, nil, "A")

	repos := f.repos
	repos.Audit = failingAudit{repos.Audit}

	name := "B"
	_, err := service.NewStorageService(repos).Update(t.Context(), testActor, a.ID, port.UpdateStorageInput{Name: &name})

	if !errors.Is(err, errAuditFailed) {
		t.Fatalf("error = %v, want %v", err, errAuditFailed)
	}

	got, err := f.storages.Get(t.Context(), a.ID)
	if err != nil || got.Name != "A" {
		t.Fatalf("after rollback = %+v, %v", got, err)
	}
}

var errAuditFailed = errors.New("audit failed")

type failingAudit struct{ port.AuditRepository }

func (failingAudit) Append(context.Context, domain.AuditEntry) error { return errAuditFailed }
