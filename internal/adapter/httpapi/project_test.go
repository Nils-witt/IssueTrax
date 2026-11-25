package httpapi_test

import (
	"net/http"
	"testing"
)

type projectJSON struct {
	ID, Key, Name, Description string
}

type pageJSON[T any] struct {
	Items      []T     `json:"items"`
	Total      int64   `json:"total"`
	NextCursor *string `json:"nextCursor"`
}

// signedIn returns an env plus a client that is logged in as Ada.
func signedIn(t *testing.T) (*testEnv, *http.Client) {
	t.Helper()

	env := newTestEnv(t)
	c := newClient(t)
	env.login(t, c, "/")

	return env, c
}

func (e *testEnv) createProject(t *testing.T, c *http.Client, body string) projectJSON {
	t.Helper()

	r := e.do(t, c, http.MethodPost, "/api/projects", body)
	wantStatus(t, r, http.StatusCreated)

	var p projectJSON
	r.json(t, &p)

	return p
}

func TestProjects_RequireSession(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	for _, path := range []string{"/api/projects", "/api/projects/TRAX", "/api/projects/TRAX/issues", "/api/projects/TRAX/tags", "/api/users"} {
		r := env.do(t, newClient(t), http.MethodGet, path, "")
		if r.status != http.StatusUnauthorized || r.errorCode(t) != "unauthenticated" {
			t.Errorf("GET %s = %d %s, want 401 unauthenticated", path, r.status, r.body)
		}
	}
}

func TestProjects_Create(t *testing.T) {
	t.Parallel()

	env, c := signedIn(t)

	p := env.createProject(t, c, `{"key":"trax","name":"IssueTrax","description":"Tracker"}`)
	if p.Key != "TRAX" || p.Name != "IssueTrax" || p.Description != "Tracker" || p.ID == "" {
		t.Fatalf("created = %+v", p)
	}

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{name: "duplicate key", body: `{"key":"TRAX","name":"Other"}`, wantStatus: http.StatusConflict, wantCode: "project_key_taken"},
		{name: "duplicate name", body: `{"key":"OTHER","name":"issuetrax"}`, wantStatus: http.StatusConflict, wantCode: "project_name_taken"},
		{name: "invalid", body: `{"key":"1","name":""}`, wantStatus: http.StatusUnprocessableEntity, wantCode: "validation_failed"},
		{name: "malformed json", body: `{`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "unknown field", body: `{"key":"AB","name":"n","color":"red"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "trailing data", body: `{"key":"AB","name":"n"}{}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := env.do(t, c, http.MethodPost, "/api/projects", tt.body)
			wantStatus(t, r, tt.wantStatus)

			if got := r.errorCode(t); got != tt.wantCode {
				t.Fatalf("code = %q, want %q (%s)", got, tt.wantCode, r.body)
			}
		})
	}

	t.Run("validation fields", func(t *testing.T) {
		t.Parallel()

		r := env.do(t, c, http.MethodPost, "/api/projects", `{"key":"1","name":""}`)

		var body struct {
			Error struct {
				Fields map[string]string `json:"fields"`
			} `json:"error"`
		}
		r.json(t, &body)

		if body.Error.Fields["key"] != "invalid" || body.Error.Fields["name"] != "required" {
			t.Fatalf("fields = %v", body.Error.Fields)
		}
	})
}

func TestProjects_List(t *testing.T) {
	t.Parallel()

	env, c := signedIn(t)
	env.createProject(t, c, `{"key":"BBB","name":"Beta"}`)
	env.createProject(t, c, `{"key":"AAA","name":"Alpha"}`)

	var first pageJSON[projectJSON]

	r := env.do(t, c, http.MethodGet, "/api/projects?limit=1", "")
	wantStatus(t, r, http.StatusOK)
	r.json(t, &first)

	if len(first.Items) != 1 || first.Items[0].Name != "Alpha" || first.Total != 2 || first.NextCursor == nil {
		t.Fatalf("first page = %+v", first)
	}

	var second pageJSON[projectJSON]

	r = env.do(t, c, http.MethodGet, "/api/projects?limit=1&cursor="+*first.NextCursor, "")
	wantStatus(t, r, http.StatusOK)
	r.json(t, &second)

	if len(second.Items) != 1 || second.Items[0].Name != "Beta" || second.NextCursor != nil {
		t.Fatalf("second page = %+v", second)
	}

	for _, q := range []string{"limit=abc", "limit=101", "cursor=%25%25"} {
		wantStatus(t, env.do(t, c, http.MethodGet, "/api/projects?"+q, ""), http.StatusUnprocessableEntity)
	}
}

func TestProjects_GetAndUpdate(t *testing.T) {
	t.Parallel()

	env, c := signedIn(t)
	env.createProject(t, c, `{"key":"TRAX","name":"IssueTrax"}`)

	wantStatus(t, env.do(t, c, http.MethodGet, "/api/projects/trax", ""), http.StatusOK)

	missing := env.do(t, c, http.MethodGet, "/api/projects/NOPE", "")
	if missing.status != http.StatusNotFound || missing.errorCode(t) != "project_not_found" {
		t.Fatalf("GET missing = %d %s", missing.status, missing.body)
	}

	r := env.do(t, c, http.MethodPatch, "/api/projects/TRAX", `{"name":"Renamed","description":"new"}`)
	wantStatus(t, r, http.StatusOK)

	var p projectJSON
	r.json(t, &p)

	if p.Name != "Renamed" || p.Description != "new" || p.Key != "TRAX" {
		t.Fatalf("PATCH = %+v", p)
	}

	r = env.do(t, c, http.MethodPatch, "/api/projects/TRAX", `{"key":"NEW"}`)
	wantStatus(t, r, http.StatusUnprocessableEntity)

	if !contains(r.body, `"key":"immutable"`) {
		t.Fatalf("PATCH key = %s", r.body)
	}
}
