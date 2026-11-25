package httpapi_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

type issueJSON struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Number      int64  `json:"number"`
	ProjectKey  string `json:"projectKey"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Tags        []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"tags"`
	Assignees []struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
	} `json:"assignees"`
	CreatedBy struct {
		DisplayName string `json:"displayName"`
	} `json:"createdBy"`
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func (e *testEnv) me(t *testing.T, c *http.Client) string {
	t.Helper()

	var me struct {
		ID string `json:"id"`
	}
	e.do(t, c, http.MethodGet, "/api/me", "").json(t, &me)

	return me.ID
}

func (e *testEnv) createIssue(t *testing.T, c *http.Client, body string) issueJSON {
	t.Helper()

	r := e.do(t, c, http.MethodPost, "/api/projects/TRAX/issues", body)
	wantStatus(t, r, http.StatusCreated)

	var i issueJSON
	r.json(t, &i)

	return i
}

func issueEnv(t *testing.T) (*testEnv, *http.Client) {
	t.Helper()

	env, c := signedIn(t)
	env.createProject(t, c, `{"key":"TRAX","name":"IssueTrax"}`)

	return env, c
}

func TestIssues_CreateAndGet(t *testing.T) {
	t.Parallel()

	env, c := issueEnv(t)
	me := env.me(t, c)

	i := env.createIssue(t, c, `{"title":"Login fails","description":"**bold**","tags":["Needs Review","bug"],"assignees":["`+me+`"]}`)

	if i.Key != "TRAX-1" || i.Number != 1 || i.ProjectKey != "TRAX" || i.Status != "open" || i.CreatedBy.DisplayName != "Ada" {
		t.Fatalf("created = %+v", i)
	}

	if len(i.Tags) != 2 || i.Tags[0].Name != "bug" || i.Tags[1].Name != "needs-review" || len(i.Assignees) != 1 || i.Assignees[0].ID != me {
		t.Fatalf("created relations = %+v", i)
	}

	r := env.do(t, c, http.MethodGet, "/api/projects/trax/issues/1", "")
	wantStatus(t, r, http.StatusOK)

	var got issueJSON
	r.json(t, &got)

	if got.ID != i.ID || got.Description != "**bold**" {
		t.Fatalf("GET = %+v", got)
	}
}

func TestIssues_GetNotFound(t *testing.T) {
	t.Parallel()

	env, c := issueEnv(t)
	env.createIssue(t, c, `{"title":"x"}`)

	for _, path := range []string{"/api/projects/TRAX/issues/2", "/api/projects/TRAX/issues/abc", "/api/projects/TRAX/issues/0"} {
		r := env.do(t, c, http.MethodGet, path, "")
		if r.status != http.StatusNotFound || r.errorCode(t) != "issue_not_found" {
			t.Errorf("GET %s = %d %s", path, r.status, r.body)
		}
	}
}

