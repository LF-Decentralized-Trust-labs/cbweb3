// This file runs end-to-end style tests for gateway routes and auth flows.
package app_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/app"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/config"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"
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
	var bindResp struct {
		UserID        string `json:"userId"`
		WalletAddress string `json:"walletAddress"`
		Status        string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&bindResp); err != nil {
		t.Fatalf("failed to decode wallet bind success response: %v", err)
	}
	if bindResp.UserID != "bank-a" {
		t.Fatalf("expected userId bank-a, got %s", bindResp.UserID)
	}
	if bindResp.WalletAddress == "" {
		t.Fatal("expected non-empty walletAddress")
	}
	if bindResp.Status != "BOUND" {
		t.Fatalf("expected status BOUND, got %s", bindResp.Status)
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
	callWalletBindSuccess(t, server, tokenA, first, "bank-a")

	second := map[string]string{
		"walletAddress": walletAddress,
		"signature":     signWalletBind(t, key, "bank-b", walletAddress),
	}
	callWalletBindError(t, server, tokenB, second, http.StatusConflict, "wallet already bound")

	invalid := map[string]string{
		"walletAddress": walletAddress,
		"signature":     "0xdeadbeef",
	}
	callWalletBindError(t, server, tokenB, invalid, http.StatusBadRequest, "invalid signature")
}

func TestWalletBindUserAlreadyBoundConflict(t *testing.T) {
	t.Parallel()

	server := mustNewGateway(t)
	token := loginAndGetToken(t, server, "bank-a", "secret-a")

	keyA, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	walletA := crypto.PubkeyToAddress(keyA.PublicKey).Hex()
	payloadA := map[string]string{
		"walletAddress": walletA,
		"signature":     signWalletBind(t, keyA, "bank-a", walletA),
	}
	callWalletBindSuccess(t, server, token, payloadA, "bank-a")

	keyB, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	walletB := crypto.PubkeyToAddress(keyB.PublicKey).Hex()
	payloadB := map[string]string{
		"walletAddress": walletB,
		"signature":     signWalletBind(t, keyB, "bank-a", walletB),
	}
	callWalletBindError(t, server, token, payloadB, http.StatusConflict, "user already bound to another wallet")
}

func TestWalletBindRequiresBearerToken(t *testing.T) {
	t.Parallel()

	server := mustNewGateway(t)

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	walletAddress := crypto.PubkeyToAddress(key.PublicKey).Hex()
	payload := map[string]string{
		"walletAddress": walletAddress,
		"signature":     signWalletBind(t, key, "bank-a", walletAddress),
	}

	body, _ := json.Marshal(payload)
	reqWithoutBearer := httptest.NewRequest(http.MethodPost, "/auth/wallet/bind", bytes.NewReader(body))
	reqWithoutBearer.Header.Set("Content-Type", "application/json")
	resp, err := server.Test(reqWithoutBearer)
	if err != nil {
		t.Fatalf("wallet bind without bearer failed: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without bearer, got %d", resp.StatusCode)
	}
	var errResp1 struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&errResp1); err != nil {
		t.Fatalf("failed to decode missing bearer error response: %v", err)
	}
	if errResp1.Error != "missing bearer token" {
		t.Fatalf("expected missing bearer token error, got %q", errResp1.Error)
	}

	body2, _ := json.Marshal(payload)
	reqInvalidBearer := httptest.NewRequest(http.MethodPost, "/auth/wallet/bind", bytes.NewReader(body2))
	reqInvalidBearer.Header.Set("Content-Type", "application/json")
	reqInvalidBearer.Header.Set("Authorization", "Bearer invalid-token")
	resp2, err := server.Test(reqInvalidBearer)
	if err != nil {
		t.Fatalf("wallet bind with invalid bearer failed: %v", err)
	}
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 with invalid bearer, got %d", resp2.StatusCode)
	}
	var errResp2 struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&errResp2); err != nil {
		t.Fatalf("failed to decode invalid bearer error response: %v", err)
	}
	if errResp2.Error != "invalid token" {
		t.Fatalf("expected invalid token error, got %q", errResp2.Error)
	}
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
	callWalletBindError(t, server, token, payload, http.StatusForbidden, "kyc status does not allow wallet binding")
}

