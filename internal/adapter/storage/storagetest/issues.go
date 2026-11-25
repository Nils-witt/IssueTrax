package storagetest

import (
	"cmp"
	"context"
	"slices"
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
	"nilswitt.dev/issuetrax/internal/core/service"
)

func testIssuesCreateGetAndUpdate(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	p := project(t, r, "TRAX", "IssueTrax")
	ada, bob := user(t, r, "ada"), user(t, r, "bob")
	bug, ui := tag(t, r, p, "bug"), tag(t, r, p, "ui")

	want := issue(t, r, p, ada, func(i *domain.Issue) {
		i.Description = "details"
		i.Status = domain.StatusDraft
		i.Tags = []domain.TagID{bug.ID, ui.ID}
		i.Assignees = []domain.UserID{ada.ID, bob.ID}
	})

	got, err := r.Issues.Get(t.Context(), p.ID, want.Number)
	must(t, err)

	if !equalIssues(got, want) {
		t.Fatalf("Get() = %+v, want %+v", got, want)
	}

	want.Title, want.Status, want.UpdatedAt = "Changed", domain.StatusClosed, now.Add(time.Minute)
	want.Tags, want.Assignees = []domain.TagID{ui.ID}, []domain.UserID{}
	must(t, r.Issues.Update(t.Context(), want))

	got, err = r.Issues.Get(t.Context(), p.ID, want.Number)
	must(t, err)

	if !equalIssues(got, want) {
		t.Fatalf("after Update() = %+v, want %+v", got, want)
	}

	// Reading inside a transaction (row lock path) returns the same data.
	err = r.Tx.WithinTx(t.Context(), func(ctx context.Context) error {
		got, err = r.Issues.Get(ctx, p.ID, want.Number)
		return err
	})
	must(t, err)

	if !equalIssues(got, want) {
		t.Fatalf("Get() in tx = %+v, want %+v", got, want)
	}
}

func testIssuesNotFound(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	p := project(t, r, "TRAX", "IssueTrax")
	other := project(t, r, "OTHER", "Other")
	i := issue(t, r, p, user(t, r, "ada"), nil)

	_, err := r.Issues.Get(t.Context(), other.ID, i.Number)
	wantErr(t, err, domain.ErrIssueNotFound)

	i.ID = domain.IssueID(newID())
	wantErr(t, r.Issues.Update(t.Context(), i), domain.ErrIssueNotFound)
}

// listFixture is a project with four issues (plus one in another project)
// covering every filter dimension.
type listFixture struct {
	r        service.Repositories
	p        domain.Project
	ada, bob domain.User
	bug, ui  domain.Tag
}

func seedIssueList(t *testing.T, newRepos NewRepositories) listFixture {
	t.Helper()

	r := newRepos(t)
	p := project(t, r, "TRAX", "IssueTrax")
	other := project(t, r, "OTHER", "Other")
	ada, bob := user(t, r, "ada"), user(t, r, "bob")
	bug, ui := tag(t, r, p, "bug"), tag(t, r, p, "ui")

	issue(t, r, p, ada, func(i *domain.Issue) {
		i.Title = "Login bug"
		i.Tags = []domain.TagID{bug.ID, ui.ID}
		i.Assignees = []domain.UserID{ada.ID}
	})
	issue(t, r, p, ada, func(i *domain.Issue) {
		i.Title, i.Description, i.Status = "Crash on save", "Stack trace mentions the EDITOR", domain.StatusClosed
		i.Tags = []domain.TagID{bug.ID}
		i.Assignees = []domain.UserID{ada.ID, bob.ID}
	})
	issue(t, r, p, ada, func(i *domain.Issue) {
		i.Title, i.Status, i.Tags = "Dark mode at 100%", domain.StatusDraft, []domain.TagID{ui.ID}
	})
	issue(t, r, p, ada, func(i *domain.Issue) { i.Title, i.Status = "Old_thing", domain.StatusArchived })
	issue(t, r, other, ada, func(i *domain.Issue) { i.Title = "Login bug elsewhere" })

	return listFixture{r: r, p: p, ada: ada, bob: bob, bug: bug, ui: ui}
}

