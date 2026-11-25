package httpapi_test

import (
	"net/http"
	"testing"
)

func TestUsers_List(t *testing.T) {
	t.Parallel()

	env, c := signedIn(t)

	r := env.do(t, c, http.MethodGet, "/api/users", "")
	wantStatus(t, r, http.StatusOK)

	var users []struct{ ID, Email, DisplayName string }
	r.json(t, &users)

	if len(users) != 1 || users[0].DisplayName != "Ada" || users[0].Email != "ada@example.com" || users[0].ID == "" {
		t.Fatalf("GET /api/users = %s", r.body)
	}
}
