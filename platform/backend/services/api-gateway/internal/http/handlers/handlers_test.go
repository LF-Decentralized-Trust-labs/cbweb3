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

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
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

type identityManagerStub struct {
	binding domain.WalletBinding
	err     error
}

func (s identityManagerStub) BindWallet(userID, walletAddress string) (domain.WalletBinding, error) {
	if s.err != nil {
		return domain.WalletBinding{}, s.err
	}
	if s.binding.UserID == "" {
		return domain.WalletBinding{UserID: userID, WalletAddress: walletAddress}, nil
	}
	return s.binding, nil
}

func (s identityManagerStub) GetByUser(_ string) (domain.WalletBinding, bool) {
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

