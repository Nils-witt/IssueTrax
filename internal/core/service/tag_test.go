package service_test

import (
	"testing"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

func TestTagService_List(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	_, err := f.tags.List(t.Context(), "NOPE")
	wantCode(t, err, domain.CodeProjectNotFound)

	f.addProject(t, "TRAX", "IssueTrax")

	tags, err := f.tags.List(t.Context(), "TRAX")
	if err != nil || tags == nil || len(tags) != 0 {
		t.Fatalf("List() = %#v, %v; want empty non-nil", tags, err)
	}
}

func TestTagService_Rename(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		project   domain.ProjectKey
		newName   string
		wantCode  string
		wantField string
	}{
		{name: "normalizes", project: "TRAX", newName: "Front End"},
		{name: "taken", project: "TRAX", newName: "ui", wantCode: domain.CodeTagNameTaken},
		{name: "invalid", project: "TRAX", newName: "#", wantField: "name"},
		{name: "other project", project: "OTHER", newName: "x", wantCode: domain.CodeTagNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			ada := f.addUser(t, "ada")
			f.addProject(t, "TRAX", "IssueTrax")
			f.addProject(t, "OTHER", "Other")
			issue := f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "x", Tags: []string{"bug", "ui"}})

			_, err := f.tags.Rename(t.Context(), tt.project, issue.Tags[0].ID, tt.newName)

			switch {
			case tt.wantField != "":
				wantField(t, err, tt.wantField, domain.FieldInvalid)
				return
			case tt.wantCode != "":
				wantCode(t, err, tt.wantCode)
				return
			case err != nil:
				t.Fatal(err)
			}

			got, err := f.issues.Get(t.Context(), "TRAX", 1)
			if err != nil {
				t.Fatal(err)
			}

			if got.Tags[0].Name != "front-end" {
				t.Fatalf("issue tag after rename = %q, want front-end", got.Tags[0].Name)
			}
		})
	}

	t.Run("unknown tag", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		f.addProject(t, "TRAX", "IssueTrax")

		_, err := f.tags.Rename(t.Context(), "TRAX", domain.TagID(uuid.New()), "x")
		wantCode(t, err, domain.CodeTagNotFound)
	})
}
