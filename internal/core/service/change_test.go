package service_test

import (
	"slices"
	"testing"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

func TestServicesAnnounceCommittedChanges(t *testing.T) {
	t.Parallel()

	f, ada, _ := commentFixture(t)

	if got, want := f.changes.take(), []domain.Change{
		{Kind: domain.ChangeProject, Project: "TRAX"},
		{Kind: domain.ChangeIssue, Project: "TRAX", Issue: 1},
	}; !slices.Equal(got, want) {
		t.Fatalf("after setup: changes = %+v, want %+v", got, want)
	}

	ctx := t.Context()
	steps := []struct {
		name string
		do   func() error
		want domain.Change
	}{
		{
			name: "update project (lower-case key)",
			do: func() error {
				_, err := f.projects.Update(ctx, ada.ID, "trax", port.UpdateProjectInput{Name: new("Renamed")})
				return err
			},
			want: domain.Change{Kind: domain.ChangeProject, Project: "TRAX"},
		},
		{
			name: "update issue",
			do: func() error {
				_, err := f.issues.Update(ctx, ada.ID, "TRAX", 1, port.UpdateIssueInput{Tags: &[]string{"bug"}})
				return err
			},
			want: domain.Change{Kind: domain.ChangeIssue, Project: "TRAX", Issue: 1},
		},
		{
			name: "update tag",
			do: func() error {
				tags, err := f.tags.List(ctx, "TRAX")
				if err != nil {
					return err
				}

				_, err = f.tags.Update(ctx, ada.ID, "trax", tags[0].ID, port.UpdateTagInput{Name: new("defect")})

				return err
			},
			want: domain.Change{Kind: domain.ChangeTags, Project: "TRAX"},
		},
		{
			name: "create comment",
			do: func() error {
				_, err := f.comments.Create(ctx, ada.ID, "trax", 1, "hi")
				return err
			},
			want: domain.Change{Kind: domain.ChangeComments, Project: "TRAX", Issue: 1},
		},
		{
			name: "delete comment",
			do: func() error {
				comments, err := f.comments.List(ctx, "TRAX", 1)
				if err != nil {
					return err
				}

				return f.comments.Delete(ctx, ada.ID, "TRAX", 1, comments[0].Comment.ID)
			},
			want: domain.Change{Kind: domain.ChangeComments, Project: "TRAX", Issue: 1},
		},
	}

	for _, step := range steps {
		if err := step.do(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}

		if got := f.changes.take(); !slices.Equal(got, []domain.Change{step.want}) {
			t.Fatalf("%s: changes = %+v, want [%+v]", step.name, got, step.want)
		}
	}
}

func TestServicesDoNotAnnounceFailedChanges(t *testing.T) {
	t.Parallel()

	f, _, bob := commentFixture(t)
	f.changes.take()

	if _, err := f.issues.Update(t.Context(), bob.ID, "TRAX", 1, port.UpdateIssueInput{Title: new("")}); err == nil {
		t.Fatal("empty title accepted")
	}

	if _, err := f.comments.Create(t.Context(), bob.ID, "TRAX", 99, "hi"); err == nil {
		t.Fatal("comment on missing issue accepted")
	}

	if got := f.changes.take(); len(got) != 0 {
		t.Fatalf("changes = %+v, want none", got)
	}
}
