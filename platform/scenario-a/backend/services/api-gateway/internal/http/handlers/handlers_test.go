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

	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
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
	handler := NewComplianceHandler(kycCheckerStub{status: domain.KYCApproved}, nil)
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

	handler := NewComplianceHandler(kycCheckerStub{status: domain.KYCApproved}, nil)
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

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAuthHandlerRefreshViaCookie(t *testing.T) {
	t.Parallel()
	handler := NewAuthHandler(authProviderStub{
		token: domain.AuthToken{AccessToken: "new-token", RefreshToken: "new-rt", ExpiresIn: 3600, TokenType: "Bearer"},
	}, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/auth/refresh", handler.Refresh)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "old-rt"})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuthHandlerLogoutSuccess(t *testing.T) {
	t.Parallel()
	handler := NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false)
	app := fiber.New()
	app.Post("/auth/logout", handler.Logout)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "some-token"})
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "some-rt"})
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
	handler := NewComplianceHandler(stub, nil)
	app := fiber.New()
	app.Post("/compliance/aml/screen", handler.AMLScreen)

	body, _ := json.Marshal(map[string]string{"subject": "bank-a"})
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
	handler := NewComplianceHandler(stub, nil)
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
	handler := NewComplianceHandler(stub, nil)
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
	handler := NewComplianceHandler(stub, nil)
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
	handler := NewComplianceHandler(stub, nil)
	app := fiber.New()
	app.Post("/compliance/participants/provision", handler.ProvisionParticipant)

	body, _ := json.Marshal(map[string]string{"subject": "bank-a", "status": "APPROVED"})
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
	handler := NewComplianceHandler(stub, nil)
	app := fiber.New()
	app.Post("/compliance/participants/provision", handler.ProvisionParticipant)

	body, _ := json.Marshal(map[string]string{"subject": "bank-a"})
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
	handler := NewComplianceHandler(stub, nil)
	app := fiber.New()
	app.Post("/compliance/accounts/freeze", handler.FreezeAccount)

	body, _ := json.Marshal(map[string]string{"subject": "bank-a"})
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
	handler := NewComplianceHandler(stub, nil)
	app := fiber.New()
	app.Post("/compliance/accounts/unfreeze", handler.UnfreezeAccount)

	body, _ := json.Marshal(map[string]string{"subject": "bank-a"})
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
	handler := NewComplianceHandler(stub, nil)
	app := fiber.New()
	app.Post("/compliance/accounts/freeze", handler.FreezeAccount)

	body, _ := json.Marshal(map[string]string{"subject": "bank-a"})
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
	handler := NewComplianceHandler(stub, nil)
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
	handler := NewComplianceHandler(stub, nil)
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
	handler := NewComplianceHandler(stub, nil)
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
	handler := NewComplianceHandler(kycManagerStub{}, nil)
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
	handler := NewComplianceHandler(stub, nil)
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
	handler := NewComplianceHandler(stub, nil)
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

// ---------- GET /compliance/participants tests ----------

// participantListerStub implements ComplianceParticipantLister for tests.
type participantListerStub struct {
	participants []complianceadapter.Participant
	err          error
	lastStatus   string
	lastSearch   string
}

func (s *participantListerStub) ListParticipants(_ context.Context, statusFilter, search string) ([]complianceadapter.Participant, error) {
	s.lastStatus = statusFilter
	s.lastSearch = search
	return s.participants, s.err
}

func TestListParticipantsNoFilter(t *testing.T) {
	t.Parallel()
	lister := &participantListerStub{
		participants: []complianceadapter.Participant{
			{UserID: "u1", InstitutionName: "Bank A", Role: "ROLE_COMMERCIAL_BANK", Status: "ACTIVE"},
			{UserID: "u2", InstitutionName: "Bank B", Role: "ROLE_COMMERCIAL_BANK", Status: "PENDING"},
		},
	}
	handler := NewComplianceHandler(kycCheckerStub{status: domain.KYCApproved}, lister)
	app := fiber.New()
	app.Get("/compliance/participants", handler.ListParticipants)

	req := httptest.NewRequest(http.MethodGet, "/compliance/participants", nil)
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
	participants, ok := body["participants"].([]any)
	if !ok {
		t.Fatalf("expected participants array in response")
	}
	if len(participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(participants))
	}
	if lister.lastStatus != "" {
		t.Errorf("expected empty status filter, got %q", lister.lastStatus)
	}
	if lister.lastSearch != "" {
		t.Errorf("expected empty search filter, got %q", lister.lastSearch)
	}
}

