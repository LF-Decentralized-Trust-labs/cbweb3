// SPDX-License-Identifier: Apache-2.0

// This file extends handler coverage for auth (PKI, refresh, wallet-bind, me,
// client-secret), governance, onboarding, and compliance branches that the
// original suite did not reach. All tests are hermetic.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ── PKI auth provider stub ────────────────────────────────────────────────────

// pkiAuthProviderStub implements IAuthProvider + IPKIAuthProvider + IClientSecretChanger.
type pkiAuthProviderStub struct {
	authProviderStub
	nonce          string
	nonceErr       error
	bindToken      domain.AuthToken
	bindErr        error
	changeSecError error
}

func (s pkiAuthProviderStub) IssueLoginNonce(_ context.Context, _, _ string) (string, error) {
	return s.nonce, s.nonceErr
}

func (s pkiAuthProviderStub) VerifyPKILogin(_ context.Context, _, _, _ string) (domain.AuthToken, error) {
	return s.bindToken, s.bindErr
}

func (s pkiAuthProviderStub) ChangeClientSecret(_ context.Context, _, _, _ string) error {
	return s.changeSecError
}

// ── Login PKI nonce flow ──────────────────────────────────────────────────────

func TestLogin_PKINonceFlow(t *testing.T) {
	t.Parallel()
	prov := pkiAuthProviderStub{nonce: "deadbeef"}
	h := NewAuthHandler(prov, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/login", h.Login)
	resp := postJSON(t, app, "/login", map[string]string{"clientId": "bank-a", "clientSecret": "s"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 nonce, got %d", resp.StatusCode)
	}
}

func TestLogin_PKIFallthroughToDirect(t *testing.T) {
	t.Parallel()
	// PKI not required → fall through to direct auth (which succeeds).
	prov := pkiAuthProviderStub{
		nonceErr:         status.Error(codes.FailedPrecondition, "PKI_NOT_REQUIRED"),
		authProviderStub: authProviderStub{token: domain.AuthToken{AccessToken: "t", TokenType: "Bearer", ExpiresIn: 1}},
	}
	h := NewAuthHandler(prov, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/login", h.Login)
	resp := postJSON(t, app, "/login", map[string]string{"clientId": "bank-a", "clientSecret": "s"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 direct, got %d", resp.StatusCode)
	}
}

func TestLogin_PKIHardReject(t *testing.T) {
	t.Parallel()
	prov := pkiAuthProviderStub{nonceErr: status.Error(codes.Unauthenticated, "bad secret")}
	h := NewAuthHandler(prov, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/login", h.Login)
	resp := postJSON(t, app, "/login", map[string]string{"clientId": "bank-a", "clientSecret": "s"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
}

func TestLogin_PKINonGRPCError(t *testing.T) {
	t.Parallel()
	prov := pkiAuthProviderStub{nonceErr: errors.New("network down")}
	h := NewAuthHandler(prov, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/login", h.Login)
	resp := postJSON(t, app, "/login", map[string]string{"clientId": "bank-a", "clientSecret": "s"})
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", resp.StatusCode)
	}
}

func TestLogin_MissingClientSecret(t *testing.T) {
	t.Parallel()
	h := NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/login", h.Login)
	resp := postJSON(t, app, "/login", map[string]string{"clientId": "bank-a"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

// ── Refresh ───────────────────────────────────────────────────────────────────

func TestRefresh(t *testing.T) {
	t.Parallel()
	h := NewAuthHandler(authProviderStub{token: domain.AuthToken{AccessToken: "t", RefreshToken: "r", TokenType: "Bearer"}}, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/refresh", h.Refresh)

	// from body
	if resp := postJSON(t, app, "/refresh", map[string]string{"refreshToken": "r"}); resp.StatusCode != http.StatusOK {
		t.Errorf("body refresh: want 200, got %d", resp.StatusCode)
	}
	// missing → 400
	if resp := postJSON(t, app, "/refresh", map[string]string{}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("missing refresh: want 400, got %d", resp.StatusCode)
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	t.Parallel()
	h := NewAuthHandler(authProviderStub{err: errors.New("expired")}, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/refresh", h.Refresh)
	if resp := postJSON(t, app, "/refresh", map[string]string{"refreshToken": "r"}); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

// ── Logout ────────────────────────────────────────────────────────────────────

func TestLogout_NoCookie(t *testing.T) {
	t.Parallel()
	h := NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/logout", h.Logout)
	resp, _ := app.Test(httptest.NewRequest(http.MethodPost, "/logout", nil))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestLogout_WithCookieAndError(t *testing.T) {
	t.Parallel()
	h := NewAuthHandler(authProviderStub{err: errors.New("revoke failed")}, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/logout", h.Logout)
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "r"})
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
}

// ── WalletBind ────────────────────────────────────────────────────────────────

func TestWalletBind_NotConfigured(t *testing.T) {
	t.Parallel()
	h := NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/bind", h.WalletBind)
	resp := postJSON(t, app, "/bind", map[string]string{"user_id": "u", "nonce_signature_hex": "x", "cert_pem": "c"})
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("want 501, got %d", resp.StatusCode)
	}
}

func TestWalletBind_Success(t *testing.T) {
	t.Parallel()
	prov := pkiAuthProviderStub{bindToken: domain.AuthToken{AccessToken: "t", RefreshToken: "r", TokenType: "Bearer", ExpiresIn: 1}}
	h := NewAuthHandler(prov, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/bind", h.WalletBind)
	resp := postJSON(t, app, "/bind", map[string]string{"user_id": "u", "nonce_signature_hex": "x", "cert_pem": "c"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestWalletBind_Validation(t *testing.T) {
	t.Parallel()
	prov := pkiAuthProviderStub{}
	h := NewAuthHandler(prov, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/bind", h.WalletBind)
	if resp := postJSON(t, app, "/bind", map[string]string{"user_id": "u"}); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestWalletBind_ErrorMapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err  error
		want int
	}{
		{status.Error(codes.Unauthenticated, "NONCE_SIGNATURE_MISMATCH"), http.StatusUnauthorized},
		{status.Error(codes.PermissionDenied, "kyc rejected"), http.StatusForbidden},
		{status.Error(codes.Internal, "boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		prov := pkiAuthProviderStub{bindErr: tc.err}
		h := NewAuthHandler(prov, kycCheckerStub{}, false)
		app := fiber.New()
		app.Post("/bind", h.WalletBind)
		resp := postJSON(t, app, "/bind", map[string]string{"user_id": "u", "nonce_signature_hex": "x", "cert_pem": "c"})
		if resp.StatusCode != tc.want {
			t.Errorf("err %v: want %d, got %d", tc.err, tc.want, resp.StatusCode)
		}
	}
}

// ── ChangeClientSecret ────────────────────────────────────────────────────────

func TestChangeClientSecret(t *testing.T) {
	t.Parallel()
	prov := pkiAuthProviderStub{}
	h := NewAuthHandler(prov, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/change", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "u"})
		return h.ChangeClientSecret(c)
	})
	// success
	if resp := postJSON(t, app, "/change", map[string]string{"current_client_secret": "a", "new_client_secret": "b"}); resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	// validation
	if resp := postJSON(t, app, "/change", map[string]string{"current_client_secret": "a"}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestChangeClientSecret_NotConfigured(t *testing.T) {
	t.Parallel()
	h := NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/change", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "u"})
		return h.ChangeClientSecret(c)
	})
	if resp := postJSON(t, app, "/change", map[string]string{"current_client_secret": "a", "new_client_secret": "b"}); resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("want 501, got %d", resp.StatusCode)
	}
}

func TestChangeClientSecret_Unauthorized(t *testing.T) {
	t.Parallel()
	prov := pkiAuthProviderStub{}
	h := NewAuthHandler(prov, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/change", h.ChangeClientSecret) // no claims
	if resp := postJSON(t, app, "/change", map[string]string{"current_client_secret": "a", "new_client_secret": "b"}); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestChangeClientSecret_InvalidCurrent(t *testing.T) {
	t.Parallel()
	prov := pkiAuthProviderStub{changeSecError: status.Error(codes.Unauthenticated, "invalid current client secret")}
	h := NewAuthHandler(prov, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/change", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "u"})
		return h.ChangeClientSecret(c)
	})
	if resp := postJSON(t, app, "/change", map[string]string{"current_client_secret": "a", "new_client_secret": "b"}); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

// ── Me ────────────────────────────────────────────────────────────────────────

func TestMe(t *testing.T) {
	t.Parallel()
	h := NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false)
	app := fiber.New()
	app.Get("/me", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{
			Subject: "u", Issuer: "iss", Roles: []string{"ROLE_X"},
			Wallet: "0xabc", Country: "BR", BankID: "bank-a", PrivacyGroup: "pg1",
		})
		return h.Me(c)
	})
	if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/me", nil)); resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}

	app2 := fiber.New()
	app2.Get("/me", h.Me) // no claims
	if resp, _ := app2.Test(httptest.NewRequest(http.MethodGet, "/me", nil)); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

// TestMe_BankCodeFallback verifies that /me returns bankId using the entity's
// configured bank code when the JWT does not carry a bank_id custom claim.
// This covers the scenario where the Keycloak realm has no bank_id token mapper,
// preventing commercial-bank portals from incorrectly displaying Accept/Reject
// buttons to the originator of an FX Agreement.
func TestMe_BankCodeFallback(t *testing.T) {
	t.Parallel()

	h := NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false, "bank-macro")
	app := fiber.New()

	// Claims without BankID — simulates a Keycloak JWT without bank_id mapper.
	app.Get("/me", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{
			Subject: "funded_operator",
			Issuer:  "http://keycloak:8080/realms/bank-macro",
			Roles:   []string{"ROLE_COMMERCIAL_BANK"},
		})
		return h.Me(c)
	})

	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/me", nil))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["bankId"] != "bank-macro" {
		t.Errorf("bankId: got %v, want bank-macro", body["bankId"])
	}
}

// TestMe_BankIDClaimTakesPrecedence verifies that the JWT bank_id claim wins
// over the entity's configured bank code when both are present.
func TestMe_BankIDClaimTakesPrecedence(t *testing.T) {
	t.Parallel()

	h := NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false, "entity-default")
	app := fiber.New()

	app.Get("/me", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{
			Subject: "u",
			Issuer:  "iss",
			Roles:   []string{"ROLE_COMMERCIAL_BANK"},
			BankID:  "jwt-bank-id",
		})
		return h.Me(c)
	})

	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/me", nil))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["bankId"] != "jwt-bank-id" {
		t.Errorf("bankId: got %v, want jwt-bank-id (JWT claim must take precedence)", body["bankId"])
	}
}

func TestContainsRole(t *testing.T) {
	t.Parallel()
	if !containsRole([]string{"a", "b"}, "b") {
		t.Error("expected true")
	}
	if containsRole([]string{"a"}, "z") {
		t.Error("expected false")
	}
}

// ── Governance handler ────────────────────────────────────────────────────────

// fullGovernanceStub implements GovernanceCompliance + UserManager and is
// configurable to return errors.
type fullGovernanceStub struct {
	err   error
	users []interfaces.UserSummary
	user  interfaces.UserDetail
}

func (s fullGovernanceStub) RegisterParticipant(context.Context, complianceadapter.Participant) error {
	return s.err
}
func (s fullGovernanceStub) ListParticipants(context.Context, string, string) ([]complianceadapter.Participant, error) {
	return []complianceadapter.Participant{{UserID: "u1"}}, s.err
}
func (s fullGovernanceStub) SignParticipantCSR(context.Context, string, string, string, string, string) (complianceadapter.SignedCSRResult, error) {
	return complianceadapter.SignedCSRResult{CertPEM: "cert"}, s.err
}
func (s fullGovernanceStub) ApproveKYC(context.Context, string, string, string) (complianceadapter.ApproveKYCResult, error) {
	return complianceadapter.ApproveKYCResult{Subject: "u1", Status: "ACTIVE", TxHash: "tx", PopNonce: "n"}, s.err
}
func (s fullGovernanceStub) ManageParticipantStatus(context.Context, string, string, string) error {
	return s.err
}
func (s fullGovernanceStub) GetAuditLogs(context.Context, string, string, string, string, int, int) ([]complianceadapter.AuditRecord, error) {
	return []complianceadapter.AuditRecord{}, s.err
}
func (s fullGovernanceStub) GetCircuitBreakerStatus(context.Context) (complianceadapter.CircuitBreakerStatus, error) {
	return complianceadapter.CircuitBreakerStatus{}, s.err
}
func (s fullGovernanceStub) ToggleCircuitBreaker(context.Context, bool, string) (bool, string, error) {
	return true, "0xtxhash", s.err
}
func (s fullGovernanceStub) GetSystemParameters(context.Context) (complianceadapter.SystemParameters, error) {
	return complianceadapter.SystemParameters{}, s.err
}
func (s fullGovernanceStub) UpdateSystemParameters(context.Context, complianceadapter.SystemParameters, string, string) error {
	return s.err
}
func (s fullGovernanceStub) ListUsers(context.Context, string, string) ([]interfaces.UserSummary, int, error) {
	return s.users, len(s.users), s.err
}
func (s fullGovernanceStub) GetUser(context.Context, string) (interfaces.UserDetail, error) {
	return s.user, s.err
}

func TestGovernanceHandler_HappyPaths(t *testing.T) {
	t.Parallel()
	stub := fullGovernanceStub{
		users: []interfaces.UserSummary{{UserID: "u1", Role: "ROLE_X", Status: "ACTIVE", InstitutionName: "I", WalletAddress: "0x", Country: "BR", BankCode: "a"}},
		user:  interfaces.UserDetail{UserID: "u1", Username: "n", Role: "ROLE_X", Status: "ACTIVE", Email: "e", InstitutionName: "I", WalletAddress: "0x", Country: "BR", BankCode: "a"},
	}
	h := NewGovernanceHandler(stub)
	app := fiber.New()
	app.Post("/participants", h.RegisterParticipant)
	app.Get("/registry", h.GetRegistry)
	app.Post("/csr", h.SubmitCSR)
	app.Post("/approve-kyc", h.ApproveKYC)
	app.Get("/accounts", h.GetAccounts)
	app.Post("/freeze", h.FreezeAccount)
	app.Post("/unfreeze", h.UnfreezeAccount)
	app.Get("/cb", h.GetCircuitBreakerStatus)
	app.Post("/cb/toggle", h.ToggleCircuitBreaker)
	app.Get("/params", h.GetParameters)
	app.Get("/audit", h.GetAuditLogs)
	app.Get("/users", h.ListUsers)
	app.Get("/users/:userId", h.GetUser)

	posts := []struct {
		path string
		body map[string]any
		want int
	}{
		{"/participants", map[string]any{"user_id": "u1", "role": "ROLE_X"}, http.StatusCreated},
		{"/csr", map[string]any{"csr_pem": "p", "user_id": "u", "role": "r", "institution_name": "i"}, http.StatusCreated},
		{"/approve-kyc", map[string]any{"subject": "u1"}, http.StatusOK},
		{"/freeze", map[string]any{"subject": "u1", "reason": "x"}, http.StatusOK},
		{"/unfreeze", map[string]any{"subject": "u1", "reason": "x"}, http.StatusOK},
		{"/cb/toggle", map[string]any{"pause": true, "reason": "x"}, http.StatusOK},
	}
	for _, tc := range posts {
		if resp := postJSON(t, app, tc.path, tc.body); resp.StatusCode != tc.want {
			t.Errorf("%s: want %d, got %d", tc.path, tc.want, resp.StatusCode)
		}
	}
	for _, p := range []string{"/registry", "/accounts", "/cb", "/params", "/audit", "/users", "/users/u1"} {
		if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, p, nil)); resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: want 200, got %d", p, resp.StatusCode)
		}
	}
}

func TestGovernanceHandler_ValidationAndErrors(t *testing.T) {
	t.Parallel()
	// validation: missing fields
	h := NewGovernanceHandler(fullGovernanceStub{})
	app := fiber.New()
	app.Post("/participants", h.RegisterParticipant)
	app.Post("/csr", h.SubmitCSR)
	app.Post("/approve-kyc", h.ApproveKYC)
	app.Post("/freeze", h.FreezeAccount)
	app.Post("/cb/toggle", h.ToggleCircuitBreaker)
	app.Put("/params", h.UpdateParameters)

	bad := []struct{ path string }{
		{"/participants"}, {"/csr"}, {"/approve-kyc"}, {"/freeze"}, {"/cb/toggle"},
	}
	for _, tc := range bad {
		if resp := postJSON(t, app, tc.path, map[string]any{}); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s validation: want 400, got %d", tc.path, resp.StatusCode)
		}
	}

	// error propagation: downstream returns error → 500
	herr := NewGovernanceHandler(fullGovernanceStub{err: errors.New("db down")})
	app2 := fiber.New()
	app2.Get("/registry", herr.GetRegistry)
	app2.Get("/accounts", herr.GetAccounts)
	app2.Get("/cb", herr.GetCircuitBreakerStatus)
	app2.Get("/params", herr.GetParameters)
	app2.Get("/audit", herr.GetAuditLogs)
	app2.Post("/approve-kyc", herr.ApproveKYC)
	for _, p := range []string{"/registry", "/accounts", "/cb", "/params", "/audit"} {
		if resp, _ := app2.Test(httptest.NewRequest(http.MethodGet, p, nil)); resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("GET %s err: want 500, got %d", p, resp.StatusCode)
		}
	}
	if resp := postJSON(t, app2, "/approve-kyc", map[string]any{"subject": "u1"}); resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("approve-kyc err: want 500, got %d", resp.StatusCode)
	}
}

func TestGovernanceHandler_UserManagerNotAvailable(t *testing.T) {
	t.Parallel()
	// governanceComplianceStub (from governance_test.go) does NOT implement UserManager.
	h := NewGovernanceHandler(&governanceComplianceStub{})
	app := fiber.New()
	app.Get("/users", h.ListUsers)
	app.Get("/users/:userId", h.GetUser)
	if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/users", nil)); resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("list users: want 501, got %d", resp.StatusCode)
	}
	if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/users/u1", nil)); resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("get user: want 501, got %d", resp.StatusCode)
	}
}

func TestGovernanceHandler_GetUserNotFound(t *testing.T) {
	t.Parallel()
	h := NewGovernanceHandler(fullGovernanceStub{err: errors.New("user not found")})
	app := fiber.New()
	app.Get("/users/:userId", h.GetUser)
	if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/users/u1", nil)); resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

// ── Onboarding handler (local CB) ─────────────────────────────────────────────

type onboardingMgrStub struct {
	credResult   interfaces.CredentialRequestResult
	credErr      error
	statusResult interfaces.OnboardingStatus
	statusErr    error
	completeRes  interfaces.CompleteOnboardingResult
	completeErr  error
}

func (s onboardingMgrStub) SubmitCredentialRequest(context.Context, interfaces.CredentialRequest) (interfaces.CredentialRequestResult, error) {
	return s.credResult, s.credErr
}
func (s onboardingMgrStub) GetOnboardingStatus(context.Context, string) (interfaces.OnboardingStatus, error) {
	return s.statusResult, s.statusErr
}
func (s onboardingMgrStub) GetOnboardingStatusByBankCode(context.Context, string) (interfaces.OnboardingStatus, error) {
	return s.statusResult, s.statusErr
}
func (s onboardingMgrStub) CompleteOnboarding(context.Context, interfaces.CompleteOnboardingRequest) (interfaces.CompleteOnboardingResult, error) {
	return s.completeRes, s.completeErr
}

func TestOnboardingHandler(t *testing.T) {
	t.Parallel()
	mgr := onboardingMgrStub{
		credResult:   interfaces.CredentialRequestResult{RequestID: "req", UserID: "u", WalletAddress: "0x", Status: "PENDING"},
		statusResult: interfaces.OnboardingStatus{RequestID: "req", UserID: "u", Status: "APPROVED", PopNonce: "n", WalletAddress: "0x"},
		completeRes:  interfaces.CompleteOnboardingResult{UserID: "u", WalletAddress: "0x", CertPEM: "c", TxHash: "tx", ClientSecret: "sec", Status: "DONE"},
	}
	h := NewOnboardingHandler(mgr)
	app := fiber.New()
	app.Post("/credential-request", h.SubmitCredentialRequest)
	app.Get("/status/:requestId", h.GetOnboardingStatus)
	app.Post("/complete", h.CompleteOnboarding)
	app.Get("/my-status", h.GetMyOnboardingStatus)

	cred := map[string]any{
		"csr_pem": "p", "blockchain_pub_key_hex": "k", "role": "ROLE_X",
		"username": "n", "email": "e", "institution_name": "i",
	}
	if resp := postJSON(t, app, "/credential-request", cred); resp.StatusCode != http.StatusCreated {
		t.Errorf("credential-request: want 201, got %d", resp.StatusCode)
	}
	if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/status/req", nil)); resp.StatusCode != http.StatusOK {
		t.Errorf("status: want 200, got %d", resp.StatusCode)
	}
	complete := map[string]any{"user_id": "u", "pop_signature_hex": "s", "blockchain_pub_key_hex": "k"}
	if resp := postJSON(t, app, "/complete", complete); resp.StatusCode != http.StatusOK {
		t.Errorf("complete: want 200, got %d", resp.StatusCode)
	}
	if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/my-status?bank_code=a", nil)); resp.StatusCode != http.StatusOK {
		t.Errorf("my-status: want 200, got %d", resp.StatusCode)
	}
}

