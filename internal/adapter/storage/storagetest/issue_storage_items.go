package storagetest

import (
	"cmp"
	"slices"
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// switchName names the network device the link tests use.
const switchName = "Switch"

func testIssuesStorageItemLinks(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	p := project(t, r, "TRAX", "IssueTrax")
	ada := user(t, r, "ada")
	st := storage(t, r, nil, "Shelf")
	drillItem := genericItem(t, r, st, drill, "")
	sw := networkDevice(t, r, st, switchName)

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
	item := genericItem(t, r, st, drill, "")
	unrelated := genericItem(t, r, st, bits, "")

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

func testStorageItemsAcrossKinds(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	st := storage(t, r, nil, "Shelf")
	drillItem := genericItem(t, r, st, drill, "cordless")
	sw := networkDevice(t, r, st, switchName)
	gone := genericItem(t, r, st, "Broken drill", "")

	deletedAt := now.Add(time.Minute)
	gone.DeletedAt, gone.UpdatedAt = &deletedAt, deletedAt
	must(t, r.GenericStorageItems.Update(t.Context(), gone))

	many, err := r.StorageItems.GetMany(t.Context(), []domain.StorageItemID{sw.ID, drillItem.ID, gone.ID, domain.StorageItemID(newID())})
	must(t, err)

	byID := map[domain.StorageItemID]domain.StorageItem{}
	for _, i := range many {
		byID[i.ID] = i
	}

	if len(many) != 3 || !equalItemBase(byID[drillItem.ID], drillItem.StorageItem) ||
		!equalItemBase(byID[sw.ID], sw.StorageItem) || !equalItemBase(byID[gone.ID], gone.StorageItem) {
		t.Fatalf("GetMany() = %+v", many)
	}

	search := func(filter port.StorageItemFilter, limit int, cursor string) domain.Page[domain.StorageItem] {
		t.Helper()

		res, err := r.StorageItems.Search(t.Context(), filter, pageOf(t, limit, cursor))
		must(t, err)

		return res
	}

	itemNames := func(items []domain.StorageItem) []string {
		out := make([]string, 0, len(items))
		for _, i := range items {
			out = append(out, i.Name)
		}

		return out
	}

	first := search(port.StorageItemFilter{}, 1, "")
	rest := search(port.StorageItemFilter{}, 5, first.NextCursor)

	if got := append(itemNames(first.Items), itemNames(rest.Items)...); !slices.Equal(got, []string{drill, switchName}) ||
		first.Total != 2 || rest.NextCursor != "" {
		t.Fatalf("paginated search = %v (total %d)", got, first.Total)
	}

	if got := itemNames(search(port.StorageItemFilter{Query: "CORDLESS"}, 0, "").Items); !slices.Equal(got, []string{drill}) {
		t.Fatalf("query search = %v", got)
	}

	if got := itemNames(search(port.StorageItemFilter{Query: "drill", Deleted: true}, 0, "").Items); !slices.Equal(got, []string{gone.Name}) {
		t.Fatalf("deleted search = %v", got)
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
