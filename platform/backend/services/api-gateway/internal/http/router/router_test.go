// This file tests route registration and basic router wiring.
package router

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/gofiber/fiber/v2"
)

type authProviderStub struct{}

func (s authProviderStub) Authenticate(_ context.Context, _, _ string) (domain.AuthToken, error) {
	return domain.AuthToken{}, errors.New("not used")
}

func (s authProviderStub) RefreshToken(_ context.Context, _ string) (domain.AuthToken, error) {
	return domain.AuthToken{}, errors.New("not used")
}

func (s authProviderStub) Logout(_ context.Context, _ string) error {
	return nil
}

type identityManagerStub struct{}

func (s identityManagerStub) BindWallet(_, _ string) (domain.WalletBinding, error) {
	return domain.WalletBinding{}, nil
}

func (s identityManagerStub) GetByUser(_ string) (domain.WalletBinding, bool) {
	return domain.WalletBinding{}, false
}

type kycCheckerStub struct{}

func (s kycCheckerStub) GetStatus(_ string) domain.KYCStatus {
	return domain.KYCApproved
}

type validatorStub struct{}

func (s validatorStub) Validate(_ context.Context, _ string) (domain.TokenClaims, error) {
	return domain.TokenClaims{Subject: "bank-a"}, nil
}

func TestSetupRegistersRoutes(t *testing.T) {
	t.Parallel()

	authHandler := handlers.NewAuthHandler(authProviderStub{}, identityManagerStub{}, kycCheckerStub{})
	complianceHandler := handlers.NewComplianceHandler(kycCheckerStub{})
	app := fiber.New()
	Setup(app, Dependencies{
		AuthHandler:       authHandler,
		ComplianceHandler: complianceHandler,
		TokenValidator:    validatorStub{},
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from health route, got %d", resp.StatusCode)
	}

	openAPIResp, err := app.Test(httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))
	if err != nil {
		t.Fatalf("unexpected error on openapi route: %v", err)
	}
	if openAPIResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from openapi route, got %d", openAPIResp.StatusCode)
	}

	swaggerResp, err := app.Test(httptest.NewRequest(http.MethodGet, "/swagger", nil))
	if err != nil {
		t.Fatalf("unexpected error on swagger route: %v", err)
	}
	if swaggerResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from swagger route, got %d", swaggerResp.StatusCode)
	}
}

