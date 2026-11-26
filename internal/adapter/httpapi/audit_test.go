package httpapi_test

import (
	"net/http"
	"net/url"
	"testing"
)

type auditEntryBody struct {
	Action      string `json:"action"`
	TargetType  string `json:"targetType"`
	TargetLabel string `json:"targetLabel"`
	Actor       *struct {
		DisplayName string `json:"displayName"`
	} `json:"actor"`
	Changes map[string]struct {
		Old      *string `json:"old"`
		New      *string `json:"new"`
		Redacted bool    `json:"redacted"`
	} `json:"changes"`
	IP        string `json:"ip"`
	UserAgent string `json:"userAgent"`
}

type auditBody struct {
	Items      []auditEntryBody `json:"items"`
	Total      int64            `json:"total"`
	NextCursor *string          `json:"nextCursor"`
}

func TestAuditLog_AdminSeesChanges(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.idp.groups = []string{"staff", testAdminGroup}
	c := newClient(t)
	env.login(t, c, "/")

	var me struct {
		Admin bool `json:"admin"`
	}
	env.do(t, c, http.MethodGet, "/api/me", "").json(t, &me)

	if !me.Admin {
		t.Fatal("GET /api/me: admin = false, want true")
	}

	wantStatus(t, env.do(t, c, http.MethodPost, "/api/projects", `{"key":"TRAX","name":"IssueTrax"}`), http.StatusCreated)
	wantStatus(t, env.do(t, c, http.MethodPost, "/api/projects/TRAX/issues", `{"title":"Old","description":"secret"}`), http.StatusCreated)
	wantStatus(t, env.do(t, c, http.MethodPatch, "/api/projects/TRAX/issues/1", `{"title":"New","description":"changed"}`), http.StatusOK)

	r := env.do(t, c, http.MethodGet, "/api/audit-log", "")
	wantStatus(t, r, http.StatusOK)

	var log auditBody
	r.json(t, &log)

	if log.Total != 4 || len(log.Items) != 4 {
		t.Fatalf("audit log = %s", r.body)
	}

	wantIssueUpdate(t, log.Items[0])

	for i, want := range []string{"issue.update", "issue.create", "project.create", "auth.login"} {
		if log.Items[i].Action != want {
			t.Fatalf("entry %d action = %q, want %q", i, log.Items[i].Action, want)
		}
	}
}

// wantIssueUpdate checks the entry of renaming TRAX-1 from Old to New and
// changing its description.
func wantIssueUpdate(t *testing.T, e auditEntryBody) {
	t.Helper()

	if e.Action != "issue.update" || e.TargetLabel != "TRAX-1" || e.Actor == nil || e.Actor.DisplayName != "Ada" || e.IP == "" {
		t.Fatalf("newest entry = %+v", e)
	}

	title := e.Changes["title"]
	if title.Old == nil || *title.Old != "Old" || title.New == nil || *title.New != "New" {
		t.Fatalf("title change = %+v", title)
	}

	if d := e.Changes["description"]; !d.Redacted || d.Old != nil || d.New != nil {
		t.Fatalf("description change = %+v, want redacted", d)
	}
}

func TestAuditLog_FiltersAndPages(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.idp.groups = []string{testAdminGroup}
	c := newClient(t)
	env.login(t, c, "/")
	wantStatus(t, env.do(t, c, http.MethodPost, "/api/projects", `{"key":"TRAX","name":"IssueTrax"}`), http.StatusCreated)
	wantStatus(t, env.do(t, c, http.MethodPost, "/api/projects/TRAX/issues", `{"title":"x"}`), http.StatusCreated)

	var log auditBody

	r := env.do(t, c, http.MethodGet, "/api/audit-log?"+url.Values{"action": {"auth.login", "project.create"}, "limit": {"1"}}.Encode(), "")
	wantStatus(t, r, http.StatusOK)
	r.json(t, &log)

	if log.Total != 2 || len(log.Items) != 1 || log.Items[0].Action != "project.create" || log.NextCursor == nil {
		t.Fatalf("filtered page 1 = %s", r.body)
	}

	r = env.do(t, c, http.MethodGet, "/api/audit-log?"+url.Values{
		"action": {"auth.login", "project.create"}, "limit": {"1"}, "cursor": {*log.NextCursor},
	}.Encode(), "")
	wantStatus(t, r, http.StatusOK)
	r.json(t, &log)

	if len(log.Items) != 1 || log.Items[0].Action != "auth.login" || log.NextCursor != nil {
		t.Fatalf("filtered page 2 = %s", r.body)
	}

	r = env.do(t, c, http.MethodGet, "/api/audit-log?action=bogus", "")
	wantStatus(t, r, http.StatusUnprocessableEntity)

	r = env.do(t, c, http.MethodGet, "/api/audit-log?actor=not-a-uuid", "")
	wantStatus(t, r, http.StatusUnprocessableEntity)
}

func TestAuditLog_NonAdminIsForbidden(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.idp.groups = []string{"staff"}
	c := newClient(t)
	env.login(t, c, "/")

	r := env.do(t, c, http.MethodGet, "/api/audit-log", "")
	wantStatus(t, r, http.StatusForbidden)

	if r.errorCode(t) != "admin_required" {
		t.Fatalf("body = %s", r.body)
	}

	wantStatus(t, env.do(t, newClient(t), http.MethodGet, "/api/audit-log", ""), http.StatusUnauthorized)
}

func TestAuditLog_ServiceAccountIsForbidden(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.idp.groups = []string{testAdminGroup}
	c := newClient(t)
	env.login(t, c, "/")

	_, key := env.newBot(t, c, "ci")

	r := env.do(t, newClient(t), http.MethodGet, "/api/audit-log", "", "Authorization", "Bearer "+key.Token)
	wantStatus(t, r, http.StatusForbidden)

	if r.errorCode(t) != "session_required" {
		t.Fatalf("body = %s", r.body)
	}
}
