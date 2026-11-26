package httpapi_test

import (
	"net/http"
	"net/url"
	"testing"
)

type storageJSON struct {
	ID         string  `json:"id"`
	ParentID   *string `json:"parentId"`
	Name       string  `json:"name"`
	ChildCount int64   `json:"childCount"`
}

func (e *testEnv) createStorage(t *testing.T, c *http.Client, body string) storageJSON {
	t.Helper()

	r := e.do(t, c, http.MethodPost, "/api/storages", body)
	wantStatus(t, r, http.StatusCreated)

	var s storageJSON
	r.json(t, &s)

	return s
}

func (e *testEnv) listStorages(t *testing.T, c *http.Client, query string) pageJSON[storageJSON] {
	t.Helper()

	r := e.do(t, c, http.MethodGet, "/api/storages"+query, "")
	wantStatus(t, r, http.StatusOK)

	var p pageJSON[storageJSON]
	r.json(t, &p)

	return p
}

func TestStorages_RequireSession(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	for _, path := range []string{"/api/storages", "/api/storages/0190d2a4-7f3e-7c4b-9e3a-2b1c0d9e8f7a"} {
		r := env.do(t, newClient(t), http.MethodGet, path, "")
		if r.status != http.StatusUnauthorized || r.errorCode(t) != "unauthenticated" {
			t.Errorf("GET %s = %d %s, want 401 unauthenticated", path, r.status, r.body)
		}
	}
}

// storageTree creates the roots Warehouse (holding Shelf) and Annex.
func storageTree(t *testing.T, env *testEnv, c *http.Client) (root, shelf storageJSON) {
	t.Helper()

	root = env.createStorage(t, c, `{"name":" Warehouse "}`)
	if root.ID == "" || root.Name != "Warehouse" || root.ParentID != nil || root.ChildCount != 0 {
		t.Fatalf("created root = %+v", root)
	}

	shelf = env.createStorage(t, c, `{"name":"Shelf","parentId":"`+root.ID+`"}`)
	if shelf.ParentID == nil || *shelf.ParentID != root.ID {
		t.Fatalf("created child = %+v", shelf)
	}

	env.createStorage(t, c, `{"name":"Annex","parentId":null}`)

	return root, shelf
}

func TestStorages_GetAndList(t *testing.T) {
	t.Parallel()

	env, c := signedIn(t)
	root, shelf := storageTree(t, env, c)

	r := env.do(t, c, http.MethodGet, "/api/storages/"+root.ID, "")
	wantStatus(t, r, http.StatusOK)

	var got storageJSON
	r.json(t, &got)

	if got.Name != "Warehouse" || got.ChildCount != 1 {
		t.Fatalf("GET = %+v", got)
	}

	roots := env.listStorages(t, c, "?limit=1")
	if roots.Total != 2 || len(roots.Items) != 1 || roots.Items[0].Name != "Annex" || roots.NextCursor == nil {
		t.Fatalf("roots page 1 = %+v", roots)
	}

	roots = env.listStorages(t, c, "?limit=1&cursor="+url.QueryEscape(*roots.NextCursor))
	if len(roots.Items) != 1 || roots.Items[0].Name != "Warehouse" || roots.NextCursor != nil {
		t.Fatalf("roots page 2 = %+v", roots)
	}

	children := env.listStorages(t, c, "?parent="+root.ID)
	if children.Total != 1 || children.Items[0].ID != shelf.ID {
		t.Fatalf("children = %+v", children)
	}
}

