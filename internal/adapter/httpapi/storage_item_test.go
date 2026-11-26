package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

type genericItemJSON struct {
	ID          string  `json:"id"`
	StorageID   string  `json:"storageId"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	DeletedAt   *string `json:"deletedAt"`
}

func (e *testEnv) createItem(t *testing.T, c *http.Client, storageID, name, description string) genericItemJSON {
	t.Helper()

	r := e.do(t, c, http.MethodPost, "/api/generic-storage-items",
		`{"storageId":"`+storageID+`","name":"`+name+`","description":"`+description+`"}`)
	wantStatus(t, r, http.StatusCreated)

	var item genericItemJSON
	r.json(t, &item)

	return item
}

func (e *testEnv) listItems(t *testing.T, c *http.Client, path string) pageJSON[genericItemJSON] {
	t.Helper()

	r := e.do(t, c, http.MethodGet, path, "")
	wantStatus(t, r, http.StatusOK)

	var p pageJSON[genericItemJSON]
	r.json(t, &p)

	return p
}

func itemJSONNames(items []genericItemJSON) string {
	names := make([]string, 0, len(items))
	for _, i := range items {
		names = append(names, i.Name)
	}

	return strings.Join(names, ",")
}

func TestGenericStorageItems_RequireSession(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	id := "0190d2a4-7f3e-7c4b-9e3a-2b1c0d9e8f7a"

	for _, path := range []string{"/api/generic-storage-items", "/api/generic-storage-items/" + id, "/api/storages/" + id + "/generic-storage-items"} {
		r := env.do(t, newClient(t), http.MethodGet, path, "")
		if r.status != http.StatusUnauthorized || r.errorCode(t) != "unauthenticated" {
			t.Errorf("GET %s = %d %s, want 401 unauthenticated", path, r.status, r.body)
		}
	}
}

func TestGenericStorageItems_CreateListAndUpdate(t *testing.T) {
	t.Parallel()

	env, c := signedIn(t)
	root, shelf := storageTree(t, env, c)

	item := env.createItem(t, c, shelf.ID, " Drill ", "cordless")
	if item.ID == "" || item.Name != "Drill" || item.StorageID != shelf.ID || item.Description != "cordless" || item.DeletedAt != nil {
		t.Fatalf("created = %+v", item)
	}

	env.createItem(t, c, root.ID, "lamp", "")

	r := env.do(t, c, http.MethodGet, "/api/storages/"+shelf.ID, "")

	var st struct {
		ItemCount int64 `json:"itemCount"`
	}
	r.json(t, &st)

	if st.ItemCount != 1 {
		t.Fatalf("shelf itemCount = %d", st.ItemCount)
	}

	if got := itemJSONNames(env.listItems(t, c, "/api/storages/"+root.ID+"/generic-storage-items").Items); got != "lamp" {
		t.Fatalf("direct = %s", got)
	}

	if got := itemJSONNames(env.listItems(t, c, "/api/storages/"+root.ID+"/generic-storage-items?recursive=true").Items); got != "Drill,lamp" {
		t.Fatalf("recursive = %s", got)
	}

	if got := itemJSONNames(env.listItems(t, c, "/api/generic-storage-items?q=CORDLESS").Items); got != "Drill" {
		t.Fatalf("search = %s", got)
	}

	r = env.do(t, c, http.MethodPatch, "/api/generic-storage-items/"+item.ID, `{"name":"Hammer","storageId":"`+root.ID+`"}`)
	wantStatus(t, r, http.StatusOK)

	var moved genericItemJSON
	r.json(t, &moved)

	if moved.Name != "Hammer" || moved.StorageID != root.ID || moved.Description != "cordless" {
		t.Fatalf("patched = %+v", moved)
	}
}

func TestGenericStorageItems_DeleteAndRestore(t *testing.T) {
	t.Parallel()

	env, c := signedIn(t)
	root, _ := storageTree(t, env, c)
	item := env.createItem(t, c, root.ID, "Hammer", "")

	wantStatus(t, env.do(t, c, http.MethodDelete, "/api/generic-storage-items/"+item.ID, ""), http.StatusNoContent)

	r := env.do(t, c, http.MethodGet, "/api/generic-storage-items/"+item.ID, "")
	wantStatus(t, r, http.StatusOK)

	var deleted genericItemJSON
	r.json(t, &deleted)

	if deleted.DeletedAt == nil {
		t.Fatalf("GET deleted = %+v", deleted)
	}

	if got := itemJSONNames(env.listItems(t, c, "/api/storages/"+root.ID+"/generic-storage-items?deleted=true").Items); got != "Hammer" {
		t.Fatalf("deleted list = %s", got)
	}

	for _, tt := range []struct{ method, path, body string }{
		{http.MethodPatch, "/api/generic-storage-items/" + item.ID, `{"name":"x"}`},
		{http.MethodDelete, "/api/generic-storage-items/" + item.ID, ""},
	} {
		r := env.do(t, c, tt.method, tt.path, tt.body)
		if r.status != http.StatusConflict || r.errorCode(t) != "storage_item_deleted" {
			t.Fatalf("%s deleted item = %d %s", tt.method, r.status, r.body)
		}
	}

	r = env.do(t, c, http.MethodPost, "/api/generic-storage-items/"+item.ID+"/restore", "")
	wantStatus(t, r, http.StatusOK)

	var restored genericItemJSON
	r.json(t, &restored)

	if restored.DeletedAt != nil {
		t.Fatalf("restored = %+v", restored)
	}

	r = env.do(t, c, http.MethodPost, "/api/generic-storage-items/"+item.ID+"/restore", "")
	if r.status != http.StatusConflict || r.errorCode(t) != "storage_item_not_deleted" {
		t.Fatalf("restore live item = %d %s", r.status, r.body)
	}
}

func TestGenericStorageItems_Errors(t *testing.T) {
	t.Parallel()

	env, c := signedIn(t)
	_, shelf := storageTree(t, env, c)
	missing := "0190d2a4-7f3e-7c4b-9e3a-2b1c0d9e8f7a"

	tests := []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{"create in unknown storage", http.MethodPost, "/api/generic-storage-items", `{"storageId":"` + missing + `","name":"x"}`, http.StatusUnprocessableEntity, "validation_failed"},
		{"create with bad storage id", http.MethodPost, "/api/generic-storage-items", `{"storageId":"nope","name":"x"}`, http.StatusUnprocessableEntity, "validation_failed"},
		{"create without name", http.MethodPost, "/api/generic-storage-items", `{"storageId":"` + shelf.ID + `","name":""}`, http.StatusUnprocessableEntity, "validation_failed"},
		{"get unknown", http.MethodGet, "/api/generic-storage-items/" + missing, "", http.StatusNotFound, "storage_item_not_found"},
		{"get malformed id", http.MethodGet, "/api/generic-storage-items/nope", "", http.StatusNotFound, "storage_item_not_found"},
		{"list unknown storage", http.MethodGet, "/api/storages/" + missing + "/generic-storage-items", "", http.StatusNotFound, "storage_not_found"},
		{"bad recursive flag", http.MethodGet, "/api/storages/" + shelf.ID + "/generic-storage-items?recursive=yes", "", http.StatusUnprocessableEntity, "validation_failed"},
		{"bad deleted flag", http.MethodGet, "/api/generic-storage-items?deleted=1", "", http.StatusUnprocessableEntity, "validation_failed"},
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
