package domain

import (
	"time"

	"github.com/google/uuid"
)

// ManufacturerID is a manufacturer's internal identifier.
type ManufacturerID uuid.UUID

func (id ManufacturerID) String() string { return uuid.UUID(id).String() }

// HardwareModelID is a hardware model's internal identifier.
type HardwareModelID uuid.UUID

func (id HardwareModelID) String() string { return uuid.UUID(id).String() }

// Manufacturer is an entry of the central manufacturer list that hardware
// modules pick from. Names are unique among live manufacturers, ignoring
// case. Deleting a manufacturer only marks it deleted: items keep it, but
// it can no longer be picked or get new models.
type Manufacturer struct {
	ID        ManufacturerID
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
	// DeletedAt is nil unless the manufacturer is deleted.
	DeletedAt *time.Time
}

// ManufacturerListItem is a manufacturer together with the number of its
// live models and of the live items whose hardware names it.
type ManufacturerListItem struct {
	Manufacturer

	ModelCount int64
	ItemCount  int64
}

// IsDeleted reports whether the manufacturer is soft-deleted.
func (m Manufacturer) IsDeleted() bool { return m.DeletedAt != nil }

// HardwareModel is a product model of one manufacturer. Names are unique
// among the live models of a manufacturer, ignoring case. A model never
// changes its manufacturer. Deleting a model only marks it deleted: items
// keep it, but it can no longer be picked.
type HardwareModel struct {
	ID             HardwareModelID
	ManufacturerID ManufacturerID
	Name           string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	// DeletedAt is nil unless the model is deleted.
	DeletedAt *time.Time
}

// HardwareModelListItem is a model together with the number of live items
// whose hardware names it.
type HardwareModelListItem struct {
	HardwareModel

	ItemCount int64
}

// IsDeleted reports whether the model is soft-deleted.
func (m HardwareModel) IsDeleted() bool { return m.DeletedAt != nil }

// NewManufacturer validates name and builds a Manufacturer. It does not
// check that the name is free; that needs storage.
func NewManufacturer(id ManufacturerID, name string, now time.Time) (Manufacturer, error) {
	f := fieldErrors{}
	m := Manufacturer{ID: id, Name: checkRequiredText(f, "name", name, maxShortTextRunes), CreatedAt: now, UpdatedAt: now}

	if err := f.err(); err != nil {
		return Manufacturer{}, err
	}

	return m, nil
}

// Rename validates and sets the name. On error m is left unchanged. A
// deleted manufacturer fails with ErrManufacturerDeleted.
func (m *Manufacturer) Rename(name string, now time.Time) error {
	if m.IsDeleted() {
		return ErrManufacturerDeleted
	}

	f := fieldErrors{}

	name = checkRequiredText(f, "name", name, maxShortTextRunes)
	if err := f.err(); err != nil {
		return err
	}

	if name != m.Name {
		m.Name, m.UpdatedAt = name, now
	}

	return nil
}

// Delete marks the manufacturer deleted at now. It fails with
// ErrManufacturerDeleted when it already is.
func (m *Manufacturer) Delete(now time.Time) error {
	if m.IsDeleted() {
		return ErrManufacturerDeleted
	}

	m.DeletedAt, m.UpdatedAt = &now, now

	return nil
}

// Restore undoes Delete. It fails with ErrManufacturerNotDeleted when the
// manufacturer is not deleted.
func (m *Manufacturer) Restore(now time.Time) error {
	if !m.IsDeleted() {
		return ErrManufacturerNotDeleted
	}

	m.DeletedAt, m.UpdatedAt = nil, now

	return nil
}

// NewHardwareModel validates name and builds a HardwareModel of
// manufacturer. It does not check the manufacturer or that the name is
// free; that needs storage.
func NewHardwareModel(id HardwareModelID, manufacturer ManufacturerID, name string, now time.Time) (HardwareModel, error) {
	f := fieldErrors{}
	m := HardwareModel{
		ID: id, ManufacturerID: manufacturer, Name: checkRequiredText(f, "name", name, maxShortTextRunes),
		CreatedAt: now, UpdatedAt: now,
	}

	if err := f.err(); err != nil {
		return HardwareModel{}, err
	}

	return m, nil
}

// Rename validates and sets the name. On error m is left unchanged. A
// deleted model fails with ErrHardwareModelDeleted.
func (m *HardwareModel) Rename(name string, now time.Time) error {
	if m.IsDeleted() {
		return ErrHardwareModelDeleted
	}

	f := fieldErrors{}

	name = checkRequiredText(f, "name", name, maxShortTextRunes)
	if err := f.err(); err != nil {
		return err
	}

	if name != m.Name {
		m.Name, m.UpdatedAt = name, now
	}

	return nil
}

// Delete marks the model deleted at now. It fails with
// ErrHardwareModelDeleted when it already is.
func (m *HardwareModel) Delete(now time.Time) error {
	if m.IsDeleted() {
		return ErrHardwareModelDeleted
	}

	m.DeletedAt, m.UpdatedAt = &now, now

	return nil
}

// Restore undoes Delete. It fails with ErrHardwareModelNotDeleted when the
// model is not deleted.
func (m *HardwareModel) Restore(now time.Time) error {
	if !m.IsDeleted() {
		return ErrHardwareModelNotDeleted
	}

	m.DeletedAt, m.UpdatedAt = nil, now

	return nil
}
