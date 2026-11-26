package storagetest

import (
	"context"
	"errors"
	"maps"
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
	"nilswitt.dev/issuetrax/internal/core/service"
)

var errBoom = errors.New("boom")

func auditEntry(t *testing.T, r service.Repositories, actor *domain.User, action domain.AuditAction, at time.Time) domain.AuditEntry {
	t.Helper()

	e := domain.AuditEntry{
		ID: domain.AuditEntryID(newID()), At: at, Action: action,
		TargetType: domain.AuditTargetIssue, TargetID: newID().String(), TargetLabel: "TRAX-1",
		Changes: domain.AuditChanges{
			"title":       {Old: new("old"), New: new("new")},
			"status":      {New: new("open")},
			"description": {Redacted: true},
		},
		IP: "192.0.2.1", UserAgent: "test",
	}

	if actor != nil {
		e.ActorID = &actor.ID
	}

	must(t, r.Audit.Append(t.Context(), e))

	return e
}

func equalAuditEntries(a, b domain.AuditEntry) bool {
	return a.ID == b.ID && a.At.Equal(b.At) && equalPtr(a.ActorID, b.ActorID) && a.Action == b.Action &&
		a.TargetType == b.TargetType && a.TargetID == b.TargetID && a.TargetLabel == b.TargetLabel &&
		a.IP == b.IP && a.UserAgent == b.UserAgent && maps.EqualFunc(a.Changes, b.Changes, equalAuditChanges)
}

func equalAuditChanges(x, y domain.AuditChange) bool {
	return x.Redacted == y.Redacted && equalPtr(x.Old, y.Old) && equalPtr(x.New, y.New)
}

func equalPtr[T comparable](a, b *T) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func auditIDs(entries []domain.AuditEntry) []domain.AuditEntryID {
	ids := make([]domain.AuditEntryID, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.ID)
	}

	return ids
}

func testAuditAppendAndList(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	ctx := t.Context()
	ada := user(t, r, "ada")
	bob := user(t, r, "bob")

	older := auditEntry(t, r, &ada, domain.AuditIssueUpdate, now)
	failed := auditEntry(t, r, nil, domain.AuditLoginFailed, now.Add(time.Minute))
	newer := auditEntry(t, r, &bob, domain.AuditIssueCreate, now.Add(2*time.Minute))

	got, err := r.Audit.List(ctx, port.AuditFilter{}, firstPage(t, 10))
	must(t, err)

	if got.Total != 3 || len(got.Items) != 3 || got.NextCursor != "" {
		t.Fatalf("List() = %+v", got)
	}

	for i, want := range []domain.AuditEntry{newer, failed, older} {
		if !equalAuditEntries(got.Items[i], want) {
			t.Fatalf("List()[%d] = %+v, want %+v", i, got.Items[i], want)
		}
	}
}

func testAuditListFilters(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	ctx := t.Context()
	ada := user(t, r, "ada")
	bob := user(t, r, "bob")

	older := auditEntry(t, r, &ada, domain.AuditIssueUpdate, now)
	failed := auditEntry(t, r, nil, domain.AuditLoginFailed, now.Add(time.Minute))
	newer := auditEntry(t, r, &bob, domain.AuditIssueCreate, now.Add(2*time.Minute))

	byActor, err := r.Audit.List(ctx, port.AuditFilter{ActorID: &ada.ID}, firstPage(t, 10))
	must(t, err)

	if byActor.Total != 1 || len(byActor.Items) != 1 || byActor.Items[0].ID != older.ID {
		t.Fatalf("List(actor) = %v", auditIDs(byActor.Items))
	}

	byAction, err := r.Audit.List(ctx, port.AuditFilter{Actions: []domain.AuditAction{domain.AuditIssueCreate, domain.AuditLoginFailed}}, firstPage(t, 10))
	must(t, err)

	if byAction.Total != 2 || len(byAction.Items) != 2 || byAction.Items[0].ID != newer.ID || byAction.Items[1].ID != failed.ID {
		t.Fatalf("List(actions) = %v", auditIDs(byAction.Items))
	}

	byTarget, err := r.Audit.List(ctx, port.AuditFilter{TargetType: domain.AuditTargetProject}, firstPage(t, 10))
	must(t, err)

	if byTarget.Total != 0 || len(byTarget.Items) != 0 {
		t.Fatalf("List(target) = %v", auditIDs(byTarget.Items))
	}
}

func testAuditPagination(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	ada := user(t, r, "ada")

	// Two entries share a timestamp, so the ID breaks the tie.
	var want []domain.AuditEntryID
	for _, at := range []time.Time{now.Add(2 * time.Minute), now.Add(time.Minute), now.Add(time.Minute), now} {
		want = append(want, auditEntry(t, r, &ada, domain.AuditLogin, at).ID)
	}

	if want[1].String() < want[2].String() {
		want[1], want[2] = want[2], want[1]
	}

	var got []domain.AuditEntryID

	page := firstPage(t, 3)

	for range 3 {
		res, err := r.Audit.List(t.Context(), port.AuditFilter{}, page)
		must(t, err)

		if res.Total != 4 {
			t.Fatalf("Total = %d, want 4", res.Total)
		}

		got = append(got, auditIDs(res.Items)...)

		if res.NextCursor == "" {
			break
		}

		page.Cursor = res.NextCursor
	}

	if len(got) != len(want) {
		t.Fatalf("pages = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pages = %v, want %v", got, want)
		}
	}
}

func testAuditDeleteBefore(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)
	ada := user(t, r, "ada")

	auditEntry(t, r, &ada, domain.AuditLogin, now.Add(-time.Hour))
	keep := auditEntry(t, r, &ada, domain.AuditLogin, now)

	n, err := r.Audit.DeleteBefore(t.Context(), now)
	must(t, err)

	if n != 1 {
		t.Fatalf("DeleteBefore() = %d, want 1", n)
	}

	got, err := r.Audit.List(t.Context(), port.AuditFilter{}, firstPage(t, 10))
	must(t, err)

	if len(got.Items) != 1 || got.Items[0].ID != keep.ID {
		t.Fatalf("List() after delete = %v", auditIDs(got.Items))
	}
}

func testAuditRollback(t *testing.T, newRepos NewRepositories) {
	t.Helper()

	r := newRepos(t)

	err := r.Tx.WithinTx(t.Context(), func(ctx context.Context) error {
		must(t, r.Audit.Append(ctx, domain.AuditEntry{ID: domain.AuditEntryID(newID()), At: now, Action: domain.AuditLogin}))
		return errBoom
	})
	wantErr(t, err, errBoom)

	got, err := r.Audit.List(t.Context(), port.AuditFilter{}, firstPage(t, 10))
	must(t, err)

	if got.Total != 0 {
		t.Fatalf("List() after rollback = %v", auditIDs(got.Items))
	}
}