func TestKYCStatusEndpoint(t *testing.T) {
	t.Parallel()

	server := mustNewGateway(t)
	token := loginAndGetToken(t, server, "bank-a", "secret-a")

	req := httptest.NewRequest(http.MethodGet, "/compliance/kyc/status/bank-a", nil)
	req.Header.Set("Authorization", "Bearer "+token)
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
	identityAddr, cleanup := startIdentityMockGRPC(t)
	t.Cleanup(cleanup)

	cfg := config.Config{
		AppPort:          "8080",
		RequestTimeout:   2 * time.Second,
		IdentityGRPCAddr: identityAddr,
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

func callWalletBindSuccess(t *testing.T, server *fiber.App, token string, payload map[string]string, expectedUserID string) {
	t.Helper()
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/auth/wallet/bind", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := server.Test(req)
	if err != nil {
		t.Fatalf("wallet bind call failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, resp.StatusCode)
	}
	var successResp struct {
		UserID        string `json:"userId"`
		WalletAddress string `json:"walletAddress"`
		Status        string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&successResp); err != nil {
		t.Fatalf("failed to decode wallet bind success response: %v", err)
	}
	if successResp.UserID != expectedUserID {
		t.Fatalf("expected userId %s, got %s", expectedUserID, successResp.UserID)
	}
	if successResp.WalletAddress == "" {
		t.Fatal("expected non-empty walletAddress")
	}
	if successResp.Status != "BOUND" {
		t.Fatalf("expected status BOUND, got %s", successResp.Status)
	}
}

func callWalletBindError(t *testing.T, server *fiber.App, token string, payload map[string]string, expectedStatus int, expectedError string) {
	t.Helper()
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/auth/wallet/bind", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := server.Test(req)
	if err != nil {
		t.Fatalf("wallet bind call failed: %v", err)
	}
	if resp.StatusCode != expectedStatus {
		t.Fatalf("expected %d, got %d", expectedStatus, resp.StatusCode)
	}
	var errorResp struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
		t.Fatalf("failed to decode wallet bind error response: %v", err)
	}
	if errorResp.Error != expectedError {
		t.Fatalf("expected error %q, got %q", expectedError, errorResp.Error)
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

type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }
func (jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
func (jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

type identityMock struct {
	mu           sync.Mutex
	byUser       map[string]string
	byWallet     map[string]string
	tokenToUser  map[string]string
	validSecrets map[string]string
	kycStatus    map[string]string // subject → KYC status string
}

type identityMockService interface{}

func startIdentityMockGRPC(t *testing.T) (string, func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start tcp listener: %v", err)
	}
	codec := jsonCodec{}
	encoding.RegisterCodec(codec)

	mock := &identityMock{
		byUser:       map[string]string{},
		byWallet:     map[string]string{},
		tokenToUser:  map[string]string{},
		validSecrets: map[string]string{"bank-a": "secret-a", "bank-b": "secret-b", "bank-z": "secret-z"},
		// bank-z is seeded as REVOKED (consistent with LocalProvider) to test the KYC gate on wallet bind.
		kycStatus: map[string]string{"bank-z": "REVOKED"},
	}

	server := grpc.NewServer(grpc.ForceServerCodec(codec))
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: "identity.v1.IdentityService",
		HandlerType: (*identityMockService)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "Login", Handler: mock.loginHandler},
			{MethodName: "ValidateToken", Handler: mock.validateHandler},
			{MethodName: "BindWallet", Handler: mock.bindHandler},
			{MethodName: "GetByUser", Handler: mock.getByUserHandler},
			{MethodName: "GetKYCStatus", Handler: mock.getKYCStatusHandler},
		},
	}, mock)

	go func() {
		_ = server.Serve(lis)
	}()

	cleanup := func() {
		server.Stop()
		_ = lis.Close()
	}
	return lis.Addr().String(), cleanup
}

func (m *identityMock) loginHandler(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	var req struct {
		User     string `json:"user"`
		Password string `json:"password"`
	}
	if err := dec(&req); err != nil {
		return nil, err
	}
	secret, ok := m.validSecrets[req.User]
	if !ok || secret != req.Password {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}
	token := "token-" + req.User
	m.mu.Lock()
	m.tokenToUser[token] = req.User
	m.mu.Unlock()
	return &struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   900,
	}, nil
}

func (m *identityMock) validateHandler(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	var req struct {
		AccessToken string `json:"access_token"`
	}
	if err := dec(&req); err != nil {
		return nil, err
	}
	m.mu.Lock()
	user, ok := m.tokenToUser[req.AccessToken]
	m.mu.Unlock()
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}
	return &struct {
		Subject string   `json:"subject"`
		Issuer  string   `json:"issuer"`
		Roles   []string `json:"roles"`
	}{Subject: user, Issuer: "identity-mock", Roles: []string{"bank"}}, nil
}

func (m *identityMock) bindHandler(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	var req struct {
		UserID        string `json:"user_id"`
		WalletAddress string `json:"wallet_address"`
	}
	if err := dec(&req); err != nil {
		return nil, err
	}
	userID := strings.TrimSpace(req.UserID)
	wallet := strings.ToLower(strings.TrimSpace(req.WalletAddress))

	m.mu.Lock()
	defer m.mu.Unlock()
	if existingUser, ok := m.byWallet[wallet]; ok && existingUser != userID {
		return nil, status.Error(codes.AlreadyExists, "wallet already bound")
	}
	if existingWallet, ok := m.byUser[userID]; ok && !strings.EqualFold(existingWallet, wallet) {
		return nil, status.Error(codes.FailedPrecondition, "user already bound")
	}
	m.byWallet[wallet] = userID
	m.byUser[userID] = wallet
	return &struct {
		UserID        string `json:"user_id"`
		WalletAddress string `json:"wallet_address"`
	}{UserID: userID, WalletAddress: wallet}, nil
}

func (m *identityMock) getByUserHandler(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := dec(&req); err != nil {
		return nil, err
	}
	m.mu.Lock()
	wallet, ok := m.byUser[req.UserID]
	m.mu.Unlock()
	if !ok {
		return &struct {
			Binding any  `json:"binding,omitempty"`
			Found   bool `json:"found"`
		}{Found: false}, nil
	}
	return &struct {
		Binding any  `json:"binding,omitempty"`
		Found   bool `json:"found"`
	}{Binding: map[string]string{"user_id": req.UserID, "wallet_address": wallet}, Found: true}, nil
}

func (m *identityMock) getKYCStatusHandler(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	var req struct {
		Subject string `json:"subject"`
	}
	if err := dec(&req); err != nil {
		return nil, err
	}
	m.mu.Lock()
	kyc, ok := m.kycStatus[req.Subject]
	m.mu.Unlock()
	if !ok {
		kyc = "APPROVED"
	}
	return &struct {
		Subject string `json:"subject"`
		Status  string `json:"status"`
	}{Subject: req.Subject, Status: kyc}, nil
}
