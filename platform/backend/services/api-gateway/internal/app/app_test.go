// This file runs end-to-end style tests for gateway routes and auth flows.
package app_test

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/app"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/config"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gofiber/fiber/v2"
)

func TestGatewayHealth(t *testing.T) {
	t.Parallel()

	server := mustNewGateway(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	resp, err := server.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestLoginAndWalletBindFlow(t *testing.T) {
	t.Parallel()

	server := mustNewGateway(t)
	token := loginAndGetToken(t, server, "bank-a", "secret-a")

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	walletAddress := crypto.PubkeyToAddress(key.PublicKey).Hex()
	signature := signWalletBind(t, key, "bank-a", walletAddress)

	payload := map[string]string{
		"walletAddress": walletAddress,
		"signature":     signature,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/auth/wallet/bind", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := server.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestWalletBindConflictsAndValidation(t *testing.T) {
	t.Parallel()

	server := mustNewGateway(t)
	tokenA := loginAndGetToken(t, server, "bank-a", "secret-a")
	tokenB := loginAndGetToken(t, server, "bank-b", "secret-b")

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	walletAddress := crypto.PubkeyToAddress(key.PublicKey).Hex()

	first := map[string]string{
		"walletAddress": walletAddress,
		"signature":     signWalletBind(t, key, "bank-a", walletAddress),
	}
	callWalletBind(t, server, tokenA, first, http.StatusOK)

	second := map[string]string{
		"walletAddress": walletAddress,
		"signature":     signWalletBind(t, key, "bank-b", walletAddress),
	}
	callWalletBind(t, server, tokenB, second, http.StatusConflict)

	invalid := map[string]string{
		"walletAddress": walletAddress,
		"signature":     "0xdeadbeef",
	}
	callWalletBind(t, server, tokenB, invalid, http.StatusBadRequest)
}

func TestRejectedKYCCannotBindWallet(t *testing.T) {
	t.Parallel()

	server := mustNewGateway(t)
	token := loginAndGetToken(t, server, "bank-z", "secret-z")

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	walletAddress := crypto.PubkeyToAddress(key.PublicKey).Hex()

	payload := map[string]string{
		"walletAddress": walletAddress,
		"signature":     signWalletBind(t, key, "bank-z", walletAddress),
	}
	callWalletBind(t, server, token, payload, http.StatusForbidden)
}

func TestKYCStatusEndpoint(t *testing.T) {
	t.Parallel()

	server := mustNewGateway(t)
	req := httptest.NewRequest(http.MethodGet, "/compliance/kyc/status/bank-a", nil)
	resp, err := server.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func mustNewGateway(t *testing.T) *fiber.App {
	t.Helper()

	cfg := config.Config{
		AppPort:        "8080",
		AuthMode:       config.ModeMock,
		TokenTTL:       15 * time.Minute,
		TokenIssuer:    "test-issuer",
		TokenAudience:  "test-audience",
		MockClients:    map[string]string{"bank-a": "secret-a", "bank-b": "secret-b", "bank-z": "secret-z"},
		RequestTimeout: 2 * time.Second,
	}

	server, err := app.New(cfg)
	if err != nil {
		t.Fatalf("failed to initialize app: %v", err)
	}
	return server
}

func loginAndGetToken(t *testing.T, server *fiber.App, clientID, secret string) string {
	t.Helper()

	body, _ := json.Marshal(map[string]string{
		"clientId":     clientID,
		"clientSecret": secret,
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.Test(req)
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 login, got %d", resp.StatusCode)
	}

	var payload struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.AccessToken == "" {
		t.Fatal("expected access token")
	}
	return payload.AccessToken
}

func callWalletBind(t *testing.T, server *fiber.App, token string, payload map[string]string, expected int) {
	t.Helper()
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/auth/wallet/bind", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := server.Test(req)
	if err != nil {
		t.Fatalf("wallet bind call failed: %v", err)
	}
	if resp.StatusCode != expected {
		t.Fatalf("expected %d, got %d", expected, resp.StatusCode)
	}
}

func signWalletBind(t *testing.T, key *ecdsa.PrivateKey, subject, walletAddress string) string {
	t.Helper()
	message := fmt.Sprintf("CBWEB3_WALLET_BIND:%s:%s", subject, strings.ToLower(walletAddress))
	hash := accounts.TextHash([]byte(message))
	sig, err := crypto.Sign(hash, key)
	if err != nil {
		t.Fatalf("failed to sign message: %v", err)
	}
	return "0x" + hex.EncodeToString(sig)
}

