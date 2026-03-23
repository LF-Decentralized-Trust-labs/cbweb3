// This file tests auth and compliance HTTP handler behaviors and responses.
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

type authProviderStub struct {
	token domain.AuthToken
	err   error
}

func (s authProviderStub) Authenticate(_ context.Context, _, _ string) (domain.AuthToken, error) {
	return s.token, s.err
}

func (s authProviderStub) RefreshToken(_ context.Context, _ string) (domain.AuthToken, error) {
	return s.token, s.err
}

func (s authProviderStub) Logout(_ context.Context, _ string) error {
	return s.err
}

func (s authProviderStub) Validate(_ context.Context, _ string) (domain.TokenClaims, error) {
	return domain.TokenClaims{Subject: "stub-user"}, s.err
}

type kycCheckerStub struct {
	status domain.KYCStatus
}

func (s kycCheckerStub) GetStatus(_ string) domain.KYCStatus {
	if s.status == "" {
		return domain.KYCApproved
	}
	return s.status
}

// kycManagerStub implements both KYCChecker and KYCManager for handler tests.
type kycManagerStub struct {
	status domain.KYCStatus
	err    error
}

func (s kycManagerStub) GetStatus(_ string) domain.KYCStatus {
	if s.status == "" {
		return domain.KYCApproved
	}
	return s.status
}

func (s kycManagerStub) GetKYCStatus(_ context.Context, _ string) (domain.KYCStatus, error) {
	if s.status == "" {
		return domain.KYCApproved, nil
	}
	return s.status, s.err
}

func (s kycManagerStub) ProvisionParticipant(_ context.Context, _ string, _ domain.KYCStatus) error {
	return s.err
}

// participantOnboarderStub implements KYCChecker + KYCManager + ParticipantOnboarder.
type participantOnboarderStub struct {
	kycManagerStub
	result interfaces.OnboardParticipantResult
	err    error
}

func (s participantOnboarderStub) OnboardParticipant(_ context.Context, _ interfaces.OnboardParticipantRequest) (interfaces.OnboardParticipantResult, error) {
	return s.result, s.err
}

