// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestOpenAPIYAML(t *testing.T) {
	app := fiber.New()
	app.Get("/openapi.yaml", OpenAPIYAML)

	req := httptest.NewRequest("GET", "/openapi.yaml", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "yaml")

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.NotEmpty(t, body)
}

func TestSwaggerUI(t *testing.T) {
	app := fiber.New()
	app.Get("/swagger", SwaggerUI)

	req := httptest.NewRequest("GET", "/swagger", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "html")

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	html := string(body)
	assert.Contains(t, html, "swagger-ui")
	assert.Contains(t, html, "/openapi.yaml")
	// Offline capability: the page must reference only gateway-local assets,
	// never an external CDN (unpkg / jsdelivr / cdnjs).
	assert.Contains(t, html, "/docs/swagger-ui/swagger-ui.css")
	assert.Contains(t, html, "/docs/swagger-ui/swagger-ui-bundle.js")
	for _, cdn := range []string{"unpkg.com", "jsdelivr", "cdnjs", "://"} {
		assert.NotContains(t, html, cdn, "swagger HTML must not reference external CDN %q", cdn)
	}
}

func TestSwaggerUIAsset(t *testing.T) {
	app := fiber.New()
	app.Get("/docs/swagger-ui/:asset", SwaggerUIAsset)

	cases := []struct {
		name        string
		path        string
		contentType string
	}{
		{"css", "/docs/swagger-ui/swagger-ui.css", "text/css"},
		{"bundle", "/docs/swagger-ui/swagger-ui-bundle.js", "javascript"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest("GET", tc.path, nil))
			assert.NoError(t, err)
			assert.Equal(t, fiber.StatusOK, resp.StatusCode)
			assert.True(t, strings.Contains(resp.Header.Get("Content-Type"), tc.contentType))
			body, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			assert.NotEmpty(t, body)
		})
	}

	// Unknown asset must 404 (allow-list enforced).
	resp, err := app.Test(httptest.NewRequest("GET", "/docs/swagger-ui/evil.js", nil))
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}
