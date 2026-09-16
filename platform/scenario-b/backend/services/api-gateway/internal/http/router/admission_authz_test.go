// SPDX-License-Identifier: Apache-2.0

// Authorization regression suite for the Admission profile (spec 042).
//
// Source of truth: specs/042-admission-onboarding-profile/contracts/authorization-matrix.md
// Each case below is one row of that contract. The load-bearing cases are the
// admission-only POSITIVE ones on routes inside `govGroup` (INV-4) and the
// admission-only NEGATIVE ones on every governance-retained route (INV-5): the
// group guard is prefix-scoped, so re-gating requires relaxing the group guard to
// the role union AND giving every route inside it an explicit per-route guard.
// Swapping a route guard alone 403s both profiles; leaving a route unguarded after
// the relaxation silently grants Admission a governance capability.
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
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/gofiber/fiber/v2"
)

// governanceComplianceStub implements handlers.GovernanceCompliance so the
// positive (2xx) cases can reach a handler instead of nil-panicking.
type governanceComplianceStub struct{}

func (governanceComplianceStub) RegisterParticipant(context.Context, complianceadapter.Participant) error {
	return nil
}

func (governanceComplianceStub) ListParticipants(context.Context, string, string) ([]complianceadapter.Participant, error) {
	return []complianceadapter.Participant{}, nil
}

func (governanceComplianceStub) SignParticipantCSR(context.Context, string, string, string, string, string) (complianceadapter.SignedCSRResult, error) {
	return complianceadapter.SignedCSRResult{}, nil
}

func (governanceComplianceStub) ApproveKYC(_ context.Context, subject, _, _ string) (complianceadapter.ApproveKYCResult, error) {
	return complianceadapter.ApproveKYCResult{Subject: subject, Status: "KYC_APPROVED"}, nil
}

func (governanceComplianceStub) ManageParticipantStatus(context.Context, string, string, string) error {
	return nil
}

func (governanceComplianceStub) GetAuditLogs(context.Context, string, string, string, string, int, int) ([]complianceadapter.AuditRecord, error) {
	return []complianceadapter.AuditRecord{}, nil
}

func (governanceComplianceStub) GetCircuitBreakerStatus(context.Context) (complianceadapter.CircuitBreakerStatus, error) {
	return complianceadapter.CircuitBreakerStatus{}, nil
}

func (governanceComplianceStub) ToggleCircuitBreaker(context.Context, bool, string) (bool, error) {
	return false, nil
}

func (governanceComplianceStub) GetSystemParameters(context.Context) (complianceadapter.SystemParameters, error) {
	return complianceadapter.SystemParameters{}, nil
}

func (governanceComplianceStub) UpdateSystemParameters(context.Context, complianceadapter.SystemParameters, string, string) error {
	return nil
}

// admissionTestApp builds a Central-Bank-shaped gateway whose token validator
// returns exactly the supplied roles.
func admissionTestApp(roles ...string) *fiber.App {
	mgr := fullKYCManagerStub{}
	compliance := governanceComplianceStub{}
	app := fiber.New()
	Setup(app, Dependencies{
		AuthHandler:       handlers.NewAuthHandler(authProviderStub{}, mgr, false),
		ComplianceHandler: handlers.NewComplianceHandler(mgr, compliance),
		GovernanceHandler: handlers.NewGovernanceHandler(compliance),
		AuthProvider:      roleAuthProviderStub{roles: roles},
	})
	return app
}

