package httpapi_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"nilswitt.dev/issuetrax/internal/adapter/changefeed"
	"nilswitt.dev/issuetrax/internal/adapter/httpapi"
	"nilswitt.dev/issuetrax/internal/adapter/storage/memory"
	"nilswitt.dev/issuetrax/internal/core/service"
)

func TestAuth_LoginFlow(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	c := newClient(t)

	wantStatus(t, env.do(t, c, http.MethodGet, "/api/me", ""), http.StatusUnauthorized)

	if landed := env.login(t, c, "/p/TRAX?q=1"); landed != "/p/TRAX?q=1" {
		t.Fatalf("landed on %q, want /p/TRAX?q=1", landed)
	}

	me := env.do(t, c, http.MethodGet, "/api/me", "")
	wantStatus(t, me, http.StatusOK)

	var user struct {
		ID, Email, DisplayName string
	}
	me.json(t, &user)

	if user.DisplayName != "Ada" || user.Email != "ada@example.com" || user.ID == "" {
		t.Fatalf("GET /api/me = %s", me.body)
	}

	out := env.do(t, c, http.MethodPost, "/auth/logout", "")
	wantStatus(t, out, http.StatusOK)

	var logout struct {
		RedirectURL string `json:"redirectUrl"`
	}
	out.json(t, &logout)

	if logout.RedirectURL != "https://idp.example/logout?id_token_hint=id-token" {
		t.Fatalf("logout redirect = %q", logout.RedirectURL)
	}

	r := env.do(t, c, http.MethodGet, "/api/me", "")
	wantStatus(t, r, http.StatusUnauthorized)

	if r.errorCode(t) != "unauthenticated" {
		t.Fatalf("body = %s", r.body)
	}
}

func TestAuth_OpenRedirectIsBlocked(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	for _, target := range []string{"https://evil.example/", "//evil.example", "/\\evil.example"} {
		if landed := env.login(t, newClient(t), target); landed != "/" {
			t.Errorf("returnTo %q landed on %q, want /", target, landed)
		}
	}
}

func TestAuth_CallbackRejects(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	// start begins a login and returns the client plus the callback URL the
	// IdP would redirect to.
	start := func(t *testing.T) (*http.Client, *url.URL) {
		t.Helper()

		c := newClient(t)
		r := env.do(t, c, http.MethodGet, "/auth/login", "")

		u, err := url.Parse(r.header.Get("Location"))
		if err != nil {
			t.Fatal(err)
		}

		return c, u
	}

	tests := []struct {
		name   string
		mutate func(t *testing.T, c *http.Client, u *url.URL)
	}{
		{name: "no login cookie", mutate: func(t *testing.T, c *http.Client, _ *url.URL) {
			t.Helper()

			c.Jar = nil
		}},
		{name: "tampered login cookie", mutate: func(t *testing.T, c *http.Client, _ *url.URL) {
			t.Helper()

			base, _ := url.Parse(env.srv.URL)
			for _, ck := range c.Jar.Cookies(base) {
				ck.Value = "x" + ck.Value
				c.Jar.SetCookies(base, []*http.Cookie{ck})
			}
		}},
		{name: "state mismatch", mutate: func(t *testing.T, _ *http.Client, u *url.URL) {
			t.Helper()

			q := u.Query()
			q.Set("state", "forged")
			u.RawQuery = q.Encode()
		}},
		{name: "idp error", mutate: func(t *testing.T, _ *http.Client, u *url.URL) {
			t.Helper()

			u.RawQuery = url.Values{"error": {"access_denied"}}.Encode()
		}},
		{name: "bad code", mutate: func(t *testing.T, _ *http.Client, u *url.URL) {
			t.Helper()

			q := u.Query()
			q.Set("code", "bad")
			u.RawQuery = q.Encode()
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, u := start(t)
			tt.mutate(t, c, u)

			r := env.do(t, c, http.MethodGet, u.String(), "")
			wantStatus(t, r, http.StatusBadRequest)

			if !strings.Contains(r.body, "Sign-in failed") {
				t.Fatalf("body = %s", r.body)
			}

			if c.Jar != nil {
				wantStatus(t, env.do(t, c, http.MethodGet, "/api/me", ""), http.StatusUnauthorized)
			}
		})
	}
}

func TestAuth_SecureCookies(t *testing.T) {
	t.Parallel()

	store := memory.NewStore()

	h, err := httpapi.NewHandler(httpapi.Config{
		Auth:        service.NewAuthService(service.Repositories{Tx: store}, &fakeIDP{callback: "https://trax.example/auth/callback"}),
		Projects:    service.NewProjectService(service.Repositories{Tx: store}),
		Issues:      service.NewIssueService(service.Repositories{Tx: store}),
		Tags:        service.NewTagService(service.Repositories{Tx: store}),
		Comments:    service.NewCommentService(service.Repositories{Tx: store}),
		Attachments: service.NewAttachmentService(service.Repositories{Tx: store}),
		Storages:    service.NewStorageService(service.Repositories{Tx: store}),
		Users:       service.NewUserService(service.Repositories{Tx: store}),

		ServiceAccounts: service.NewBotService(service.Repositories{Tx: store}),
		Audit:           service.NewAuditService(service.Repositories{Tx: store}, 0),
		Changes:         changefeed.New(),
		Frontend:        fstest.MapFS{"index.html": {Data: []byte("x")}},
		Logger:          slog.New(slog.DiscardHandler),
		PublicURL:       "https://trax.example",
		CookieKey:       []byte(strings.Repeat("k", 32)),
	})
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://trax.example/auth/login", nil))

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %v, want the login cookie", cookies)
	}

	ck := cookies[0]
	if ck.Name != "__Host-issuetrax_login" || !ck.Secure || !ck.HttpOnly || ck.Path != "/" || ck.SameSite != http.SameSiteLaxMode || ck.MaxAge != 600 {
		t.Fatalf("login cookie = %+v", ck)
	}

	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Fatal("missing HSTS header with secure cookies")
	}
}
