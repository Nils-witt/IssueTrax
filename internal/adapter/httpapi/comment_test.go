package httpapi_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

type commentJSON struct {
	ID     string `json:"id"`
	Body   string `json:"body"`
	Author struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
	} `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

const commentsPath = "/api/projects/TRAX/issues/1/comments"

func (e *testEnv) createComment(t *testing.T, c *http.Client, body string) commentJSON {
	t.Helper()

	r := e.do(t, c, http.MethodPost, commentsPath, body)
	wantStatus(t, r, http.StatusCreated)

	var cm commentJSON
	r.json(t, &cm)

	return cm
}

func (e *testEnv) listComments(t *testing.T, c *http.Client) []commentJSON {
	t.Helper()

	r := e.do(t, c, http.MethodGet, commentsPath, "")
	wantStatus(t, r, http.StatusOK)

	var list []commentJSON
	r.json(t, &list)

	return list
}

func TestComments_Lifecycle(t *testing.T) {
	t.Parallel()

	env, c := issueEnv(t)
	env.createIssue(t, c, `{"title":"x"}`)

	if r := env.do(t, c, http.MethodGet, commentsPath, ""); r.body != "[]\n" {
		t.Fatalf("empty list = %q, want []", r.body)
	}

	cm := env.createComment(t, c, `{"body":"  **LGTM**  "}`)
	if cm.Body != "**LGTM**" || cm.Author.DisplayName != "Ada" || cm.Author.ID != env.me(t, c) || cm.ID == "" {
		t.Fatalf("created = %+v", cm)
	}

	r := env.do(t, c, http.MethodPatch, commentsPath+"/"+cm.ID, `{"body":"edited"}`)
	wantStatus(t, r, http.StatusOK)

	if list := env.listComments(t, c); len(list) != 1 || list[0].Body != "edited" {
		t.Fatalf("list after edit = %+v", list)
	}

	wantStatus(t, env.do(t, c, http.MethodDelete, commentsPath+"/"+cm.ID, ""), http.StatusNoContent)

	if list := env.listComments(t, c); len(list) != 0 {
		t.Fatalf("list after delete = %+v", list)
	}
}

func TestComments_Errors(t *testing.T) {
	t.Parallel()

	env, c := issueEnv(t)
	env.createIssue(t, c, `{"title":"x"}`)
	cm := env.createComment(t, c, `{"body":"first"}`)
	one := commentsPath + "/" + cm.ID

	tests := []struct {
		name, method, path, body string
		wantStatus               int
		wantCode                 string
	}{
		{name: "empty body", method: http.MethodPost, path: commentsPath, body: `{"body":" "}`, wantStatus: http.StatusUnprocessableEntity, wantCode: "validation_failed"},
		{name: "unknown field", method: http.MethodPost, path: commentsPath, body: `{"text":"x"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "unknown issue", method: http.MethodPost, path: "/api/projects/TRAX/issues/9/comments", body: `{"body":"x"}`, wantStatus: http.StatusNotFound, wantCode: "issue_not_found"},
		{name: "bad issue number", method: http.MethodGet, path: "/api/projects/TRAX/issues/x/comments", wantStatus: http.StatusNotFound, wantCode: "issue_not_found"},
		{name: "unknown project", method: http.MethodGet, path: "/api/projects/NOPE/issues/1/comments", wantStatus: http.StatusNotFound, wantCode: "project_not_found"},
		{name: "bad comment id", method: http.MethodPatch, path: commentsPath + "/x", body: `{"body":"x"}`, wantStatus: http.StatusNotFound, wantCode: "comment_not_found"},
		{name: "unknown comment", method: http.MethodDelete, path: commentsPath + "/" + uuid.NewString(), wantStatus: http.StatusNotFound, wantCode: "comment_not_found"},
		{name: "edit to empty", method: http.MethodPatch, path: one, body: `{"body":""}`, wantStatus: http.StatusUnprocessableEntity, wantCode: "validation_failed"},
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

func TestComments_OnlyAuthorChanges(t *testing.T) {
	t.Parallel()

	env, c := issueEnv(t)
	i := env.createIssue(t, c, `{"title":"x"}`)

	now := time.Now().UTC()

	bob, err := domain.NewUser(domain.UserID(uuid.New()), domain.Identity{Issuer: "iss", Subject: "bob", DisplayName: "Bob"}, now)
	if err != nil {
		t.Fatal(err)
	}

	if err := env.repos.Users.Create(t.Context(), bob); err != nil {
		t.Fatal(err)
	}

	issueID, err := uuid.Parse(i.ID)
	if err != nil {
		t.Fatal(err)
	}

	theirs, err := domain.NewComment(domain.CommentID(uuid.New()), domain.IssueID(issueID), bob.ID, "by bob", now)
	if err != nil {
		t.Fatal(err)
	}

	if err := env.repos.Comments.Create(t.Context(), theirs); err != nil {
		t.Fatal(err)
	}

	if list := env.listComments(t, c); len(list) != 1 || list[0].Author.DisplayName != "Bob" {
		t.Fatalf("list = %+v", list)
	}

	path := commentsPath + "/" + theirs.ID.String()

	for _, r := range []response{
		env.do(t, c, http.MethodPatch, path, `{"body":"hijack"}`),
		env.do(t, c, http.MethodDelete, path, ""),
	} {
		if r.status != http.StatusForbidden || r.errorCode(t) != "not_comment_author" {
			t.Fatalf("changing someone else's comment = %d %s", r.status, r.body)
		}
	}
}

func TestComments_ArchivedIssue(t *testing.T) {
	t.Parallel()

	env, c := issueEnv(t)
	env.createIssue(t, c, `{"title":"x"}`)
	cm := env.createComment(t, c, `{"body":"first"}`)
	wantStatus(t, env.do(t, c, http.MethodPatch, "/api/projects/TRAX/issues/1", `{"status":"archived"}`), http.StatusOK)

	for _, r := range []response{
		env.do(t, c, http.MethodPost, commentsPath, `{"body":"x"}`),
		env.do(t, c, http.MethodPatch, commentsPath+"/"+cm.ID, `{"body":"x"}`),
		env.do(t, c, http.MethodDelete, commentsPath+"/"+cm.ID, ""),
	} {
		if r.status != http.StatusConflict || r.errorCode(t) != "issue_archived" {
			t.Fatalf("change on archived issue = %d %s", r.status, r.body)
		}
	}

	if list := env.listComments(t, c); len(list) != 1 {
		t.Fatalf("list on archived issue = %+v", list)
	}
}

func TestComments_RequireSession(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	wantStatus(t, env.do(t, env.client, http.MethodGet, commentsPath, ""), http.StatusUnauthorized)
}
