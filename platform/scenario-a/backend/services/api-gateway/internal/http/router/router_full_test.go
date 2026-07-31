// SPDX-License-Identifier: Apache-2.0

// This file exercises the full route-registration matrix in Setup: the Central
// Bank configuration (PaymentHandler + Governance + TransferLimit + Onboarding)
// and the Commercial Bank configuration (PaymentProxy + OnboardingProxy). It
// asserts that the conditional route groups are registered (routes resolve to a
// handler rather than 404), without needing live backends.
package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/gofiber/fiber/v2"
)

// govComplianceStub satisfies handlers.GovernanceCompliance for router wiring.
type govComplianceStub struct{}

func (govComplianceStub) RegisterParticipant(context.Context, complianceadapter.Participant) error {
	return nil
}
func (govComplianceStub) ListParticipants(context.Context, string, string) ([]complianceadapter.Participant, error) {
	return nil, nil
}
func (govComplianceStub) SignParticipantCSR(context.Context, string, string, string, string, string) (complianceadapter.SignedCSRResult, error) {
	return complianceadapter.SignedCSRResult{}, nil
}
func (govComplianceStub) ApproveKYC(context.Context, string, string, string) (complianceadapter.ApproveKYCResult, error) {
	return complianceadapter.ApproveKYCResult{}, nil
}
func (govComplianceStub) ManageParticipantStatus(context.Context, string, string, string) error {
	return nil
}
func (govComplianceStub) GetAuditLogs(context.Context, string, string, string, string, int, int) ([]complianceadapter.AuditRecord, error) {
	return nil, nil
}
func (govComplianceStub) GetCircuitBreakerStatus(context.Context) (complianceadapter.CircuitBreakerStatus, error) {
	return complianceadapter.CircuitBreakerStatus{}, nil
}
func (govComplianceStub) ToggleCircuitBreaker(context.Context, bool, string) (bool, string, error) {
	return false, "", nil
}
func (govComplianceStub) GetSystemParameters(context.Context) (complianceadapter.SystemParameters, error) {
	return complianceadapter.SystemParameters{}, nil
}
func (govComplianceStub) UpdateSystemParameters(context.Context, complianceadapter.SystemParameters, string, string) error {
	return nil
}

func newCentralBankApp(t *testing.T) *fiber.App {
	t.Helper()
	t.Setenv("INTERNAL_RELAY_AUTH_SECRET", "relay-secret")

	mgr := fullKYCManagerStub{}
	authProvider := roleAuthProviderStub{roles: []string{domain.RoleGovernance, domain.RoleTreasury}}

	app := fiber.New()
	Setup(app, Dependencies{
		AuthHandler:          handlers.NewAuthHandler(authProvider, mgr, false),
		ComplianceHandler:    handlers.NewComplianceHandler(mgr, nil),
		GovernanceHandler:    handlers.NewGovernanceHandler(govComplianceStub{}),
		TransferLimitHandler: handlers.NewTransferLimitHandler(noopTransferLimitManager{}),
		PaymentHandler:       handlers.NewPaymentHandler(nil, "bank-a"),
		OnboardingHandler:    handlers.NewOnboardingHandler(onboardingManagerStub{}),
		AuthProvider:         authProvider,
	})
	return app
}

func TestSetup_CentralBankRoutesRegistered(t *testing.T) {
	app := newCentralBankApp(t)

	// Each route should resolve to a handler (NOT 404). With a nil payment
	// adapter the handler may 500/panic-recover, but the router-level assertion
	// is simply: the route exists.
	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/governance/registry"},
		{http.MethodGet, "/api/v1/governance/parameters"},
		{http.MethodGet, "/api/v1/governance/users"},
		{http.MethodGet, "/api/v1/treasury/transfer-limits"},
		{http.MethodGet, "/api/v1/onboarding/status/req-1"},
		{http.MethodGet, "/api/v1/onboarding/my-status?bank_code=a"},
	}
	for _, tc := range routes {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.AddCookie(&http.Cookie{Name: "access_token", Value: "tok"})
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.method, tc.path, err)
		}
		if resp.StatusCode == http.StatusNotFound {
			t.Errorf("%s %s: route not registered (404)", tc.method, tc.path)
		}
	}
}

func TestSetup_InternalRelayRoutesRegistered(t *testing.T) {
	app := newCentralBankApp(t)

	// Internal relay routes require X-Relay-Auth; without it → 401 (proves the
	// route + middleware are wired, not 404).
	for _, path := range []string{
		"/internal/v1/payments/fx/agreements",
		"/internal/v1/payments/deposits",
		"/internal/v1/payments/escrows",
		"/internal/v1/payments/redeems",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		if resp.StatusCode == http.StatusNotFound {
			t.Errorf("GET %s: internal route not registered (404)", path)
		}
	}
}

func TestSetup_CommercialBankRoutesRegistered(t *testing.T) {
	t.Parallel()

	authProvider := roleAuthProviderStub{roles: []string{domain.RoleCommercialBank}}
	proxyHandler := handlers.NewOnboardingProxyHandler("http://localhost:19999", "", "bank-a", nil)
	paymentProxy := handlers.NewPaymentProxyHandler("http://localhost:19999", nil, "0x", "pal", "cbpal", "")

	app := fiber.New()
	Setup(app, Dependencies{
		AuthHandler:            handlers.NewAuthHandler(authProvider, fullKYCManagerStub{}, false),
		ComplianceHandler:      handlers.NewComplianceHandler(fullKYCManagerStub{}, nil),
		PaymentHandler:         handlers.NewPaymentHandler(nil, "bank-a"),
		PaymentProxyHandler:    paymentProxy,
		OnboardingProxyHandler: proxyHandler,
		AuthProvider:           authProvider,
	})

	// Commercial-bank payment proxy routes exist.
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/payments/deposits"},
		{http.MethodGet, "/api/v1/payments/escrows"},
		{http.MethodGet, "/api/v1/payments/redeems"},
		{http.MethodPost, "/api/v1/onboarding/initiate"},
		{http.MethodGet, "/api/v1/auth/me"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		req.AddCookie(&http.Cookie{Name: "access_token", Value: "tok"})
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.method, tc.path, err)
		}
		if resp.StatusCode == http.StatusNotFound {
			t.Errorf("%s %s: route not registered (404)", tc.method, tc.path)
		}
	}

	// Governance + transfer-limit routes must be ABSENT on a commercial bank.
	for _, path := range []string{
		"/api/v1/governance/registry",
		"/api/v1/treasury/transfer-limits",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(&http.Cookie{Name: "access_token", Value: "tok"})
		resp, _ := app.Test(req)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s: expected 404 on commercial bank, got %d", path, resp.StatusCode)
		}
	}
}
