package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"nilswitt.dev/issuetrax/internal/adapter/changefeed"
	"nilswitt.dev/issuetrax/internal/adapter/httpapi"
	"nilswitt.dev/issuetrax/internal/adapter/storage/memory"
	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/service"
)

// fakeIDP authorizes immediately: its "authorize URL" is our own callback
// with code=good, as if the user had approved at the provider.
type fakeIDP struct {
	callback string
	// groups are asserted for the signed-in person; testAdminGroup makes them an admin.
	groups []string
}

// testAdminGroup is the identity provider group of admins in tests.
const testAdminGroup = "issuetrax-admins"

func (f *fakeIDP) AuthCodeURL(state, _, _ string) string {
	return f.callback + "?" + url.Values{"state": {state}, "code": {"good"}}.Encode()
}

func (f *fakeIDP) Exchange(_ context.Context, code, _, _ string) (domain.Identity, string, error) {
	if code != "good" {
		return domain.Identity{}, "", errors.New("invalid grant")
	}

	return domain.Identity{
		Issuer: "https://idp.example", Subject: "ada", Email: "ada@example.com", DisplayName: "Ada", Groups: f.groups,
	}, "id-token", nil
}

func (f *fakeIDP) EndSessionURL(idToken string) string {
	return "https://idp.example/logout?id_token_hint=" + idToken
}

type testEnv struct {
	srv    *httptest.Server
	repos  service.Repositories
	idp    *fakeIDP
	client *http.Client
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	store := memory.NewStore()
	repos := service.Repositories{
		Tx:               store,
		Projects:         memory.NewProjectRepository(store),
		Issues:           memory.NewIssueRepository(store),
		Tags:             memory.NewTagRepository(store),
		Comments:         memory.NewCommentRepository(store),
		Attachments:      memory.NewAttachmentRepository(store),
		Storages:         memory.NewStorageRepository(store),
		ItemTypes:        memory.NewItemTypeRepository(store),
		Manufacturers:    memory.NewManufacturerRepository(store),
		HardwareModels:   memory.NewHardwareModelRepository(store),
		StorageItems:     memory.NewStorageItemRepository(store),
		AssetTagCounters: memory.NewAssetTagCounterRepository(store),
		Users:            memory.NewUserRepository(store),
		Sessions:         memory.NewSessionRepository(store),
		APIKeys:          memory.NewAPIKeyRepository(store),
		Audit:            memory.NewAuditRepository(store),
	}

	srv := httptest.NewUnstartedServer(nil)
	publicURL := "http://" + srv.Listener.Addr().String()
	idp := &fakeIDP{callback: publicURL + "/auth/callback"}
	changes := changefeed.New()
	notify := service.WithChangeNotifier(changes)

	h, err := httpapi.NewHandler(httpapi.Config{
		Auth:           service.NewAuthService(repos, idp, service.WithAdminGroup(testAdminGroup)),
		Projects:       service.NewProjectService(repos, notify),
		Issues:         service.NewIssueService(repos, notify),
		Tags:           service.NewTagService(repos, notify),
		Comments:       service.NewCommentService(repos, notify),
		Attachments:    service.NewAttachmentService(repos, notify),
		Storages:       service.NewStorageService(repos, notify),
		ItemTypes:      service.NewItemTypeService(repos, notify),
		Manufacturers:  service.NewManufacturerService(repos, notify),
		HardwareModels: service.NewHardwareModelService(repos, notify),
		StorageItems:   service.NewStorageItemService(repos, notify),
		Users:          service.NewUserService(repos),

		ServiceAccounts: service.NewBotService(repos),
		Audit:           service.NewAuditService(repos, 0),
		Changes:         changes,
		Frontend: fstest.MapFS{
			"index.html":    {Data: []byte("<html>spa</html>")},
			"assets/app.js": {Data: []byte("console.log(1)")},
		},
		OpenAPISpec:     []byte("openapi: 3.1.0\n"),
		Logger:          slog.New(slog.DiscardHandler),
		PublicURL:       publicURL,
		CookieKey:       []byte(strings.Repeat("k", 32)),
		InsecureCookies: true,
	})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	srv.Config.Handler = h
	srv.Start()
	t.Cleanup(srv.Close)

	return &testEnv{srv: srv, repos: repos, idp: idp, client: newClient(t)}
}

// newClient returns a cookie-keeping client that does not follow redirects.
func newClient(t *testing.T) *http.Client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	return &http.Client{
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

type response struct {
	status int
	header http.Header
	body   string
}

func (r response) json(t *testing.T, v any) {
	t.Helper()

	if err := json.Unmarshal([]byte(r.body), v); err != nil {
		t.Fatalf("decode %q: %v", r.body, err)
	}
}

// errorCode returns error.code from a JSON error body.
func (r response) errorCode(t *testing.T) string {
	t.Helper()

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	r.json(t, &body)

	return body.Error.Code
}

func (e *testEnv) do(t *testing.T, c *http.Client, method, path, body string, headers ...string) response {
	t.Helper()

	target := path
	if !strings.HasPrefix(path, "http") {
		target = e.srv.URL + path
	}

	req, err := http.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}

	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}

	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	return response{status: resp.StatusCode, header: resp.Header, body: string(b)}
}

// login runs the browser sign-in flow for c and returns where it landed.
func (e *testEnv) login(t *testing.T, c *http.Client, returnTo string) string {
	t.Helper()

	start := e.do(t, c, http.MethodGet, "/auth/login?returnTo="+url.QueryEscape(returnTo), "")
	if start.status != http.StatusFound {
		t.Fatalf("GET /auth/login = %d", start.status)
	}

	done := e.do(t, c, http.MethodGet, start.header.Get("Location"), "")
	if done.status != http.StatusFound {
		t.Fatalf("GET /auth/callback = %d: %s", done.status, done.body)
	}

	return done.header.Get("Location")
}

func wantStatus(t *testing.T, r response, status int) {
	t.Helper()

	if r.status != status {
		t.Fatalf("status = %d, want %d (body %s)", r.status, status, r.body)
	}
}
