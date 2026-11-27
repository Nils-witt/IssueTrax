package httpapi_test

import (
	"net/http"
	"testing"
)

type moduleDefaultJSON struct {
	Module string `json:"module"`
	Count  int    `json:"count"`
}

type itemTypeRefJSON struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Deleted bool   `json:"deleted"`
}

type itemTypeJSON struct {
	ID                 string              `json:"id"`
	ParentID           *string             `json:"parentId"`
	Name               string              `json:"name"`
	TagPrefix          string              `json:"tagPrefix"`
	Defaults           []moduleDefaultJSON `json:"defaults"`
	ChildCount         int64               `json:"childCount"`
	ItemCount          int64               `json:"itemCount"`
	DeletedAt          *string             `json:"deletedAt"`
	Path               []itemTypeRefJSON   `json:"path"`
	EffectiveDefaults  []moduleDefaultJSON `json:"effectiveDefaults"`
	EffectiveTagPrefix string              `json:"effectiveTagPrefix"`
}

// signedInAdmin is a test environment with an admin signed in.
func signedInAdmin(t *testing.T) (*testEnv, *http.Client) {
	t.Helper()

	env := newTestEnv(t)
	env.idp.groups = []string{testAdminGroup}
	c := newClient(t)
	env.login(t, c, "/")

	return env, c
}

func (e *testEnv) createItemType(t *testing.T, c *http.Client, body string) itemTypeJSON {
	t.Helper()

	r := e.do(t, c, http.MethodPost, "/api/item-types", body)
	wantStatus(t, r, http.StatusCreated)

	var typ itemTypeJSON
	r.json(t, &typ)

	return typ
}

func TestItemTypes_RequireSession(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	r := env.do(t, newClient(t), http.MethodGet, "/api/item-types", "")
	if r.status != http.StatusUnauthorized {
		t.Fatalf("GET /api/item-types = %d", r.status)
	}
}

func TestItemTypes_AdminOnlyChanges(t *testing.T) {
	t.Parallel()

	env, c := signedIn(t)

	r := env.do(t, c, http.MethodPost, "/api/item-types", `{"name":"Switch"}`)
	if r.status != http.StatusForbidden || r.errorCode(t) != "admin_required" {
		t.Fatalf("POST as non-admin = %d %s", r.status, r.body)
	}

	wantStatus(t, env.do(t, c, http.MethodGet, "/api/item-types", ""), http.StatusOK)
}

func TestItemTypes_TreeAndInheritance(t *testing.T) {
	t.Parallel()

	env, c := signedInAdmin(t)
	network := env.createItemType(t, c, `{"name":"Network","tagPrefix":"NET-","defaults":[{"module":"hardware","count":1}]}`)
	sw := env.createItemType(t, c, `{"name":"Switch","parentId":"`+network.ID+`","defaults":[{"module":"network_interface","count":24}]}`)

	if len(sw.Path) != 1 || sw.Path[0].ID != network.ID || sw.EffectiveTagPrefix != "NET-" || len(sw.EffectiveDefaults) != 2 ||
		sw.EffectiveDefaults[1] != (moduleDefaultJSON{Module: "network_interface", Count: 24}) {
		t.Fatalf("switch = %+v", sw)
	}

	var page pageJSON[itemTypeJSON]

	r := env.do(t, c, http.MethodGet, "/api/item-types", "")
	wantStatus(t, r, http.StatusOK)
	r.json(t, &page)

	if len(page.Items) != 1 || page.Items[0].Name != "Network" || page.Items[0].ChildCount != 1 {
		t.Fatalf("roots = %+v", page.Items)
	}

	r = env.do(t, c, http.MethodGet, "/api/item-types?parent="+network.ID, "")
	r.json(t, &page)

	if len(page.Items) != 1 || page.Items[0].ID != sw.ID {
		t.Fatalf("children = %+v", page.Items)
	}

	r = env.do(t, c, http.MethodGet, "/api/item-types?q=WIT", "")
	r.json(t, &page)

	if len(page.Items) != 1 || len(page.Items[0].Path) != 1 {
		t.Fatalf("search = %+v", page.Items)
	}
}

