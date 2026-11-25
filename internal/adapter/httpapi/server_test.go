package httpapi_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"nilswitt.dev/issuetrax/internal/adapter/httpapi"
	"nilswitt.dev/issuetrax/internal/adapter/storage/memory"
	"nilswitt.dev/issuetrax/internal/core/service"
)

const issuesPath = "/api/issues"

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	dist := fstest.MapFS{
		"index.html":    {Data: []byte("<html>spa</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}

	h, err := httpapi.NewHandler(
		service.NewIssueService(memory.NewIssueRepository()),
		dist,
		slog.New(slog.DiscardHandler),
	)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	return srv
}

func TestHandler(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantBody   string
	}{
		{name: "healthz", method: http.MethodGet, path: "/healthz", wantStatus: http.StatusOK, wantBody: `"ok"`},
		{name: "list empty", method: http.MethodGet, path: issuesPath, wantStatus: http.StatusOK, wantBody: "[]"},
		{name: "create", method: http.MethodPost, path: issuesPath, body: `{"title":"Bug"}`, wantStatus: http.StatusCreated, wantBody: `"title":"Bug"`},
		{name: "create invalid", method: http.MethodPost, path: issuesPath, body: `{"title":""}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "create bad json", method: http.MethodPost, path: issuesPath, body: `{`, wantStatus: http.StatusBadRequest},
		{name: "get missing", method: http.MethodGet, path: "/api/issues/nope", wantStatus: http.StatusNotFound},
		{name: "unknown api path", method: http.MethodGet, path: "/api/nope", wantStatus: http.StatusNotFound, wantBody: `"error"`},
		{name: "static asset", method: http.MethodGet, path: "/assets/app.js", wantStatus: http.StatusOK, wantBody: "console.log"},
		{name: "spa fallback", method: http.MethodGet, path: "/issues/42", wantStatus: http.StatusOK, wantBody: "spa"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(t.Context(), tt.method, srv.URL+tt.path, strings.NewReader(tt.body))
			if err != nil {
				t.Fatal(err)
			}

			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", resp.StatusCode, tt.wantStatus, body)
			}

			if !strings.Contains(string(body), tt.wantBody) {
				t.Fatalf("body = %s, want it to contain %s", body, tt.wantBody)
			}
		})
	}
}
