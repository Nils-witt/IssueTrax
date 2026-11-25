package httpapi_test

import (
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"nilswitt.dev/issuetrax/internal/adapter/httpapi"
	"nilswitt.dev/issuetrax/internal/adapter/storage/memory"
	"nilswitt.dev/issuetrax/internal/core/service"
)

func TestNewHandler_RejectsBadConfig(t *testing.T) {
	t.Parallel()

	store := memory.NewStore()
	valid := httpapi.Config{
		Auth:      service.NewAuthService(service.Repositories{Tx: store}, &fakeIDP{}),
		Projects:  service.NewProjectService(service.Repositories{Tx: store}),
		Issues:    service.NewIssueService(service.Repositories{Tx: store}),
		Tags:      service.NewTagService(service.Repositories{Tx: store}),
		Comments:  service.NewCommentService(service.Repositories{Tx: store}),
		Users:     service.NewUserService(service.Repositories{Tx: store}),
		Frontend:  fstest.MapFS{"index.html": {Data: []byte("x")}},
		Logger:    slog.New(slog.DiscardHandler),
		PublicURL: "https://trax.example",
		CookieKey: []byte(strings.Repeat("k", 32)),
	}

	if _, err := httpapi.NewHandler(valid); err != nil {
		t.Fatalf("valid config: %v", err)
	}

	tests := map[string]func(c *httpapi.Config){
		"short cookie key": func(c *httpapi.Config) { c.CookieKey = []byte("short") },
		"bad public url":   func(c *httpapi.Config) { c.PublicURL = "trax.example" },
		"no index.html":    func(c *httpapi.Config) { c.Frontend = fstest.MapFS{} },
		"no auth service":  func(c *httpapi.Config) { c.Auth = nil },
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg := valid
			mutate(&cfg)

			if _, err := httpapi.NewHandler(cfg); err == nil {
				t.Fatal("NewHandler() = nil error")
			}
		})
	}
}

func TestHandler_PublicRoutes(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{name: "healthz", path: "/healthz", wantStatus: http.StatusOK, wantBody: `"ok"`},
		{name: "version", path: "/api/version", wantStatus: http.StatusOK, wantBody: `"version"`},
		{name: "openapi spec", path: "/api/openapi.yaml", wantStatus: http.StatusOK, wantBody: "openapi: 3.1.0"},
		{name: "unknown api path", path: "/api/nope", wantStatus: http.StatusNotFound, wantBody: `"code":"not_found"`},
		{name: "unknown auth path", path: "/auth/nope", wantStatus: http.StatusNotFound, wantBody: `"code":"not_found"`},
		{name: "static asset", path: "/assets/app.js", wantStatus: http.StatusOK, wantBody: "console.log"},
		{name: "spa fallback", path: "/p/TRAX/issues/42", wantStatus: http.StatusOK, wantBody: "spa"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := env.do(t, http.DefaultClient, http.MethodGet, tt.path, "")
			wantStatus(t, r, tt.wantStatus)

			if !strings.Contains(r.body, tt.wantBody) {
				t.Fatalf("body = %s, want it to contain %s", r.body, tt.wantBody)
			}

			if r.header.Get("Content-Security-Policy") == "" || r.header.Get("X-Content-Type-Options") != "nosniff" {
				t.Fatalf("missing security headers: %v", r.header)
			}
		})
	}
}

func TestHandler_CrossOriginProtection(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	tests := []struct {
		name    string
		headers []string
		want    int
	}{
		{name: "cross-site fetch", headers: []string{"Sec-Fetch-Site", "cross-site"}, want: http.StatusForbidden},
		{name: "foreign origin", headers: []string{"Origin", "https://evil.example"}, want: http.StatusForbidden},
		{name: "same-origin fetch", headers: []string{"Sec-Fetch-Site", "same-origin"}, want: http.StatusOK},
		{name: "trusted public origin", headers: []string{"Origin", env.srv.URL, "Sec-Fetch-Site", "same-origin"}, want: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := env.do(t, newClient(t), http.MethodPost, "/auth/logout", "", tt.headers...)
			wantStatus(t, r, tt.want)

			if tt.want == http.StatusForbidden && r.errorCode(t) != "cross_origin_rejected" {
				t.Fatalf("body = %s", r.body)
			}
		})
	}
}
