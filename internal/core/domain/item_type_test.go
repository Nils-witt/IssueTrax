package domain_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestNewItemType(t *testing.T) {
	t.Parallel()

	id := domain.ItemTypeID(uuid.New())

	tests := []struct {
		name       string
		params     domain.NewItemTypeParams
		want       []domain.ModuleDefault
		wantFields map[string]string
	}{
		{
			name: "sorts defaults",
			params: domain.NewItemTypeParams{Name: " Switch ", TagPrefix: " NET- ", Defaults: []domain.ModuleDefault{
				{Module: domain.ModuleNetworkInterface, Count: 24}, {Module: domain.ModuleHardware, Count: 1},
			}},
			want: []domain.ModuleDefault{{Module: domain.ModuleHardware, Count: 1}, {Module: domain.ModuleNetworkInterface, Count: 24}},
		},
		{name: "missing name", params: domain.NewItemTypeParams{Name: " "}, wantFields: map[string]string{"name": domain.FieldRequired}},
		{
			name:       "bad prefix",
			params:     domain.NewItemTypeParams{Name: "x", TagPrefix: "NET #"},
			wantFields: map[string]string{"tagPrefix": domain.FieldInvalid},
		},
		{
			name:       "long prefix",
			params:     domain.NewItemTypeParams{Name: "x", TagPrefix: strings.Repeat("A", 21)},
			wantFields: map[string]string{"tagPrefix": domain.FieldTooLong},
		},
		{
			name:       "own parent",
			params:     domain.NewItemTypeParams{ID: id, ParentID: &id, Name: "x"},
			wantFields: map[string]string{"parentId": domain.FieldInvalid},
		},
		{
			name: "bad defaults",
			params: domain.NewItemTypeParams{Name: "x", Defaults: []domain.ModuleDefault{
				{Module: ""},
				{Module: "nope", Count: 1},
				{Module: domain.ModuleHardware, Count: 2},
				{Module: domain.ModuleNetworkInterface, Count: 0},
				{Module: domain.ModulePurchase, Count: 1},
				{Module: domain.ModulePurchase, Count: 1},
				{Module: domain.ModuleNetworkHost, Count: domain.MaxDefaultCount + 1},
			}},
			wantFields: map[string]string{
				"defaults[0].module": domain.FieldRequired, "defaults[1].module": domain.FieldInvalid,
				"defaults[2].count": domain.FieldInvalid, "defaults[3].count": domain.FieldOutOfRange,
				"defaults[5].module": domain.FieldInvalid, "defaults[6].count": domain.FieldOutOfRange,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := tt.params
			if p.ID == (domain.ItemTypeID{}) {
				p.ID = id
			}

			p.Now = testNow

			got, err := domain.NewItemType(p)
			wantFields(t, err, tt.wantFields)

			if err != nil {
				return
			}

			if got.Name != "Switch" || got.TagPrefix != "NET-" || !slices.Equal(got.Defaults, tt.want) {
				t.Fatalf("NewItemType() = %+v", got)
			}
		})
	}
}

func TestItemTypeApply(t *testing.T) {
	t.Parallel()

	typ, err := domain.NewItemType(domain.NewItemTypeParams{ID: domain.ItemTypeID(uuid.New()), Name: "Switch", Now: testNow})
	if err != nil {
		t.Fatal(err)
	}

	later := testNow.Add(time.Hour)

	if err := typ.Apply(domain.ItemTypeChanges{Name: new("Switch")}, later); err != nil || !typ.UpdatedAt.Equal(testNow) {
		t.Fatalf("no-op Apply: err %v, updatedAt %v", err, typ.UpdatedAt)
	}

	parent := domain.ItemTypeID(uuid.New())
	defaults := []domain.ModuleDefault{{Module: domain.ModuleNetworkInterface, Count: 8}}

	err = typ.Apply(domain.ItemTypeChanges{
		Name: new("Router"), Parent: &domain.ItemTypeParentChange{ID: &parent}, TagPrefix: new("RT-"), Defaults: &defaults,
	}, later)
	if err != nil {
		t.Fatal(err)
	}

	if typ.Name != "Router" || *typ.ParentID != parent || typ.TagPrefix != "RT-" || len(typ.Defaults) != 1 || !typ.UpdatedAt.Equal(later) {
		t.Fatalf("Apply() = %+v", typ)
	}

	before := typ
	err = typ.Apply(domain.ItemTypeChanges{Name: new(""), Parent: &domain.ItemTypeParentChange{ID: &typ.ID}}, later)
	wantFields(t, err, map[string]string{"name": domain.FieldRequired, "parentId": domain.FieldInvalid})

	if typ.Name != before.Name {
		t.Fatal("failed Apply changed the type")
	}

	if err := typ.Apply(domain.ItemTypeChanges{Parent: &domain.ItemTypeParentChange{}}, later); err != nil || typ.ParentID != nil {
		t.Fatalf("make root: err %v, parent %v", err, typ.ParentID)
	}
}

