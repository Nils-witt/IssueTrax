package httpapi_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

type attachmentJSON struct {
	ID        string `json:"id"`
	FileName  string `json:"fileName"`
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Uploader  struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
	} `json:"uploader"`
	CreatedAt time.Time `json:"createdAt"`
}

const attachmentsPath = "/api/projects/TRAX/issues/1/attachments"

// pngHeader is enough of a PNG for content sniffing.
var pngHeader = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

// multipartBody returns a form with one file part (field "file") and its
// content type.
func multipartBody(t *testing.T, field, fileName, mediaType string, content []byte) (body, contentType string) {
	t.Helper()

	var buf bytes.Buffer

	mw := multipart.NewWriter(&buf)

	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="`+field+`"; filename="`+fileName+`"`)

	if mediaType != "" {
		h.Set("Content-Type", mediaType)
	}

	part, err := mw.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}

	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	return buf.String(), mw.FormDataContentType()
}

func (e *testEnv) upload(t *testing.T, c *http.Client, fileName, mediaType string, content []byte) response {
	t.Helper()

	body, ct := multipartBody(t, "file", fileName, mediaType, content)

	return e.do(t, c, http.MethodPost, attachmentsPath, body, "Content-Type", ct)
}

func (e *testEnv) createAttachment(t *testing.T, c *http.Client, fileName, mediaType string, content []byte) attachmentJSON {
	t.Helper()

	r := e.upload(t, c, fileName, mediaType, content)
	wantStatus(t, r, http.StatusCreated)

	var a attachmentJSON
	r.json(t, &a)

	return a
}

func (e *testEnv) listAttachments(t *testing.T, c *http.Client) []attachmentJSON {
	t.Helper()

	r := e.do(t, c, http.MethodGet, attachmentsPath, "")
	wantStatus(t, r, http.StatusOK)

	var list []attachmentJSON
	r.json(t, &list)

	return list
}

func TestAttachments_Lifecycle(t *testing.T) {
	t.Parallel()

	env, c := issueEnv(t)
	env.createIssue(t, c, `{"title":"x"}`)

	if r := env.do(t, c, http.MethodGet, attachmentsPath, ""); r.body != "[]\n" {
		t.Fatalf("empty list = %q, want []", r.body)
	}

	a := env.createAttachment(t, c, "notes.txt", "text/plain", []byte("hello world"))

	type summary struct {
		name, mediaType, uploader string
		size                      int64
	}

	if got, want := (summary{a.FileName, a.MediaType, a.Uploader.ID, a.Size}), (summary{"notes.txt", "text/plain", env.me(t, c), 11}); got != want || a.ID == "" {
		t.Fatalf("created = %+v, want %+v", a, want)
	}

	if list := env.listAttachments(t, c); len(list) != 1 || list[0].ID != a.ID {
		t.Fatalf("list = %+v", list)
	}

	one := attachmentsPath + "/" + a.ID

	r := env.do(t, c, http.MethodGet, one, "")
	wantStatus(t, r, http.StatusOK)

	wantHeaders := map[string]string{
		"Content-Type":            "text/plain",
		"Content-Disposition":     "attachment; filename=notes.txt",
		"Content-Security-Policy": "default-src 'none'; sandbox",
		"X-Content-Type-Options":  "nosniff",
	}
	for name, want := range wantHeaders {
		if got := r.header.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	if r.body != "hello world" {
		t.Fatalf("download = %q", r.body)
	}

	wantStatus(t, env.do(t, c, http.MethodDelete, one, ""), http.StatusNoContent)

	if list := env.listAttachments(t, c); len(list) != 0 {
		t.Fatalf("list after delete = %+v", list)
	}

	if r := env.do(t, c, http.MethodGet, one, ""); r.status != http.StatusNotFound || r.errorCode(t) != "attachment_not_found" {
		t.Fatalf("download after delete = %d %s", r.status, r.body)
	}
}

func TestAttachments_RangeAndConditionalDownloads(t *testing.T) {
	t.Parallel()

	env, c := issueEnv(t)
	env.createIssue(t, c, `{"title":"x"}`)
	one := attachmentsPath + "/" + env.createAttachment(t, c, "notes.txt", "text/plain", []byte("hello world")).ID

	partial := env.do(t, c, http.MethodGet, one, "", "Range", "bytes=6-")
	wantStatus(t, partial, http.StatusPartialContent)

	if partial.body != "world" {
		t.Fatalf("range = %q, want world", partial.body)
	}

	etag := partial.header.Get("ETag")
	if etag == "" {
		t.Fatal("no ETag")
	}

	wantStatus(t, env.do(t, c, http.MethodGet, one, "", "If-None-Match", etag), http.StatusNotModified)
}

func TestAttachments_ServedSafely(t *testing.T) {
	t.Parallel()

	env, c := issueEnv(t)
	env.createIssue(t, c, `{"title":"x"}`)

	tests := []struct {
		name, fileName, mediaType string
		content                   []byte
		wantType, wantDisposition string
	}{
		{
			name: "png is inline", fileName: "shot.png", mediaType: "image/png", content: pngHeader,
			wantType: "image/png", wantDisposition: "inline; filename=shot.png",
		},
		{
			name: "png declared as anything is inline as png", fileName: "shot", mediaType: "", content: pngHeader,
			wantType: "image/png", wantDisposition: "inline; filename=shot",
		},
		{
			name: "html is a download", fileName: "x.html", mediaType: "text/html", content: []byte("<script>alert(1)</script>"),
			wantType: "text/html", wantDisposition: "attachment; filename=x.html",
		},
		{
			name: "html posing as png is a download", fileName: "x.png", mediaType: "image/png", content: []byte("<html><script>alert(1)</script>"),
			wantType: "image/png", wantDisposition: "attachment; filename=x.png",
		},
		{
			name: "svg is a download", fileName: "x.svg", mediaType: "image/svg+xml", content: []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
			wantType: "image/svg+xml", wantDisposition: "attachment; filename=x.svg",
		},
		{
			name: "unicode file name", fileName: "Übersicht.pdf", mediaType: "application/pdf", content: []byte("%PDF-1.7"),
			wantType: "application/pdf", wantDisposition: "attachment; filename*=utf-8''%C3%9Cbersicht.pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			a := env.createAttachment(t, c, tt.fileName, tt.mediaType, tt.content)

			r := env.do(t, c, http.MethodGet, attachmentsPath+"/"+a.ID, "")
			wantStatus(t, r, http.StatusOK)

			if got := r.header.Get("Content-Type"); got != tt.wantType {
				t.Errorf("Content-Type = %q, want %q", got, tt.wantType)
			}

			if got := r.header.Get("Content-Disposition"); got != tt.wantDisposition {
				t.Errorf("Content-Disposition = %q, want %q", got, tt.wantDisposition)
			}

			if got := r.header.Get("Content-Security-Policy"); !strings.Contains(got, "sandbox") {
				t.Errorf("Content-Security-Policy = %q, want sandbox", got)
			}
		})
	}
}

func TestAttachments_Errors(t *testing.T) {
	t.Parallel()

	env, c := issueEnv(t)
	env.createIssue(t, c, `{"title":"x"}`)

	noFile, noFileType := multipartBody(t, "other", "a.txt", "text/plain", []byte("x"))
	empty, emptyType := multipartBody(t, "file", "a.txt", "text/plain", nil)
	noName, noNameType := multipartBody(t, "file", "", "text/plain", []byte("x"))
	tooBig, tooBigType := multipartBody(t, "file", "big.bin", "", make([]byte, domain.MaxAttachmentBytes+1))
	huge, hugeType := multipartBody(t, "file", "huge.bin", "", make([]byte, domain.MaxAttachmentBytes+200<<10))
	// The file part never ends: the closing boundary is missing.
	truncated := "--x\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a\"\r\n\r\nabc"
	valid, validType := multipartBody(t, "file", "a.txt", "text/plain", []byte("x"))

	tests := []struct {
		name, method, path, body, contentType string
		wantStatus                            int
		wantCode                              string
	}{
		{name: "json body", method: http.MethodPost, path: attachmentsPath, body: `{"file":"x"}`, contentType: "application/json", wantStatus: http.StatusBadRequest, wantCode: "invalid_upload"},
		{name: "truncated multipart", method: http.MethodPost, path: attachmentsPath, body: truncated, contentType: "multipart/form-data; boundary=x", wantStatus: http.StatusBadRequest, wantCode: "invalid_upload"},
		{name: "no file field", method: http.MethodPost, path: attachmentsPath, body: noFile, contentType: noFileType, wantStatus: http.StatusUnprocessableEntity, wantCode: "validation_failed"},
		{name: "empty file", method: http.MethodPost, path: attachmentsPath, body: empty, contentType: emptyType, wantStatus: http.StatusUnprocessableEntity, wantCode: "validation_failed"},
		{name: "no file name", method: http.MethodPost, path: attachmentsPath, body: noName, contentType: noNameType, wantStatus: http.StatusUnprocessableEntity, wantCode: "validation_failed"},
		{name: "one byte too big", method: http.MethodPost, path: attachmentsPath, body: tooBig, contentType: tooBigType, wantStatus: http.StatusUnprocessableEntity, wantCode: "validation_failed"},
		{name: "body over the limit", method: http.MethodPost, path: attachmentsPath, body: huge, contentType: hugeType, wantStatus: http.StatusUnprocessableEntity, wantCode: "validation_failed"},
		{name: "unknown issue", method: http.MethodPost, path: "/api/projects/TRAX/issues/9/attachments", body: valid, contentType: validType, wantStatus: http.StatusNotFound, wantCode: "issue_not_found"},
		{name: "bad issue number", method: http.MethodGet, path: "/api/projects/TRAX/issues/x/attachments", wantStatus: http.StatusNotFound, wantCode: "issue_not_found"},
		{name: "unknown project", method: http.MethodGet, path: "/api/projects/NOPE/issues/1/attachments", wantStatus: http.StatusNotFound, wantCode: "project_not_found"},
		{name: "bad attachment id", method: http.MethodGet, path: attachmentsPath + "/x", wantStatus: http.StatusNotFound, wantCode: "attachment_not_found"},
		{name: "unknown attachment", method: http.MethodDelete, path: attachmentsPath + "/" + uuid.NewString(), wantStatus: http.StatusNotFound, wantCode: "attachment_not_found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var headers []string
			if tt.contentType != "" {
				headers = []string{"Content-Type", tt.contentType}
			}

			r := env.do(t, c, tt.method, tt.path, tt.body, headers...)
			if r.status != tt.wantStatus || r.errorCode(t) != tt.wantCode {
				t.Fatalf("%s %s = %d %s, want %d %s", tt.method, tt.path, r.status, r.body, tt.wantStatus, tt.wantCode)
			}
		})
	}
}

func TestAttachments_OnlyUploaderDeletes(t *testing.T) {
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

	theirs, err := domain.NewAttachment(domain.AttachmentID(uuid.New()), domain.IssueID(issueID), bob.ID, "b.txt", "text/plain", 1, now)
	if err != nil {
		t.Fatal(err)
	}

	if err := env.repos.Attachments.Create(t.Context(), theirs, []byte("b")); err != nil {
		t.Fatal(err)
	}

	if list := env.listAttachments(t, c); len(list) != 1 || list[0].Uploader.DisplayName != "Bob" {
		t.Fatalf("list = %+v", list)
	}

	path := attachmentsPath + "/" + theirs.ID.String()

	if r := env.do(t, c, http.MethodDelete, path, ""); r.status != http.StatusForbidden || r.errorCode(t) != "not_attachment_uploader" {
		t.Fatalf("deleting someone else's attachment = %d %s", r.status, r.body)
	}

	// Everyone may still download it.
	if r := env.do(t, c, http.MethodGet, path, ""); r.status != http.StatusOK || r.body != "b" {
		t.Fatalf("download someone else's attachment = %d %s", r.status, r.body)
	}
}

func TestAttachments_ArchivedIssue(t *testing.T) {
	t.Parallel()

	env, c := issueEnv(t)
	env.createIssue(t, c, `{"title":"x"}`)
	a := env.createAttachment(t, c, "a.txt", "text/plain", []byte("x"))
	wantStatus(t, env.do(t, c, http.MethodPatch, "/api/projects/TRAX/issues/1", `{"status":"archived"}`), http.StatusOK)

	for _, r := range []response{
		env.upload(t, c, "b.txt", "text/plain", []byte("y")),
		env.do(t, c, http.MethodDelete, attachmentsPath+"/"+a.ID, ""),
	} {
		if r.status != http.StatusConflict || r.errorCode(t) != "issue_archived" {
			t.Fatalf("change on archived issue = %d %s", r.status, r.body)
		}
	}

	wantStatus(t, env.do(t, c, http.MethodGet, attachmentsPath+"/"+a.ID, ""), http.StatusOK)
}

func TestAttachments_RequireAuthAndSameOrigin(t *testing.T) {
	t.Parallel()

	env, c := issueEnv(t)
	env.createIssue(t, c, `{"title":"x"}`)
	a := env.createAttachment(t, c, "a.txt", "text/plain", []byte("x"))

	anon := newClient(t)
	wantStatus(t, env.do(t, anon, http.MethodGet, attachmentsPath, ""), http.StatusUnauthorized)
	wantStatus(t, env.do(t, anon, http.MethodGet, attachmentsPath+"/"+a.ID, ""), http.StatusUnauthorized)

	// A form on another site must not be able to upload with the user's cookie.
	body, ct := multipartBody(t, "file", "evil.txt", "text/plain", []byte("x"))
	r := env.do(t, c, http.MethodPost, attachmentsPath, body, "Content-Type", ct, "Sec-Fetch-Site", "cross-site")
	wantStatus(t, r, http.StatusForbidden)

	if list := env.listAttachments(t, c); len(list) != 1 {
		t.Fatalf("list after cross-site upload = %+v", list)
	}
}
