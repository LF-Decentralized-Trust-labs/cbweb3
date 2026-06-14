package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/gofiber/fiber/v2"
)

type supervisorAuditStub struct {
	logs []complianceadapter.AuditRecord
	err  error
}

func (s *supervisorAuditStub) GetAuditLogs(_ context.Context, _, _, _, _ string, _, _ int) ([]complianceadapter.AuditRecord, error) {
	return s.logs, s.err
}

func (s *supervisorAuditStub) CreateAuditLog(_ context.Context, _ complianceadapter.AuditEntry) error {
	return nil
}

func newSupervisorApp(stub ComplianceAuditLister) *fiber.App {
	app := fiber.New()
	h := NewSupervisorHandler(stub)
	app.Get("/audit/logs", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "sup-1", Roles: []string{domain.RoleSupervisor}})
		return h.GetAuditLogs(c)
	})
	return app
}

func newSupervisorAppWithMiddleware(stub ComplianceAuditLister) *fiber.App {
	app := fiber.New()
	h := NewSupervisorHandler(stub)
	app.Get("/audit/logs",
		func(c *fiber.Ctx) error {
			c.Locals("claims", domain.TokenClaims{Subject: "other-user", Roles: []string{"ROLE_OPERATOR"}})
			return c.Next()
		},
		middleware.RequireSupervisorRole(),
		h.GetAuditLogs,
	)
	return app
}

// (a) request without ROLE_SUPERVISOR gets 403
func TestSupervisorHandler_GetAuditLogs_RequiresSupervisorRole(t *testing.T) {
	t.Parallel()
	app := newSupervisorAppWithMiddleware(&supervisorAuditStub{})

	req := httptest.NewRequest(http.MethodGet, "/audit/logs", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

// (b) supervisor gets logs with pagination metadata
func TestSupervisorHandler_GetAuditLogs_Success(t *testing.T) {
	t.Parallel()
	stub := &supervisorAuditStub{
		logs: []complianceadapter.AuditRecord{
			{LogID: "l-1", ActionType: "PAYMENT", Severity: "INFO", Category: "PAYMENTS"},
		},
	}
	app := newSupervisorApp(stub)

	req := httptest.NewRequest(http.MethodGet, "/audit/logs?limit=20&page=2", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	payload, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	logs, ok := body["logs"].([]any)
	if !ok {
		t.Fatal("expected logs array in response")
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if body["page"] != float64(2) {
		t.Fatalf("expected page=2, got %v", body["page"])
	}
	if body["limit"] != float64(20) {
		t.Fatalf("expected limit=20, got %v", body["limit"])
	}
}

// (c) empty result returns empty logs array (not null)
func TestSupervisorHandler_GetAuditLogs_EmptyResult(t *testing.T) {
	t.Parallel()
	app := newSupervisorApp(&supervisorAuditStub{logs: []complianceadapter.AuditRecord{}})

	req := httptest.NewRequest(http.MethodGet, "/audit/logs", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body map[string]any
	payload, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(payload, &body)

	if _, ok := body["logs"].([]any); !ok {
		t.Fatal("logs must be array, not null")
	}
}

// (d) limit exceeding max is clamped to 100
func TestSupervisorHandler_GetAuditLogs_LimitClamped(t *testing.T) {
	t.Parallel()
	app := newSupervisorApp(&supervisorAuditStub{logs: []complianceadapter.AuditRecord{}})

	req := httptest.NewRequest(http.MethodGet, "/audit/logs?limit=9999", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body map[string]any
	payload, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(payload, &body)

	if body["limit"] != float64(100) {
		t.Fatalf("expected limit clamped to 100, got %v", body["limit"])
	}
}

// (e) negative page is normalized to 1
func TestSupervisorHandler_GetAuditLogs_NegativePage(t *testing.T) {
	t.Parallel()
	app := newSupervisorApp(&supervisorAuditStub{logs: []complianceadapter.AuditRecord{}})

	req := httptest.NewRequest(http.MethodGet, "/audit/logs?page=-5", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body map[string]any
	payload, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(payload, &body)

	if body["page"] != float64(1) {
		t.Fatalf("expected page normalized to 1, got %v", body["page"])
	}
}

// (f) ROLE_GOVERNANCE is explicitly forbidden from the audit log endpoint (SC-SUP-006).
func TestSupervisorHandler_GetAuditLogs_ForbiddenForGovernanceRole(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	h := NewSupervisorHandler(&supervisorAuditStub{})
	app.Get("/audit/logs",
		func(c *fiber.Ctx) error {
			c.Locals("claims", domain.TokenClaims{Subject: "gov-1", Roles: []string{domain.RoleGovernance}})
			return c.Next()
		},
		middleware.RequireSupervisorRole(),
		h.GetAuditLogs,
	)

	req := httptest.NewRequest(http.MethodGet, "/audit/logs", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for ROLE_GOVERNANCE, got %d", resp.StatusCode)
	}
}

// (g) upstream error returns 502
func TestSupervisorHandler_GetAuditLogs_UpstreamError(t *testing.T) {
	t.Parallel()
	app := newSupervisorApp(&supervisorAuditStub{err: errors.New("compliance unavailable")})

	req := httptest.NewRequest(http.MethodGet, "/audit/logs", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", resp.StatusCode)
	}
	payload, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(payload), "compliance unavailable") {
		t.Fatalf("expected error message in body, got: %s", string(payload))
	}
}
