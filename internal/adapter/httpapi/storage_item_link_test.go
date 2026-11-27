package httpapi_test

import (
	"net/http"
	"testing"
)

type storageItemRefJSON struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	StorageID string `json:"storageId"`
	Name      string `json:"name"`
	Deleted   bool   `json:"deleted"`
}

type linkedIssueJSON struct {
	Key          string               `json:"key"`
	StorageItems []storageItemRefJSON `json:"storageItems"`
}

func TestStorageItems_RequireSession(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	for _, path := range []string{"/api/storage-items", "/api/storage-items/0190d2a4-7f3e-7c4b-9e3a-2b1c0d9e8f7a/issues"} {
		r := env.do(t, newClient(t), http.MethodGet, path, "")
		if r.status != http.StatusUnauthorized || r.errorCode(t) != "unauthenticated" {
			t.Errorf("GET %s = %d %s, want 401 unauthenticated", path, r.status, r.body)
		}
	}
}

func TestStorageItems_SearchAcrossKinds(t *testing.T) {
	t.Parallel()

	env, c := signedIn(t)
	_, shelf := storageTree(t, env, c)
	env.createItem(t, c, shelf.ID, "Drill", "")
	env.createDevice(t, c, shelf.ID)

	r := env.do(t, c, http.MethodGet, "/api/storage-items", "")
	wantStatus(t, r, http.StatusOK)

	var page pageJSON[storageItemRefJSON]
	r.json(t, &page)

	kinds := map[string]bool{}
	for _, i := range page.Items {
		kinds[i.Kind] = true

		if i.StorageID != shelf.ID {
			t.Fatalf("item %+v not in shelf", i)
		}
	}

	if len(page.Items) != 2 || !kinds["generic"] || !kinds["network_device"] {
		t.Fatalf("search = %+v", page.Items)
	}

	r = env.do(t, c, http.MethodGet, "/api/storage-items?q=dri", "")
	r.json(t, &page)

	if len(page.Items) != 1 || page.Items[0].Name != "Drill" {
		t.Fatalf("query search = %+v", page.Items)
	}

	wantStatus(t, env.do(t, c, http.MethodGet, "/api/storage-items?deleted=maybe", ""), http.StatusUnprocessableEntity)
}

func TestIssues_StorageItemLinks(t *testing.T) {
	t.Parallel()

	env, c := issueEnv(t)
	_, shelf := storageTree(t, env, c)
	drill := env.createItem(t, c, shelf.ID, "Drill", "")
	saw := env.createItem(t, c, shelf.ID, "Saw", "")

	r := env.do(t, c, http.MethodPost, "/api/projects/TRAX/issues", `{"title":"Broken","storageItems":["`+drill.ID+`"]}`)
	wantStatus(t, r, http.StatusCreated)

	var created linkedIssueJSON
	r.json(t, &created)

	if len(created.StorageItems) != 1 || created.StorageItems[0] != (storageItemRefJSON{
		ID: drill.ID, Kind: "generic", StorageID: shelf.ID, Name: "Drill",
	}) {
		t.Fatalf("created storage items = %+v", created.StorageItems)
	}

	r = env.do(t, c, http.MethodPatch, "/api/projects/TRAX/issues/1", `{"storageItems":["`+drill.ID+`","`+saw.ID+`"]}`)
	wantStatus(t, r, http.StatusOK)

	env.createIssue(t, c, `{"title":"Other"}`)

	r = env.do(t, c, http.MethodGet, "/api/projects/TRAX/issues?storageItem="+saw.ID, "")
	wantStatus(t, r, http.StatusOK)

	var listed pageJSON[linkedIssueJSON]
	r.json(t, &listed)

	if len(listed.Items) != 1 || listed.Items[0].Key != "TRAX-1" {
		t.Fatalf("filtered list = %+v", listed.Items)
	}

	r = env.do(t, c, http.MethodGet, "/api/storage-items/"+saw.ID+"/issues", "")
	wantStatus(t, r, http.StatusOK)
	r.json(t, &listed)

	if len(listed.Items) != 1 || listed.Items[0].Key != "TRAX-1" || len(listed.Items[0].StorageItems) != 2 {
		t.Fatalf("linked issues = %+v", listed.Items)
	}

	tests := []struct {
		name, method, path, body, code string
		status                         int
	}{
		{
			name: "unknown item", method: http.MethodPatch, path: "/api/projects/TRAX/issues/1",
			body: `{"storageItems":["0190d2a4-7f3e-7c4b-9e3a-2b1c0d9e8f7a"]}`, status: http.StatusUnprocessableEntity, code: "validation_failed",
		},
		{
			name: "malformed item", method: http.MethodPost, path: "/api/projects/TRAX/issues",
			body: `{"title":"x","storageItems":["nope"]}`, status: http.StatusUnprocessableEntity, code: "validation_failed",
		},
		{
			name: "malformed filter", method: http.MethodGet, path: "/api/projects/TRAX/issues?storageItem=nope",
			status: http.StatusUnprocessableEntity, code: "validation_failed",
		},
		{
			name: "issues of unknown item", method: http.MethodGet, path: "/api/storage-items/0190d2a4-7f3e-7c4b-9e3a-2b1c0d9e8f7a/issues",
			status: http.StatusNotFound, code: "storage_item_not_found",
		},
		{
			name: "issues of malformed item", method: http.MethodGet, path: "/api/storage-items/nope/issues",
			status: http.StatusNotFound, code: "storage_item_not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := env.do(t, c, tt.method, tt.path, tt.body)
			if r.status != tt.status || r.errorCode(t) != tt.code {
				t.Fatalf("%s %s = %d %s, want %d %s", tt.method, tt.path, r.status, r.body, tt.status, tt.code)
			}
		})
	}
}
