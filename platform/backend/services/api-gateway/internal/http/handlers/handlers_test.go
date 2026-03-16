// This file tests auth and compliance HTTP handler behaviors and responses.
package handlers

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
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

type identityManagerStub struct {
	binding domain.WalletBinding
	err     error
}

func (s identityManagerStub) BindWallet(_ context.Context, userID, walletAddress string) (domain.WalletBinding, error) {
	if s.err != nil {
		return domain.WalletBinding{}, s.err
	}
	if s.binding.UserID == "" {
		return domain.WalletBinding{UserID: userID, WalletAddress: walletAddress}, nil
	}
	return s.binding, nil
}

func (s identityManagerStub) GetByUser(_ context.Context, _ string) (domain.WalletBinding, bool) {
	return domain.WalletBinding{}, false
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
	status    domain.KYCStatus
	vcResult  interfaces.KYCCredentialResult
	verifyOk  bool
	err       error
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

func (s kycManagerStub) IssueKYCCredential(_ context.Context, _, _, _, _, _ string) (interfaces.KYCCredentialResult, error) {
	return s.vcResult, s.err
}

func (s kycManagerStub) VerifyKYCProof(_ context.Context, _ string) (bool, error) {
	return s.verifyOk, s.err
}

func (s kycManagerStub) ProvisionParticipant(_ context.Context, _ string, _ domain.KYCStatus) error {
	return s.err
}

// participantRegistrarStub implements IIdentityManager + KYCChecker + KYCManager + ParticipantRegistrar.
type participantRegistrarStub struct {
	kycManagerStub
	regResult interfaces.RegisterParticipantResult
	regErr    error
}

func (s participantRegistrarStub) BindWallet(_ context.Context, userID, walletAddress string) (domain.WalletBinding, error) {
	return domain.WalletBinding{UserID: userID, WalletAddress: walletAddress}, nil
}

func (s participantRegistrarStub) GetByUser(_ context.Context, _ string) (domain.WalletBinding, bool) {
	return domain.WalletBinding{}, false
}

func (s participantRegistrarStub) RegisterParticipant(_ context.Context, _, _, _, _, _ string) (interfaces.RegisterParticipantResult, error) {
	return s.regResult, s.regErr
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
	}, identityManagerStub{}, kycCheckerStub{})
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
		handler := NewAuthHandler(authProviderStub{}, identityManagerStub{}, kycCheckerStub{})
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
		handler := NewAuthHandler(authProviderStub{err: errors.New("invalid")}, identityManagerStub{}, kycCheckerStub{})
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
		handler := NewAuthHandler(authProviderStub{}, identityManagerStub{}, kycCheckerStub{})
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

