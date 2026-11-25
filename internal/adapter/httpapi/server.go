// Package httpapi is the inbound HTTP adapter: it exposes the core's driving
// ports as a JSON API under /api and serves the embedded single-page app on
// every other path.
package httpapi

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"

	"nilswitt.dev/issuetrax/internal/version"
)

// Handler routes HTTP requests to the core services and the SPA.
type Handler struct {
	logger *slog.Logger
}

// NewHandler builds the root http.Handler. frontend must contain the built
// SPA with index.html at its root.
func NewHandler(frontend fs.FS, logger *slog.Logger) (http.Handler, error) {
	h := &Handler{logger: logger}

	spa, err := newSPAHandler(frontend)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /api/version", h.version)
	// Unknown /api paths must 404 as JSON rather than fall through to the SPA.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})
	mux.Handle("/", spa)

	return securityHeaders(mux), nil
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

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body) //nolint:errchkjson // headers are already sent; nothing useful to do on failure
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}
