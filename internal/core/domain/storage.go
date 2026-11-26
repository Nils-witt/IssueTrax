package domain

import (
	"time"

	"github.com/google/uuid"
)

// StorageID is a storage's internal identifier.
type StorageID uuid.UUID

func (id StorageID) String() string { return uuid.UUID(id).String() }

// Storage is a physical place where things are kept (a building, room,
// shelf or box). Storages form a tree through their optional parent; names
// are unique among siblings, ignoring case. Storages are global, not tied
// to a project, and are never deleted.
type Storage struct {
	ID StorageID
	// ParentID is nil for a root storage.
	ParentID  *StorageID
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// StorageListItem is a storage together with the number of its direct
// children, so a tree view knows which nodes can be expanded.
type StorageListItem struct {
	Storage

	ChildCount int64
}

// NewStorageParams are the inputs for NewStorage.
type NewStorageParams struct {
	ID       StorageID
	ParentID *StorageID
	Name     string
	Now      time.Time
}

// NewStorage validates p and builds a Storage. It does not check that the
// parent exists or that the name is free; that needs storage.
func NewStorage(p NewStorageParams) (Storage, error) {
	f := fieldErrors{}

	storage := Storage{
		ID:        p.ID,
		ParentID:  p.ParentID,
		Name:      checkRequiredText(f, "name", p.Name, maxProjectNameRunes),
		CreatedAt: p.Now,
		UpdatedAt: p.Now,
	}

	if err := f.err(); err != nil {
		return Storage{}, err
	}

	return storage, nil
}

// StorageParentChange moves a storage. A nil ID makes it a root.
type StorageParentChange struct {
	ID *StorageID
}

// StorageChanges lists the mutable storage fields; nil means unchanged.
type StorageChanges struct {
	Name   *string
	Parent *StorageParentChange
}

// Apply validates and applies c. On error s is left unchanged. It rejects
// making s its own parent; deeper cycles need storage to detect.
func (s *Storage) Apply(c StorageChanges, now time.Time) error {
	f := fieldErrors{}
	next := *s

	if c.Name != nil {
		next.Name = checkRequiredText(f, "name", *c.Name, maxProjectNameRunes)
	}

	if c.Parent != nil {
		if c.Parent.ID != nil && *c.Parent.ID == s.ID {
			f.add("parentId", FieldInvalid)
		}

		next.ParentID = c.Parent.ID
	}

	if err := f.err(); err != nil {
		return err
	}

	if next.Name != s.Name || !sameStorageID(next.ParentID, s.ParentID) {
		next.UpdatedAt = now
	}

	*s = next

	return nil
}

// ParentString returns the parent ID as a string, or "" for a root.
func (s Storage) ParentString() string {
	if s.ParentID == nil {
		return ""
	}

	return s.ParentID.String()
}

func sameStorageID(a, b *StorageID) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}