func TestAuthHandlerWalletBindRejected(t *testing.T) {
	t.Parallel()
	handler := NewAuthHandler(authProviderStub{}, identityManagerStub{}, kycCheckerStub{status: domain.KYCRejected})
	app := fiber.New()
	app.Post("/auth/wallet/bind", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "bank-a"})
		return handler.WalletBind(c)
	})

	payload, _ := json.Marshal(map[string]string{
		"walletAddress": "0x1111111111111111111111111111111111111111",
		"signature":     "0xdeadbeef",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/wallet/bind", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestAuthHandlerWalletBindSuccess(t *testing.T) {
	t.Parallel()
	handler := NewAuthHandler(authProviderStub{}, identityManagerStub{}, kycCheckerStub{status: domain.KYCApproved})
	app := fiber.New()
	app.Post("/auth/wallet/bind", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "bank-a"})
		return handler.WalletBind(c)
	})

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	wallet := crypto.PubkeyToAddress(key.PublicKey).Hex()
	msg := fmt.Sprintf("CBWEB3_WALLET_BIND:%s:%s", "bank-a", strings.ToLower(wallet))
	hash := accounts.TextHash([]byte(msg))
	sig, err := crypto.Sign(hash, key)
	if err != nil {
		t.Fatalf("failed to sign message: %v", err)
	}

	payload, _ := json.Marshal(map[string]string{
		"walletAddress": wallet,
		"signature":     "0x" + hex.EncodeToString(sig),
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/wallet/bind", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuthHandlerWalletBindInvalidCases(t *testing.T) {
	t.Parallel()

	t.Run("missing claims", func(t *testing.T) {
		handler := NewAuthHandler(authProviderStub{}, identityManagerStub{}, kycCheckerStub{})
		app := fiber.New()
		app.Post("/auth/wallet/bind", handler.WalletBind)
		req := httptest.NewRequest(http.MethodPost, "/auth/wallet/bind", bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		handler := NewAuthHandler(authProviderStub{}, identityManagerStub{}, kycCheckerStub{})
		app := fiber.New()
		app.Post("/auth/wallet/bind", func(c *fiber.Ctx) error {
			c.Locals("claims", domain.TokenClaims{Subject: "bank-a"})
			return handler.WalletBind(c)
		})
		req := httptest.NewRequest(http.MethodPost, "/auth/wallet/bind", bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("internal error", func(t *testing.T) {
		handler := NewAuthHandler(authProviderStub{}, identityManagerStub{err: errors.New("db down")}, kycCheckerStub{status: domain.KYCApproved})
		app := fiber.New()
		app.Post("/auth/wallet/bind", func(c *fiber.Ctx) error {
			c.Locals("claims", domain.TokenClaims{Subject: "bank-a"})
			return handler.WalletBind(c)
		})
		key, err := crypto.GenerateKey()
		if err != nil {
			t.Fatalf("failed to generate key: %v", err)
		}
		wallet := crypto.PubkeyToAddress(key.PublicKey).Hex()
		msg := fmt.Sprintf("CBWEB3_WALLET_BIND:%s:%s", "bank-a", strings.ToLower(wallet))
		hash := accounts.TextHash([]byte(msg))
		sig, err := crypto.Sign(hash, key)
		if err != nil {
			t.Fatalf("failed to sign message: %v", err)
		}
		payload, _ := json.Marshal(map[string]string{
			"walletAddress": wallet,
			"signature":     "0x" + hex.EncodeToString(sig),
		})
		req := httptest.NewRequest(http.MethodPost, "/auth/wallet/bind", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", resp.StatusCode)
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
	}, identityManagerStub{}, kycCheckerStub{})
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
	handler := NewAuthHandler(authProviderStub{err: errors.New("expired")}, identityManagerStub{}, kycCheckerStub{})
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
	handler := NewAuthHandler(authProviderStub{}, identityManagerStub{}, kycCheckerStub{})
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
	handler := NewAuthHandler(authProviderStub{}, identityManagerStub{}, kycCheckerStub{})
	app := fiber.New()
	app.Post("/auth/logout", handler.Logout)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer some-token")
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
	handler := NewAuthHandler(authProviderStub{}, identityManagerStub{}, kycCheckerStub{})
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

// ---------- Onboarding tests ----------

func TestOnboardingCentralBankBypassKYC(t *testing.T) {
	t.Parallel()
	stub := participantRegistrarStub{
		kycManagerStub: kycManagerStub{status: domain.KYCPending},
		regResult: interfaces.RegisterParticipantResult{
			UserID: "cb-001", DID: "did:lac:cb-001", WalletAddress: "0xABC",
		},
	}
	handler := NewAuthHandler(authProviderStub{}, stub, stub)
	app := fiber.New()
	app.Post("/auth/onboarding", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "cb-001", Roles: []string{domain.RoleCentralBank}})
		return handler.Onboarding(c)
	})

	body, _ := json.Marshal(map[string]string{
		"country": "BR", "bank_code": "0000", "role": "CENTRAL_BANK", "institution_name": "BCB",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/onboarding", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestOnboardingKYCApprovedProceed(t *testing.T) {
	t.Parallel()
	stub := participantRegistrarStub{
		kycManagerStub: kycManagerStub{status: domain.KYCApproved},
		regResult: interfaces.RegisterParticipantResult{
			UserID: "bank-001", DID: "did:lac:bank-001", WalletAddress: "0xDEF",
		},
	}
	handler := NewAuthHandler(authProviderStub{}, stub, stub)
	app := fiber.New()
	app.Post("/auth/onboarding", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "bank-001", Roles: []string{domain.RoleCommercialBank}})
		return handler.Onboarding(c)
	})

	body, _ := json.Marshal(map[string]string{"role": "COMMERCIAL_BANK"})
	req := httptest.NewRequest(http.MethodPost, "/auth/onboarding", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestOnboardingKYCPendingReturns202(t *testing.T) {
	t.Parallel()
	stub := participantRegistrarStub{
		kycManagerStub: kycManagerStub{status: domain.KYCPending},
	}
	handler := NewAuthHandler(authProviderStub{}, stub, stub)
	app := fiber.New()
	app.Post("/auth/onboarding", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "bank-002", Roles: []string{domain.RoleCommercialBank}})
		return handler.Onboarding(c)
	})

	body, _ := json.Marshal(map[string]string{"role": "COMMERCIAL_BANK"})
	req := httptest.NewRequest(http.MethodPost, "/auth/onboarding", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
}

func TestOnboardingKYCFrozenReturns403(t *testing.T) {
	t.Parallel()
	stub := participantRegistrarStub{
		kycManagerStub: kycManagerStub{status: domain.KYCFrozen},
	}
	handler := NewAuthHandler(authProviderStub{}, stub, stub)
	app := fiber.New()
	app.Post("/auth/onboarding", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "bank-003", Roles: []string{domain.RoleCommercialBank}})
		return handler.Onboarding(c)
	})

	body, _ := json.Marshal(map[string]string{"role": "COMMERCIAL_BANK"})
	req := httptest.NewRequest(http.MethodPost, "/auth/onboarding", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestOnboardingMissingClaims(t *testing.T) {
	t.Parallel()
	handler := NewAuthHandler(authProviderStub{}, identityManagerStub{}, kycCheckerStub{})
	app := fiber.New()
	app.Post("/auth/onboarding", handler.Onboarding)

	body, _ := json.Marshal(map[string]string{"role": "COMMERCIAL_BANK"})
	req := httptest.NewRequest(http.MethodPost, "/auth/onboarding", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// ---------- Compliance extended tests ----------

func TestIssueKYCCredentialSuccess(t *testing.T) {
	t.Parallel()
	stub := kycManagerStub{
		vcResult: interfaces.KYCCredentialResult{
			VCJWT:      "vc.jwt.token",
			ZKPPointer: "abc123hash",
			IssuedAt:   time.Now().UTC().Format(time.RFC3339),
		},
	}
	handler := NewComplianceHandler(stub)
	app := fiber.New()
	app.Post("/compliance/kyc/issue-credential", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "cb-001", Roles: []string{domain.RoleCentralBank}})
		return handler.IssueKYCCredential(c)
	})

	body, _ := json.Marshal(map[string]string{"subject": "bank-001", "country_code": "BR"})
	req := httptest.NewRequest(http.MethodPost, "/compliance/kyc/issue-credential", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestIssueKYCCredentialMissingSubject(t *testing.T) {
	t.Parallel()
	stub := kycManagerStub{}
	handler := NewComplianceHandler(stub)
	app := fiber.New()
	app.Post("/compliance/kyc/issue-credential", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "cb-001"})
		return handler.IssueKYCCredential(c)
	})

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/compliance/kyc/issue-credential", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestIssueKYCCredentialNoAuth(t *testing.T) {
	t.Parallel()
	stub := kycManagerStub{}
	handler := NewComplianceHandler(stub)
	app := fiber.New()
	app.Post("/compliance/kyc/issue-credential", handler.IssueKYCCredential)

	body, _ := json.Marshal(map[string]string{"subject": "bank-001"})
	req := httptest.NewRequest(http.MethodPost, "/compliance/kyc/issue-credential", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestVerifyKYCProofSuccess(t *testing.T) {
	t.Parallel()
	stub := kycManagerStub{verifyOk: true}
	handler := NewComplianceHandler(stub)
	app := fiber.New()
	app.Post("/compliance/kyc/verify-proof", handler.VerifyKYCProof)

	body, _ := json.Marshal(map[string]string{"zkpPointer": "somehash"})
	req := httptest.NewRequest(http.MethodPost, "/compliance/kyc/verify-proof", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestVerifyKYCProofMissingPointer(t *testing.T) {
	t.Parallel()
	stub := kycManagerStub{}
	handler := NewComplianceHandler(stub)
	app := fiber.New()
	app.Post("/compliance/kyc/verify-proof", handler.VerifyKYCProof)

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/compliance/kyc/verify-proof", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

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