func TestListParticipantsWithStatusFilter(t *testing.T) {
	t.Parallel()
	lister := &participantListerStub{
		participants: []complianceadapter.Participant{
			{UserID: "u1", Status: "ACTIVE"},
		},
	}
	handler := NewComplianceHandler(kycCheckerStub{status: domain.KYCApproved}, lister)
	app := fiber.New()
	app.Get("/compliance/participants", handler.ListParticipants)

	req := httptest.NewRequest(http.MethodGet, "/compliance/participants?status=ACTIVE", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if lister.lastStatus != "ACTIVE" {
		t.Errorf("expected status filter ACTIVE, got %q", lister.lastStatus)
	}
}

func TestListParticipantsWithSearchFilter(t *testing.T) {
	t.Parallel()
	lister := &participantListerStub{
		participants: []complianceadapter.Participant{},
	}
	handler := NewComplianceHandler(kycCheckerStub{status: domain.KYCApproved}, lister)
	app := fiber.New()
	app.Get("/compliance/participants", handler.ListParticipants)

	req := httptest.NewRequest(http.MethodGet, "/compliance/participants?status=ACTIVE&search=Itau", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if lister.lastStatus != "ACTIVE" {
		t.Errorf("expected status filter ACTIVE, got %q", lister.lastStatus)
	}
	if lister.lastSearch != "Itau" {
		t.Errorf("expected search filter Itau, got %q", lister.lastSearch)
	}
}

func TestListParticipantsInvalidStatus(t *testing.T) {
	t.Parallel()
	lister := &participantListerStub{
		participants: []complianceadapter.Participant{},
	}
	handler := NewComplianceHandler(kycCheckerStub{status: domain.KYCApproved}, lister)
	app := fiber.New()
	app.Get("/compliance/participants", handler.ListParticipants)

	req := httptest.NewRequest(http.MethodGet, "/compliance/participants?status=XPTO", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 (pass-through), got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	participants, ok := body["participants"].([]any)
	if !ok {
		t.Fatalf("expected participants array in response")
	}
	if len(participants) != 0 {
		t.Fatalf("expected 0 participants for invalid status, got %d", len(participants))
	}
}

func TestListParticipantsGRPCError(t *testing.T) {
	t.Parallel()
	lister := &participantListerStub{
		err: errors.New("compliance service unavailable"),
	}
	handler := NewComplianceHandler(kycCheckerStub{status: domain.KYCApproved}, lister)
	app := fiber.New()
	app.Get("/compliance/participants", handler.ListParticipants)

	req := httptest.NewRequest(http.MethodGet, "/compliance/participants", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

// ---------- OnboardingHandler.GetMyOnboardingStatus tests ----------

// onboardingManagerStub implements interfaces.OnboardingManager for handler tests.
type onboardingManagerStub struct {
	statusResult interfaces.OnboardingStatus
	statusErr    error
	byBankResult interfaces.OnboardingStatus
	byBankErr    error
}

func (s onboardingManagerStub) SubmitCredentialRequest(_ context.Context, _ interfaces.CredentialRequest) (interfaces.CredentialRequestResult, error) {
	return interfaces.CredentialRequestResult{}, nil
}

func (s onboardingManagerStub) GetOnboardingStatus(_ context.Context, _ string) (interfaces.OnboardingStatus, error) {
	return s.statusResult, s.statusErr
}

func (s onboardingManagerStub) GetOnboardingStatusByBankCode(_ context.Context, _ string) (interfaces.OnboardingStatus, error) {
	return s.byBankResult, s.byBankErr
}

func (s onboardingManagerStub) CompleteOnboarding(_ context.Context, _ interfaces.CompleteOnboardingRequest) (interfaces.CompleteOnboardingResult, error) {
	return interfaces.CompleteOnboardingResult{}, nil
}

func TestGetMyOnboardingStatusSuccess(t *testing.T) {
	t.Parallel()
	stub := onboardingManagerStub{
		byBankResult: interfaces.OnboardingStatus{
			RequestID: "req-uuid-001",
			UserID:    "user-uuid-001",
			Status:    "KYC_APPROVED",
		},
	}
	handler := NewOnboardingHandler(stub)
	app := fiber.New()
	app.Get("/onboarding/my-status", handler.GetMyOnboardingStatus)

	req := httptest.NewRequest(http.MethodGet, "/onboarding/my-status?bank_code=a", nil)
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
	if body["request_id"] != "req-uuid-001" {
		t.Errorf("expected request_id req-uuid-001, got %v", body["request_id"])
	}
	if body["status"] != "KYC_APPROVED" {
		t.Errorf("expected status KYC_APPROVED, got %v", body["status"])
	}
	// wallet_address and pop_nonce are not included in this endpoint's response.
	if _, exists := body["wallet_address"]; exists {
		t.Errorf("wallet_address must be absent from my-status response, but it was present")
	}
	if _, exists := body["pop_nonce"]; exists {
		t.Errorf("pop_nonce must be absent from my-status response, but it was present")
	}
}

func TestGetMyOnboardingStatusMissingBankCode(t *testing.T) {
	t.Parallel()
	stub := onboardingManagerStub{}
	handler := NewOnboardingHandler(stub)
	app := fiber.New()
	app.Get("/onboarding/my-status", handler.GetMyOnboardingStatus)

	req := httptest.NewRequest(http.MethodGet, "/onboarding/my-status", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGetMyOnboardingStatusNotFound(t *testing.T) {
	t.Parallel()
	stub := onboardingManagerStub{
		byBankErr: errors.New("not found: no request for this bank"),
	}
	handler := NewOnboardingHandler(stub)
	app := fiber.New()
	app.Get("/onboarding/my-status", handler.GetMyOnboardingStatus)

	req := httptest.NewRequest(http.MethodGet, "/onboarding/my-status?bank_code=x", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetMyOnboardingStatusInternalError(t *testing.T) {
	t.Parallel()
	stub := onboardingManagerStub{
		byBankErr: errors.New("database connection refused"),
	}
	handler := NewOnboardingHandler(stub)
	app := fiber.New()
	app.Get("/onboarding/my-status", handler.GetMyOnboardingStatus)

	req := httptest.NewRequest(http.MethodGet, "/onboarding/my-status?bank_code=a", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestGetMyOnboardingStatusResponseShape(t *testing.T) {
	t.Parallel()
	stub := onboardingManagerStub{
		byBankResult: interfaces.OnboardingStatus{
			RequestID: "req-uuid-002",
			UserID:    "user-uuid-002",
			Status:    "CREDENTIAL_REQUESTED",
		},
	}
	handler := NewOnboardingHandler(stub)
	app := fiber.New()
	app.Get("/onboarding/my-status", handler.GetMyOnboardingStatus)

	req := httptest.NewRequest(http.MethodGet, "/onboarding/my-status?bank_code=b", nil)
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
	// Only request_id, user_id, status are returned.
	for _, absent := range []string{"pop_nonce", "wallet_address"} {
		if _, exists := body[absent]; exists {
			t.Errorf("%s must be absent from my-status response, but it was present", absent)
		}
	}
	if body["status"] != "CREDENTIAL_REQUESTED" {
		t.Errorf("expected status CREDENTIAL_REQUESTED, got %v", body["status"])
	}
	if body["request_id"] != "req-uuid-002" {
		t.Errorf("expected request_id req-uuid-002, got %v", body["request_id"])
	}
	if body["user_id"] != "user-uuid-002" {
		t.Errorf("expected user_id user-uuid-002, got %v", body["user_id"])
	}
}

func TestGetMyOnboardingStatusFromJWT(t *testing.T) {
	t.Parallel()
	stub := onboardingManagerStub{
		byBankResult: interfaces.OnboardingStatus{
			RequestID: "req-uuid-jwt",
			UserID:    "user-uuid-jwt",
			Status:    "ACTIVE",
		},
	}
	handler := NewOnboardingHandler(stub)
	app := fiber.New()
	// Simulate auth middleware injecting claims with BankID.
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "user-uuid-jwt", BankID: "a"})
		return c.Next()
	})
	app.Get("/onboarding/my-status", handler.GetMyOnboardingStatus)

	// No bank_code param — resolved from JWT.
	req := httptest.NewRequest(http.MethodGet, "/onboarding/my-status", nil)
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
	if body["request_id"] != "req-uuid-jwt" {
		t.Errorf("expected request_id req-uuid-jwt, got %v", body["request_id"])
	}
	if body["status"] != "ACTIVE" {
		t.Errorf("expected status ACTIVE, got %v", body["status"])
	}
}
