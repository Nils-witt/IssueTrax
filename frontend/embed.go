// Package frontend embeds the built Vite SPA (frontend/dist, produced by
// `npm run build`) so the Go binary can serve it.
package frontend

import "embed"

// DistFS holds the Vite build output under "dist".
//
//go:embed all:dist
var DistFS embed.FS