func TestHealth(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/healthz", Health)
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuthHandlerLogin(t *testing.T) {
	t.Parallel()
	handler := NewAuthHandler(authProviderStub{
		token: domain.AuthToken{AccessToken: "token", ExpiresIn: 1, TokenType: "Bearer"},
	}, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/auth/login", handler.Login)

	body, _ := json.Marshal(map[string]string{"clientId": "bank-a", "clientSecret": "secret-a"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuthHandlerLoginInvalidCases(t *testing.T) {
	t.Parallel()

	t.Run("missing fields", func(t *testing.T) {
		handler := NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false)
		app := fiber.New()
		app.Post("/auth/login", handler.Login)
		body, _ := json.Marshal(map[string]string{"clientId": ""})
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("invalid credentials", func(t *testing.T) {
		handler := NewAuthHandler(authProviderStub{err: errors.New("invalid")}, kycCheckerStub{}, false)
		app := fiber.New()
		app.Post("/auth/login", handler.Login)
		body, _ := json.Marshal(map[string]string{"clientId": "bank", "clientSecret": "bad"})
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("invalid body", func(t *testing.T) {
		handler := NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false)
		app := fiber.New()
		app.Post("/auth/login", handler.Login)
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader([]byte("{")))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})
}

func TestComplianceHandlerStatus(t *testing.T) {
	t.Parallel()
	handler := NewComplianceHandler(kycCheckerStub{status: domain.KYCApproved})
	app := fiber.New()
	app.Get("/compliance/kyc/status/:subject", handler.GetKYCStatus)

	req := httptest.NewRequest(http.MethodGet, "/compliance/kyc/status/bank-a", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestComplianceHandlerMissingSubject(t *testing.T) {
	t.Parallel()

	handler := NewComplianceHandler(kycCheckerStub{status: domain.KYCApproved})
	app := fiber.New()
	app.Get("/compliance/kyc/status", handler.GetKYCStatus)

	req := httptest.NewRequest(http.MethodGet, "/compliance/kyc/status", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// ---------- Refresh / Logout tests ----------

func TestAuthHandlerRefreshSuccess(t *testing.T) {
	t.Parallel()
	handler := NewAuthHandler(authProviderStub{
		token: domain.AuthToken{AccessToken: "new-token", RefreshToken: "rt", ExpiresIn: 3600, TokenType: "Bearer"},
	}, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/auth/refresh", handler.Refresh)

	body, _ := json.Marshal(map[string]string{"refreshToken": "old-rt"})
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuthHandlerRefreshInvalid(t *testing.T) {
	t.Parallel()
	handler := NewAuthHandler(authProviderStub{err: errors.New("expired")}, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/auth/refresh", handler.Refresh)

	body, _ := json.Marshal(map[string]string{"refreshToken": "bad-rt"})
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthHandlerRefreshMissingBody(t *testing.T) {
	t.Parallel()
	handler := NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/auth/refresh", handler.Refresh)

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAuthHandlerLogoutSuccess(t *testing.T) {
	t.Parallel()
	handler := NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/auth/logout", handler.Logout)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "some-token"})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuthHandlerLogoutMissingToken(t *testing.T) {
	t.Parallel()
	handler := NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/auth/logout", handler.Logout)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestOpenAPIWalletBindEndpointIsActive(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("failed to read openapi.yaml: %v", err)
	}
	spec := string(data)
	start := strings.Index(spec, "/api/v1/auth/wallet/bind:")
	if start == -1 {
		t.Fatalf("wallet/bind path missing from OpenAPI")
	}
	rest := spec[start:]
	nextPath := strings.Index(rest, "\n  /")
	section := rest
	if nextPath != -1 {
		section = rest[:nextPath]
	}

	if strings.Contains(section, "deprecated: true") {
		t.Fatalf("wallet/bind endpoint must not be deprecated in OpenAPI")
	}
	if strings.Contains(section, "\"410\":") {
		t.Fatalf("wallet/bind endpoint must not declare 410 Gone in OpenAPI")
	}
	if !strings.Contains(section, "\"200\":") {
		t.Fatalf("wallet/bind endpoint must declare 200 success response in OpenAPI")
	}
}

// ---------- Compliance extended tests ----------

func TestAMLScreenApproved(t *testing.T) {
	t.Parallel()
	stub := kycManagerStub{status: domain.KYCApproved}
	handler := NewComplianceHandler(stub)
	app := fiber.New()
	app.Post("/compliance/aml/screen", handler.AMLScreen)

	body, _ := json.Marshal(map[string]string{"subject": "bank-001"})
	req := httptest.NewRequest(http.MethodPost, "/compliance/aml/screen", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAMLScreenFrozenSanctioned(t *testing.T) {
	t.Parallel()
	stub := kycManagerStub{status: domain.KYCFrozen}
	handler := NewComplianceHandler(stub)
	app := fiber.New()
	app.Post("/compliance/aml/screen", handler.AMLScreen)

	body, _ := json.Marshal(map[string]string{"subject": "frozen-bank"})
	req := httptest.NewRequest(http.MethodPost, "/compliance/aml/screen", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestAMLScreenRevokedSanctioned(t *testing.T) {
	t.Parallel()
	stub := kycManagerStub{status: domain.KYCRevoked}
	handler := NewComplianceHandler(stub)
	app := fiber.New()
	app.Post("/compliance/aml/screen", handler.AMLScreen)

	body, _ := json.Marshal(map[string]string{"subject": "revoked-bank"})
	req := httptest.NewRequest(http.MethodPost, "/compliance/aml/screen", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestAMLScreenMissingSubject(t *testing.T) {
	t.Parallel()
	stub := kycManagerStub{}
	handler := NewComplianceHandler(stub)
	app := fiber.New()
	app.Post("/compliance/aml/screen", handler.AMLScreen)

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/compliance/aml/screen", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestProvisionParticipantSuccess(t *testing.T) {
	t.Parallel()
	stub := kycManagerStub{}
	handler := NewComplianceHandler(stub)
	app := fiber.New()
	app.Post("/compliance/participants/provision", handler.ProvisionParticipant)

	body, _ := json.Marshal(map[string]string{"subject": "bank-001", "status": "APPROVED"})
	req := httptest.NewRequest(http.MethodPost, "/compliance/participants/provision", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestProvisionParticipantMissingFields(t *testing.T) {
	t.Parallel()
	stub := kycManagerStub{}
	handler := NewComplianceHandler(stub)
	app := fiber.New()
	app.Post("/compliance/participants/provision", handler.ProvisionParticipant)

	body, _ := json.Marshal(map[string]string{"subject": "bank-001"})
	req := httptest.NewRequest(http.MethodPost, "/compliance/participants/provision", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestFreezeAccountSuccess(t *testing.T) {
	t.Parallel()
	stub := kycManagerStub{}
	handler := NewComplianceHandler(stub)
	app := fiber.New()
	app.Post("/compliance/accounts/freeze", handler.FreezeAccount)

	body, _ := json.Marshal(map[string]string{"subject": "bank-001"})
	req := httptest.NewRequest(http.MethodPost, "/compliance/accounts/freeze", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestUnfreezeAccountSuccess(t *testing.T) {
	t.Parallel()
	stub := kycManagerStub{}
	handler := NewComplianceHandler(stub)
	app := fiber.New()
	app.Post("/compliance/accounts/unfreeze", handler.UnfreezeAccount)

	body, _ := json.Marshal(map[string]string{"subject": "bank-001"})
	req := httptest.NewRequest(http.MethodPost, "/compliance/accounts/unfreeze", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFreezeAccountError(t *testing.T) {
	t.Parallel()
	stub := kycManagerStub{err: errors.New("db error")}
	handler := NewComplianceHandler(stub)
	app := fiber.New()
	app.Post("/compliance/accounts/freeze", handler.FreezeAccount)

	body, _ := json.Marshal(map[string]string{"subject": "bank-001"})
	req := httptest.NewRequest(http.MethodPost, "/compliance/accounts/freeze", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestRegisterParticipantSuccess(t *testing.T) {
	t.Parallel()
	stub := participantOnboarderStub{
		result: interfaces.OnboardParticipantResult{
			UserID:        "user-uuid-123",
			WalletAddress: "0xABCD",
			CertPEM:       "-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----",
			TxHash:        "0xtx",
		},
	}
	handler := NewComplianceHandler(stub)
	app := fiber.New()
	app.Post("/compliance/register", handler.RegisterParticipant)

	body, _ := json.Marshal(map[string]string{
		"username": "banco-brasil",
		"email":    "admin@bb.com",
		"role":     "ROLE_COMMERCIAL_BANK",
	})
	req := httptest.NewRequest(http.MethodPost, "/compliance/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestRegisterParticipantMissingFields(t *testing.T) {
	t.Parallel()
	stub := participantOnboarderStub{}
	handler := NewComplianceHandler(stub)
	app := fiber.New()
	app.Post("/compliance/register", handler.RegisterParticipant)

	body, _ := json.Marshal(map[string]string{"username": "banco-brasil"})
	req := httptest.NewRequest(http.MethodPost, "/compliance/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRegisterParticipantConflict(t *testing.T) {
	t.Parallel()
	stub := participantOnboarderStub{err: errors.New("already exists: user already registered")}
	handler := NewComplianceHandler(stub)
	app := fiber.New()
	app.Post("/compliance/register", handler.RegisterParticipant)

	body, _ := json.Marshal(map[string]string{
		"username": "banco-brasil",
		"email":    "admin@bb.com",
		"role":     "ROLE_COMMERCIAL_BANK",
	})
	req := httptest.NewRequest(http.MethodPost, "/compliance/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestRegisterParticipantNotAvailable(t *testing.T) {
	t.Parallel()
	// kycManagerStub does NOT implement ParticipantOnboarder
	handler := NewComplianceHandler(kycManagerStub{})
	app := fiber.New()
	app.Post("/compliance/register", handler.RegisterParticipant)

	body, _ := json.Marshal(map[string]string{
		"username": "banco-brasil",
		"email":    "admin@bb.com",
		"role":     "ROLE_COMMERCIAL_BANK",
	})
	req := httptest.NewRequest(http.MethodPost, "/compliance/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", resp.StatusCode)
	}
}

func TestGetKYCStatusViaManager(t *testing.T) {
	t.Parallel()
	stub := kycManagerStub{status: domain.KYCApproved}
	handler := NewComplianceHandler(stub)
	app := fiber.New()
	app.Get("/compliance/kyc/status/:subject", handler.GetKYCStatus)

	req := httptest.NewRequest(http.MethodGet, "/compliance/kyc/status/bank-x", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// ---------- GET /auth/me tests ----------

func TestMeSuccess(t *testing.T) {
	t.Parallel()
	provider := authProviderStub{
		token: domain.AuthToken{AccessToken: "tok", TokenType: "Bearer", ExpiresIn: 3600},
	}
	handler := NewAuthHandler(provider, kycCheckerStub{}, false)
	app := fiber.New()
	app.Get("/auth/me", middleware.RequireCookieAuth(provider), handler.Me)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "valid-token"})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body["subject"] != "stub-user" {
		t.Errorf("expected subject stub-user, got %v", body["subject"])
	}
}

func TestMeMissingToken(t *testing.T) {
	t.Parallel()
	provider := authProviderStub{}
	handler := NewAuthHandler(provider, kycCheckerStub{}, false)
	app := fiber.New()
	app.Get("/auth/me", middleware.RequireCookieAuth(provider), handler.Me)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestMeInvalidToken(t *testing.T) {
	t.Parallel()
	provider := authProviderStub{err: errors.New("bad token")}
	handler := NewAuthHandler(provider, kycCheckerStub{}, false)
	app := fiber.New()
	app.Get("/auth/me", middleware.RequireCookieAuth(provider), handler.Me)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "bad-token"})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// ---------- RegisterParticipant role validation ----------

func TestRegisterParticipantInvalidRole(t *testing.T) {
	t.Parallel()
	stub := participantOnboarderStub{
		result: interfaces.OnboardParticipantResult{UserID: "uid"},
	}
	handler := NewComplianceHandler(stub)
	app := fiber.New()
	app.Post("/compliance/register", handler.RegisterParticipant)

	body, _ := json.Marshal(map[string]string{
		"username": "banco-brasil",
		"email":    "admin@bb.com",
		"role":     "ROLE_GOVERNANCE",
	})
	req := httptest.NewRequest(http.MethodPost, "/compliance/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid role, got %d", resp.StatusCode)
	}
}

// ---------- userManagerStub for GovernanceHandler tests ----------

type userManagerStub struct {
	users []interfaces.UserSummary
	user  interfaces.UserDetail
	err   error
}

func (s userManagerStub) ListUsers(_ context.Context, _, _ string) ([]interfaces.UserSummary, int, error) {
	return s.users, len(s.users), s.err
}

func (s userManagerStub) GetUser(_ context.Context, _ string) (interfaces.UserDetail, error) {
	return s.user, s.err
}

// ---------- GovernanceHandler ListUsers tests ----------

func TestListUsersSuccess(t *testing.T) {
	t.Parallel()
	stub := userManagerStub{
		users: []interfaces.UserSummary{
			{UserID: "uid-1", InstitutionName: "Banco do Brasil", Role: "ROLE_COMMERCIAL_BANK", Status: "ACTIVE", WalletAddress: "0xABCD"},
		},
	}
	handler := NewGovernanceHandler(nil).WithUserManager(stub)
	app := fiber.New()
	app.Get("/governance/users", handler.ListUsers)

	req := httptest.NewRequest(http.MethodGet, "/governance/users", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body["total"].(float64) != 1 {
		t.Errorf("expected total=1, got %v", body["total"])
	}
}

func TestListUsersNotAvailable(t *testing.T) {
	t.Parallel()
	handler := NewGovernanceHandler(nil)
	app := fiber.New()
	app.Get("/governance/users", handler.ListUsers)

	req := httptest.NewRequest(http.MethodGet, "/governance/users", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", resp.StatusCode)
	}
}

func TestListUsersError(t *testing.T) {
	t.Parallel()
	stub := userManagerStub{err: errors.New("backend error")}
	handler := NewGovernanceHandler(nil).WithUserManager(stub)
	app := fiber.New()
	app.Get("/governance/users", handler.ListUsers)

	req := httptest.NewRequest(http.MethodGet, "/governance/users", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

// ---------- GovernanceHandler GetUser tests ----------

func TestGetUserSuccess(t *testing.T) {
	t.Parallel()
	stub := userManagerStub{
		user: interfaces.UserDetail{
			UserID:   "uid-1",
			Username: "banco-brasil",
			Email:    "admin@bb.com",
			Role:     "ROLE_COMMERCIAL_BANK",
			Status:   "ACTIVE",
		},
	}
	handler := NewGovernanceHandler(nil).WithUserManager(stub)
	app := fiber.New()
	app.Get("/governance/users/:userId", handler.GetUser)

	req := httptest.NewRequest(http.MethodGet, "/governance/users/uid-1", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body["username"] != "banco-brasil" {
		t.Errorf("expected username banco-brasil, got %v", body["username"])
	}
}

func TestGetUserNotFound(t *testing.T) {
	t.Parallel()
	stub := userManagerStub{err: errors.New("not found: user not found")}
	handler := NewGovernanceHandler(nil).WithUserManager(stub)
	app := fiber.New()
	app.Get("/governance/users/:userId", handler.GetUser)

	req := httptest.NewRequest(http.MethodGet, "/governance/users/unknown-id", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetUserNotAvailable(t *testing.T) {
	t.Parallel()
	handler := NewGovernanceHandler(nil)
	app := fiber.New()
	app.Get("/governance/users/:userId", handler.GetUser)

	req := httptest.NewRequest(http.MethodGet, "/governance/users/uid-1", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", resp.StatusCode)
	}
}