func TestStorages_Errors(t *testing.T) {
	t.Parallel()

	env, c := signedIn(t)
	storageTree(t, env, c)

	tests := []struct {
		name, method, path, body string
		wantStatus               int
		wantCode                 string
	}{
		{"duplicate root", http.MethodPost, "/api/storages", `{"name":"warehouse"}`, http.StatusConflict, "storage_name_taken"},
		{"empty name", http.MethodPost, "/api/storages", `{"name":""}`, http.StatusUnprocessableEntity, "validation_failed"},
		{"bad parent id", http.MethodPost, "/api/storages", `{"name":"x","parentId":"nope"}`, http.StatusUnprocessableEntity, "validation_failed"},
		{
			"unknown parent", http.MethodPost, "/api/storages",
			`{"name":"x","parentId":"0190d2a4-7f3e-7c4b-9e3a-2b1c0d9e8f7a"}`, http.StatusUnprocessableEntity, "validation_failed",
		},
		{"unknown field", http.MethodPost, "/api/storages", `{"name":"x","color":"red"}`, http.StatusBadRequest, "invalid_json"},
		{"get malformed id", http.MethodGet, "/api/storages/nope", "", http.StatusNotFound, "storage_not_found"},
		{"get unknown", http.MethodGet, "/api/storages/0190d2a4-7f3e-7c4b-9e3a-2b1c0d9e8f7a", "", http.StatusNotFound, "storage_not_found"},
		{"list bad parent", http.MethodGet, "/api/storages?parent=nope", "", http.StatusUnprocessableEntity, "validation_failed"},
		{
			"list unknown parent", http.MethodGet, "/api/storages?parent=0190d2a4-7f3e-7c4b-9e3a-2b1c0d9e8f7a", "",
			http.StatusNotFound, "storage_not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := env.do(t, c, tt.method, tt.path, tt.body)
			if r.status != tt.wantStatus || r.errorCode(t) != tt.wantCode {
				t.Fatalf("%s %s = %d %s, want %d %s", tt.method, tt.path, r.status, r.body, tt.wantStatus, tt.wantCode)
			}
		})
	}
}

func TestStorages_Update(t *testing.T) {
	t.Parallel()

	env, c := signedIn(t)

	a := env.createStorage(t, c, `{"name":"A"}`)
	b := env.createStorage(t, c, `{"name":"B"}`)
	box := env.createStorage(t, c, `{"name":"Box","parentId":"`+a.ID+`"}`)

	patch := func(t *testing.T, id, body string) storageJSON {
		t.Helper()

		r := env.do(t, c, http.MethodPatch, "/api/storages/"+id, body)
		wantStatus(t, r, http.StatusOK)

		var s storageJSON
		r.json(t, &s)

		return s
	}

	// Renaming alone leaves the parent as it is.
	got := patch(t, box.ID, `{"name":"Crate"}`)
	if got.Name != "Crate" || got.ParentID == nil || *got.ParentID != a.ID {
		t.Fatalf("after rename = %+v", got)
	}

	got = patch(t, box.ID, `{"parentId":"`+b.ID+`"}`)
	if got.ParentID == nil || *got.ParentID != b.ID {
		t.Fatalf("after move = %+v", got)
	}

	got = patch(t, box.ID, `{"parentId":null}`)
	if got.ParentID != nil {
		t.Fatalf("after move to root = %+v", got)
	}

	got = patch(t, b.ID, `{"parentId":"`+a.ID+`"}`)
	if got.ParentID == nil || *got.ParentID != a.ID {
		t.Fatalf("after moving B under A = %+v", got)
	}

	tests := []struct {
		name, id, body string
		wantStatus     int
		wantCode       string
	}{
		{"into own child", a.ID, `{"parentId":"` + b.ID + `"}`, http.StatusConflict, "storage_cycle"},
		{"under itself", a.ID, `{"parentId":"` + a.ID + `"}`, http.StatusUnprocessableEntity, "validation_failed"},
		{"name taken", box.ID, `{"name":"a"}`, http.StatusConflict, "storage_name_taken"},
		{"bad parent id", box.ID, `{"parentId":"nope"}`, http.StatusUnprocessableEntity, "validation_failed"},
		{"unknown storage", "0190d2a4-7f3e-7c4b-9e3a-2b1c0d9e8f7a", `{"name":"x"}`, http.StatusNotFound, "storage_not_found"},
		{"unknown field", box.ID, `{"childCount":1}`, http.StatusBadRequest, "invalid_json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := env.do(t, c, http.MethodPatch, "/api/storages/"+tt.id, tt.body)
			if r.status != tt.wantStatus || r.errorCode(t) != tt.wantCode {
				t.Fatalf("PATCH = %d %s, want %d %s", r.status, r.body, tt.wantStatus, tt.wantCode)
			}
		})
	}
}
