package service_test

import (
	"context"
	"testing"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
	"nilswitt.dev/issuetrax/internal/core/service"
)

// auditLog returns all entries, newest first.
func (f *fixture) auditLog(t *testing.T) []domain.AuditEntry {
	t.Helper()

	page, err := f.repos.Audit.List(t.Context(), port.AuditFilter{}, firstPage(t, domain.MaxPageSize))
	if err != nil {
		t.Fatal(err)
	}

	return page.Items
}

func wantActions(t *testing.T, entries []domain.AuditEntry, want ...domain.AuditAction) {
	t.Helper()

	got := make([]domain.AuditAction, 0, len(entries))
	for _, e := range entries {
		got = append(got, e.Action)
	}

	if len(got) != len(want) {
		t.Fatalf("audit actions = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("audit actions = %v, want %v", got, want)
		}
	}
}

func wantChange(t *testing.T, e domain.AuditEntry, field, before, after string) {
	t.Helper()

	c, ok := e.Changes[field]

	switch {
	case !ok:
		t.Fatalf("%s: no change of %q in %+v", e.Action, field, e.Changes)
	case before == "" && c.Old != nil && *c.Old != "", before != "" && (c.Old == nil || *c.Old != before):
		t.Fatalf("%s: %s old = %v, want %q", e.Action, field, c.Old, before)
	case c.New == nil || *c.New != after:
		t.Fatalf("%s: %s new = %v, want %q", e.Action, field, c.New, after)
	}
}

func TestAudit_LoginSetsAdminAndIsRecorded(t *testing.T) {
	t.Parallel()

	f := newAuthFixture(t)

	_, user := f.login(t)
	if user.Admin {
		t.Fatal("user outside the admin group is admin")
	}

	f.idp.ident.Groups = []string{"staff", "admins"}

	ctx := port.WithRequestInfo(t.Context(), port.RequestInfo{IP: "192.0.2.7", UserAgent: "ua"})

	_, state, _ := f.auth.BeginLogin(ctx, "/")

	token, user, err := f.auth.CompleteLogin(ctx, state, state.State, "good")
	if err != nil || !user.Admin {
		t.Fatalf("CompleteLogin() = %+v, %v; want admin", user, err)
	}

	if got, _ := f.auth.Authenticate(t.Context(), token); !got.Admin {
		t.Fatal("Authenticate() user is not admin")
	}

	log := f.auditLog(t)
	wantActions(t, log, domain.AuditLogin, domain.AuditLogin)

	e := log[0]
	if e.ActorID == nil || *e.ActorID != user.ID || e.TargetID != user.ID.String() || e.IP != "192.0.2.7" || e.UserAgent != "ua" {
		t.Fatalf("login entry = %+v", e)
	}

	wantChange(t, e, "admin", "", "true")

	// Leaving the group takes effect at the next sign-in.
	f.idp.ident.Groups = nil

	if _, user = f.login(t); user.Admin {
		t.Fatal("user is still admin after leaving the group")
	}
}

func TestAudit_FailedLoginAndLogout(t *testing.T) {
	t.Parallel()

	f := newAuthFixture(t)

	_, state, _ := f.auth.BeginLogin(t.Context(), "/")

	// A forged state is not recorded: anyone can hit the callback.
	if _, _, err := f.auth.CompleteLogin(t.Context(), state, "forged", "good"); err == nil {
		t.Fatal("CompleteLogin(forged state) succeeded")
	}

	if _, _, err := f.auth.CompleteLogin(t.Context(), state, state.State, "bad"); err == nil {
		t.Fatal("CompleteLogin(bad code) succeeded")
	}

	token, _ := f.login(t)

	if _, err := f.auth.Logout(t.Context(), token); err != nil {
		t.Fatal(err)
	}

	// Logging out again finds no session and records nothing.
	if _, err := f.auth.Logout(t.Context(), token); err != nil {
		t.Fatal(err)
	}

	log := f.auditLog(t)
	wantActions(t, log, domain.AuditLogout, domain.AuditLogin, domain.AuditLoginFailed)

	if failed := log[2]; failed.ActorID != nil {
		t.Fatalf("failed login has actor %v", failed.ActorID)
	}

	wantChange(t, log[2], "reason", "", "exchange_failed")
}

func TestAudit_IssueChanges(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "ada")
	bob := f.addUser(t, "bob")
	f.addProject(t, "TRAX", "IssueTrax")

	f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "Crash", Description: "stack trace", Tags: []string{"bug"}})

	_, err := f.issues.Update(t.Context(), ada.ID, "TRAX", 1, port.UpdateIssueInput{
		Title:       new("Crash on start"),
		Description: new("new trace"),
		Tags:        &[]string{"bug", "ui"},
		Assignees:   &[]domain.UserID{bob.ID},
	})
	if err != nil {
		t.Fatal(err)
	}

	// An update that changes nothing is not recorded.
	if _, err := f.issues.Update(t.Context(), ada.ID, "TRAX", 1, port.UpdateIssueInput{Title: new("Crash on start")}); err != nil {
		t.Fatal(err)
	}

	log := f.auditLog(t)
	wantActions(t, log, domain.AuditIssueUpdate, domain.AuditIssueCreate, domain.AuditProjectCreate)

	update := log[0]
	if update.TargetLabel != "TRAX-1" || update.TargetType != domain.AuditTargetIssue || *update.ActorID != ada.ID {
		t.Fatalf("update entry = %+v", update)
	}

	wantChange(t, update, "title", "Crash", "Crash on start")
	wantChange(t, update, "tags", "bug", "bug, ui")
	wantChange(t, update, "assignees", "", "bob")

	if d := update.Changes["description"]; !d.Redacted || d.Old != nil || d.New != nil {
		t.Fatalf("description change = %+v, want redacted", d)
	}

	if _, ok := update.Changes["status"]; ok {
		t.Fatal("unchanged status was recorded")
	}

	create := log[1]
	wantChange(t, create, "title", "", "Crash")
	wantChange(t, create, "status", "", string(domain.StatusOpen))
	wantChange(t, create, "tags", "", "bug")
}

