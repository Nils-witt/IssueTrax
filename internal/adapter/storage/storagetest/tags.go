package storagetest

import (
	"testing"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func testTagsEnsureIsIdempotentPerProject(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	p := project(t, r, "TRAX", "IssueTrax")
	other := project(t, r, "OTHER", "Other")

	first := tag(t, r, p, "bug")
	again := tag(t, r, p, "bug")
	elsewhere := tag(t, r, other, "bug")

	if again != first {
		t.Fatalf("second Ensure() = %+v, want existing %+v", again, first)
	}

	if elsewhere.ID == first.ID {
		t.Fatal("Ensure() in another project reused the tag")
	}
}

func testTagsLookups(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	p := project(t, r, "TRAX", "IssueTrax")
	other := project(t, r, "OTHER", "Other")
	ui, bug := tag(t, r, p, "ui"), tag(t, r, p, "bug")
	foreign := tag(t, r, other, "ui")

	list, err := r.Tags.ListByProject(t.Context(), p.ID)
	must(t, err)

	if len(list) != 2 || list[0] != bug || list[1] != ui {
		t.Fatalf("ListByProject() = %+v, want [bug ui]", list)
	}

	got, err := r.Tags.Get(t.Context(), p.ID, ui.ID)
	if err != nil || got != ui {
		t.Fatalf("Get() = %+v, %v", got, err)
	}

	_, err = r.Tags.Get(t.Context(), p.ID, foreign.ID)
	wantErr(t, err, domain.ErrTagNotFound)

	many, err := r.Tags.GetMany(t.Context(), []domain.TagID{ui.ID, foreign.ID, domain.TagID(newID())})
	must(t, err)

	if len(many) != 2 {
		t.Fatalf("GetMany() = %+v, want 2 tags", many)
	}

	byName, err := r.Tags.FindByNames(t.Context(), p.ID, []domain.TagName{"ui", "missing"})
	must(t, err)

	if len(byName) != 1 || byName[0] != ui {
		t.Fatalf("FindByNames() = %+v, want [ui]", byName)
	}

	none, err := r.Tags.GetMany(t.Context(), nil)
	if err != nil || len(none) != 0 {
		t.Fatalf("GetMany(nil) = %+v, %v", none, err)
	}
}

func testTagsUpdate(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	p := project(t, r, "TRAX", "IssueTrax")
	bug := tag(t, r, p, "bug")
	tag(t, r, p, "ui")

	bug.Name, bug.Color = "defect", "#123456"
	must(t, r.Tags.Update(t.Context(), bug))

	got, err := r.Tags.Get(t.Context(), p.ID, bug.ID)
	if err != nil || got.Name != "defect" || got.Color != "#123456" {
		t.Fatalf("after Update() = %+v, %v", got, err)
	}

	bug.Name = "ui"
	wantErr(t, r.Tags.Update(t.Context(), bug), domain.ErrTagNameTaken)

	bug.ID, bug.Name = domain.TagID(newID()), "fresh"
	wantErr(t, r.Tags.Update(t.Context(), bug), domain.ErrTagNotFound)
}
