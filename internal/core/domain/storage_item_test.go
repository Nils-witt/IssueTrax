package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestStorageItemDeleteAndRestore(t *testing.T) {
	t.Parallel()

	later := testNow.Add(time.Hour)

	item, err := domain.NewStorageItem(validItem())
	if err != nil {
		t.Fatal(err)
	}

	if err := item.Restore(later); !errors.Is(err, domain.ErrStorageItemNotDeleted) {
		t.Fatalf("Restore() of live item = %v", err)
	}

	if err := item.Delete(later); err != nil || item.DeletedAt == nil || !item.DeletedAt.Equal(later) || !item.UpdatedAt.Equal(later) {
		t.Fatalf("Delete() = %v, item %+v", err, item)
	}

	if err := item.Delete(later); !errors.Is(err, domain.ErrStorageItemDeleted) {
		t.Fatalf("second Delete() = %v", err)
	}

	if err := item.Restore(later.Add(time.Hour)); err != nil || item.IsDeleted() || !item.UpdatedAt.Equal(later.Add(time.Hour)) {
		t.Fatalf("Restore() = %v, item %+v", err, item)
	}
}

func validItem() domain.NewStorageItemParams {
	return domain.NewStorageItemParams{
		ID: domain.StorageItemID(uuid.New()), AssetTag: "INV-00001", Name: "Switch",
		StorageID: domain.StorageID(uuid.New()), TypeID: domain.ItemTypeID(uuid.New()), Usable: new(true),
		NewModuleID: newModuleID, Now: testNow,
	}
}

func TestNewStorageItem(t *testing.T) {
	t.Parallel()

	day := func(n int) *time.Time { return new(testNow.AddDate(0, 0, n)) }

	tests := []struct {
		name       string
		edit       func(*domain.NewStorageItemParams)
		wantFields map[string]string
	}{
		{name: "valid"},
		{name: "all dates in order", edit: func(p *domain.NewStorageItemParams) {
			p.OrderDate, p.CommissionedAt, p.DecommissionedAt = day(0), day(1), day(2)
		}},
		{name: "equal dates", edit: func(p *domain.NewStorageItemParams) {
			p.OrderDate, p.CommissionedAt, p.DecommissionedAt = day(0), day(0), day(0)
		}},
		{name: "reason without date", edit: func(p *domain.NewStorageItemParams) { p.DecommissioningReason = "broken" }},
		{
			name: "missing required",
			edit: func(p *domain.NewStorageItemParams) { p.Name, p.AssetTag, p.Usable = "", " ", nil },
			wantFields: map[string]string{
				"name": domain.FieldRequired, "assetTag": domain.FieldRequired, "usable": domain.FieldRequired,
			},
		},
		{
			name:       "bad asset tag",
			edit:       func(p *domain.NewStorageItemParams) { p.AssetTag = "INV 1" },
			wantFields: map[string]string{"assetTag": domain.FieldInvalid},
		},
		{
			name:       "long asset tag",
			edit:       func(p *domain.NewStorageItemParams) { p.AssetTag = strings.Repeat("A", 51) },
			wantFields: map[string]string{"assetTag": domain.FieldTooLong},
		},
		{
			name:       "commissioned before order",
			edit:       func(p *domain.NewStorageItemParams) { p.OrderDate, p.CommissionedAt = day(1), day(0) },
			wantFields: map[string]string{"commissionedAt": domain.FieldOutOfOrder},
		},
		{
			name:       "decommissioned before commissioned",
			edit:       func(p *domain.NewStorageItemParams) { p.CommissionedAt, p.DecommissionedAt = day(1), day(0) },
			wantFields: map[string]string{"decommissionedAt": domain.FieldOutOfOrder},
		},
		{
			name:       "decommissioned before order",
			edit:       func(p *domain.NewStorageItemParams) { p.OrderDate, p.DecommissionedAt = day(1), day(0) },
			wantFields: map[string]string{"decommissionedAt": domain.FieldOutOfOrder},
		},
		{
			name:       "reason too long",
			edit:       func(p *domain.NewStorageItemParams) { p.DecommissioningReason = strings.Repeat("a", 1001) },
			wantFields: map[string]string{"decommissioningReason": domain.FieldTooLong},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := validItem()
			if tt.edit != nil {
				tt.edit(&p)
			}

			item, err := domain.NewStorageItem(p)
			wantFields(t, err, tt.wantFields)

			if err == nil && (item.Name != "Switch" || !item.Usable || item.AssetTag != "INV-00001") {
				t.Fatalf("NewStorageItem() = %+v", item)
			}
		})
	}
}