func TestAudit_TagCommentAndKeyChanges(t *testing.T) {
	t.Parallel()

	f := newAuthFixture(t)
	_, ada := f.login(t)
	f.addProject(t, "TRAX", "IssueTrax")
	issue := f.addIssue(t, ada.ID, "TRAX", port.CreateIssueInput{Title: "x", Tags: []string{"bug"}})

	if _, err := f.tags.Update(t.Context(), ada.ID, "TRAX", issue.Tags[0].ID, port.UpdateTagInput{Name: new("defect")}); err != nil {
		t.Fatal(err)
	}

	c, err := f.comments.Create(t.Context(), ada.ID, "TRAX", 1, "hello")
	if err != nil {
		t.Fatal(err)
	}

	if err := f.comments.Delete(t.Context(), ada.ID, "TRAX", 1, c.Comment.ID); err != nil {
		t.Fatal(err)
	}

	bot, err := f.accounts.Create(t.Context(), ada, "ci")
	if err != nil {
		t.Fatal(err)
	}

	key, _, err := f.accounts.CreateKey(t.Context(), ada, bot.ID, "deploy", nil)
	if err != nil {
		t.Fatal(err)
	}

	for range 2 { // revoking twice records once
		if _, err := f.accounts.RevokeKey(t.Context(), ada, bot.ID, key.ID); err != nil {
			t.Fatal(err)
		}
	}

	log := f.auditLog(t)
	wantActions(t, log,
		domain.AuditAPIKeyRevoke, domain.AuditAPIKeyCreate, domain.AuditServiceAccountCreate,
		domain.AuditCommentDelete, domain.AuditCommentCreate, domain.AuditTagUpdate,
		domain.AuditIssueCreate, domain.AuditProjectCreate, domain.AuditLogin,
	)

	wantChange(t, log[5], "name", "bug", "defect")

	if log[5].TargetLabel != "TRAX: defect" || log[4].TargetLabel != "TRAX-1" || log[0].TargetLabel != "bot-ci: deploy" {
		t.Fatalf("labels = %q, %q, %q", log[5].TargetLabel, log[4].TargetLabel, log[0].TargetLabel)
	}

	wantChange(t, log[1], "name", "", "deploy")
	wantChange(t, log[2], "name", "", "bot-ci")
}

func TestAudit_FailedChangeIsNotRecorded(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.addProject(t, "TRAX", "IssueTrax")

	if _, err := f.projects.Create(t.Context(), testActor, port.CreateProjectInput{Key: "TRAX", Name: "Other"}); err == nil {
		t.Fatal("duplicate project created")
	}

	if _, err := f.projects.Update(t.Context(), testActor, "TRAX", port.UpdateProjectInput{Name: new("")}); err == nil {
		t.Fatal("invalid update succeeded")
	}

	wantActions(t, f.auditLog(t), domain.AuditProjectCreate)
}

func TestAuditService_List(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "ada")
	f.addProject(t, "TRAX", "IssueTrax")

	audit := service.NewAuditService(f.repos, 0)

	_, err := audit.List(t.Context(), ada, port.AuditFilter{}, firstPage(t, 10))
	wantCode(t, err, domain.CodeAdminRequired)

	ada.Admin = true

	page, err := audit.List(t.Context(), ada, port.AuditFilter{}, firstPage(t, 10))
	if err != nil || len(page.Items) != 1 || page.Items[0].Actor == nil || page.Items[0].Actor.ID != testActor {
		t.Fatalf("List() = %+v, %v", page, err)
	}

	_, err = audit.List(t.Context(), ada, port.AuditFilter{Actions: []domain.AuditAction{"nope"}}, firstPage(t, 10))
	wantField(t, err, "action", domain.FieldInvalid)
}

func TestAuditService_DeleteExpired(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := context.Background()

	for _, at := range []time.Time{testNow.Add(-48 * time.Hour), testNow.Add(-time.Hour)} {
		if err := f.repos.Audit.Append(ctx, domain.AuditEntry{ID: domain.AuditEntryID(testActor), At: at, Action: domain.AuditLogin}); err != nil {
			t.Fatal(err)
		}
	}

	clock := service.WithClock(func() time.Time { return testNow })

	if n, err := service.NewAuditService(f.repos, 0, clock).DeleteExpired(ctx); err != nil || n != 0 {
		t.Fatalf("DeleteExpired(keep forever) = %d, %v", n, err)
	}

	if n, err := service.NewAuditService(f.repos, 24*time.Hour, clock).DeleteExpired(ctx); err != nil || n != 1 {
		t.Fatalf("DeleteExpired(1 day) = %d, %v; want 1", n, err)
	}

	if got := f.auditLog(t); len(got) != 1 || !got[0].At.Equal(testNow.Add(-time.Hour)) {
		t.Fatalf("remaining = %+v", got)
	}
}