func TestItemTypeDeleteRestore(t *testing.T) {
	t.Parallel()

	typ, err := domain.NewItemType(domain.NewItemTypeParams{ID: domain.ItemTypeID(uuid.New()), Name: "Switch", Now: testNow})
	if err != nil {
		t.Fatal(err)
	}

	later := testNow.Add(time.Hour)

	if err := typ.Delete(later); err != nil || !typ.IsDeleted() {
		t.Fatalf("Delete: %v", err)
	}

	if err := typ.Delete(later); !errors.Is(err, domain.ErrItemTypeDeleted) {
		t.Fatalf("Delete twice = %v", err)
	}

	if err := typ.Apply(domain.ItemTypeChanges{Name: new("x")}, later); !errors.Is(err, domain.ErrItemTypeDeleted) {
		t.Fatalf("Apply on deleted = %v", err)
	}

	if err := typ.Restore(later); err != nil || typ.IsDeleted() {
		t.Fatalf("Restore: %v", err)
	}

	if err := typ.Restore(later); !errors.Is(err, domain.ErrItemTypeNotDeleted) {
		t.Fatalf("Restore twice = %v", err)
	}
}

func TestEffectiveDefaultsAndPrefix(t *testing.T) {
	t.Parallel()

	root := domain.ItemType{TagPrefix: "DEV-", Defaults: []domain.ModuleDefault{
		{Module: domain.ModuleHardware, Count: 1}, {Module: domain.ModuleNetworkInterface, Count: 1},
	}}
	mid := domain.ItemType{Defaults: []domain.ModuleDefault{
		{Module: domain.ModuleHardware, Count: 1}, {Module: domain.ModulePurchase, Count: 1},
	}}
	leaf := domain.ItemType{Defaults: []domain.ModuleDefault{{Module: domain.ModuleNetworkInterface, Count: 24}}}

	got := domain.EffectiveDefaults([]domain.ItemType{root, mid, leaf})
	want := []domain.ModuleDefault{
		{Module: domain.ModuleHardware, Count: 1},
		{Module: domain.ModulePurchase, Count: 1},
		{Module: domain.ModuleNetworkInterface, Count: 24},
	}

	if !slices.Equal(got, want) {
		t.Fatalf("EffectiveDefaults() = %v, want %v", got, want)
	}

	if got := domain.EffectiveTagPrefix([]domain.ItemType{root, mid, leaf}, "INV-"); got != "DEV-" {
		t.Fatalf("inherited prefix = %q", got)
	}

	leaf.TagPrefix = "SW-"
	if got := domain.EffectiveTagPrefix([]domain.ItemType{root, mid, leaf}, "INV-"); got != "SW-" {
		t.Fatalf("own prefix = %q", got)
	}

	if got := domain.EffectiveTagPrefix([]domain.ItemType{mid}, "INV-"); got != "INV-" {
		t.Fatalf("fallback prefix = %q", got)
	}

	if got := domain.FormatAssetTag("NET-", 42); got != "NET-00042" {
		t.Fatalf("FormatAssetTag = %q", got)
	}

	if got := domain.FormatAssetTag("", 1234567); got != "1234567" {
		t.Fatalf("FormatAssetTag long = %q", got)
	}
}
