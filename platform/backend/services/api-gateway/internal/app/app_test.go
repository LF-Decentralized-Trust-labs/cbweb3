// This file runs end-to-end style tests for gateway routes and auth flows.
package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/app"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/config"
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

func TestKYCStatusEndpoint(t *testing.T) {
	t.Parallel()

	server := mustNewGateway(t)
	token := loginAndGetToken(t, server, "bank-a", "secret-a")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/compliance/kyc/status/bank-a", nil)
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
	identityAddr, identityCleanup := startIdentityMockGRPC(t)
	t.Cleanup(identityCleanup)

	complianceAddr, complianceCleanup := startComplianceMockGRPC(t)
	t.Cleanup(complianceCleanup)

	cfg := config.Config{
		AppPort:            "8080",
		RequestTimeout:     2 * time.Second,
		AuthGRPCAddr:       identityAddr,
		ComplianceGRPCAddr: complianceAddr,
	}

	server, err := app.New(cfg)
	if err != nil {
		t.Fatalf("failed to initialize app: %v", err)
	}
	return server
}

type complianceMockService interface{}

func startComplianceMockGRPC(t *testing.T) (string, func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start compliance tcp listener: %v", err)
	}
	codec := jsonCodec{}
	server := grpc.NewServer(grpc.ForceServerCodec(codec))
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: "compliance.v1.ComplianceService",
		HandlerType: (*complianceMockService)(nil),
		Methods:     []grpc.MethodDesc{},
	}, struct{ complianceMockService }{})
	go func() { _ = server.Serve(lis) }()
	return lis.Addr().String(), func() {
		server.Stop()
		_ = lis.Close()
	}
}

func loginAndGetToken(t *testing.T, server *fiber.App, clientID, secret string) string {
	t.Helper()

	body, _ := json.Marshal(map[string]string{
		"clientId":     clientID,
		"clientSecret": secret,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
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
		tokenToUser:  map[string]string{},
		validSecrets: map[string]string{"bank-a": "secret-a", "bank-b": "secret-b", "bank-z": "secret-z"},
		kycStatus:    map[string]string{"bank-z": "REVOKED"},
	}

	server := grpc.NewServer(grpc.ForceServerCodec(codec))
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: "auth.v1.AuthService",
		HandlerType: (*identityMockService)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "Login", Handler: mock.loginHandler},
			{MethodName: "ValidateToken", Handler: mock.validateHandler},
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
