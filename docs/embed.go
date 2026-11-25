// Package docs embeds the hand-written API documentation so the server can
// serve it.
package docs

import _ "embed"

// OpenAPI is the OpenAPI 3.1 description of the /api JSON API (openapi.yaml).
//
//go:embed openapi.yaml
var OpenAPI []byte