func TestItemTypes_UpdateAndNextAssetTag(t *testing.T) {
	t.Parallel()

	env, c := signedInAdmin(t)
	network := env.createItemType(t, c, `{"name":"Network","tagPrefix":"NET-"}`)
	sw := env.createItemType(t, c, `{"name":"Switch","parentId":"`+network.ID+`"}`)

	var tag struct {
		AssetTag string `json:"assetTag"`
	}

	r := env.do(t, c, http.MethodGet, "/api/item-types/"+sw.ID+"/next-asset-tag", "")
	wantStatus(t, r, http.StatusOK)
	r.json(t, &tag)

	if tag.AssetTag != "NET-00001" {
		t.Fatalf("next asset tag = %q", tag.AssetTag)
	}

	r = env.do(t, c, http.MethodPatch, "/api/item-types/"+sw.ID, `{"parentId":null,"tagPrefix":"SW-","defaults":[]}`)
	wantStatus(t, r, http.StatusOK)

	var moved itemTypeJSON
	r.json(t, &moved)

	if moved.ParentID != nil || moved.EffectiveTagPrefix != "SW-" || len(moved.Defaults) != 0 {
		t.Fatalf("moved = %+v", moved)
	}
}

func TestItemTypes_DeleteAndRestore(t *testing.T) {
	t.Parallel()

	env, c := signedInAdmin(t)
	typ := env.createItemType(t, c, `{"name":"Chair"}`)

	wantStatus(t, env.do(t, c, http.MethodDelete, "/api/item-types/"+typ.ID, ""), http.StatusNoContent)

	r := env.do(t, c, http.MethodPost, "/api/item-types", `{"name":"x","parentId":"`+typ.ID+`"}`)
	if r.status != http.StatusConflict || r.errorCode(t) != "item_type_deleted" {
		t.Fatalf("create under deleted = %d %s", r.status, r.body)
	}

	var page pageJSON[itemTypeJSON]

	env.do(t, c, http.MethodGet, "/api/item-types?includeDeleted=true", "").json(t, &page)

	if len(page.Items) != 1 || page.Items[0].DeletedAt == nil {
		t.Fatalf("deleted list = %+v", page.Items)
	}

	r = env.do(t, c, http.MethodPost, "/api/item-types/"+typ.ID+"/restore", "")
	wantStatus(t, r, http.StatusOK)
}

func TestItemTypes_Errors(t *testing.T) {
	t.Parallel()

	env, c := signedInAdmin(t)
	env.createItemType(t, c, `{"name":"Chair"}`)

	tests := []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{"duplicate name", http.MethodPost, "/api/item-types", `{"name":"chair"}`, http.StatusConflict, "item_type_name_taken"},
		{"bad defaults", http.MethodPost, "/api/item-types", `{"name":"x","defaults":[{"module":"nope","count":1}]}`, http.StatusUnprocessableEntity, "validation_failed"},
		{"unknown parent", http.MethodPost, "/api/item-types", `{"name":"x","parentId":"0190d2a4-7f3e-7c4b-9e3a-2b1c0d9e8f7a"}`, http.StatusUnprocessableEntity, "validation_failed"},
		{"malformed id", http.MethodGet, "/api/item-types/nope", "", http.StatusNotFound, "item_type_not_found"},
		{"bad parent filter", http.MethodGet, "/api/item-types?parent=nope", "", http.StatusUnprocessableEntity, "validation_failed"},
		{"bad json", http.MethodPost, "/api/item-types", `{"nope":1}`, http.StatusBadRequest, "invalid_json"},
	}

	for _, tt := range tests {
		r := env.do(t, c, tt.method, tt.path, tt.body)
		if r.status != tt.status || r.errorCode(t) != tt.code {
			t.Errorf("%s: %d %s, want %d %s", tt.name, r.status, r.body, tt.status, tt.code)
		}
	}
}
