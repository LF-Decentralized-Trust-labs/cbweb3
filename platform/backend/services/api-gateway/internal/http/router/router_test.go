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

func (s authProviderStub) Validate(_ context.Context, _ string) (domain.TokenClaims, error) {
	return domain.TokenClaims{Subject: "bank-a"}, nil
}

// roleValidatorStub returns a configurable set of roles for every token.
type roleAuthProviderStub struct{ roles []string }

func (s roleAuthProviderStub) Authenticate(_ context.Context, _, _ string) (domain.AuthToken, error) {
	return domain.AuthToken{}, errors.New("not used")
}

func (s roleAuthProviderStub) RefreshToken(_ context.Context, _ string) (domain.AuthToken, error) {
	return domain.AuthToken{}, errors.New("not used")
}

func (s roleAuthProviderStub) Logout(_ context.Context, _ string) error {
	return nil
}

func (s roleAuthProviderStub) Validate(_ context.Context, _ string) (domain.TokenClaims, error) {
	return domain.TokenClaims{Subject: "bank-a", Roles: s.roles}, nil
}

type kycCheckerStub struct{}

func (s kycCheckerStub) GetStatus(_ string) domain.KYCStatus {
	return domain.KYCApproved
}

// fullKYCManagerStub implements KYCChecker + KYCManager + ParticipantOnboarder.
type fullKYCManagerStub struct{}

func (s fullKYCManagerStub) GetStatus(_ string) domain.KYCStatus { return domain.KYCApproved }

func (s fullKYCManagerStub) GetKYCStatus(_ context.Context, _ string) (domain.KYCStatus, error) {
	return domain.KYCApproved, nil
}

func (s fullKYCManagerStub) ProvisionParticipant(_ context.Context, _ string, _ domain.KYCStatus) error {
	return nil
}

func (s fullKYCManagerStub) OnboardParticipant(_ context.Context, _ interfaces.OnboardParticipantRequest) (interfaces.OnboardParticipantResult, error) {
	return interfaces.OnboardParticipantResult{UserID: "stub-user-id"}, nil
}

func TestRequireRoleBlocksCommercialBank(t *testing.T) {
	t.Parallel()

	mgr := fullKYCManagerStub{}
	authHandler := handlers.NewAuthHandler(authProviderStub{}, mgr, false)
	complianceHandler := handlers.NewComplianceHandler(mgr, nil)
	app := fiber.New()
	// Validator always returns COMMERCIAL_BANK role — never ROLE_GOVERNANCE.
	authProvider := roleAuthProviderStub{roles: []string{domain.RoleCommercialBank}}
	Setup(app, Dependencies{
		AuthHandler:       authHandler,
		ComplianceHandler: complianceHandler,
		AuthProvider:      authProvider,
	})

	protectedRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/compliance/participants"},
		{http.MethodPost, "/api/v1/compliance/kyc/issue-credential"},
		{http.MethodPost, "/api/v1/compliance/participants/provision"},
		{http.MethodPost, "/api/v1/compliance/accounts/freeze"},
		{http.MethodPost, "/api/v1/compliance/accounts/unfreeze"},
		{http.MethodPost, "/api/v1/compliance/register"},
	}
	for _, tc := range protectedRoutes {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		req.AddCookie(&http.Cookie{Name: "access_token", Value: "fake-token"})
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

	authHandler := handlers.NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false)
	complianceHandler := handlers.NewComplianceHandler(kycCheckerStub{}, nil)
	app := fiber.New()
	Setup(app, Dependencies{
		AuthHandler:       authHandler,
		ComplianceHandler: complianceHandler,
		AuthProvider:      authProviderStub{},
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

	swaggerResp, err := app.Test(httptest.NewRequest(http.MethodGet, "/docs", nil))
	if err != nil {
		t.Fatalf("unexpected error on swagger route: %v", err)
	}
	if swaggerResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from swagger route, got %d", swaggerResp.StatusCode)
	}
}

func TestMeRouteRequiresToken(t *testing.T) {
	t.Parallel()

	authHandler := handlers.NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false)
	complianceHandler := handlers.NewComplianceHandler(kycCheckerStub{}, nil)
	app := fiber.New()
	Setup(app, Dependencies{
		AuthHandler:       authHandler,
		ComplianceHandler: complianceHandler,
		AuthProvider:      authProviderStub{},
	})

	// Without cookie → 401 from RequireCookieAuth middleware.
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", resp.StatusCode)
	}

	// With cookie → 200 (stub always validates).
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "valid-token"})
	resp2, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with token, got %d", resp2.StatusCode)
	}
}

