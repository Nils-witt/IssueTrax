package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestNewStorage(t *testing.T) {
	t.Parallel()

	parent := domain.StorageID(uuid.New())

	tests := []struct {
		name       string
		params     domain.NewStorageParams
		wantName   string
		wantFields map[string]string
	}{
		{name: "root", params: domain.NewStorageParams{Name: " Shelf A "}, wantName: "Shelf A"},
		{name: "child", params: domain.NewStorageParams{Name: "Box", ParentID: &parent}, wantName: "Box"},
		{name: "name at limit", params: domain.NewStorageParams{Name: strings.Repeat("ä", 100)}, wantName: strings.Repeat("ä", 100)},
		{name: "missing name", params: domain.NewStorageParams{Name: "  "}, wantFields: map[string]string{"name": domain.FieldRequired}},
		{
			name:       "name too long",
			params:     domain.NewStorageParams{Name: strings.Repeat("a", 101)},
			wantFields: map[string]string{"name": domain.FieldTooLong},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.params.Now = testNow

			s, err := domain.NewStorage(tt.params)
			wantFields(t, err, tt.wantFields)

			if err != nil {
				return
			}

			if s.Name != tt.wantName || s.ParentID != tt.params.ParentID || !s.CreatedAt.Equal(testNow) || !s.UpdatedAt.Equal(testNow) {
				t.Fatalf("NewStorage = %+v", s)
			}
		})
	}
}

func TestStorageApply(t *testing.T) {
	t.Parallel()

	later := testNow.Add(time.Hour)
	self := domain.StorageID(uuid.New())
	other := domain.StorageID(uuid.New())
	name := func(s string) *string { return &s }
	to := func(id domain.StorageID) *domain.StorageParentChange { return &domain.StorageParentChange{ID: &id} }

	tests := []struct {
		name        string
		parent      *domain.StorageID
		changes     domain.StorageChanges
		wantName    string
		wantParent  string
		wantUpdated time.Time
		wantFields  map[string]string
	}{
		{name: "rename", changes: domain.StorageChanges{Name: name(" Rack ")}, wantName: "Rack", wantUpdated: later},
		{name: "move under parent", changes: domain.StorageChanges{Parent: to(other)}, wantName: "Shelf", wantParent: other.String(), wantUpdated: later},
		{name: "move to root", parent: &other, changes: domain.StorageChanges{Parent: &domain.StorageParentChange{}}, wantName: "Shelf", wantUpdated: later},
		{
			name: "no change keeps updated at", parent: &other, changes: domain.StorageChanges{Name: name("Shelf"), Parent: to(other)},
			wantName: "Shelf", wantParent: other.String(), wantUpdated: testNow,
		},
		{name: "own parent", changes: domain.StorageChanges{Name: name("New"), Parent: to(self)}, wantFields: map[string]string{"parentId": domain.FieldInvalid}},
		{name: "empty name", changes: domain.StorageChanges{Name: name("")}, wantFields: map[string]string{"name": domain.FieldRequired}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := domain.Storage{ID: self, ParentID: tt.parent, Name: "Shelf", CreatedAt: testNow, UpdatedAt: testNow}
			before := s

			err := s.Apply(tt.changes, later)
			wantFields(t, err, tt.wantFields)

			if err != nil {
				if s.Name != before.Name || s.ParentString() != before.ParentString() || !s.UpdatedAt.Equal(testNow) {
					t.Fatalf("storage changed on error: %+v", s)
				}

				return
			}

			if s.Name != tt.wantName || s.ParentString() != tt.wantParent || !s.UpdatedAt.Equal(tt.wantUpdated) {
				t.Fatalf("after Apply() = %+v (parent %q)", s, s.ParentString())
			}
		})
	}
}