func TestIssues_CreateErrors(t *testing.T) {
	t.Parallel()

	env, c := issueEnv(t)

	tests := []struct {
		name, path, body string
		status           int
		field            string
	}{
		{name: "missing title", path: "/api/projects/TRAX/issues", body: `{}`, status: http.StatusUnprocessableEntity, field: `"title":"required"`},
		{name: "bad status", path: "/api/projects/TRAX/issues", body: `{"title":"x","status":"done"}`, status: http.StatusUnprocessableEntity, field: `"status":"invalid"`},
		{name: "bad tag", path: "/api/projects/TRAX/issues", body: `{"title":"x","tags":["#"]}`, status: http.StatusUnprocessableEntity, field: `"tags":"invalid"`},
		{
			name: "malformed assignee", path: "/api/projects/TRAX/issues", body: `{"title":"x","assignees":["nope"]}`,
			status: http.StatusUnprocessableEntity, field: `"assignees":"invalid"`,
		},
		{
			name: "unknown assignee", path: "/api/projects/TRAX/issues", body: `{"title":"x","assignees":["01920000-0000-7000-8000-000000000000"]}`,
			status: http.StatusUnprocessableEntity, field: `"assignees":"unknown"`,
		},
		{name: "unknown project", path: "/api/projects/NOPE/issues", body: `{"title":"x"}`, status: http.StatusNotFound, field: `"project_not_found"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := env.do(t, c, http.MethodPost, tt.path, tt.body)
			wantStatus(t, r, tt.status)

			if !contains(r.body, tt.field) {
				t.Fatalf("body = %s, want %s", r.body, tt.field)
			}
		})
	}
}

func TestIssues_List(t *testing.T) {
	t.Parallel()

	env, c := issueEnv(t)
	me := env.me(t, c)

	first := env.createIssue(t, c, `{"title":"Login bug","tags":["bug","ui"],"assignees":["`+me+`"]}`)
	env.createIssue(t, c, `{"title":"Crash","status":"closed","tags":["bug"]}`)
	env.createIssue(t, c, `{"title":"Dark mode","status":"draft","tags":["ui"]}`)
	env.createIssue(t, c, `{"title":"Old","status":"archived"}`)

	bug, ui := first.Tags[0].ID, first.Tags[1].ID

	tests := []struct {
		query string
		want  []string
	}{
		{query: "", want: []string{"TRAX-3", "TRAX-2", "TRAX-1"}},
		{query: "status=archived", want: []string{"TRAX-4"}},
		{query: "status=open&status=closed", want: []string{"TRAX-2", "TRAX-1"}},
		{query: "tag=" + bug + "&tag=" + ui, want: []string{"TRAX-1"}},
		{query: "tag=" + bug + "&tag=" + ui + "&tagMode=any", want: []string{"TRAX-3", "TRAX-2", "TRAX-1"}},
		{query: "assignee=" + me, want: []string{"TRAX-1"}},
		{query: "q=DARK", want: []string{"TRAX-3"}},
		{query: "limit=1", want: []string{"TRAX-3"}},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			t.Parallel()

			r := env.do(t, c, http.MethodGet, "/api/projects/TRAX/issues?"+tt.query, "")
			wantStatus(t, r, http.StatusOK)

			var page pageJSON[issueJSON]
			r.json(t, &page)

			keys := make([]string, 0, len(page.Items))
			for _, i := range page.Items {
				keys = append(keys, i.Key)
			}

			if !slices.Equal(keys, tt.want) {
				t.Fatalf("keys = %v, want %v", keys, tt.want)
			}
		})
	}

	for _, q := range []string{"tag=nope", "assignee=nope", "tagMode=some", "assigneeMode=some", "status=done", "limit=0x"} {
		wantStatus(t, env.do(t, c, http.MethodGet, "/api/projects/TRAX/issues?"+q, ""), http.StatusUnprocessableEntity)
	}
}

func TestIssues_Update(t *testing.T) {
	t.Parallel()

	env, c := issueEnv(t)
	env.createIssue(t, c, `{"title":"Old","tags":["bug"]}`)

	patch := func(body string) response {
		t.Helper()

		return env.do(t, c, http.MethodPatch, "/api/projects/TRAX/issues/1", body)
	}

	r := patch(`{"title":"New","tags":[]}`)
	wantStatus(t, r, http.StatusOK)

	var i issueJSON
	r.json(t, &i)

	if i.Title != "New" || len(i.Tags) != 0 {
		t.Fatalf("PATCH = %+v", i)
	}

	wantStatus(t, patch(`{"status":"archived"}`), http.StatusOK)

	archived := patch(`{"title":"Edited"}`)
	if archived.status != http.StatusConflict || archived.errorCode(t) != "issue_archived" {
		t.Fatalf("PATCH archived = %d %s", archived.status, archived.body)
	}

	wantStatus(t, patch(`{"status":"open"}`), http.StatusOK)
	wantStatus(t, patch(`{"title":"Edited"}`), http.StatusOK)
	wantStatus(t, patch(`{"assignees":["nope"]}`), http.StatusUnprocessableEntity)
	wantStatus(t, patch(`{"number":5}`), http.StatusBadRequest)
	wantStatus(t, env.do(t, c, http.MethodPatch, "/api/projects/TRAX/issues/x", `{}`), http.StatusNotFound)
}
