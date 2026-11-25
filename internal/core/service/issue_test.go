package service_test

import (
	"slices"
	"testing"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

func tagNames(v port.IssueView) []domain.TagName {
	names := make([]domain.TagName, 0, len(v.Tags))
	for _, t := range v.Tags {
		names = append(names, t.Name)
	}

	return names
}

func wantKeys(t *testing.T, views []port.IssueView, want ...string) {
	t.Helper()

	got := make([]string, 0, len(views))
	for _, v := range views {
		got = append(got, v.Issue.Key(v.ProjectKey))
	}

	if !slices.Equal(got, want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}
}

func TestIssueService_Create(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "ada")
	bob := f.addUser(t, "bob")
	f.addProject(t, "TRAX", "IssueTrax")

	first := f.addIssue(t, ada.ID, "trax", port.CreateIssueInput{
		Title:     " Login fails ",
		Tags:      []string{"Needs Review", "bug", "BUG"},
		Assignees: []domain.UserID{bob.ID, bob.ID},
	})
	second := f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "Second", Status: domain.StatusDraft, Tags: []string{"bug"}})

	wantKeys(t, []port.IssueView{first, second}, "TRAX-1", "TRAX-2")

	if first.Issue.Title != "Login fails" || first.Issue.Status != domain.StatusOpen || first.CreatedBy.ID != ada.ID {
		t.Fatalf("first = %+v", first)
	}

	if got := tagNames(first); !slices.Equal(got, []domain.TagName{"needs-review", "bug"}) {
		t.Fatalf("first tags = %v", got)
	}

	if len(first.Assignees) != 1 || first.Assignees[0].DisplayName != "bob" {
		t.Fatalf("first assignees = %+v", first.Assignees)
	}

	if second.Issue.Status != domain.StatusDraft || second.Tags[0].ID != first.Tags[1].ID {
		t.Fatalf("second = %+v, want draft reusing tag %v", second, first.Tags[1])
	}

	tags, err := f.tags.List(t.Context(), "TRAX")
	if err != nil {
		t.Fatal(err)
	}

	if len(tags) != 2 || tags[0].Name != "bug" || tags[1].Name != "needs-review" {
		t.Fatalf("project tags = %+v", tags)
	}
}

func TestIssueService_CreateErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		project   domain.ProjectKey
		in        port.CreateIssueInput
		wantCode  string
		wantField string
	}{
		{name: "missing project", project: "NOPE", in: port.CreateIssueInput{Title: "x"}, wantCode: domain.CodeProjectNotFound},
		{name: "missing title", project: "TRAX", in: port.CreateIssueInput{Tags: []string{"new"}}, wantField: "title"},
		{name: "invalid tag", project: "TRAX", in: port.CreateIssueInput{Title: "x", Tags: []string{"ok", "#"}}, wantField: "tags"},
		{
			name:      "unknown assignee",
			project:   "TRAX",
			in:        port.CreateIssueInput{Title: "x", Tags: []string{"new"}, Assignees: []domain.UserID{domain.UserID(uuid.New())}},
			wantField: "assignees",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			ada := f.addUser(t, "ada")
			f.addProject(t, "TRAX", "IssueTrax")

			_, err := f.issues.Create(t.Context(), ada.ID, tt.project, tt.in)
			if tt.wantField != "" {
				wantCode(t, err, domain.CodeValidationFailed)
			} else {
				wantCode(t, err, tt.wantCode)
			}

			// The failed create must not leave tags or consume a number.
			tags, err := f.tags.List(t.Context(), "TRAX")
			if err != nil || len(tags) != 0 {
				t.Fatalf("tags after failed create = %+v, %v", tags, err)
			}

			next := f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "next"})
			if next.Issue.Number != 1 {
				t.Fatalf("next number = %d, want 1", next.Issue.Number)
			}
		})
	}
}

func TestIssueService_Get(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "ada")
	f.addProject(t, "TRAX", "IssueTrax")
	f.addProject(t, "OTHER", "Other")
	created := f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "x", Tags: []string{"bug"}})

	got, err := f.issues.Get(t.Context(), "trax", 1)
	if err != nil || got.Issue.ID != created.Issue.ID || len(got.Tags) != 1 {
		t.Fatalf("Get() = %+v, %v", got, err)
	}

	_, err = f.issues.Get(t.Context(), "TRAX", 2)
	wantCode(t, err, domain.CodeIssueNotFound)

	_, err = f.issues.Get(t.Context(), "OTHER", 1)
	wantCode(t, err, domain.CodeIssueNotFound)
}

