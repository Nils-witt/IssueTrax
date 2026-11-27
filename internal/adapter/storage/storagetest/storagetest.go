// Package storagetest is a contract test suite for implementations of the
// storage ports. Every adapter (memory, postgres) runs it, so all of them
// behave the same way.
package storagetest

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/service"
)

// NewRepositories returns fresh, empty repositories for one test.
type NewRepositories func(t *testing.T) service.Repositories

var now = time.Date(2026, 1, 2, 3, 4, 5, 123000, time.UTC)

var contract = []struct {
	name string
	run  func(t *testing.T, newRepos NewRepositories)
}{
	{name: "Projects/create and get", run: testProjectsCreateAndGet},
	{name: "Projects/unique key and name", run: testProjectsUniqueKeyAndName},
	{name: "Projects/update", run: testProjectsUpdate},
	{name: "Projects/list", run: testProjectsList},
	{name: "Projects/allocate issue numbers", run: testProjectsAllocateIssueNumbers},
	{name: "Projects/get many", run: testProjectsGetMany},
	{name: "Issues/create, get and update", run: testIssuesCreateGetAndUpdate},
	{name: "Issues/not found", run: testIssuesNotFound},
	{name: "Issues/list filters", run: testIssuesListFilters},
	{name: "Issues/list pagination", run: testIssuesListPagination},
	{name: "Issues/storage item links", run: testIssuesStorageItemLinks},
	{name: "Issues/list by storage item", run: testIssuesListByStorageItem},
	{name: "Tags/ensure is idempotent per project", run: testTagsEnsureIsIdempotentPerProject},
	{name: "Tags/lookups", run: testTagsLookups},
	{name: "Tags/update", run: testTagsUpdate},
	{name: "Comments/create, get, update and delete", run: testCommentsLifecycle},
	{name: "Comments/list by issue", run: testCommentsListByIssue},
	{name: "Attachments/create, get, content and delete", run: testAttachmentsLifecycle},
	{name: "Attachments/list by issue", run: testAttachmentsListByIssue},
	{name: "Storages/create and get", run: testStoragesCreateAndGet},
	{name: "Storages/unique sibling names", run: testStoragesUniqueSiblingNames},
	{name: "Storages/update", run: testStoragesUpdate},
	{name: "Storages/list children", run: testStoragesListChildren},
	{name: "Storages/is descendant", run: testStoragesIsDescendant},
	{name: "ItemTypes/create, get and update", run: testItemTypesCreateGetAndUpdate},
	{name: "ItemTypes/live sibling names", run: testItemTypesSiblingNames},
	{name: "ItemTypes/list and search", run: testItemTypesListSearchAndChain},
	{name: "ItemTypes/chain and descendants", run: testItemTypesChain},
	{name: "AssetTagCounters/get and set", run: testAssetTagCounters},
	{name: "Manufacturers/create, get and update", run: testManufacturersCreateGetAndUpdate},
	{name: "Manufacturers/live names", run: testManufacturersLiveNames},
	{name: "Manufacturers/list, search and get many", run: testManufacturersList},
	{name: "HardwareModels/create, get and update", run: testHardwareModelsCreateGetAndUpdate},
	{name: "HardwareModels/live names per manufacturer", run: testHardwareModelsLiveNames},
	{name: "HardwareModels/list, search and get many", run: testHardwareModelsList},
	{name: "StorageItems/create, get and update", run: testStorageItemsCreateGetAndUpdate},
	{name: "StorageItems/modules keep their IDs", run: testStorageItemsModuleIDs},
	{name: "StorageItems/asset tags are unique", run: testStorageItemsAssetTags},
	{name: "StorageItems/list and filters", run: testStorageItemsListAndFilters},
	{name: "StorageItems/search", run: testStorageItemsSearch},
	{name: "StorageItems/pagination and get many", run: testStorageItemsPagination},
	{name: "Users/create, get and update", run: testUsersCreateGetAndUpdate},
	{name: "Users/list and get many", run: testUsersListAndGetMany},
	{name: "Users/identity is unique", run: testUsersIdentityIsUnique},
	{name: "Users/list by issuer", run: testUsersListByIssuer},
	{name: "Sessions/lifecycle", run: testSessionsLifecycle},
	{name: "APIKeys/lifecycle", run: testAPIKeysLifecycle},
	{name: "APIKeys/list by user", run: testAPIKeysListByUser},
	{name: "Audit/append and list", run: testAuditAppendAndList},
	{name: "Audit/list filters", run: testAuditListFilters},
	{name: "Audit/list pagination", run: testAuditPagination},
	{name: "Audit/delete before", run: testAuditDeleteBefore},
	{name: "Audit/rolled back with the transaction", run: testAuditRollback},
	{name: "Tx/commit and rollback", run: testTx},
}

