package httpapi

import (
	"fmt"
	"io/fs"
	"net/http"
	"strings"
)

// newSPAHandler serves the Vite-built SPA from dist: real files (JS/CSS
// chunks, favicon, ...) are served directly, any other path falls back to
// index.html so the client-side router can handle deep links and refreshes.
func newSPAHandler(dist fs.FS) (http.Handler, error) {
	index, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		return nil, fmt.Errorf("read frontend index.html (did you run `make frontend`?): %w", err)
	}

	fileServer := http.FileServerFS(dist)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fileExists(dist, r.URL.Path) {
			// Vite fingerprints everything under /assets, so it can be cached forever.
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}

			fileServer.ServeHTTP(w, r)

			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	}), nil
}

func fileExists(dist fs.FS, urlPath string) bool {
	name := strings.TrimPrefix(urlPath, "/")
	if name == "" || !fs.ValidPath(name) {
		return false
	}

	info, err := fs.Stat(dist, name)

	return err == nil && !info.IsDir()
}
