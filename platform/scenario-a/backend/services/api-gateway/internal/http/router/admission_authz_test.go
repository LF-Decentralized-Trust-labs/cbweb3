// SPDX-License-Identifier: Apache-2.0

// Authorization regression suite for the Admission profile (spec 042, Scenario A).
//
// Source of truth: specs/042-admission-onboarding-profile/contracts/authorization-matrix.md
//
// Scenario A is the harder of the two scenarios: BOTH portal-critical routes live
// inside the prefix-scoped govGroup (the portal reads GET /governance/registry and
// approves via POST /governance/approve-kyc — see
// frontend/apps/governance/src/services/api/registry.api.ts:14,19,39), and a second
// governance sub-group (centralBankRoutes, a Group("") on complianceGroup) covers the
// compliance-side onboarding routes. Both groups must be relaxed to the role union
// with explicit per-route guards; a route-level swap alone 403s both profiles, and a
// route left unguarded after the relaxation is reachable by both.
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

// admissionGovComplianceStub implements handlers.GovernanceCompliance so the positive
// (2xx) cases reach a handler instead of nil-panicking.
type admissionGovComplianceStub struct{}

func (admissionGovComplianceStub) RegisterParticipant(context.Context, complianceadapter.Participant) error {
	return nil
}

func (admissionGovComplianceStub) ListParticipants(context.Context, string, string) ([]complianceadapter.Participant, error) {
	return []complianceadapter.Participant{}, nil
}

func (admissionGovComplianceStub) SignParticipantCSR(context.Context, string, string, string, string, string) (complianceadapter.SignedCSRResult, error) {
	return complianceadapter.SignedCSRResult{}, nil
}

func (admissionGovComplianceStub) ApproveKYC(_ context.Context, subject, _, _ string) (complianceadapter.ApproveKYCResult, error) {
	return complianceadapter.ApproveKYCResult{Subject: subject, Status: "KYC_APPROVED"}, nil
}

func (admissionGovComplianceStub) ManageParticipantStatus(context.Context, string, string, string) error {
	return nil
}

func (admissionGovComplianceStub) GetAuditLogs(context.Context, string, string, string, string, int, int) ([]complianceadapter.AuditRecord, error) {
	return []complianceadapter.AuditRecord{}, nil
}

func (admissionGovComplianceStub) GetCircuitBreakerStatus(context.Context) (complianceadapter.CircuitBreakerStatus, error) {
	return complianceadapter.CircuitBreakerStatus{}, nil
}

func (admissionGovComplianceStub) ToggleCircuitBreaker(context.Context, bool, string) (bool, string, error) {
	return false, "", nil
}

func (admissionGovComplianceStub) GetSystemParameters(context.Context) (complianceadapter.SystemParameters, error) {
	return complianceadapter.SystemParameters{}, nil
}

func (admissionGovComplianceStub) UpdateSystemParameters(context.Context, complianceadapter.SystemParameters, string, string) error {
	return nil
}

// admissionTestApp builds a Central-Bank-shaped gateway (PaymentProxyHandler nil, so
// the governance group is registered) whose validator returns exactly these roles.
func admissionTestApp(roles ...string) *fiber.App {
	mgr := fullKYCManagerStub{}
	compliance := admissionGovComplianceStub{}
	app := fiber.New()
	Setup(app, Dependencies{
		AuthHandler:       handlers.NewAuthHandler(authProviderStub{}, mgr, false),
		ComplianceHandler: handlers.NewComplianceHandler(mgr, compliance),
		GovernanceHandler: handlers.NewGovernanceHandler(compliance),
		AuthProvider:      roleAuthProviderStub{roles: roles},
	})
	return app
}

