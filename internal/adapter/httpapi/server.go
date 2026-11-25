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
	Users    port.UserService
	// ServiceAccounts manages "bot-<name>" users and their API keys.
	ServiceAccounts port.ServiceAccountService
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
	auth     port.AuthService
	projects port.ProjectService
	issues   port.IssueService
	tags     port.TagService
	comments port.CommentService
	users    port.UserService
	accounts port.ServiceAccountService
	logger   *slog.Logger
	cookies  cookies
	openAPI  []byte
}

// NewHandler builds the root http.Handler.
func NewHandler(cfg Config) (http.Handler, error) {
	if len(cfg.CookieKey) < minCookieKeyBytes {
		return nil, fmt.Errorf("cookie key must be at least %d bytes", minCookieKeyBytes)
	}

	if cfg.Auth == nil || cfg.Projects == nil || cfg.Issues == nil || cfg.Tags == nil || cfg.Comments == nil || cfg.Users == nil ||
		cfg.ServiceAccounts == nil || cfg.Logger == nil {
		return nil, errors.New("all services and the logger are required")
	}

	h := &Handler{
		auth:     cfg.Auth,
		projects: cfg.Projects,
		issues:   cfg.Issues,
		tags:     cfg.Tags,
		comments: cfg.Comments,
		users:    cfg.Users,
		accounts: cfg.ServiceAccounts,
		logger:   cfg.Logger,
		cookies:  cookies{secure: !cfg.InsecureCookies, key: cfg.CookieKey},
		openAPI:  cfg.OpenAPISpec,
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
	// service accounts are managed by people only.
	mux.Handle("GET /api/me", h.authed(h.me))
	mux.Handle("GET /api/users", h.authed(h.listUsers))
	mux.Handle("GET /api/service-accounts", h.sessionOnly(h.listServiceAccounts))
	mux.Handle("POST /api/service-accounts", h.sessionOnly(h.createServiceAccount))
	mux.Handle("GET /api/service-accounts/{id}/keys", h.sessionOnly(h.listAPIKeys))
	mux.Handle("POST /api/service-accounts/{id}/keys", h.sessionOnly(h.createAPIKey))
	mux.Handle("DELETE /api/service-accounts/{id}/keys/{keyId}", h.sessionOnly(h.revokeAPIKey))
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

	// Unknown /api paths must 404 as JSON rather than fall through to the SPA.
	mux.HandleFunc("/api/", notFound)
	mux.Handle("/", spa)

	csrf, err := crossOriginProtection(cfg.PublicURL)
	if err != nil {
		return nil, err
	}

	return securityHeaders(csrf.Handler(mux), !cfg.InsecureCookies), nil
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
