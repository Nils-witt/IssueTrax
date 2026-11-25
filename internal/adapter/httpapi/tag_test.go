package httpapi_test

import (
	"net/http"
	"testing"
)

func TestTags_ListAndRename(t *testing.T) {
	t.Parallel()

	env, c := issueEnv(t)
	i := env.createIssue(t, c, `{"title":"x","tags":["bug","ui"]}`)
	bug := i.Tags[0].ID

	r := env.do(t, c, http.MethodGet, "/api/projects/TRAX/tags", "")
	wantStatus(t, r, http.StatusOK)

	var tags []struct{ ID, Name string }
	r.json(t, &tags)

	if len(tags) != 2 || tags[0].Name != "bug" || tags[1].Name != "ui" {
		t.Fatalf("GET tags = %+v", tags)
	}

	r = env.do(t, c, http.MethodPatch, "/api/projects/TRAX/tags/"+bug, `{"name":"Defect Report"}`)
	wantStatus(t, r, http.StatusOK)

	if !contains(r.body, `"name":"defect-report"`) {
		t.Fatalf("rename = %s", r.body)
	}

	taken := env.do(t, c, http.MethodPatch, "/api/projects/TRAX/tags/"+bug, `{"name":"ui"}`)
	if taken.status != http.StatusConflict || taken.errorCode(t) != "tag_name_taken" {
		t.Fatalf("rename taken = %d %s", taken.status, taken.body)
	}

	wantStatus(t, env.do(t, c, http.MethodPatch, "/api/projects/TRAX/tags/not-a-uuid", `{"name":"x"}`), http.StatusNotFound)
	wantStatus(t, env.do(t, c, http.MethodPatch, "/api/projects/TRAX/tags/"+bug, `{"name":"#"}`), http.StatusUnprocessableEntity)
}
