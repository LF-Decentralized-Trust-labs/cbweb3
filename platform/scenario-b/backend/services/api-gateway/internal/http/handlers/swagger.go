// SPDX-License-Identifier: Apache-2.0

// This file serves OpenAPI YAML and Swagger UI endpoints.
//
// Swagger UI assets (CSS + JS bundle) are vendored locally under
// docs/swagger-ui and embedded into the binary, so the /docs page renders
// fully offline with no external CDN dependency.
package handlers

import (
	apidocs "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/docs"
	"github.com/gofiber/fiber/v2"
)

// swaggerHTML loads Swagger UI from gateway-local, embedded assets served at
// /docs/swagger-ui/*. No unpkg/CDN reference, so the page works air-gapped.
const swaggerHTML = `<!doctype html>
<html>
  <head>
    <meta charset="utf-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1"/>
    <title>CBWeb3 API Gateway Swagger</title>
    <link rel="stylesheet" href="/docs/swagger-ui/swagger-ui.css" />
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script src="/docs/swagger-ui/swagger-ui-bundle.js"></script>
    <script>
      window.ui = SwaggerUIBundle({
        url: "/openapi.yaml",
        dom_id: "#swagger-ui"
      });
    </script>
  </body>
</html>`

// OpenAPIYAML returns the OpenAPI specification file.
func OpenAPIYAML(c *fiber.Ctx) error {
	if len(apidocs.OpenAPIYAML) == 0 {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "unable to load openapi specification"})
	}

	c.Set(fiber.HeaderContentType, "application/yaml; charset=utf-8")
	return c.Send(apidocs.OpenAPIYAML)
}

// SwaggerUI returns a static Swagger UI page bound to /openapi.yaml.
func SwaggerUI(c *fiber.Ctx) error {
	c.Type("html")
	return c.SendString(swaggerHTML)
}

// SwaggerUIAsset serves a vendored Swagger UI asset (CSS/JS) from the embedded
// filesystem. The asset name is taken from the ":asset" route parameter.
// Only known assets are served; anything else returns 404. This keeps the
// /docs page fully self-contained (no external CDN).
func SwaggerUIAsset(c *fiber.Ctx) error {
	name := c.Params("asset")
	data, contentType, ok := apidocs.SwaggerUIAsset(name)
	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "unknown swagger-ui asset"})
	}
	c.Set(fiber.HeaderContentType, contentType)
	return c.Send(data)
}