// probe issues an authenticated request and returns the status code.
//
// It carries a valid CSRF token as well as the session. These tests are about
// AUTHORIZATION, and the app-wide CSRF guard now answers before any role check —
// without the token every assertion here would read 403 and prove nothing about the
// role rules it exists to pin.
func probe(t *testing.T, app *fiber.App, method, path, body string) int {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "fake-token"})
	csrf, err := middleware.NewCSRFToken(nil, "fake-token")
	if err != nil {
		t.Fatalf("mint csrf token: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: csrf})
	req.Header.Set(middleware.CSRFHeaderName, csrf)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("%s %s: unexpected error: %v", method, path, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

type routeCase struct {
	method string
	path   string
	body   string
}

// The two routes Scenario B's governance portal actually calls
// (frontend/apps/governance/src/services/api/registry.api.ts:20,35,65).
var (
	bPortalRead   = routeCase{http.MethodGet, "/api/v1/compliance/participants", ""}
	bPortalMutate = routeCase{http.MethodPost, "/api/v1/compliance/approve-kyc", `{"subject":"bank-a"}`}

	// Inside the prefix-scoped govGroup — these are the group-relaxation cases.
	bGovRegistryRead   = routeCase{http.MethodGet, "/api/v1/governance/registry", ""}
	bGovRegisterMutate = routeCase{http.MethodPost, "/api/v1/governance/participants", `{"user_id":"u1","role":"ROLE_COMMERCIAL_BANK"}`}

	// Governance-retained routes inside govGroup. After the group guard is relaxed
	// to the role union, each of these needs its own explicit governance guard.
	bGovernanceRetained = []routeCase{
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

// T008 / US1 — an Admission-only caller performs every onboarding action.
func TestAdmissionOnlyAllowedOnOnboardingRoutes(t *testing.T) {
	t.Parallel()
	app := admissionTestApp(domain.RoleAdmission)

	for _, tc := range []routeCase{bPortalRead, bPortalMutate, bGovRegistryRead, bGovRegisterMutate} {
		if got := probe(t, app, tc.method, tc.path, tc.body); got == http.StatusForbidden {
			t.Errorf("%s %s: admission-only caller was forbidden; want allowed", tc.method, tc.path)
		}
	}
}

// T010a / US1 / INV-4 — the group-relaxation regression. Both of these live inside
// govGroup, whose guard is prefix-scoped: a per-route guard alone cannot admit
// Admission here, so this test fails unless the group guard was relaxed too.
func TestAdmissionOnlyAllowedInsideGovGroup(t *testing.T) {
	t.Parallel()
	app := admissionTestApp(domain.RoleAdmission)

	for _, tc := range []routeCase{bGovRegistryRead, bGovRegisterMutate} {
		if got := probe(t, app, tc.method, tc.path, tc.body); got == http.StatusForbidden {
			t.Errorf("%s %s: admission-only caller 403 inside govGroup — the group guard "+
				"was not relaxed to the role union (INV-4)", tc.method, tc.path)
		}
	}
}

// T009 / US3 — a governance-only caller keeps the reads, loses the onboarding mutations.
func TestGovernanceOnlyLosesOnboardingMutationsKeepsReads(t *testing.T) {
	t.Parallel()
	app := admissionTestApp(domain.RoleGovernance)

	for _, tc := range []routeCase{bPortalRead, bGovRegistryRead} {
		if got := probe(t, app, tc.method, tc.path, tc.body); got == http.StatusForbidden {
			t.Errorf("%s %s: governance-only caller forbidden on a read view; want allowed (FR-003a)", tc.method, tc.path)
		}
	}
	for _, tc := range []routeCase{bPortalMutate, bGovRegisterMutate} {
		if got := probe(t, app, tc.method, tc.path, tc.body); got != http.StatusForbidden {
			t.Errorf("%s %s: governance-only caller got %d; want 403 (FR-003)", tc.method, tc.path, got)
		}
	}
}

// T009 / INV-5 — no over-relaxation. Relaxing the group guard widens it, so every
// governance-retained route must still refuse an Admission-only caller while
// continuing to admit governance. Enumerates the full contract table.
func TestGovernanceRetainedRoutesRejectAdmissionAndKeepGovernance(t *testing.T) {
	t.Parallel()
	admissionApp := admissionTestApp(domain.RoleAdmission)
	governanceApp := admissionTestApp(domain.RoleGovernance)

	for _, tc := range bGovernanceRetained {
		if got := probe(t, admissionApp, tc.method, tc.path, tc.body); got != http.StatusForbidden {
			t.Errorf("%s %s: admission-only caller got %d; want 403 — route left unguarded "+
				"after the group relaxation (INV-5, FR-002a/FR-004)", tc.method, tc.path, got)
		}
		if got := probe(t, governanceApp, tc.method, tc.path, tc.body); got == http.StatusForbidden {
			t.Errorf("%s %s: governance-only caller lost a retained capability (FR-004)", tc.method, tc.path)
		}
	}
}

// T010 / US4 / INV-1 — the local/pilot dual-grant operator passes everywhere.
func TestDualGrantAllowedOnAllOnboardingRoutes(t *testing.T) {
	t.Parallel()
	app := admissionTestApp(domain.RoleGovernance, domain.RoleAdmission)

	all := append([]routeCase{bPortalRead, bPortalMutate, bGovRegistryRead, bGovRegisterMutate}, bGovernanceRetained...)
	for _, tc := range all {
		if got := probe(t, app, tc.method, tc.path, tc.body); got == http.StatusForbidden {
			t.Errorf("%s %s: dual-grant operator was forbidden; want allowed (INV-1)", tc.method, tc.path)
		}
	}
}

// T012 / INV-2 — no Admission-authorized route reaches IdentityRegistry.
//
// Scenario B has three on-chain register+verify call sites. Two are outside the
// Admission surface by construction and stay that way:
//
//	auth/internal/grpc/server/onboarding.go   CompleteOnboarding      — bank-driven, CB-signed: the intended path
//	compliance/internal/grpc/server/server.go RegisterParticipantOnChain — /internal/v1/spokes/register, relay-auth, no user role
//
// The third, auth OnboardParticipant (server.go:317,321), performs an inline
// CB-signed register+verify and is reachable only from POST /compliance/register —
// a route Scenario B does NOT register. That absence is what keeps B safe, and today
// it is incidental. This test makes it deliberate: if someone wires
// ComplianceHandler.RegisterParticipant into B's router, the Admission profile would
// gain a path that triggers an on-chain registration inside its own request, which is
// exactly the shape FR-015a forbids.
func TestComplianceRegisterRouteStaysUnregistered(t *testing.T) {
	t.Parallel()
	app := admissionTestApp(domain.RoleGovernance, domain.RoleAdmission)

	got := probe(t, app, http.MethodPost, "/api/v1/compliance/register",
		`{"username":"u","email":"u@example.org","role":"ROLE_COMMERCIAL_BANK"}`)
	if got != http.StatusNotFound {
		t.Errorf("POST /api/v1/compliance/register returned %d; want 404. Scenario B must not route "+
			"this endpoint: it reaches auth OnboardParticipant, which registers+verifies on-chain "+
			"inline, so exposing it would put an on-chain call inside an Admission-authorized "+
			"request (FR-015a / INV-2). If this endpoint is genuinely needed, split its on-chain "+
			"leg out first, as Scenario A must do (T029a).", got)
	}
}

// INV-3 — the supervisor summary is unaffected by this feature: neither the
// governance nor the Admission profile may read it. The route is registered only
// when a SupervisorHandler is wired (router.go:80), which admissionTestApp does not
// provide, so an unregistered route (404) is an equally valid "not reachable"
// outcome here; what must never happen is a 2xx.
func TestSupervisorSummaryUnaffected(t *testing.T) {
	t.Parallel()
	for _, role := range []string{domain.RoleGovernance, domain.RoleAdmission} {
		app := admissionTestApp(role)
		got := probe(t, app, http.MethodGet, "/api/v1/compliance/participants/summary", "")
		if got >= 200 && got < 300 {
			t.Errorf("role %s: participants/summary returned %d; must not be readable by "+
				"either profile (supervisor-only)", role, got)
		}
	}
}
