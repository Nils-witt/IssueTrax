package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

type serviceAccountBody struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	ServiceAccount bool   `json:"serviceAccount"`
}

type apiKeyBody struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Token      string     `json:"token"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	RevokedAt  *time.Time `json:"revokedAt"`
}

// newBot signs c in, creates the service account name and a key for it,
// and returns the account and the key's token.
func (e *testEnv) newBot(t *testing.T, c *http.Client, name string) (serviceAccountBody, apiKeyBody) {
	t.Helper()

	r := e.do(t, c, http.MethodPost, "/api/service-accounts", `{"name":"`+name+`"}`)
	wantStatus(t, r, http.StatusCreated)

	var bot serviceAccountBody
	r.json(t, &bot)

	r = e.do(t, c, http.MethodPost, "/api/service-accounts/"+bot.ID+"/keys", `{"name":"ci key"}`)
	wantStatus(t, r, http.StatusCreated)

	if r.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", r.header.Get("Cache-Control"))
	}

	var key apiKeyBody
	r.json(t, &key)

	return bot, key
}

func TestServiceAccounts_Create(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	c := newClient(t)
	env.login(t, c, "/")

	bot, key := env.newBot(t, c, "CI")

	if bot.Name != "ci" || bot.DisplayName != "bot-ci" || !bot.ServiceAccount {
		t.Fatalf("created account = %+v", bot)
	}

	if !strings.HasPrefix(key.Token, "itx_") || !strings.HasPrefix(key.Token, key.Prefix) || key.Name != "ci key" {
		t.Fatalf("created key = %+v", key)
	}

	r := env.do(t, c, http.MethodPost, "/api/service-accounts", `{"name":"bot-ci"}`)
	wantStatus(t, r, http.StatusConflict)

	if r.errorCode(t) != "service_account_name_taken" {
		t.Fatalf("duplicate: %s", r.body)
	}

	r = env.do(t, c, http.MethodGet, "/api/service-accounts", "")
	wantStatus(t, r, http.StatusOK)

	var list []serviceAccountBody
	r.json(t, &list)

	if len(list) != 1 || list[0].ID != bot.ID {
		t.Fatalf("list = %s", r.body)
	}
}

func TestServiceAccounts_APIKeyAuth(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	c := newClient(t)
	env.login(t, c, "/")

	bot, key := env.newBot(t, c, "ci")

	// The bot authenticates with its key, without cookies.
	anon := newClient(t)
	bearer := []string{"Authorization", "Bearer " + key.Token}

	me := env.do(t, anon, http.MethodGet, "/api/me", "", bearer...)
	wantStatus(t, me, http.StatusOK)

	var meBody serviceAccountBody
	me.json(t, &meBody)

	if meBody.ID != bot.ID || !meBody.ServiceAccount {
		t.Fatalf("GET /api/me as bot = %s", me.body)
	}

	wantStatus(t, env.do(t, anon, http.MethodPost, "/api/projects", `{"key":"BOT","name":"Bot project"}`, bearer...), http.StatusCreated)
	wantStatus(t, env.do(t, anon, http.MethodPost, "/api/projects/BOT/issues", `{"title":"From CI"}`, bearer...), http.StatusCreated)

	env.wantBotCannotManage(t, anon, bot, key, bearer)

	r := env.do(t, c, http.MethodGet, "/api/service-accounts/"+bot.ID+"/keys", "")
	wantStatus(t, r, http.StatusOK)

	var keys []apiKeyBody
	r.json(t, &keys)

	if len(keys) != 1 || keys[0].Token != "" || keys[0].LastUsedAt == nil {
		t.Fatalf("keys = %s", r.body)
	}

	wantStatus(t, env.do(t, c, http.MethodDelete, "/api/service-accounts/"+bot.ID+"/keys/"+key.ID, ""), http.StatusNoContent)

	r = env.do(t, anon, http.MethodGet, "/api/me", "", bearer...)
	wantStatus(t, r, http.StatusUnauthorized)

	if r.errorCode(t) != "invalid_api_key" || !strings.HasPrefix(r.header.Get("WWW-Authenticate"), "Bearer") {
		t.Fatalf("revoked key: %d %v %s", r.status, r.header, r.body)
	}
}

// wantBotCannotManage checks that a service account cannot manage
// service accounts or keys, not even its own.
func (e *testEnv) wantBotCannotManage(t *testing.T, c *http.Client, bot serviceAccountBody, key apiKeyBody, bearer []string) {
	t.Helper()

	for _, req := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/service-accounts", ""},
		{http.MethodPost, "/api/service-accounts", `{"name":"evil"}`},
		{http.MethodPost, "/api/service-accounts/" + bot.ID + "/keys", `{"name":"more"}`},
		{http.MethodDelete, "/api/service-accounts/" + bot.ID + "/keys/" + key.ID, ""},
	} {
		r := e.do(t, c, req.method, req.path, req.body, bearer...)
		wantStatus(t, r, http.StatusForbidden)

		if r.errorCode(t) != "session_required" {
			t.Fatalf("%s %s as bot: %s", req.method, req.path, r.body)
		}
	}
}

func TestServiceAccounts_BadCredentials(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	c := newClient(t)
	env.login(t, c, "/")

	for _, header := range []string{"Bearer", "Bearer itx_nope", "Basic YWRhOmFkYQ==", "itx_nope"} {
		// Even with a valid session cookie, a bad Authorization header fails.
		r := env.do(t, c, http.MethodGet, "/api/me", "", "Authorization", header)
		wantStatus(t, r, http.StatusUnauthorized)

		if r.errorCode(t) != "invalid_api_key" {
			t.Fatalf("Authorization %q: %s", header, r.body)
		}
	}
}

func TestServiceAccounts_NotFound(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	c := newClient(t)
	env.login(t, c, "/")

	r := env.do(t, c, http.MethodGet, "/api/me", "")

	var me serviceAccountBody
	r.json(t, &me)

	for _, path := range []string{"/api/service-accounts/nope/keys", "/api/service-accounts/" + me.ID + "/keys"} {
		r := env.do(t, c, http.MethodGet, path, "")
		wantStatus(t, r, http.StatusNotFound)

		if r.errorCode(t) != "service_account_not_found" {
			t.Fatalf("GET %s: %s", path, r.body)
		}
	}

	bot, _ := env.newBot(t, c, "ci")
	r = env.do(t, c, http.MethodDelete, "/api/service-accounts/"+bot.ID+"/keys/nope", "")
	wantStatus(t, r, http.StatusNotFound)

	r = env.do(t, c, http.MethodPost, "/api/service-accounts/"+bot.ID+"/keys", `{"name":"x","expiresAt":"2000-01-01T00:00:00Z"}`)
	wantStatus(t, r, http.StatusUnprocessableEntity)
}
