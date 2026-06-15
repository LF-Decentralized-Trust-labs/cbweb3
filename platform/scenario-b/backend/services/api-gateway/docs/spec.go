// SPDX-License-Identifier: Apache-2.0

package docs

import "embed"

// OpenAPIYAML contains the embedded OpenAPI specification for this service.
//
//go:embed openapi.yaml
var OpenAPIYAML []byte

// swaggerUIFS embeds the vendored Swagger UI assets (CSS + JS bundle) so the
// /docs page renders fully offline with no external CDN dependency.
//
//go:embed swagger-ui/*
var swaggerUIFS embed.FS

// swaggerUIContentTypes maps vendored asset names to their HTTP content type.
var swaggerUIContentTypes = map[string]string{
	"swagger-ui.css":       "text/css",
	"swagger-ui-bundle.js": "application/javascript",
}

// SwaggerUIAsset returns the bytes and content type for a vendored Swagger UI
// asset. ok is false if the asset is unknown or cannot be read. Only the
// allow-listed assets in swaggerUIContentTypes are served.
func SwaggerUIAsset(name string) (data []byte, contentType string, ok bool) {
	contentType, known := swaggerUIContentTypes[name]
	if !known {
		return nil, "", false
	}
	b, err := swaggerUIFS.ReadFile("swagger-ui/" + name)
	if err != nil {
		return nil, "", false
	}
	return b, contentType, true
}
