package docs

import _ "embed"

// OpenAPIYAML contains the embedded OpenAPI specification for this service.
//
//go:embed openapi.yaml
var OpenAPIYAML []byte
