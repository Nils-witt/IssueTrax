package domain

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Item type limits.
const (
	maxTagPrefixBytes = 20
	// MaxDefaultCount bounds how many instances of a module a type
	// pre-fills.
	MaxDefaultCount = 512
)

// tagPrefixPattern lists the characters of tag prefixes and asset tags.
var tagPrefixPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]*$`)

// ItemTypeID is an item type's internal identifier.
type ItemTypeID uuid.UUID

func (id ItemTypeID) String() string { return uuid.UUID(id).String() }

// ItemType classifies storage items. Types form a tree through their
// optional parent; names are unique among live siblings, ignoring case.
// A type names the modules new items of the type start with; children
// inherit their ancestors' defaults and the nearest asset tag prefix.
// Deleting a type only marks it deleted: its items and children keep it,
// but it can no longer be picked for items or as a parent.
type ItemType struct {
	ID ItemTypeID
	// ParentID is nil for a root type.
	ParentID *ItemTypeID
	Name     string
	// TagPrefix starts the asset tags proposed for items of the type
	// (separator included, e.g. "NET-"); empty inherits it.
	TagPrefix string
	// Defaults are the type's own default modules in display order,
	// without the inherited ones.
	Defaults  []ModuleDefault
	CreatedAt time.Time
	UpdatedAt time.Time
	// DeletedAt is nil unless the type is deleted.
	DeletedAt *time.Time
}

// ModuleDefault is a module new items of a type start with, Count times.
// Count is always 1 for single modules.
type ModuleDefault struct {
	Module ModuleKind
	Count  int
}

// ItemTypeListItem is a type together with the number of its direct
// children (live and deleted) and of the live items of exactly this type.
type ItemTypeListItem struct {
	ItemType

	ChildCount int64
	ItemCount  int64
}

// IsDeleted reports whether the type is soft-deleted.
func (t ItemType) IsDeleted() bool { return t.DeletedAt != nil }

// NewItemTypeParams are the inputs for NewItemType.
type NewItemTypeParams struct {
	ID        ItemTypeID
	ParentID  *ItemTypeID
	Name      string
	TagPrefix string
	Defaults  []ModuleDefault
	Now       time.Time
}

// NewItemType validates p and builds an ItemType. It does not check the
// parent or that the name is free; that needs storage.
func NewItemType(p NewItemTypeParams) (ItemType, error) {
	f := fieldErrors{}

	t := ItemType{
		ID:        p.ID,
		ParentID:  p.ParentID,
		Name:      checkRequiredText(f, "name", p.Name, maxProjectNameRunes),
		TagPrefix: checkTagPrefix(f, p.TagPrefix),
		Defaults:  checkDefaults(f, p.Defaults),
		CreatedAt: p.Now,
		UpdatedAt: p.Now,
	}

	if p.ParentID != nil && *p.ParentID == p.ID {
		f.add("parentId", FieldInvalid)
	}

	if err := f.err(); err != nil {
		return ItemType{}, err
	}

	return t, nil
}

// ItemTypeParentChange moves a type. A nil ID makes it a root.
type ItemTypeParentChange struct {
	ID *ItemTypeID
}

// ItemTypeChanges lists the mutable fields; nil means unchanged. Defaults
// replaces the type's own defaults.
type ItemTypeChanges struct {
	Name      *string
	Parent    *ItemTypeParentChange
	TagPrefix *string
	Defaults  *[]ModuleDefault
}

// Apply validates and applies c. On error t is left unchanged. A deleted
// type fails with ErrItemTypeDeleted. It rejects making t its own parent;
// deeper cycles need storage to detect.
func (t *ItemType) Apply(c ItemTypeChanges, now time.Time) error {
	if t.IsDeleted() {
		return ErrItemTypeDeleted
	}

	f := fieldErrors{}
	next := *t

	if c.Name != nil {
		next.Name = checkRequiredText(f, "name", *c.Name, maxProjectNameRunes)
	}

	if c.Parent != nil {
		next.ParentID = c.Parent.ID
		if next.ParentID != nil && *next.ParentID == t.ID {
			f.add("parentId", FieldInvalid)
		}
	}

	if c.TagPrefix != nil {
		next.TagPrefix = checkTagPrefix(f, *c.TagPrefix)
	}

	if c.Defaults != nil {
		next.Defaults = checkDefaults(f, *c.Defaults)
	}

	if err := f.err(); err != nil {
		return err
	}

	if !next.sameContent(*t) {
		next.UpdatedAt = now
	}

	*t = next

	return nil
}

func (t ItemType) sameContent(o ItemType) bool {
	return t.Name == o.Name && equalPtr(t.ParentID, o.ParentID, func(a, b ItemTypeID) bool { return a == b }) &&
		t.TagPrefix == o.TagPrefix && slices.Equal(t.Defaults, o.Defaults)
}

// Delete marks the type deleted at now. It fails with ErrItemTypeDeleted
// when it already is.
func (t *ItemType) Delete(now time.Time) error {
	if t.IsDeleted() {
		return ErrItemTypeDeleted
	}

	t.DeletedAt = &now
	t.UpdatedAt = now

	return nil
}

// Restore undoes Delete. It fails with ErrItemTypeNotDeleted when the type
// is not deleted.
func (t *ItemType) Restore(now time.Time) error {
	if !t.IsDeleted() {
		return ErrItemTypeNotDeleted
	}

	t.DeletedAt = nil
	t.UpdatedAt = now

	return nil
}

// EffectiveDefaults merges the defaults of chain, a type's ancestors from
// the root down to the type itself: the deepest entry for a module wins,
// and a single module is defaulted once however often it is listed. The
// result is in display order.
func EffectiveDefaults(chain []ItemType) []ModuleDefault {
	counts := map[ModuleKind]int{}

	for _, t := range chain {
		for _, d := range t.Defaults {
			counts[d.Module] = d.Count
		}
	}

	out := make([]ModuleDefault, 0, len(counts))
	for _, kind := range ModuleKinds {
		if n, ok := counts[kind]; ok {
			if !kind.Multi() {
				n = 1
			}

			out = append(out, ModuleDefault{Module: kind, Count: n})
		}
	}

	return out
}

// EffectiveTagPrefix returns the prefix of the deepest type in chain (root
// first) that has one, else fallback.
func EffectiveTagPrefix(chain []ItemType, fallback string) string {
	for _, t := range slices.Backward(chain) {
		if t.TagPrefix != "" {
			return t.TagPrefix
		}
	}

	return fallback
}

// ValidTagPrefix reports whether s can start asset tags.
func ValidTagPrefix(s string) bool {
	return len(s) <= maxTagPrefixBytes && tagPrefixPattern.MatchString(s)
}

func checkTagPrefix(f fieldErrors, s string) string {
	s = strings.TrimSpace(s)

	switch {
	case len(s) > maxTagPrefixBytes:
		f.add("tagPrefix", FieldTooLong)
	case !tagPrefixPattern.MatchString(s):
		f.add("tagPrefix", FieldInvalid)
	}

	return s
}

// checkDefaults validates defaults and returns them in display order.
func checkDefaults(f fieldErrors, defaults []ModuleDefault) []ModuleDefault {
	out := make([]ModuleDefault, 0, len(defaults))
	seen := map[ModuleKind]bool{}

	for i, d := range defaults {
		field := fmt.Sprintf("defaults[%d]", i)

		switch {
		case d.Module == "":
			f.add(field+".module", FieldRequired)
			continue
		case !d.Module.Valid() || seen[d.Module]:
			f.add(field+".module", FieldInvalid)
			continue
		}

		seen[d.Module] = true

		switch {
		case d.Count < 1 || d.Count > MaxDefaultCount:
			f.add(field+".count", FieldOutOfRange)
		case !d.Module.Multi() && d.Count != 1:
			f.add(field+".count", FieldInvalid)
		}

		out = append(out, d)
	}

	slices.SortFunc(out, func(a, b ModuleDefault) int { return cmp.Compare(a.Module.order(), b.Module.order()) })

	return out
}
