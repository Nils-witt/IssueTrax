package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type changeJSON struct {
	Type    string `json:"type"`
	Project string `json:"project"`
	Issue   int64  `json:"issue"`
}

// dialEvents opens /api/events with c's cookies and the given headers. It
// returns the handshake's HTTP status (0 without a response).
func (e *testEnv) dialEvents(t *testing.T, c *http.Client, header http.Header) (*websocket.Conn, int, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	conn, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(e.srv.URL, "http")+"/api/events",
		&websocket.DialOptions{HTTPClient: c, HTTPHeader: header})

	status := 0
	if resp != nil {
		status = resp.StatusCode
	}

	// After a successful upgrade the body is nil; the connection owns the stream.
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	if conn != nil {
		t.Cleanup(func() { _ = conn.CloseNow() })
	}

	return conn, status, err //nolint:wrapcheck // test helper
}

func readChange(t *testing.T, conn *websocket.Conn) changeJSON {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	var msg changeJSON
	if err := wsjson.Read(ctx, conn, &msg); err != nil {
		t.Fatalf("read change: %v", err)
	}

	return msg
}

func TestEvents_StreamsCommittedChanges(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	c := newClient(t)
	env.login(t, c, "/")

	conn, _, err := env.dialEvents(t, c, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	env.createProject(t, c, `{"key":"TRAX","name":"IssueTrax"}`)
	env.createIssue(t, c, `{"title":"Live"}`)
	wantStatus(t, env.do(t, c, http.MethodPost, "/api/projects/TRAX/issues/1/comments", `{"body":"hi"}`), http.StatusCreated)

	for _, want := range []changeJSON{
		{Type: "project", Project: "TRAX"},
		{Type: "issue", Project: "TRAX", Issue: 1},
		{Type: "comments", Project: "TRAX", Issue: 1},
	} {
		if got := readChange(t, conn); got != want {
			t.Fatalf("change = %+v, want %+v", got, want)
		}
	}
}

func TestEvents_ServiceAccountWithAPIKey(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	c := newClient(t)
	env.login(t, c, "/")
	_, key := env.newBot(t, c, "watcher")

	conn, _, err := env.dialEvents(t, newClient(t), http.Header{"Authorization": {"Bearer " + key.Token}})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	env.createProject(t, c, `{"key":"TRAX","name":"IssueTrax"}`)

	if got := readChange(t, conn); got.Type != "project" || got.Project != "TRAX" {
		t.Fatalf("change = %+v", got)
	}
}

func TestEvents_RejectsHandshake(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	signedIn := newClient(t)
	env.login(t, signedIn, "/")

	tests := []struct {
		name   string
		client *http.Client
		header http.Header
		want   int
	}{
		{name: "anonymous", client: newClient(t), want: http.StatusUnauthorized},
		{name: "bad api key", client: newClient(t), header: http.Header{"Authorization": {"Bearer itx_nope"}}, want: http.StatusUnauthorized},
		// Cross-site WebSocket hijacking: a foreign page riding on the cookie.
		{name: "foreign origin", client: signedIn, header: http.Header{"Origin": {"https://evil.example"}}, want: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, status, err := env.dialEvents(t, tt.client, tt.header)
			if err == nil {
				t.Fatal("dial succeeded")
			}

			if status != tt.want {
				t.Fatalf("status = %d, want %d", status, tt.want)
			}
		})
	}
}

func TestEvents_AllowsPublicOrigin(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	c := newClient(t)
	env.login(t, c, "/")

	// The test server's URL is its public URL; same-origin pages must connect.
	if _, _, err := env.dialEvents(t, c, http.Header{"Origin": {env.srv.URL}}); err != nil {
		t.Fatalf("dial: %v", err)
	}
}
