package storagetest

import (
	"cmp"
	"slices"
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// switchName names an item the link tests use.
const switchName = "Switch"

func testIssuesStorageItemLinks(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	p := project(t, r, "TRAX", "IssueTrax")
	ada := user(t, r, "ada")
	st := storage(t, r, nil, "Shelf")
	tool := itemType(t, r, "Tool")
	drillItem := storageItem(t, r, st, tool, drill)
	sw := storageItem(t, r, st, tool, switchName)

	both := issue(t, r, p, ada, func(i *domain.Issue) {
		i.StorageItems = []domain.StorageItemID{drillItem.ID, sw.ID}
	})
	onlyDrill := issue(t, r, p, ada, func(i *domain.Issue) { i.StorageItems = []domain.StorageItemID{drillItem.ID} })
	issue(t, r, p, ada, nil)

	got, err := r.Issues.Get(t.Context(), p.ID, both.Number)
	must(t, err)

	if !equalIssues(got, both) {
		t.Fatalf("Get() = %+v, want %+v", got, both)
	}

	filtered := func(mode port.MatchMode, ids ...domain.StorageItemID) []int64 {
		t.Helper()

		f := listFilter(port.IssueFilter{StorageItemIDs: ids, StorageItemMode: cmp.Or(mode, port.MatchAll)})

		page, err := r.Issues.List(t.Context(), p.ID, f, firstPage(t, 0))
		must(t, err)

		numbers := make([]int64, 0, len(page.Items))
		for _, i := range page.Items {
			numbers = append(numbers, i.Number)
		}

		return numbers
	}

	if got := filtered(port.MatchAll, drillItem.ID, sw.ID); !slices.Equal(got, []int64{both.Number}) {
		t.Fatalf("all items = %v", got)
	}

	if got := filtered(port.MatchAny, drillItem.ID, sw.ID); !slices.Equal(got, []int64{onlyDrill.Number, both.Number}) {
		t.Fatalf("any item = %v", got)
	}

	both.StorageItems, both.UpdatedAt = []domain.StorageItemID{sw.ID}, now.Add(time.Minute)
	must(t, r.Issues.Update(t.Context(), both))

	got, err = r.Issues.Get(t.Context(), p.ID, both.Number)
	must(t, err)

	if !equalIssues(got, both) {
		t.Fatalf("after Update() = %+v, want %+v", got, both)
	}

	if got := filtered(port.MatchAll, drillItem.ID); !slices.Equal(got, []int64{onlyDrill.Number}) {
		t.Fatalf("drill after unlink = %v", got)
	}
}

func testIssuesListByStorageItem(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	p := project(t, r, "TRAX", "IssueTrax")
	other := project(t, r, "OTHER", "Other")
	ada := user(t, r, "ada")
	st := storage(t, r, nil, "Shelf")
	tool := itemType(t, r, "Tool")
	item := storageItem(t, r, st, tool, drill)
	unrelated := storageItem(t, r, st, tool, bits)

	link := func(proj domain.Project, at time.Time, status domain.IssueStatus) domain.Issue {
		return issue(t, r, proj, ada, func(i *domain.Issue) {
			i.StorageItems, i.CreatedAt, i.UpdatedAt, i.Status = []domain.StorageItemID{item.ID}, at, at, status
		})
	}

	oldest := link(p, now, domain.StatusOpen)
	archived := link(other, now.Add(time.Hour), domain.StatusArchived)
	newest := link(p, now.Add(2*time.Hour), domain.StatusClosed)
	issue(t, r, p, ada, func(i *domain.Issue) { i.StorageItems = []domain.StorageItemID{unrelated.ID} })

	page := firstPage(t, 2)

	var got []domain.IssueID

	for {
		res, err := r.Issues.ListByStorageItem(t.Context(), item.ID, page)
		must(t, err)

		if res.Total != 3 {
			t.Fatalf("Total = %d, want 3", res.Total)
		}

		for _, i := range res.Items {
			got = append(got, i.ID)
		}

		if res.NextCursor == "" {
			break
		}

		page = pageOf(t, 2, res.NextCursor)
	}

	if want := []domain.IssueID{newest.ID, archived.ID, oldest.ID}; !slices.Equal(got, want) {
		t.Fatalf("ListByStorageItem() = %v, want %v", got, want)
	}

	none, err := r.Issues.ListByStorageItem(t.Context(), domain.StorageItemID(newID()), firstPage(t, 0))
	must(t, err)

	if len(none.Items) != 0 || none.Total != 0 {
		t.Fatalf("unknown item = %+v", none)
	}
}

func testProjectsGetMany(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	a := project(t, r, "AAA", "A")
	b := project(t, r, "BBB", "B")
	project(t, r, "CCC", "C")

	got, err := r.Projects.GetMany(t.Context(), []domain.ProjectID{b.ID, a.ID, domain.ProjectID(newID())})
	must(t, err)

	keys := make([]domain.ProjectKey, 0, len(got))
	for _, p := range got {
		keys = append(keys, p.Key)
	}

	slices.Sort(keys)

	if !slices.Equal(keys, []domain.ProjectKey{"AAA", "BBB"}) {
		t.Fatalf("GetMany() = %v", keys)
	}

	none, err := r.Projects.GetMany(t.Context(), nil)
	must(t, err)

	if len(none) != 0 {
		t.Fatalf("GetMany(nil) = %v", none)
	}
}
