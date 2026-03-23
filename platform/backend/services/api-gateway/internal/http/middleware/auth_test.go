// This file tests bearer-token middleware authorization scenarios.
package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/gofiber/fiber/v2"
)

type tokenValidatorStub struct {
	claims domain.TokenClaims
	err    error
}

func (s tokenValidatorStub) Authenticate(_ context.Context, _, _ string) (domain.AuthToken, error) {
	return domain.AuthToken{}, s.err
}

func (s tokenValidatorStub) RefreshToken(_ context.Context, _ string) (domain.AuthToken, error) {
	return domain.AuthToken{}, s.err
}

func (s tokenValidatorStub) Logout(_ context.Context, _ string) error {
	return s.err
}

func (s tokenValidatorStub) Validate(_ context.Context, _ string) (domain.TokenClaims, error) {
	return s.claims, s.err
}

func TestRequireBearerToken(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/protected", RequireBearerToken(tokenValidatorStub{
		claims: domain.TokenClaims{Subject: "bank-a"},
	}), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRequireBearerTokenMissing(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/protected", RequireBearerToken(tokenValidatorStub{}), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

