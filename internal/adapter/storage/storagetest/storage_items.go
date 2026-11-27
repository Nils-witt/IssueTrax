package storagetest

import (
	"testing"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/service"
)

// Item names several tests use.
const (
	drill = "Drill"
	bits  = "bits"
)

// itemType stores a root item type called name.
func itemType(t *testing.T, r service.Repositories, name string) domain.ItemType {
	t.Helper()

	typ := domain.ItemType{ID: domain.ItemTypeID(newID()), Name: name, Defaults: []domain.ModuleDefault{}, CreatedAt: now, UpdatedAt: now}
	must(t, r.ItemTypes.Create(t.Context(), typ))

	return typ
}

// storageItem stores a usable item of typ called name in st; its asset tag
// is the name.
func storageItem(t *testing.T, r service.Repositories, st domain.Storage, typ domain.ItemType, name string) domain.StorageItem {
	t.Helper()

	item := domain.StorageItem{
		ID: domain.StorageItemID(newID()), AssetTag: name, StorageID: st.ID, TypeID: typ.ID, Name: name, Usable: true,
		Modules: []domain.Module{}, CreatedAt: now, UpdatedAt: now,
	}
	must(t, r.StorageItems.Create(t.Context(), item))

	return item
}

func pageOf(t *testing.T, limit int, cursor string) domain.PageRequest {
	t.Helper()

	p, err := domain.NewPageRequest(limit, cursor)
	must(t, err)

	return p
}
