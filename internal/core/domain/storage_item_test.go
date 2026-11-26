package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestNewGenericStorageItem(t *testing.T) {
	t.Parallel()

	storage := domain.StorageID(uuid.New())

	tests := []struct {
		name       string
		params     domain.NewGenericStorageItemParams
		wantName   string
		wantDesc   string
		wantFields map[string]string
	}{
		{name: "trims", params: domain.NewGenericStorageItemParams{Name: " Drill ", Description: " cordless \n"}, wantName: "Drill", wantDesc: "cordless"},
		{name: "no description", params: domain.NewGenericStorageItemParams{Name: "Drill"}, wantName: "Drill"},
		{name: "name at limit", params: domain.NewGenericStorageItemParams{Name: strings.Repeat("ä", 100)}, wantName: strings.Repeat("ä", 100)},
		{name: "missing name", params: domain.NewGenericStorageItemParams{Name: " "}, wantFields: map[string]string{"name": domain.FieldRequired}},
		{
			name:       "name too long",
			params:     domain.NewGenericStorageItemParams{Name: strings.Repeat("a", 101)},
			wantFields: map[string]string{"name": domain.FieldTooLong},
		},
		{
			name:       "description too long",
			params:     domain.NewGenericStorageItemParams{Name: "Drill", Description: strings.Repeat("a", domain.MaxDescriptionBytes+1)},
			wantFields: map[string]string{"description": domain.FieldTooLong},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.params.Now = testNow
			tt.params.StorageID = storage

			item, err := domain.NewGenericStorageItem(tt.params)
			wantFields(t, err, tt.wantFields)

			if err != nil {
				return
			}

			if item.Name != tt.wantName || item.Description != tt.wantDesc || item.StorageID != storage ||
				item.Kind != domain.StorageItemKindGeneric || item.IsDeleted() ||
				!item.CreatedAt.Equal(testNow) || !item.UpdatedAt.Equal(testNow) {
				t.Fatalf("NewGenericStorageItem = %+v", item)
			}
		})
	}
}

func TestGenericStorageItemApply(t *testing.T) {
	t.Parallel()

	later := testNow.Add(time.Hour)
	home := domain.StorageID(uuid.New())
	other := domain.StorageID(uuid.New())
	text := func(s string) *string { return &s }

	tests := []struct {
		name        string
		changes     domain.GenericStorageItemChanges
		deleted     bool
		wantName    string
		wantDesc    string
		wantStorage domain.StorageID
		wantUpdated time.Time
		wantFields  map[string]string
		wantErr     error
	}{
		{
			name: "edit", changes: domain.GenericStorageItemChanges{Name: text(" Saw "), Description: text(" sharp ")},
			wantName: "Saw", wantDesc: "sharp", wantStorage: home, wantUpdated: later,
		},
		{
			name: "move", changes: domain.GenericStorageItemChanges{StorageID: &other},
			wantName: "Drill", wantDesc: "old", wantStorage: other, wantUpdated: later,
		},
		{
			name: "clear description", changes: domain.GenericStorageItemChanges{Description: text("")},
			wantName: "Drill", wantStorage: home, wantUpdated: later,
		},
		{
			name: "no change keeps updated at", changes: domain.GenericStorageItemChanges{Name: text("Drill"), StorageID: &home},
			wantName: "Drill", wantDesc: "old", wantStorage: home, wantUpdated: testNow,
		},
		{name: "empty name", changes: domain.GenericStorageItemChanges{Name: text("")}, wantFields: map[string]string{"name": domain.FieldRequired}},
		{name: "deleted", deleted: true, changes: domain.GenericStorageItemChanges{Name: text("Saw")}, wantErr: domain.ErrStorageItemDeleted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			item := genericItem(home)
			if tt.deleted {
				item.DeletedAt = &testNow
			}

			before := item

			err := item.Apply(tt.changes, later)

			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Apply() error = %v, want %v", err, tt.wantErr)
				}
			default:
				wantFields(t, err, tt.wantFields)
			}

			if err != nil {
				if item.Name != before.Name || item.Description != before.Description || !item.UpdatedAt.Equal(testNow) {
					t.Fatalf("item changed on error: %+v", item)
				}

				return
			}

			if item.Name != tt.wantName || item.Description != tt.wantDesc || item.StorageID != tt.wantStorage ||
				!item.UpdatedAt.Equal(tt.wantUpdated) {
				t.Fatalf("after Apply() = %+v", item)
			}
		})
	}
}

func TestStorageItemDeleteAndRestore(t *testing.T) {
	t.Parallel()

	later := testNow.Add(time.Hour)
	item := genericItem(domain.StorageID(uuid.New()))

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

func genericItem(storage domain.StorageID) domain.GenericStorageItem {
	return domain.GenericStorageItem{
		ID: domain.StorageItemID(uuid.New()), StorageID: storage, Kind: domain.StorageItemKindGeneric,
		Name: "Drill", Description: "old", CreatedAt: testNow, UpdatedAt: testNow,
	}
}