func TestStorageItemApply(t *testing.T) {
	t.Parallel()

	p := validItem()
	p.Modules = []domain.ModuleParams{
		{Kind: domain.ModuleHardware, Hardware: &domain.HardwareParams{Model: "X1"}},
		{Kind: domain.ModuleNetworkInterface, NetworkInterface: iface("eth0")},
	}

	item, err := domain.NewStorageItem(p)
	if err != nil {
		t.Fatal(err)
	}

	later := testNow.Add(time.Hour)

	hw, eth := item.Modules[0], item.Modules[1]
	user := domain.UserID(uuid.New())
	typ := domain.ItemTypeID(uuid.New())

	err = item.Apply(domain.StorageItemChanges{
		Usable: new(false), TypeID: &typ, Responsible: &domain.StorageItemResponsibleChange{ID: &user},
		DecommissionedAt: &domain.StorageItemDateChange{At: &later}, DecommissioningReason: new(" broken "),
		Modules: &[]domain.ModuleParams{
			{ID: &eth.ID, Kind: domain.ModuleNetworkInterface, Primary: true, NetworkInterface: iface("eth1")},
			{Kind: domain.ModuleNetworkInterface, NetworkInterface: iface("eth0")},
		},
		NewModuleID: newModuleID,
	}, later)
	if err != nil {
		t.Fatal(err)
	}

	if item.Usable || item.TypeID != typ || *item.ResponsibleID != user || item.DecommissioningReason != "broken" ||
		!item.UpdatedAt.Equal(later) || len(item.Modules) != 2 {
		t.Fatalf("Apply() = %+v", item)
	}

	wantModuleIDs(t, item.Modules, eth, hw)
}

// wantModuleIDs checks that modules[0] is kept (eth renamed to eth1 and
// made primary) and modules[1] is new.
func wantModuleIDs(t *testing.T, modules []domain.Module, kept, removed domain.Module) {
	t.Helper()

	if got := modules[0]; got.ID != kept.ID || !got.Primary || got.NetworkInterface.Name != "eth1" {
		t.Fatalf("kept module = %+v", got)
	}

	if got := modules[1]; got.ID == kept.ID || got.ID == removed.ID {
		t.Fatalf("new module kept an old ID: %+v", got)
	}
}

func TestStorageItemApplyErrors(t *testing.T) {
	t.Parallel()

	p := validItem()
	p.Modules = []domain.ModuleParams{{Kind: domain.ModuleNetworkInterface, NetworkInterface: iface("eth0")}}
	p.DecommissionedAt = new(testNow.Add(time.Hour))

	item, err := domain.NewStorageItem(p)
	if err != nil {
		t.Fatal(err)
	}

	later := testNow.Add(time.Hour)
	eth := item.Modules[0]
	hw := domain.Module{ID: newModuleID()}
	before := item

	if err := item.Apply(domain.StorageItemChanges{Name: new("Switch"), Usable: new(true)}, later); err != nil || !item.UpdatedAt.Equal(testNow) {
		t.Fatalf("no-op Apply: err %v, updatedAt %v", err, item.UpdatedAt)
	}

	err = item.Apply(domain.StorageItemChanges{
		Modules: &[]domain.ModuleParams{{ID: &hw.ID, Kind: domain.ModuleHardware, Hardware: &domain.HardwareParams{}}},
	}, later)
	wantFields(t, err, map[string]string{"modules[0].id": domain.FieldInvalid})

	err = item.Apply(domain.StorageItemChanges{
		Modules: &[]domain.ModuleParams{{ID: &eth.ID, Kind: domain.ModuleHardware, Hardware: &domain.HardwareParams{}}},
	}, later)
	wantFields(t, err, map[string]string{"modules[0].id": domain.FieldInvalid})

	err = item.Apply(domain.StorageItemChanges{CommissionedAt: &domain.StorageItemDateChange{At: new(later.Add(time.Hour))}}, later)
	wantFields(t, err, map[string]string{"decommissionedAt": domain.FieldOutOfOrder})

	if !item.SameContent(before) {
		t.Fatal("failed Apply changed the item")
	}

	if err := item.Apply(domain.StorageItemChanges{DecommissionedAt: &domain.StorageItemDateChange{}}, later); err != nil || item.DecommissionedAt != nil {
		t.Fatalf("clear date: err %v, %v", err, item.DecommissionedAt)
	}

	if err := item.Delete(later); err != nil {
		t.Fatal(err)
	}

	if err := item.Apply(domain.StorageItemChanges{Name: new("x")}, later); !errors.Is(err, domain.ErrStorageItemDeleted) {
		t.Fatalf("Apply on deleted = %v", err)
	}
}
