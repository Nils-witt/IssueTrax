// Package httpapi is the inbound HTTP adapter: it exposes the core's driving
// ports as a JSON API under /api, runs the browser sign-in flow under /auth
// and serves the embedded single-page app on every other path.
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"slices"

	"nilswitt.dev/issuetrax/internal/core/port"
	"nilswitt.dev/issuetrax/internal/version"
)

// minCookieKeyBytes is the minimum length of the cookie signing key.
const minCookieKeyBytes = 32

// Config holds the handler's dependencies and settings.
type Config struct {
	Auth     port.AuthService
	Projects port.ProjectService
	Issues   port.IssueService
	Tags     port.TagService
	Comments port.CommentService
	// Attachments stores and serves files attached to issues.
	Attachments port.AttachmentService
	// Storages manages the physical storage location tree.
	Storages port.StorageService
	// GenericStorageItems manages the generic items kept in storages.
	GenericStorageItems port.GenericStorageItemService
	// NetworkDevices manages the network devices kept in storages.
	NetworkDevices port.NetworkDeviceService
	// StorageItems finds storage items of any kind, e.g. to link them to issues.
	StorageItems port.StorageItemLookupService
	Users        port.UserService
	// ServiceAccounts manages "bot-<name>" users and their API keys.
	ServiceAccounts port.ServiceAccountService
	// Audit reads the audit log (admins only).
	Audit port.AuditService
	// Changes feeds /api/events, the WebSocket that tells clients what to reload.
	Changes port.ChangeFeed
	// Frontend contains the built SPA with index.html at its root.
	Frontend fs.FS
	// OpenAPISpec is the OpenAPI document (YAML) served at
	// /api/openapi.yaml. Optional: without it that path answers 404.
	OpenAPISpec []byte
	Logger      *slog.Logger
	// PublicURL is the origin browsers use, e.g. https://trax.example.com.
	// Cross-origin requests from it are trusted.
	PublicURL string
	// CookieKey signs the short-lived login-state cookie (>= 32 bytes).
	CookieKey []byte
	// InsecureCookies drops the Secure flag and __Host- prefix so cookies
	// work on http://localhost. Development only.
	InsecureCookies bool
}

// Handler routes HTTP requests to the core services and the SPA.
type Handler struct {
	auth           port.AuthService
	projects       port.ProjectService
	issues         port.IssueService
	tags           port.TagService
	comments       port.CommentService
	attachments    port.AttachmentService
	storages       port.StorageService
	genericItems   port.GenericStorageItemService
	networkDevices port.NetworkDeviceService
	storageItems   port.StorageItemLookupService
	users          port.UserService
	accounts       port.ServiceAccountService
	audit          port.AuditService
	changes        port.ChangeFeed
	logger         *slog.Logger
	cookies        cookies
	openAPI        []byte
	// wsOrigins are the origins (besides the request host) allowed to
	// open WebSockets: the public URL's.
	wsOrigins []string
}