func admissionProbe(t *testing.T, app *fiber.App, method, path, body string) int {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "fake-token"})
	attachCSRF(t, req, "fake-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("%s %s: unexpected error: %v", method, path, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

type admissionRoute struct {
	method string
	path   string
	body   string
}

var (
	// Portal-critical pair — both inside govGroup, both needed by an Admission operator.
	aPortalRegistryRead = admissionRoute{http.MethodGet, "/api/v1/governance/registry", ""}
	aPortalApprove      = admissionRoute{http.MethodPost, "/api/v1/governance/approve-kyc", `{"subject":"bank-a"}`}

	// Compliance-side onboarding routes, inside the centralBankRoutes sub-group.
	aComplianceRead    = admissionRoute{http.MethodGet, "/api/v1/compliance/participants", ""}
	aComplianceApprove = admissionRoute{http.MethodPost, "/api/v1/compliance/approve-kyc", `{"subject":"bank-a"}`}

	// Registering the participant record (govGroup).
	aGovRegister = admissionRoute{http.MethodPost, "/api/v1/governance/participants", `{"user_id":"u1","role":"ROLE_COMMERCIAL_BANK"}`}

	admissionAllowed = []admissionRoute{
		aComplianceRead, aComplianceApprove, aPortalRegistryRead, aPortalApprove, aGovRegister,
	}

	// Governance-retained. Each needs an explicit guard once its group guard is a union.
	// /compliance/register stays here per T029a: it reaches auth OnboardParticipant,
	// which registers+verifies on-chain inline, and it provisions CB operator accounts
	// (its IsAdminRole allowlist admits TREASURY/NOC/SUPERVISOR) — not bank admission.
	aGovernanceRetained = []admissionRoute{
		{http.MethodPost, "/api/v1/compliance/register", `{"username":"u","email":"u@example.org","role":"ROLE_COMMERCIAL_BANK"}`},
		{http.MethodPost, "/api/v1/compliance/participants/provision", `{"subject":"bank-a","status":"FROZEN"}`},
		{http.MethodPost, "/api/v1/compliance/accounts/freeze", `{"subject":"bank-a"}`},
		{http.MethodPost, "/api/v1/compliance/accounts/unfreeze", `{"subject":"bank-a"}`},
		{http.MethodPost, "/api/v1/governance/registry/csr", `{"csr_pem":"x","user_id":"u1","role":"ROLE_COMMERCIAL_BANK","institution_name":"Bank"}`},
		{http.MethodGet, "/api/v1/governance/accounts", ""},
		{http.MethodPost, "/api/v1/governance/accounts/freeze", `{"subject":"bank-a"}`},
		{http.MethodPost, "/api/v1/governance/accounts/unfreeze", `{"subject":"bank-a"}`},
		{http.MethodGet, "/api/v1/governance/circuit-breaker/status", ""},
		{http.MethodPost, "/api/v1/governance/circuit-breaker/toggle", `{"pause":true}`},
		{http.MethodGet, "/api/v1/governance/parameters", ""},
		{http.MethodPut, "/api/v1/governance/parameters", `{}`},
		{http.MethodGet, "/api/v1/governance/audit/logs", ""},
		{http.MethodGet, "/api/v1/governance/users", ""},
		{http.MethodGet, "/api/v1/governance/users/some-id", ""},
	}
)

// T023 / US1 — an Admission-only caller performs every onboarding action.
func TestAdmissionOnlyAllowedOnOnboardingRoutes(t *testing.T) {
	t.Parallel()
	app := admissionTestApp(domain.RoleAdmission)

	for _, tc := range admissionAllowed {
		if got := admissionProbe(t, app, tc.method, tc.path, tc.body); got == http.StatusForbidden {
			t.Errorf("%s %s: admission-only caller was forbidden; want allowed (FR-002)", tc.method, tc.path)
		}
	}
}

// T023 / INV-4 — the group-relaxation regression, and in Scenario A it is what makes
// the profile usable at all: these two routes are the portal's onboarding surface and
// both sit inside govGroup.
func TestAdmissionOnlyReachesPortalCriticalRoutes(t *testing.T) {
	t.Parallel()
	app := admissionTestApp(domain.RoleAdmission)

	for _, tc := range []admissionRoute{aPortalRegistryRead, aPortalApprove} {
		if got := admissionProbe(t, app, tc.method, tc.path, tc.body); got == http.StatusForbidden {
			t.Errorf("%s %s: admission-only caller 403 inside govGroup — the group guard was not "+
				"relaxed to the role union, so the Admission profile cannot use the portal (INV-4)",
				tc.method, tc.path)
		}
	}
}

// T024 / US3 — governance keeps the reads, loses the onboarding mutations.
func TestGovernanceOnlyLosesOnboardingMutationsKeepsReads(t *testing.T) {
	t.Parallel()
	app := admissionTestApp(domain.RoleGovernance)

	for _, tc := range []admissionRoute{aComplianceRead, aPortalRegistryRead} {
		if got := admissionProbe(t, app, tc.method, tc.path, tc.body); got == http.StatusForbidden {
			t.Errorf("%s %s: governance-only forbidden on a read view; want allowed (FR-003a)", tc.method, tc.path)
		}
	}
	for _, tc := range []admissionRoute{aComplianceApprove, aPortalApprove, aGovRegister} {
		if got := admissionProbe(t, app, tc.method, tc.path, tc.body); got != http.StatusForbidden {
			t.Errorf("%s %s: governance-only got %d; want 403 (FR-003)", tc.method, tc.path, got)
		}
	}
}

// T024 / INV-5 — no over-relaxation. Both directions over the full retained table.
func TestGovernanceRetainedRoutesRejectAdmissionAndKeepGovernance(t *testing.T) {
	t.Parallel()
	admissionApp := admissionTestApp(domain.RoleAdmission)
	governanceApp := admissionTestApp(domain.RoleGovernance)

	for _, tc := range aGovernanceRetained {
		if got := admissionProbe(t, admissionApp, tc.method, tc.path, tc.body); got != http.StatusForbidden {
			t.Errorf("%s %s: admission-only got %d; want 403 — route left unguarded after the group "+
				"relaxation (INV-5, FR-002a/FR-003b/FR-004)", tc.method, tc.path, got)
		}
		if got := admissionProbe(t, governanceApp, tc.method, tc.path, tc.body); got == http.StatusForbidden {
			t.Errorf("%s %s: governance-only lost a retained capability (FR-004)", tc.method, tc.path)
		}
	}
}

// T025 / US4 / INV-1 — the local/pilot dual-grant operator passes everywhere.
func TestDualGrantAllowedOnAllOnboardingRoutes(t *testing.T) {
	t.Parallel()
	app := admissionTestApp(domain.RoleGovernance, domain.RoleAdmission)

	all := append(append([]admissionRoute{}, admissionAllowed...), aGovernanceRetained...)
	for _, tc := range all {
		if got := admissionProbe(t, app, tc.method, tc.path, tc.body); got == http.StatusForbidden {
			t.Errorf("%s %s: dual-grant operator was forbidden; want allowed (INV-1)", tc.method, tc.path)
		}
	}
}

// INV-3 — the supervisor summary stays supervisor-only for both profiles. It is
// registered only when a SupervisorHandler is wired, which this app does not provide,
// so 404 is an equally valid "not reachable"; a 2xx never is.
func TestSupervisorSummaryUnaffected(t *testing.T) {
	t.Parallel()
	for _, role := range []string{domain.RoleGovernance, domain.RoleAdmission} {
		app := admissionTestApp(role)
		got := admissionProbe(t, app, http.MethodGet, "/api/v1/compliance/participants/summary", "")
		if got >= 200 && got < 300 {
			t.Errorf("role %s: participants/summary returned %d; must not be readable by either "+
				"profile (supervisor-only)", role, got)
		}
	}
}
