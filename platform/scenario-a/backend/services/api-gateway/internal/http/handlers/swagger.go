// SPDX-License-Identifier: Apache-2.0

// This file serves OpenAPI YAML and Swagger UI endpoints.
package handlers

import (
	apidocs "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/docs"
	"github.com/gofiber/fiber/v2"
)

const swaggerHTML = `<!doctype html>
<html>
  <head>
    <meta charset="utf-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1"/>
    <title>CBWeb3 API Gateway Swagger</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
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