func TestIssueService_List(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "ada")
	bob := f.addUser(t, "bob")
	f.addProject(t, "TRAX", "IssueTrax")

	f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "Login bug", Tags: []string{"bug", "ui"}, Assignees: []domain.UserID{ada.ID}})
	f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{
		Title: "Crash on save", Description: "Stack trace mentions the Editor",
		Status: domain.StatusClosed, Tags: []string{"bug"}, Assignees: []domain.UserID{ada.ID, bob.ID},
	})
	f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "Dark mode", Status: domain.StatusDraft, Tags: []string{"ui"}})
	f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "Old thing", Status: domain.StatusArchived})

	tags, err := f.tags.List(t.Context(), "TRAX")
	if err != nil {
		t.Fatal(err)
	}

	bug, ui := tags[0].ID, tags[1].ID

	tests := []struct {
		name   string
		filter port.IssueFilter
		want   []int64
	}{
		{name: "default hides archived", filter: port.IssueFilter{}, want: []int64{3, 2, 1}},
		{name: "archived only", filter: port.IssueFilter{Statuses: []domain.IssueStatus{domain.StatusArchived}}, want: []int64{4}},
		{name: "one tag", filter: port.IssueFilter{TagIDs: []domain.TagID{bug}}, want: []int64{2, 1}},
		{name: "all tags", filter: port.IssueFilter{TagIDs: []domain.TagID{bug, ui}}, want: []int64{1}},
		{name: "any tag", filter: port.IssueFilter{TagIDs: []domain.TagID{bug, ui}, TagMode: port.MatchAny}, want: []int64{3, 2, 1}},
		{name: "all assignees", filter: port.IssueFilter{AssigneeIDs: []domain.UserID{ada.ID, bob.ID}}, want: []int64{2}},
		{
			name:   "any assignee",
			filter: port.IssueFilter{AssigneeIDs: []domain.UserID{ada.ID, bob.ID}, AssigneeMode: port.MatchAny},
			want:   []int64{2, 1},
		},
		{name: "query title ignoring case", filter: port.IssueFilter{Query: " BUG "}, want: []int64{1}},
		{name: "query description", filter: port.IssueFilter{Query: "editor"}, want: []int64{2}},
		{
			name:   "criteria combine with and",
			filter: port.IssueFilter{TagIDs: []domain.TagID{ui}, Statuses: []domain.IssueStatus{domain.StatusOpen}},
			want:   []int64{1},
		},
		{name: "no match", filter: port.IssueFilter{Query: "nothing"}, want: []int64{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := f.issues.List(t.Context(), "TRAX", tt.filter, firstPage(t, 0))
			if err != nil {
				t.Fatal(err)
			}

			numbers := make([]int64, 0, len(got.Items))
			for _, v := range got.Items {
				numbers = append(numbers, v.Issue.Number)
			}

			if !slices.Equal(numbers, tt.want) || got.Total != int64(len(tt.want)) || got.NextCursor != "" {
				t.Fatalf("List() = %v (total %d, cursor %q), want %v", numbers, got.Total, got.NextCursor, tt.want)
			}
		})
	}
}

func TestIssueService_ListInvalidFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		filter port.IssueFilter
		field  string
	}{
		{name: "status", filter: port.IssueFilter{Statuses: []domain.IssueStatus{"done"}}, field: "status"},
		{name: "tag mode", filter: port.IssueFilter{TagMode: "some"}, field: "tagMode"},
		{name: "assignee mode", filter: port.IssueFilter{AssigneeMode: "some"}, field: "assigneeMode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			f.addProject(t, "TRAX", "IssueTrax")

			_, err := f.issues.List(t.Context(), "TRAX", tt.filter, firstPage(t, 0))
			wantField(t, err, tt.field, domain.FieldInvalid)
		})
	}
}

func TestIssueService_ListPagination(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "ada")
	f.addProject(t, "TRAX", "IssueTrax")

	for range 5 {
		f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "x"})
	}

	var pages [][]int64

	page := firstPage(t, 2)

	for {
		got, err := f.issues.List(t.Context(), "TRAX", port.IssueFilter{}, page)
		if err != nil {
			t.Fatal(err)
		}

		if got.Total != 5 {
			t.Fatalf("Total = %d, want 5", got.Total)
		}

		var numbers []int64
		for _, v := range got.Items {
			numbers = append(numbers, v.Issue.Number)
		}

		pages = append(pages, numbers)

		if got.NextCursor == "" {
			break
		}

		page.Cursor = got.NextCursor
	}

	want := [][]int64{{5, 4}, {3, 2}, {1}}
	if !slices.EqualFunc(pages, want, slices.Equal) {
		t.Fatalf("pages = %v, want %v", pages, want)
	}
}

func TestIssueService_Update(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "ada")
	f.addProject(t, "TRAX", "IssueTrax")
	f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "Old", Tags: []string{"bug"}})

	got, err := f.issues.Update(t.Context(), "TRAX", 1, port.UpdateIssueInput{
		Title:     new("New"),
		Tags:      &[]string{"ui", "Bug"},
		Assignees: &[]domain.UserID{ada.ID},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got.Issue.Title != "New" || !slices.Equal(tagNames(got), []domain.TagName{"ui", "bug"}) || len(got.Assignees) != 1 {
		t.Fatalf("Update() = %+v", got)
	}

	_, err = f.issues.Update(t.Context(), "TRAX", 1, port.UpdateIssueInput{Assignees: &[]domain.UserID{domain.UserID(uuid.New())}})
	wantField(t, err, "assignees", domain.FieldUnknown)

	_, err = f.issues.Update(t.Context(), "TRAX", 2, port.UpdateIssueInput{Title: new("x")})
	wantCode(t, err, domain.CodeIssueNotFound)
}

func TestIssueService_UpdateArchived(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "ada")
	f.addProject(t, "TRAX", "IssueTrax")
	f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "Old"})

	update := func(in port.UpdateIssueInput) error {
		_, err := f.issues.Update(t.Context(), "TRAX", 1, in)
		return err
	}

	if err := update(port.UpdateIssueInput{Status: new(domain.StatusArchived)}); err != nil {
		t.Fatalf("archive: %v", err)
	}

	wantCode(t, update(port.UpdateIssueInput{Title: new("New")}), domain.CodeIssueArchived)
	wantCode(t, update(port.UpdateIssueInput{Status: new(domain.StatusOpen), Tags: &[]string{"fresh"}}), domain.CodeIssueArchived)

	tags, err := f.tags.List(t.Context(), "TRAX")
	if err != nil || len(tags) != 0 {
		t.Fatalf("rejected update created tags: %+v, %v", tags, err)
	}

	if err := update(port.UpdateIssueInput{Status: new(domain.StatusOpen)}); err != nil {
		t.Fatalf("unarchive: %v", err)
	}

	if err := update(port.UpdateIssueInput{Title: new("New")}); err != nil {
		t.Fatalf("edit after unarchive: %v", err)
	}
}