func TestOnboardingHandler_Validation(t *testing.T) {
	t.Parallel()
	h := NewOnboardingHandler(onboardingMgrStub{})
	app := fiber.New()
	app.Post("/credential-request", h.SubmitCredentialRequest)
	app.Get("/status/:requestId", h.GetOnboardingStatus)
	app.Post("/complete", h.CompleteOnboarding)
	app.Get("/my-status", h.GetMyOnboardingStatus)

	if resp := postJSON(t, app, "/credential-request", map[string]any{}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("cred validation: want 400, got %d", resp.StatusCode)
	}
	if resp := postJSON(t, app, "/complete", map[string]any{}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("complete validation: want 400, got %d", resp.StatusCode)
	}
	// my-status without bank_code and no claims → 400
	if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/my-status", nil)); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("my-status missing bank_code: want 400, got %d", resp.StatusCode)
	}
}

func TestOnboardingHandler_Errors(t *testing.T) {
	t.Parallel()
	// conflict on credential-request
	hc := NewOnboardingHandler(onboardingMgrStub{credErr: errors.New("user already exists")})
	app := fiber.New()
	app.Post("/credential-request", hc.SubmitCredentialRequest)
	cred := map[string]any{"csr_pem": "p", "blockchain_pub_key_hex": "k", "role": "r", "username": "n", "email": "e", "institution_name": "i"}
	if resp := postJSON(t, app, "/credential-request", cred); resp.StatusCode != http.StatusConflict {
		t.Errorf("conflict: want 409, got %d", resp.StatusCode)
	}

	// not found on status
	hnf := NewOnboardingHandler(onboardingMgrStub{statusErr: errors.New("request not found")})
	app2 := fiber.New()
	app2.Get("/status/:requestId", hnf.GetOnboardingStatus)
	app2.Get("/my-status", hnf.GetMyOnboardingStatus)
	if resp, _ := app2.Test(httptest.NewRequest(http.MethodGet, "/status/x", nil)); resp.StatusCode != http.StatusNotFound {
		t.Errorf("status not found: want 404, got %d", resp.StatusCode)
	}
	if resp, _ := app2.Test(httptest.NewRequest(http.MethodGet, "/my-status?bank_code=a", nil)); resp.StatusCode != http.StatusNotFound {
		t.Errorf("my-status not found: want 404, got %d", resp.StatusCode)
	}

	// internal error on complete
	hi := NewOnboardingHandler(onboardingMgrStub{completeErr: errors.New("boom")})
	app3 := fiber.New()
	app3.Post("/complete", hi.CompleteOnboarding)
	complete := map[string]any{"user_id": "u", "pop_signature_hex": "s", "blockchain_pub_key_hex": "k"}
	if resp := postJSON(t, app3, "/complete", complete); resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("complete error: want 500, got %d", resp.StatusCode)
	}
}

