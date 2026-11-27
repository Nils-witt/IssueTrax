package domain

import (
	"time"

	"github.com/google/uuid"
)

// StorageItemID is a storage item's internal identifier. IDs are unique
// across all item kinds.
type StorageItemID uuid.UUID

func (id StorageItemID) String() string { return uuid.UUID(id).String() }

// StorageItemKind names the type of a storage item. Every kind shares the
// fields of StorageItem; kinds with extra fields add them on their own type.
type StorageItemKind string

// Storage item kinds.
const (
	// StorageItemKindGeneric is a plain item described only by name and
	// description.
	StorageItemKindGeneric StorageItemKind = "generic"
	// StorageItemKindNetworkDevice is a NetworkDevice.
	StorageItemKindNetworkDevice StorageItemKind = "network_device"
)

// StorageItem holds the fields every kind of item kept in a storage has.
// Items are global like storages. Deleting an item only marks it deleted
// (DeletedAt), so it can be restored; deleted items cannot be changed.
type StorageItem struct {
	ID        StorageItemID
	StorageID StorageID
	Kind      StorageItemKind
	Name      string
	// Description is plain text and may be empty.
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	// DeletedAt is nil unless the item is deleted.
	DeletedAt *time.Time
}

// IsDeleted reports whether the item is soft-deleted.
func (i StorageItem) IsDeleted() bool { return i.DeletedAt != nil }

// Delete marks the item deleted at now. It fails with
// ErrStorageItemDeleted when it already is.
func (i *StorageItem) Delete(now time.Time) error {
	if i.IsDeleted() {
		return ErrStorageItemDeleted
	}

	i.DeletedAt = &now
	i.UpdatedAt = now

	return nil
}

// Restore undoes Delete. It fails with ErrStorageItemNotDeleted when the
// item is not deleted.
func (i *StorageItem) Restore(now time.Time) error {
	if !i.IsDeleted() {
		return ErrStorageItemNotDeleted
	}

	i.DeletedAt = nil
	i.UpdatedAt = now

	return nil
}

// GenericStorageItem is a storage item of kind StorageItemKindGeneric.
type GenericStorageItem struct {
	StorageItem
}

// NewGenericStorageItemParams are the inputs for NewGenericStorageItem.
type NewGenericStorageItemParams struct {
	ID          StorageItemID
	StorageID   StorageID
	Name        string
	Description string
	Now         time.Time
}

// NewGenericStorageItem validates p and builds a GenericStorageItem. It
// does not check that the storage exists; that needs storage.
func NewGenericStorageItem(p NewGenericStorageItemParams) (GenericStorageItem, error) {
	f := fieldErrors{}

	item := GenericStorageItem{
		ID:          p.ID,
		StorageID:   p.StorageID,
		Kind:        StorageItemKindGeneric,
		Name:        checkRequiredText(f, "name", p.Name, maxProjectNameRunes),
		Description: checkDescription(f, "description", p.Description),
		CreatedAt:   p.Now,
		UpdatedAt:   p.Now,
	}

	if err := f.err(); err != nil {
		return GenericStorageItem{}, err
	}

	return item, nil
}

// GenericStorageItemChanges lists the mutable fields; nil means unchanged.
// StorageID moves the item to another storage.
type GenericStorageItemChanges struct {
	Name        *string
	Description *string
	StorageID   *StorageID
}

// Apply validates and applies c. On error i is left unchanged. A deleted
// item fails with ErrStorageItemDeleted.
func (i *GenericStorageItem) Apply(c GenericStorageItemChanges, now time.Time) error {
	if i.IsDeleted() {
		return ErrStorageItemDeleted
	}

	f := fieldErrors{}
	next := *i

	if c.Name != nil {
		next.Name = checkRequiredText(f, "name", *c.Name, maxProjectNameRunes)
	}

	if c.Description != nil {
		next.Description = checkDescription(f, "description", *c.Description)
	}

	if c.StorageID != nil {
		next.StorageID = *c.StorageID
	}

	if err := f.err(); err != nil {
		return err
	}

	if next.Name != i.Name || next.Description != i.Description || next.StorageID != i.StorageID {
		next.UpdatedAt = now
	}

	*i = next

	return nil
}
