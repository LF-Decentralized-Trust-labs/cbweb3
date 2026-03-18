// This file tests route registration and basic router wiring.
package router

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
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

type kycCheckerStub struct{}

func (s kycCheckerStub) GetStatus(_ string) domain.KYCStatus {
	return domain.KYCApproved
}

type validatorStub struct{}

func (s validatorStub) Validate(_ context.Context, _ string) (domain.TokenClaims, error) {
	return domain.TokenClaims{Subject: "bank-a"}, nil
}

// roleValidatorStub returns a configurable set of roles for every token.
type roleValidatorStub struct{ roles []string }

func (s roleValidatorStub) Validate(_ context.Context, _ string) (domain.TokenClaims, error) {
	return domain.TokenClaims{Subject: "bank-a", Roles: s.roles}, nil
}

// fullKYCManagerStub implements KYCChecker + KYCManager + ParticipantRegistrar.
type fullKYCManagerStub struct{}

func (s fullKYCManagerStub) GetStatus(_ string) domain.KYCStatus { return domain.KYCApproved }

func (s fullKYCManagerStub) GetKYCStatus(_ context.Context, _ string) (domain.KYCStatus, error) {
	return domain.KYCApproved, nil
}

func (s fullKYCManagerStub) ProvisionParticipant(_ context.Context, _ string, _ domain.KYCStatus) error {
	return nil
}

func (s fullKYCManagerStub) RegisterParticipant(_ context.Context, _, _, _, _, _ string) (interfaces.RegisterParticipantResult, error) {
	return interfaces.RegisterParticipantResult{}, nil
}

func TestRequireRoleBlocksCommercialBank(t *testing.T) {
	t.Parallel()

	mgr := fullKYCManagerStub{}
	authHandler := handlers.NewAuthHandler(authProviderStub{}, mgr)
	complianceHandler := handlers.NewComplianceHandler(mgr)
	app := fiber.New()
	// Validator always returns COMMERCIAL_BANK role — never CENTRAL_BANK.
	validator := roleValidatorStub{roles: []string{domain.RoleCommercialBank}}
	Setup(app, Dependencies{
		AuthHandler:       authHandler,
		ComplianceHandler: complianceHandler,
		TokenValidator:    validator,
	})

	protectedRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/compliance/kyc/issue-credential"},
		{http.MethodPost, "/compliance/participants/provision"},
		{http.MethodPost, "/compliance/accounts/freeze"},
		{http.MethodPost, "/compliance/accounts/unfreeze"},
	}
	for _, tc := range protectedRoutes {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer fake-token")
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("%s %s: unexpected error: %v", tc.method, tc.path, err)
		}
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s %s: expected 403, got %d", tc.method, tc.path, resp.StatusCode)
		}
	}
}

func TestSetupRegistersRoutes(t *testing.T) {
	t.Parallel()

	authHandler := handlers.NewAuthHandler(authProviderStub{}, kycCheckerStub{})
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