// Run executes the whole contract suite; each test gets fresh repositories.
func Run(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	for _, c := range contract {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			c.run(t, newRepos)
		})
	}
}

func newID() uuid.UUID { return uuid.Must(uuid.NewV7()) }

func must(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}

func wantErr(t *testing.T, err, want error) {
	t.Helper()

	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func project(t *testing.T, r service.Repositories, key, name string) domain.Project {
	t.Helper()

	p := domain.Project{
		ID: domain.ProjectID(newID()), Key: domain.ProjectKey(key), Name: name,
		Description: "about " + name, CreatedAt: now, UpdatedAt: now,
	}
	must(t, r.Projects.Create(t.Context(), p))

	return p
}

func user(t *testing.T, r service.Repositories, name string) domain.User {
	t.Helper()

	u := domain.User{
		ID: domain.UserID(newID()), Issuer: "https://idp", Subject: "sub-" + name,
		Email: name + "@example.com", DisplayName: name, CreatedAt: now, LastLoginAt: now,
	}
	must(t, r.Users.Create(t.Context(), u))

	return u
}

func serviceAccount(t *testing.T, r service.Repositories, name string) domain.User {
	t.Helper()

	u, err := domain.NewServiceAccount(domain.UserID(newID()), name, now)
	must(t, err)
	must(t, r.Users.Create(t.Context(), u))

	return u
}

func tag(t *testing.T, r service.Repositories, p domain.Project, name string) domain.Tag {
	t.Helper()

	got, err := r.Tags.Ensure(t.Context(), domain.NewTag(domain.TagID(newID()), p.ID, domain.TagName(name), now))
	must(t, err)

	return got
}

// issue creates an issue with the next number of p.
func issue(t *testing.T, r service.Repositories, p domain.Project, by domain.User, mutate func(*domain.Issue)) domain.Issue {
	t.Helper()

	n, err := r.Projects.AllocateIssueNumber(t.Context(), p.ID)
	must(t, err)

	i := domain.Issue{
		ID: domain.IssueID(newID()), ProjectID: p.ID, Number: n, Title: "Issue " + strconv.FormatInt(n, 10),
		Status: domain.StatusOpen, Tags: []domain.TagID{}, Assignees: []domain.UserID{}, StorageItems: []domain.StorageItemID{},
		CreatedBy: by.ID, CreatedAt: now, UpdatedAt: now,
	}

	if mutate != nil {
		mutate(&i)
	}

	must(t, r.Issues.Create(t.Context(), i))

	return i
}

// sorted returns i with order-insensitive relations sorted, for comparison.
func sorted(i domain.Issue) domain.Issue {
	i.Tags = slices.SortedFunc(slices.Values(i.Tags), func(a, b domain.TagID) int { return cmp.Compare(a.String(), b.String()) })
	i.Assignees = slices.SortedFunc(slices.Values(i.Assignees), func(a, b domain.UserID) int { return cmp.Compare(a.String(), b.String()) })
	i.StorageItems = slices.SortedFunc(slices.Values(i.StorageItems), func(a, b domain.StorageItemID) int {
		return cmp.Compare(a.String(), b.String())
	})

	return i
}

func equalIssues(a, b domain.Issue) bool {
	a, b = sorted(a), sorted(b)

	return a.ID == b.ID && a.ProjectID == b.ProjectID && a.Number == b.Number && a.Title == b.Title &&
		a.Description == b.Description && a.Status == b.Status && a.CreatedBy == b.CreatedBy &&
		a.CreatedAt.Equal(b.CreatedAt) && a.UpdatedAt.Equal(b.UpdatedAt) &&
		slices.Equal(a.Tags, b.Tags) && slices.Equal(a.Assignees, b.Assignees) && slices.Equal(a.StorageItems, b.StorageItems)
}

func firstPage(t *testing.T, limit int) domain.PageRequest {
	t.Helper()

	p, err := domain.NewPageRequest(limit, "")
	must(t, err)

	return p
}

func testTx(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	boom := errors.New("boom")

	err := r.Tx.WithinTx(t.Context(), func(ctx context.Context) error {
		must(t, r.Projects.Create(ctx, domain.Project{ID: domain.ProjectID(newID()), Key: "ROLL", Name: "rolled back", CreatedAt: now, UpdatedAt: now}))
		return boom
	})
	wantErr(t, err, boom)

	_, err = r.Projects.GetByKey(t.Context(), "ROLL")
	wantErr(t, err, domain.ErrProjectNotFound)

	err = r.Tx.WithinTx(t.Context(), func(ctx context.Context) error {
		return r.Projects.Create(ctx, domain.Project{ID: domain.ProjectID(newID()), Key: "KEEP", Name: "kept", CreatedAt: now, UpdatedAt: now})
	})
	must(t, err)

	_, err = r.Projects.GetByKey(t.Context(), "KEEP")
	must(t, err)
}