// NewHandler builds the root http.Handler.
func NewHandler(cfg Config) (http.Handler, error) {
	if len(cfg.CookieKey) < minCookieKeyBytes {
		return nil, fmt.Errorf("cookie key must be at least %d bytes", minCookieKeyBytes)
	}

	if cfg.missingDependency() {
		return nil, errors.New("all services and the logger are required")
	}

	h := &Handler{
		auth:           cfg.Auth,
		projects:       cfg.Projects,
		issues:         cfg.Issues,
		tags:           cfg.Tags,
		comments:       cfg.Comments,
		attachments:    cfg.Attachments,
		storages:       cfg.Storages,
		genericItems:   cfg.GenericStorageItems,
		networkDevices: cfg.NetworkDevices,
		storageItems:   cfg.StorageItems,
		users:          cfg.Users,
		accounts:       cfg.ServiceAccounts,
		audit:          cfg.Audit,
		changes:        cfg.Changes,
		logger:         cfg.Logger,
		cookies:        cookies{secure: !cfg.InsecureCookies, key: cfg.CookieKey},
		openAPI:        cfg.OpenAPISpec,
	}

	csrf, err := crossOriginProtection(cfg.PublicURL)
	if err != nil {
		return nil, err
	}

	// crossOriginProtection has validated the URL.
	if u, err := url.Parse(cfg.PublicURL); err == nil {
		h.wsOrigins = []string{u.Scheme + "://" + u.Host}
	}

	spa, err := newSPAHandler(cfg.Frontend)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /api/version", h.version)

	if len(h.openAPI) > 0 {
		mux.HandleFunc("GET /api/openapi.yaml", h.openAPISpec)
	}

	mux.HandleFunc("GET /auth/login", h.login)
	mux.HandleFunc("GET /auth/callback", h.callback)
	mux.HandleFunc("POST /auth/logout", h.logout)
	mux.HandleFunc("/auth/", notFound)

	// Everything else under /api requires a session or an API key;
	// service accounts are managed by people only, and the audit log is
	// read by admins (checked by the audit service).
	mux.Handle("GET /api/me", h.authed(h.me))
	mux.Handle("GET /api/users", h.authed(h.listUsers))
	mux.Handle("GET /api/service-accounts", h.sessionOnly(h.listServiceAccounts))
	mux.Handle("POST /api/service-accounts", h.sessionOnly(h.createServiceAccount))
	mux.Handle("GET /api/service-accounts/{id}/keys", h.sessionOnly(h.listAPIKeys))
	mux.Handle("POST /api/service-accounts/{id}/keys", h.sessionOnly(h.createAPIKey))
	mux.Handle("DELETE /api/service-accounts/{id}/keys/{keyId}", h.sessionOnly(h.revokeAPIKey))
	mux.Handle("GET /api/audit-log", h.sessionOnly(h.listAuditLog))
	mux.Handle("GET /api/events", h.authed(h.events))
	mux.Handle("GET /api/projects", h.authed(h.listProjects))
	mux.Handle("POST /api/projects", h.authed(h.createProject))
	mux.Handle("GET /api/projects/{key}", h.authed(h.getProject))
	mux.Handle("PATCH /api/projects/{key}", h.authed(h.updateProject))
	mux.Handle("GET /api/projects/{key}/tags", h.authed(h.listTags))
	mux.Handle("PATCH /api/projects/{key}/tags/{tagId}", h.authed(h.updateTag))
	mux.Handle("GET /api/projects/{key}/issues", h.authed(h.listIssues))
	mux.Handle("POST /api/projects/{key}/issues", h.authed(h.createIssue))
	mux.Handle("GET /api/projects/{key}/issues/{number}", h.authed(h.getIssue))
	mux.Handle("PATCH /api/projects/{key}/issues/{number}", h.authed(h.updateIssue))
	mux.Handle("GET /api/projects/{key}/issues/{number}/comments", h.authed(h.listComments))
	mux.Handle("POST /api/projects/{key}/issues/{number}/comments", h.authed(h.createComment))
	mux.Handle("PATCH /api/projects/{key}/issues/{number}/comments/{commentId}", h.authed(h.updateComment))
	mux.Handle("DELETE /api/projects/{key}/issues/{number}/comments/{commentId}", h.authed(h.deleteComment))
	mux.Handle("GET /api/projects/{key}/issues/{number}/attachments", h.authed(h.listAttachments))
	mux.Handle("POST /api/projects/{key}/issues/{number}/attachments", h.authed(h.uploadAttachment))
	mux.Handle("GET /api/projects/{key}/issues/{number}/attachments/{attachmentId}", h.authed(h.downloadAttachment))
	mux.Handle("DELETE /api/projects/{key}/issues/{number}/attachments/{attachmentId}", h.authed(h.deleteAttachment))
	mux.Handle("GET /api/storages", h.authed(h.listStorages))
	mux.Handle("POST /api/storages", h.authed(h.createStorage))
	mux.Handle("GET /api/storages/{id}", h.authed(h.getStorage))
	mux.Handle("PATCH /api/storages/{id}", h.authed(h.updateStorage))
	mux.Handle("GET /api/storages/{id}/generic-storage-items", h.authed(h.listGenericStorageItemsInStorage))
	mux.Handle("GET /api/generic-storage-items", h.authed(h.searchGenericStorageItems))
	mux.Handle("POST /api/generic-storage-items", h.authed(h.createGenericStorageItem))
	mux.Handle("GET /api/generic-storage-items/{id}", h.authed(h.getGenericStorageItem))
	mux.Handle("PATCH /api/generic-storage-items/{id}", h.authed(h.updateGenericStorageItem))
	mux.Handle("DELETE /api/generic-storage-items/{id}", h.authed(h.deleteGenericStorageItem))
	mux.Handle("POST /api/generic-storage-items/{id}/restore", h.authed(h.restoreGenericStorageItem))
	mux.Handle("GET /api/storages/{id}/network-devices", h.authed(h.listNetworkDevicesInStorage))
	mux.Handle("GET /api/network-devices", h.authed(h.searchNetworkDevices))
	mux.Handle("POST /api/network-devices", h.authed(h.createNetworkDevice))
	mux.Handle("GET /api/network-devices/{id}", h.authed(h.getNetworkDevice))
	mux.Handle("PATCH /api/network-devices/{id}", h.authed(h.updateNetworkDevice))
	mux.Handle("DELETE /api/network-devices/{id}", h.authed(h.deleteNetworkDevice))
	mux.Handle("POST /api/network-devices/{id}/restore", h.authed(h.restoreNetworkDevice))
	mux.Handle("GET /api/storage-items", h.authed(h.searchStorageItems))
	mux.Handle("GET /api/storage-items/{id}/issues", h.authed(h.listStorageItemIssues))

	// Unknown /api paths must 404 as JSON rather than fall through to the SPA.
	mux.HandleFunc("/api/", notFound)
	mux.Handle("/", spa)

	return securityHeaders(csrf.Handler(withRequestInfo(mux)), !cfg.InsecureCookies), nil
}