// listFilter fills in the defaults the service would apply.
func listFilter(f port.IssueFilter) port.IssueFilter {
	if f.Statuses == nil {
		f.Statuses = []domain.IssueStatus{domain.StatusDraft, domain.StatusOpen, domain.StatusClosed}
	}

	f.TagMode = cmp.Or(f.TagMode, port.MatchAll)
	f.AssigneeMode = cmp.Or(f.AssigneeMode, port.MatchAll)

	return f
}

func testIssuesListFilters(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	fx := seedIssueList(t, newRepos)
	r, p, ada, bob, bug, ui := fx.r, fx.p, fx.ada, fx.bob, fx.bug, fx.ui
	filter, anyOf := listFilter, port.MatchAny

	tests := []struct {
		name   string
		filter port.IssueFilter
		want   []int64
	}{
		{name: "active statuses", filter: filter(port.IssueFilter{}), want: []int64{3, 2, 1}},
		{name: "archived", filter: filter(port.IssueFilter{Statuses: []domain.IssueStatus{domain.StatusArchived}}), want: []int64{4}},
		{name: "one tag", filter: filter(port.IssueFilter{TagIDs: []domain.TagID{bug.ID}}), want: []int64{2, 1}},
		{name: "all tags", filter: filter(port.IssueFilter{TagIDs: []domain.TagID{bug.ID, ui.ID}}), want: []int64{1}},
		{name: "any tag", filter: filter(port.IssueFilter{TagIDs: []domain.TagID{bug.ID, ui.ID}, TagMode: anyOf}), want: []int64{3, 2, 1}},
		{name: "all assignees", filter: filter(port.IssueFilter{AssigneeIDs: []domain.UserID{ada.ID, bob.ID}}), want: []int64{2}},
		{
			name:   "any assignee",
			filter: filter(port.IssueFilter{AssigneeIDs: []domain.UserID{ada.ID, bob.ID}, AssigneeMode: anyOf}),
			want:   []int64{2, 1},
		},
		{name: "title substring ignoring case", filter: filter(port.IssueFilter{Query: "LOGIN"}), want: []int64{1}},
		{name: "description substring", filter: filter(port.IssueFilter{Query: "editor"}), want: []int64{2}},
		{name: "percent is literal", filter: filter(port.IssueFilter{Query: "100%"}), want: []int64{3}},
		{name: "underscore is literal", filter: filter(port.IssueFilter{Query: "_", Statuses: domain.IssueStatuses()}), want: []int64{4}},
		{
			name:   "criteria combine",
			filter: filter(port.IssueFilter{TagIDs: []domain.TagID{ui.ID}, Statuses: []domain.IssueStatus{domain.StatusOpen}}),
			want:   []int64{1},
		},
		{name: "no match", filter: filter(port.IssueFilter{Query: "nothing"}), want: []int64{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := r.Issues.List(t.Context(), p.ID, tt.filter, firstPage(t, 0))
			must(t, err)

			numbers := make([]int64, 0, len(got.Items))
			for _, i := range got.Items {
				numbers = append(numbers, i.Number)
			}

			if !slices.Equal(numbers, tt.want) || got.Total != int64(len(tt.want)) || got.NextCursor != "" {
				t.Fatalf("List() = %v (total %d, cursor %q), want %v", numbers, got.Total, got.NextCursor, tt.want)
			}
		})
	}
}

func testIssuesListPagination(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	fx := seedIssueList(t, newRepos)
	r, p := fx.r, fx.p

	f := listFilter(port.IssueFilter{Statuses: domain.IssueStatuses()})
	page := firstPage(t, 3)

	var pages [][]int64

	for {
		got, err := r.Issues.List(t.Context(), p.ID, f, page)
		must(t, err)

		if got.Total != 4 {
			t.Fatalf("Total = %d, want 4", got.Total)
		}

		var numbers []int64
		for _, i := range got.Items {
			numbers = append(numbers, i.Number)

			if i.Number == 1 && len(i.Tags) != 2 {
				t.Fatalf("issue 1 tags = %v, want 2 loaded", i.Tags)
			}
		}

		pages = append(pages, numbers)

		if got.NextCursor == "" {
			break
		}

		page.Cursor = got.NextCursor
	}

	if want := [][]int64{{4, 3, 2}, {1}}; !slices.EqualFunc(pages, want, slices.Equal) {
		t.Fatalf("pages = %v, want %v", pages, want)
	}
}
