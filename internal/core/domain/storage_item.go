package domain

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// StorageItemID is a storage item's internal identifier.
type StorageItemID uuid.UUID

func (id StorageItemID) String() string { return uuid.UUID(id).String() }

// StorageItem is a thing kept in a storage. Every item has a type; its
// other fields beyond the common ones come from modules, which can be
// added and removed per item. Items are global like storages. Deleting an
// item only marks it deleted (DeletedAt), so it can be restored; deleted
// items cannot be changed.
type StorageItem struct {
	ID        StorageItemID
	StorageID StorageID
	Name      string
	// Description is plain text and may be empty.
	Description string
	// AssetTag is the item's user-visible inventory number, unique across
	// all items (deleted ones included) ignoring case. It never changes.
	AssetTag string
	TypeID   ItemTypeID
	// Usable is set by people; no other field changes it.
	Usable bool
	// OrderDate, CommissionedAt and DecommissionedAt are nil when unknown;
	// those that are set are in this order.
	OrderDate        *time.Time
	CommissionedAt   *time.Time
	DecommissionedAt *time.Time
	// DecommissioningReason is plain text and may be empty, also while
	// DecommissionedAt is nil.
	DecommissioningReason string
	// ResponsibleID is the person (never a service account) responsible
	// for the item; nil for nobody.
	ResponsibleID *UserID
	// Modules are the item's module instances in display order
	// (SortModules).
	Modules   []Module
	CreatedAt time.Time
	UpdatedAt time.Time
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

// Storage item limits.
const (
	maxAssetTagBytes              = 50
	maxDecommissioningReasonRunes = 1000
	// AssetTagDigits is how many digits the number of a proposed asset
	// tag has at least.
	AssetTagDigits = 5
)

// FormatAssetTag builds the proposed asset tag number n with prefix, e.g.
// "NET-00042".
func FormatAssetTag(prefix string, n int64) string {
	return fmt.Sprintf("%s%0*d", prefix, AssetTagDigits, n)
}

// NewStorageItemParams are the inputs for NewStorageItem. Usable is
// required. NewModuleID makes the IDs of the module instances.
type NewStorageItemParams struct {
	ID                    StorageItemID
	AssetTag              string
	Name                  string
	Description           string
	StorageID             StorageID
	TypeID                ItemTypeID
	Usable                *bool
	OrderDate             *time.Time
	CommissionedAt        *time.Time
	DecommissionedAt      *time.Time
	DecommissioningReason string
	ResponsibleID         *UserID
	Modules               []ModuleParams
	NewModuleID           func() ModuleID
	Now                   time.Time
}

// NewStorageItem validates p and builds a StorageItem. It does not check
// that the storage, type and responsible user exist or that the asset tag
// is free; that needs storage. Field errors use the API field names, e.g.
// "modules[1].networkInterface.ipv4.gateway".
func NewStorageItem(p NewStorageItemParams) (StorageItem, error) {
	f := fieldErrors{}

	item := StorageItem{
		ID:                    p.ID,
		StorageID:             p.StorageID,
		Name:                  checkRequiredText(f, "name", p.Name, maxProjectNameRunes),
		Description:           checkDescription(f, "description", p.Description),
		AssetTag:              checkAssetTag(f, p.AssetTag),
		TypeID:                p.TypeID,
		OrderDate:             p.OrderDate,
		CommissionedAt:        p.CommissionedAt,
		DecommissionedAt:      p.DecommissionedAt,
		DecommissioningReason: checkOptionalText(f, "decommissioningReason", p.DecommissioningReason, maxDecommissioningReasonRunes),
		ResponsibleID:         p.ResponsibleID,
		Modules:               checkModules(f, p.Modules, nil, p.NewModuleID),
		CreatedAt:             p.Now,
		UpdatedAt:             p.Now,
	}

	if p.Usable == nil {
		f.add("usable", FieldRequired)
	} else {
		item.Usable = *p.Usable
	}

	item.checkDates(f)

	if err := f.err(); err != nil {
		return StorageItem{}, err
	}

	return item, nil
}

// StorageItemDateChange sets an optional date; a nil At clears it.
type StorageItemDateChange struct {
	At *time.Time
}

// StorageItemResponsibleChange sets the responsible person; a nil ID
// clears it.
type StorageItemResponsibleChange struct {
	ID *UserID
}

// StorageItemChanges lists the mutable fields; nil means unchanged. The
// asset tag cannot change. StorageID moves the item; Modules replaces all
// module instances (instances keep their ID when it is passed back).
type StorageItemChanges struct {
	Name                  *string
	Description           *string
	StorageID             *StorageID
	TypeID                *ItemTypeID
	Usable                *bool
	OrderDate             *StorageItemDateChange
	CommissionedAt        *StorageItemDateChange
	DecommissionedAt      *StorageItemDateChange
	DecommissioningReason *string
	Responsible           *StorageItemResponsibleChange
	Modules               *[]ModuleParams
	NewModuleID           func() ModuleID
}

// Apply validates and applies c. On error i is left unchanged. A deleted
// item fails with ErrStorageItemDeleted.
func (i *StorageItem) Apply(c StorageItemChanges, now time.Time) error {
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

	if c.DecommissioningReason != nil {
		next.DecommissioningReason = checkOptionalText(
			f, "decommissioningReason", *c.DecommissioningReason, maxDecommissioningReasonRunes)
	}

	next.applyRefs(c)

	if c.Modules != nil {
		next.Modules = checkModules(f, *c.Modules, i.Modules, c.NewModuleID)
	}

	next.checkDates(f)

	if err := f.err(); err != nil {
		return err
	}

	if !next.SameContent(*i) {
		next.UpdatedAt = now
	}

	*i = next

	return nil
}

// applyRefs applies the fields of c that need no validation here.
func (i *StorageItem) applyRefs(c StorageItemChanges) {
	if c.StorageID != nil {
		i.StorageID = *c.StorageID
	}

	if c.TypeID != nil {
		i.TypeID = *c.TypeID
	}

	if c.Usable != nil {
		i.Usable = *c.Usable
	}

	if c.OrderDate != nil {
		i.OrderDate = c.OrderDate.At
	}

	if c.CommissionedAt != nil {
		i.CommissionedAt = c.CommissionedAt.At
	}

	if c.DecommissionedAt != nil {
		i.DecommissionedAt = c.DecommissionedAt.At
	}

	if c.Responsible != nil {
		i.ResponsibleID = c.Responsible.ID
	}
}

// SameContent reports whether i and o have the same mutable fields.
func (i StorageItem) SameContent(o StorageItem) bool {
	eqTime := func(a, b *time.Time) bool { return equalPtr(a, b, time.Time.Equal) }

	return i.Name == o.Name && i.Description == o.Description && i.StorageID == o.StorageID &&
		i.TypeID == o.TypeID && i.Usable == o.Usable && eqTime(i.OrderDate, o.OrderDate) &&
		eqTime(i.CommissionedAt, o.CommissionedAt) && eqTime(i.DecommissionedAt, o.DecommissionedAt) &&
		i.DecommissioningReason == o.DecommissioningReason &&
		equalPtr(i.ResponsibleID, o.ResponsibleID, func(a, b UserID) bool { return a == b }) &&
		slices.EqualFunc(i.Modules, o.Modules, Module.Equal)
}

// checkDates records out_of_order on the later of two set dates that are
// in the wrong order: order date, commissioning, decommissioning.
func (i StorageItem) checkDates(f fieldErrors) {
	before := func(later, earlier *time.Time) bool { return later != nil && earlier != nil && later.Before(*earlier) }

	if before(i.CommissionedAt, i.OrderDate) {
		f.add("commissionedAt", FieldOutOfOrder)
	}

	if before(i.DecommissionedAt, i.CommissionedAt) || before(i.DecommissionedAt, i.OrderDate) {
		f.add("decommissionedAt", FieldOutOfOrder)
	}
}

func checkAssetTag(f fieldErrors, s string) string {
	s = strings.TrimSpace(s)

	switch {
	case s == "":
		f.add("assetTag", FieldRequired)
	case len(s) > maxAssetTagBytes:
		f.add("assetTag", FieldTooLong)
	case !tagPrefixPattern.MatchString(s):
		f.add("assetTag", FieldInvalid)
	}

	return s
}