func TestOnboardingHandler_MyStatusFromClaims(t *testing.T) {
	t.Parallel()
	mgr := onboardingMgrStub{statusResult: interfaces.OnboardingStatus{RequestID: "r", UserID: "u", Status: "OK"}}
	h := NewOnboardingHandler(mgr)
	app := fiber.New()
	app.Get("/my-status", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{BankID: "bank-a"})
		return h.GetMyOnboardingStatus(c)
	})
	if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/my-status", nil)); resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

// ── Compliance handler additional branches ────────────────────────────────────

func TestComplianceHandler_ListParticipants(t *testing.T) {
	t.Parallel()
	lister := complianceListerStub{participants: []complianceadapter.Participant{{UserID: "u1"}}}
	h := NewComplianceHandler(kycCheckerStub{}, lister)
	app := fiber.New()
	app.Get("/participants", h.ListParticipants)
	if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/participants", nil)); resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}

	herr := NewComplianceHandler(kycCheckerStub{}, complianceListerStub{err: errors.New("down")})
	app2 := fiber.New()
	app2.Get("/participants", herr.ListParticipants)
	if resp, _ := app2.Test(httptest.NewRequest(http.MethodGet, "/participants", nil)); resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_RegisterParticipant_NotImplemented(t *testing.T) {
	t.Parallel()
	// kycCheckerStub does not implement ParticipantOnboarder → 501.
	h := NewComplianceHandler(kycCheckerStub{}, nil)
	app := fiber.New()
	app.Post("/register", h.RegisterParticipant)
	body := map[string]any{"username": "n", "email": "e", "role": domain.RoleCommercialBank}
	if resp := postJSON(t, app, "/register", body); resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("want 501, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_RegisterParticipant_FullFlow(t *testing.T) {
	t.Parallel()
	ob := participantOnboarderStub{result: interfaces.OnboardParticipantResult{UserID: "u", WalletAddress: "0x", TxHash: "tx", ClientSecret: "s"}}
	h := NewComplianceHandler(ob, nil)
	app := fiber.New()
	app.Post("/register", h.RegisterParticipant)

	// success
	good := map[string]any{"username": "n", "email": "e", "role": domain.RoleCommercialBank}
	if resp := postJSON(t, app, "/register", good); resp.StatusCode != http.StatusCreated {
		t.Errorf("want 201, got %d", resp.StatusCode)
	}
	// invalid role
	badRole := map[string]any{"username": "n", "email": "e", "role": "ROLE_BOGUS"}
	if resp := postJSON(t, app, "/register", badRole); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid role: want 400, got %d", resp.StatusCode)
	}
	// missing fields
	if resp := postJSON(t, app, "/register", map[string]any{"username": "n"}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("missing fields: want 400, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_RegisterParticipant_Conflict(t *testing.T) {
	t.Parallel()
	ob := participantOnboarderStub{err: errors.New("participant already exists")}
	h := NewComplianceHandler(ob, nil)
	app := fiber.New()
	app.Post("/register", h.RegisterParticipant)
	good := map[string]any{"username": "n", "email": "e", "role": domain.RoleCommercialBank}
	if resp := postJSON(t, app, "/register", good); resp.StatusCode != http.StatusConflict {
		t.Errorf("want 409, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_ProvisionFreezeUnfreeze(t *testing.T) {
	t.Parallel()
	mgr := kycManagerStub{}
	h := NewComplianceHandler(mgr, nil)
	app := fiber.New()
	app.Post("/provision", h.ProvisionParticipant)
	app.Post("/freeze", h.FreezeAccount)
	app.Post("/unfreeze", h.UnfreezeAccount)

	if resp := postJSON(t, app, "/provision", map[string]any{"subject": "u", "status": "ACTIVE"}); resp.StatusCode != http.StatusOK {
		t.Errorf("provision: want 200, got %d", resp.StatusCode)
	}
	if resp := postJSON(t, app, "/freeze", map[string]any{"subject": "u"}); resp.StatusCode != http.StatusOK {
		t.Errorf("freeze: want 200, got %d", resp.StatusCode)
	}
	if resp := postJSON(t, app, "/unfreeze", map[string]any{"subject": "u"}); resp.StatusCode != http.StatusOK {
		t.Errorf("unfreeze: want 200, got %d", resp.StatusCode)
	}

	// validation: missing subject
	if resp := postJSON(t, app, "/provision", map[string]any{"status": "ACTIVE"}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("provision missing subject: want 400, got %d", resp.StatusCode)
	}
	if resp := postJSON(t, app, "/freeze", map[string]any{}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("freeze missing subject: want 400, got %d", resp.StatusCode)
	}
	if resp := postJSON(t, app, "/unfreeze", map[string]any{}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("unfreeze missing subject: want 400, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_ProvisionErrors(t *testing.T) {
	t.Parallel()
	mgr := kycManagerStub{err: errors.New("boom")}
	h := NewComplianceHandler(mgr, nil)
	app := fiber.New()
	app.Post("/provision", h.ProvisionParticipant)
	app.Post("/freeze", h.FreezeAccount)
	app.Post("/unfreeze", h.UnfreezeAccount)
	if resp := postJSON(t, app, "/provision", map[string]any{"subject": "u", "status": "ACTIVE"}); resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("provision err: want 500, got %d", resp.StatusCode)
	}
	if resp := postJSON(t, app, "/freeze", map[string]any{"subject": "u"}); resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("freeze err: want 500, got %d", resp.StatusCode)
	}
	if resp := postJSON(t, app, "/unfreeze", map[string]any{"subject": "u"}); resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("unfreeze err: want 500, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_NotImplementedWithoutManager(t *testing.T) {
	t.Parallel()
	// kycCheckerStub is not a KYCManager → provision/freeze/unfreeze return 501.
	h := NewComplianceHandler(kycCheckerStub{}, nil)
	app := fiber.New()
	app.Post("/provision", h.ProvisionParticipant)
	app.Post("/freeze", h.FreezeAccount)
	app.Post("/unfreeze", h.UnfreezeAccount)
	if resp := postJSON(t, app, "/provision", map[string]any{"subject": "u", "status": "ACTIVE"}); resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("provision: want 501, got %d", resp.StatusCode)
	}
	if resp := postJSON(t, app, "/freeze", map[string]any{"subject": "u"}); resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("freeze: want 501, got %d", resp.StatusCode)
	}
	if resp := postJSON(t, app, "/unfreeze", map[string]any{"subject": "u"}); resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("unfreeze: want 501, got %d", resp.StatusCode)
	}
}

// complianceListerStub satisfies ComplianceParticipantLister.
type complianceListerStub struct {
	participants []complianceadapter.Participant
	err          error
}

func (s complianceListerStub) ListParticipants(context.Context, string, string) ([]complianceadapter.Participant, error) {
	return s.participants, s.err
}

// ── domain ────────────────────────────────────────────────────────────────────

func TestIsAdminRole(t *testing.T) {
	t.Parallel()
	if !domain.IsAdminRole(domain.RoleCommercialBank) {
		t.Error("expected commercial bank to be admin role")
	}
	if domain.IsAdminRole("ROLE_BOGUS") {
		t.Error("expected bogus role to be non-admin")
	}
}