// missingDependency reports whether a required service or the logger is nil.
func (c Config) missingDependency() bool {
	// Only interface values: a nil pointer in an any is not == nil.
	services := []any{
		c.Auth, c.Projects, c.Issues, c.Tags, c.Comments, c.Attachments, c.Storages, c.GenericStorageItems,
		c.NetworkDevices, c.StorageItems, c.Users, c.ServiceAccounts, c.Audit, c.Changes,
	}

	return c.Logger == nil || slices.Contains(services, nil)
}

// crossOriginProtection rejects cross-origin state-changing requests
// (CSRF), using Sec-Fetch-Site / Origin. The public URL is trusted so the
// app also works behind proxies that rewrite the Host header.
func crossOriginProtection(publicURL string) (*http.CrossOriginProtection, error) {
	cop := http.NewCrossOriginProtection()

	u, err := url.Parse(publicURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid public url %q", publicURL)
	}

	if err := cop.AddTrustedOrigin(u.Scheme + "://" + u.Host); err != nil {
		return nil, fmt.Errorf("trust public url: %w", err)
	}

	cop.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusForbidden, codeCrossOrigin, "cross-origin request rejected")
	}))

	return cop, nil
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version": version.Version,
		"commit":  version.Commit,
	})
}

// openAPISpec serves the API description. It is public, like /api/version:
// it documents the API but grants no access to it.
func (h *Handler) openAPISpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(h.openAPI)
}

func notFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, codeNotFound, "not found")
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body) //nolint:errchkjson // headers are already sent; nothing useful to do on failure
}

// contentSecurityPolicy allows only same-origin scripts, styles and
// connections. Images may also come from https URLs because issue
// descriptions are Markdown and may embed them.
const contentSecurityPolicy = "default-src 'self'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; " +
	"object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"

func securityHeaders(next http.Handler, hsts bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Content-Security-Policy", contentSecurityPolicy)

		if hsts {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}