func TestGovernanceUsersRoutesRequireRole(t *testing.T) {
	t.Parallel()

	mgr := fullKYCManagerStub{}
	authHandler := handlers.NewAuthHandler(authProviderStub{}, mgr, false)
	complianceHandler := handlers.NewComplianceHandler(mgr, nil)
	app := fiber.New()
	// ROLE_COMMERCIAL_BANK → must be blocked from governance routes.
	authProvider := roleAuthProviderStub{roles: []string{domain.RoleCommercialBank}}
	Setup(app, Dependencies{
		AuthHandler:       authHandler,
		ComplianceHandler: complianceHandler,
		AuthProvider:      authProvider,
	})

	for _, path := range []string{
		"/api/v1/governance/users",
		"/api/v1/governance/users/some-id",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(&http.Cookie{Name: "access_token", Value: "fake-token"})
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("GET %s: unexpected error: %v", path, err)
		}
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("GET %s: expected 403, got %d", path, resp.StatusCode)
		}
	}
}

// onboardingManagerStub is a minimal interfaces.OnboardingManager for router wiring tests.
type onboardingManagerStub struct {
	result interfaces.OnboardingStatus
	err    error
}

func (s onboardingManagerStub) SubmitCredentialRequest(_ context.Context, _ interfaces.CredentialRequest) (interfaces.CredentialRequestResult, error) {
	return interfaces.CredentialRequestResult{}, nil
}

func (s onboardingManagerStub) GetOnboardingStatus(_ context.Context, _ string) (interfaces.OnboardingStatus, error) {
	return s.result, s.err
}

func (s onboardingManagerStub) GetOnboardingStatusByBankCode(_ context.Context, _ string) (interfaces.OnboardingStatus, error) {
	return s.result, s.err
}

func (s onboardingManagerStub) CompleteOnboarding(_ context.Context, _ interfaces.CompleteOnboardingRequest) (interfaces.CompleteOnboardingResult, error) {
	return interfaces.CompleteOnboardingResult{}, nil
}

// TestCBMyStatusRouteIsRegistered verifies that GET /api/v1/onboarding/my-status
// is registered on a Central Bank gateway (OnboardingHandler mode) and is
// accessible without authentication (public endpoint).
func TestCBMyStatusRouteIsRegistered(t *testing.T) {
	t.Parallel()

	mgr := onboardingManagerStub{
		result: interfaces.OnboardingStatus{
			RequestID: "req-abc",
			UserID:    "user-abc",
			Status:    "CREDENTIAL_REQUESTED",
		},
	}
	app := fiber.New()
	Setup(app, Dependencies{
		AuthHandler:       handlers.NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false),
		ComplianceHandler: handlers.NewComplianceHandler(kycCheckerStub{}, nil),
		OnboardingHandler: handlers.NewOnboardingHandler(mgr),
		AuthProvider:      authProviderStub{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/onboarding/my-status?bank_code=a", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Returns 200 (stub returns a valid result) or at worst 400/404; should NOT be 404 router-wise.
	if resp.StatusCode == http.StatusMethodNotAllowed {
		t.Fatalf("route not registered (405 Method Not Allowed)")
	}
	// bank_code=a has a result → expect 200
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from CB my-status route, got %d", resp.StatusCode)
	}
}

// TestBankMyStatusRouteRequiresAuth verifies that GET /api/v1/onboarding/my-status
// on a Commercial Bank gateway (OnboardingProxyHandler mode) is protected by
// RequireCookieAuth — unauthenticated requests must return 401.
func TestBankMyStatusRouteRequiresAuth(t *testing.T) {
	t.Parallel()

	// OnboardingProxyHandler with no live CB — the auth middleware fires first.
	proxyHandler := handlers.NewOnboardingProxyHandler("http://localhost:19999", "", "a", nil)
	app := fiber.New()
	Setup(app, Dependencies{
		AuthHandler:            handlers.NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false),
		ComplianceHandler:      handlers.NewComplianceHandler(kycCheckerStub{}, nil),
		OnboardingProxyHandler: proxyHandler,
		AuthProvider:           authProviderStub{},
	})

	// No cookie → middleware rejects before the handler runs.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/onboarding/my-status", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth cookie, got %d", resp.StatusCode)
	}
}
