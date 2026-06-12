// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"io"
	"net/http/httptest"
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
	assert.Contains(t, string(body), "swagger-ui")
	assert.Contains(t, string(body), "/openapi.yaml")
}
