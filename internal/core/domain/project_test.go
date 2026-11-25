package domain_test

import (
	"strings"
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

var testNow = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func TestNormalizeProjectKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in    string
		want  domain.ProjectKey
		valid bool
	}{
		{in: " trax ", want: "TRAX", valid: true},
		{in: "AB", want: "AB", valid: true},
		{in: "A1B2C3D4E5", want: "A1B2C3D4E5", valid: true},
		{in: "A", want: "A"},
		{in: "ABCDEFGHIJK", want: "ABCDEFGHIJK"},
		{in: "1AB", want: "1AB"},
		{in: "TR-X", want: "TR-X"},
		{in: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			got := domain.NormalizeProjectKey(tt.in)
			if got != tt.want || got.Valid() != tt.valid {
				t.Fatalf("NormalizeProjectKey(%q) = %q (valid %v), want %q (valid %v)", tt.in, got, got.Valid(), tt.want, tt.valid)
			}
		})
	}
}

func TestNewProject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		params     domain.NewProjectParams
		wantFields map[string]string
	}{
		{name: "valid", params: domain.NewProjectParams{Key: "trax", Name: " IssueTrax ", Description: " d "}},
		{name: "name at limit", params: domain.NewProjectParams{Key: "AB", Name: strings.Repeat("ä", 100)}},
		{
			name:       "missing everything",
			params:     domain.NewProjectParams{},
			wantFields: map[string]string{"key": domain.FieldRequired, "name": domain.FieldRequired},
		},
		{
			name:       "bad key",
			params:     domain.NewProjectParams{Key: "1x", Name: "n"},
			wantFields: map[string]string{"key": domain.FieldInvalid},
		},
		{
			name:       "name too long",
			params:     domain.NewProjectParams{Key: "AB", Name: strings.Repeat("a", 101)},
			wantFields: map[string]string{"name": domain.FieldTooLong},
		},
		{
			name:       "description too long",
			params:     domain.NewProjectParams{Key: "AB", Name: "n", Description: strings.Repeat("a", domain.MaxDescriptionBytes+1)},
			wantFields: map[string]string{"description": domain.FieldTooLong},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.params.Now = testNow

			got, err := domain.NewProject(tt.params)
			wantFields(t, err, tt.wantFields)

			if tt.wantFields != nil {
				return
			}

			if string(got.Key) != strings.ToUpper(tt.params.Key) || got.Name != strings.TrimSpace(tt.params.Name) ||
				got.Description != strings.TrimSpace(tt.params.Description) || !got.CreatedAt.Equal(testNow) {
				t.Fatalf("NewProject() = %+v", got)
			}
		})
	}
}

func TestProject_Apply(t *testing.T) {
	t.Parallel()

	later := testNow.Add(time.Hour)

	newProject := func(t *testing.T) domain.Project {
		t.Helper()

		p, err := domain.NewProject(domain.NewProjectParams{Key: "TRAX", Name: "Old", Now: testNow})
		if err != nil {
			t.Fatal(err)
		}

		return p
	}

	t.Run("changes fields", func(t *testing.T) {
		t.Parallel()

		p := newProject(t)
		if err := p.Apply(domain.ProjectChanges{Name: new(" New "), Description: new("desc")}, later); err != nil {
			t.Fatal(err)
		}

		if p.Name != "New" || p.Description != "desc" || !p.UpdatedAt.Equal(later) || p.Key != "TRAX" {
			t.Fatalf("Apply() = %+v", p)
		}
	})

	t.Run("invalid leaves project unchanged", func(t *testing.T) {
		t.Parallel()

		p := newProject(t)
		before := p

		err := p.Apply(domain.ProjectChanges{Name: new(" "), Description: new("desc")}, later)
		wantFields(t, err, map[string]string{"name": domain.FieldRequired})

		if p != before {
			t.Fatalf("Apply() changed project to %+v", p)
		}
	})

	t.Run("no changes keeps timestamp", func(t *testing.T) {
		t.Parallel()

		p := newProject(t)
		if err := p.Apply(domain.ProjectChanges{}, later); err != nil {
			t.Fatal(err)
		}

		if !p.UpdatedAt.Equal(testNow) {
			t.Fatalf("UpdatedAt = %v, want %v", p.UpdatedAt, testNow)
		}
	})
}
