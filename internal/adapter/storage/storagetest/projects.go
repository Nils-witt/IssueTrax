package storagetest

import (
	"slices"
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func testProjectsCreateAndGet(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	want := project(t, r, "TRAX", "IssueTrax")

	got, err := r.Projects.GetByKey(t.Context(), "TRAX")
	must(t, err)

	if got != want {
		t.Fatalf("GetByKey() = %+v, want %+v", got, want)
	}

	_, err = r.Projects.GetByKey(t.Context(), "NOPE")
	wantErr(t, err, domain.ErrProjectNotFound)
}

func testProjectsUniqueKeyAndName(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	p := project(t, r, "TRAX", "IssueTrax")

	dup := p
	dup.ID = domain.ProjectID(newID())
	dup.Name = "Other"
	wantErr(t, r.Projects.Create(t.Context(), dup), domain.ErrProjectKeyTaken)

	dup.Key, dup.Name = "OTHER", "ISSUETRAX"
	wantErr(t, r.Projects.Create(t.Context(), dup), domain.ErrProjectNameTaken)
}

func testProjectsUpdate(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	p := project(t, r, "TRAX", "IssueTrax")
	project(t, r, "OTHER", "Other")

	p.Name, p.Description, p.UpdatedAt = "Renamed", "new", now.Add(time.Hour)
	must(t, r.Projects.Update(t.Context(), p))

	got, err := r.Projects.GetByKey(t.Context(), "TRAX")
	must(t, err)

	if got != p {
		t.Fatalf("after Update() = %+v, want %+v", got, p)
	}

	p.Name = "other"
	wantErr(t, r.Projects.Update(t.Context(), p), domain.ErrProjectNameTaken)

	p.ID = domain.ProjectID(newID())
	p.Name = "Unique"
	wantErr(t, r.Projects.Update(t.Context(), p), domain.ErrProjectNotFound)
}

func testProjectsList(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	for i, name := range []string{"e", "D", "c", "B", "a"} {
		project(t, r, "P"+string(rune('A'+i)), name)
	}

	var names []string

	page := firstPage(t, 2)

	for {
		got, err := r.Projects.List(t.Context(), page)
		must(t, err)

		if got.Total != 5 {
			t.Fatalf("Total = %d, want 5", got.Total)
		}

		for _, p := range got.Items {
			names = append(names, p.Name)
		}

		if got.NextCursor == "" {
			break
		}

		page.Cursor = got.NextCursor
	}

	if want := []string{"a", "B", "c", "D", "e"}; !slices.Equal(names, want) {
		t.Fatalf("names = %v, want %v", names, want)
	}

	empty, err := newRepos(t).Projects.List(t.Context(), firstPage(t, 0))
	must(t, err)

	if empty.Items == nil || len(empty.Items) != 0 || empty.Total != 0 || empty.NextCursor != "" {
		t.Fatalf("empty List() = %#v", empty)
	}

	_, err = r.Projects.List(t.Context(), domain.PageRequest{Limit: 1, Cursor: "garbage!"})
	wantErr(t, err, domain.ErrInvalid)
}

func testProjectsAllocateIssueNumbers(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	a := project(t, r, "AAA", "A")
	b := project(t, r, "BBB", "B")

	var got []int64

	for _, p := range []domain.Project{a, a, b, a} {
		n, err := r.Projects.AllocateIssueNumber(t.Context(), p.ID)
		must(t, err)

		got = append(got, n)
	}

	if want := []int64{1, 2, 1, 3}; !slices.Equal(got, want) {
		t.Fatalf("numbers = %v, want %v", got, want)
	}

	_, err := r.Projects.AllocateIssueNumber(t.Context(), domain.ProjectID(newID()))
	wantErr(t, err, domain.ErrProjectNotFound)
}
