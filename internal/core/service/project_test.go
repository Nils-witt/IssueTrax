package service_test

import (
	"slices"
	"testing"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

func TestProjectService_Create(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in       port.CreateProjectInput
		wantCode string
	}{
		{name: "valid", in: port.CreateProjectInput{Key: "new", Name: "New"}},
		{name: "duplicate key", in: port.CreateProjectInput{Key: "trax", Name: "Other"}, wantCode: domain.CodeProjectKeyTaken},
		{name: "duplicate name ignoring case", in: port.CreateProjectInput{Key: "NEW", Name: "ISSUETRAX"}, wantCode: domain.CodeProjectNameTaken},
		{name: "invalid", in: port.CreateProjectInput{Key: "x", Name: ""}, wantCode: domain.CodeValidationFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			f.addProject(t, "TRAX", "IssueTrax")

			got, err := f.projects.Create(t.Context(), testActor, tt.in)
			if tt.wantCode != "" {
				wantCode(t, err, tt.wantCode)
				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if got.Key != "NEW" || got.Name != "New" || !got.CreatedAt.Equal(testNow) {
				t.Fatalf("Create() = %+v", got)
			}
		})
	}
}

func TestProjectService_Get(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	want := f.addProject(t, "TRAX", "IssueTrax")

	tests := []struct {
		name     string
		key      domain.ProjectKey
		wantCode string
	}{
		{name: "exact", key: "TRAX"},
		{name: "lower case", key: "trax"},
		{name: "missing", key: "NOPE", wantCode: domain.CodeProjectNotFound},
		{name: "malformed", key: "../x", wantCode: domain.CodeProjectNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := f.projects.Get(t.Context(), tt.key)
			if tt.wantCode != "" {
				wantCode(t, err, tt.wantCode)
				return
			}

			if err != nil || got != want {
				t.Fatalf("Get() = %+v, %v; want %+v", got, err, want)
			}
		})
	}
}

func TestProjectService_List(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	for i, name := range []string{"e", "D", "c", "B", "a"} {
		f.addProject(t, "P"+string(rune('A'+i)), name)
	}

	var (
		names []string
		pages int
	)

	page := firstPage(t, 2)

	for {
		got, err := f.projects.List(t.Context(), page)
		if err != nil {
			t.Fatal(err)
		}

		if got.Total != 5 {
			t.Fatalf("Total = %d, want 5", got.Total)
		}

		for _, p := range got.Items {
			names = append(names, p.Name)
		}

		pages++

		if got.NextCursor == "" {
			break
		}

		page.Cursor = got.NextCursor
	}

	if want := []string{"a", "B", "c", "D", "e"}; !slices.Equal(names, want) || pages != 3 {
		t.Fatalf("names = %v over %d pages, want %v over 3", names, pages, want)
	}
}

func TestProjectService_Update(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in       port.UpdateProjectInput
		wantName string
		wantCode string
	}{
		{name: "rename", in: port.UpdateProjectInput{Name: new("Renamed"), Description: new("d")}, wantName: "Renamed"},
		{name: "keep name on description change", in: port.UpdateProjectInput{Description: new("d")}, wantName: "IssueTrax"},
		{name: "name taken", in: port.UpdateProjectInput{Name: new("other")}, wantName: "IssueTrax", wantCode: domain.CodeProjectNameTaken},
		{name: "invalid", in: port.UpdateProjectInput{Name: new("")}, wantName: "IssueTrax", wantCode: domain.CodeValidationFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			f.addProject(t, "TRAX", "IssueTrax")
			f.addProject(t, "OTHER", "Other")

			_, err := f.projects.Update(t.Context(), testActor, "trax", tt.in)
			if tt.wantCode != "" {
				wantCode(t, err, tt.wantCode)
			} else if err != nil {
				t.Fatal(err)
			}

			got, err := f.projects.Get(t.Context(), "TRAX")
			if err != nil {
				t.Fatal(err)
			}

			if got.Name != tt.wantName {
				t.Fatalf("Name = %q, want %q", got.Name, tt.wantName)
			}
		})
	}
}
