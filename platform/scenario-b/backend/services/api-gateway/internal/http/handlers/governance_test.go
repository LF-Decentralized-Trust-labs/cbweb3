package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/gofiber/fiber/v2"
)

type governanceComplianceStub struct {
	lastActorSubject string
	lastRequestID    string
	updateErr        error
}

type testContextKey string

func (s *governanceComplianceStub) RegisterParticipant(context.Context, complianceadapter.Participant) error {
	return nil
}

func (s *governanceComplianceStub) ListParticipants(context.Context, string, string) ([]complianceadapter.Participant, error) {
	return nil, nil
}

func (s *governanceComplianceStub) SignParticipantCSR(context.Context, string, string, string, string, string) (complianceadapter.SignedCSRResult, error) {
	return complianceadapter.SignedCSRResult{}, nil
}

func (s *governanceComplianceStub) ApproveKYC(context.Context, string, string, string) (complianceadapter.ApproveKYCResult, error) {
	return complianceadapter.ApproveKYCResult{}, nil
}

func (s *governanceComplianceStub) ManageParticipantStatus(context.Context, string, string, string) error {
	return nil
}

func (s *governanceComplianceStub) GetAuditLogs(context.Context, string, string, string, string, int, int) ([]complianceadapter.AuditRecord, error) {
	return nil, nil
}

func (s *governanceComplianceStub) GetCircuitBreakerStatus(context.Context) (complianceadapter.CircuitBreakerStatus, error) {
	return complianceadapter.CircuitBreakerStatus{}, nil
}

func (s *governanceComplianceStub) ToggleCircuitBreaker(context.Context, bool, string) (bool, error) {
	return false, nil
}

func (s *governanceComplianceStub) GetSystemParameters(context.Context) (complianceadapter.SystemParameters, error) {
	return complianceadapter.SystemParameters{}, nil
}

func (s *governanceComplianceStub) UpdateSystemParameters(ctx context.Context, _ complianceadapter.SystemParameters, _ string, actorSubject string) error {
	s.lastActorSubject = actorSubject
	if reqID, ok := ctx.Value(testContextKey("request-id")).(string); ok {
		s.lastRequestID = reqID
	}
	return s.updateErr
}

func TestGovernanceUpdateParametersUsesClaimsSubject(t *testing.T) {
	t.Parallel()

	stub := &governanceComplianceStub{}
	handler := NewGovernanceHandler(stub)

	app := fiber.New()
	app.Put("/api/v1/governance/parameters", func(c *fiber.Ctx) error {
		c.SetUserContext(context.WithValue(c.UserContext(), testContextKey("request-id"), "corr-123"))
		c.Locals("claims", domain.TokenClaims{Subject: "gov-123"})
		return handler.UpdateParameters(c)
	})

	body, _ := json.Marshal(map[string]any{
		"transaction_minimum": "10",
		"transaction_maximum": "1000",
		"slippage_tolerance":  0.1,
		"settlement_window":   10,
		"reason":              "ajuste operacional",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/governance/parameters", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if stub.lastActorSubject != "gov-123" {
		t.Fatalf("expected actor subject gov-123, got %q", stub.lastActorSubject)
	}
	if stub.lastRequestID != "corr-123" {
		t.Fatalf("expected propagated request id corr-123, got %q", stub.lastRequestID)
	}
}
