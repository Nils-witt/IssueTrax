// Package version holds build metadata set at compile time via
// `-ldflags "-X ..."` (see Makefile and Dockerfile).
package version

var (
	// Version is the git tag the binary was built from.
	Version = "dev"
	// Commit is the short git commit hash the binary was built from.
	Commit = "unknown"
)
