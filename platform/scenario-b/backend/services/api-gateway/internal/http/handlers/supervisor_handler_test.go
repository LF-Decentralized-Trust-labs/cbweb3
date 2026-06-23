// SPDX-License-Identifier: Apache-2.0

// Package handlers provides contract tests for SupervisorHandler (T007/T027 / FR-SUP-001 / US1/US2).
package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAuditLister struct {
	logs []complianceadapter.AuditRecord
	err  error
}

func (m *mockAuditLister) GetAuditLogs(_ context.Context, _, _, _, _ string, _, _ int) ([]complianceadapter.AuditRecord, error) {
	return m.logs, m.err
}

func newSupervisorFiber(lister handlers.ComplianceAuditLister) *fiber.App {
	app := fiber.New()
	h := handlers.NewSupervisorHandler(lister, nil)
	app.Get("/audit/logs", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "sup-1", Roles: []string{domain.RoleSupervisor}})
		return h.GetAuditLogs(c)
	})
	return app
}

func newSupervisorFiberWithMiddleware(lister handlers.ComplianceAuditLister) *fiber.App {
	app := fiber.New()
	h := handlers.NewSupervisorHandler(lister, nil)
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
	lister := &mockAuditLister{}
	app := newSupervisorFiberWithMiddleware(lister)

	req := httptest.NewRequest(http.MethodGet, "/audit/logs", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// (b) supervisor gets logs with pagination metadata
func TestSupervisorHandler_GetAuditLogs_Success(t *testing.T) {
	lister := &mockAuditLister{
		logs: []complianceadapter.AuditRecord{
			{LogID: "l-1", ActionType: "SWAP", Severity: "INFO", Category: "PAYMENTS"},
			{LogID: "l-2", ActionType: "KYC", Severity: "CRITICAL", Category: "COMPLIANCE"},
		},
	}
	app := newSupervisorFiber(lister)

	req := httptest.NewRequest(http.MethodGet, "/audit/logs?severity=CRITICAL&limit=10&page=1", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	payload, _ := io.ReadAll(resp.Body)
	require.NoError(t, json.Unmarshal(payload, &body))

	logs, ok := body["logs"].([]any)
	require.True(t, ok)
	assert.Len(t, logs, 2)
	assert.EqualValues(t, 1, body["page"])
	assert.EqualValues(t, 10, body["limit"])
}

// (c) empty result returns empty logs array (not null)
func TestSupervisorHandler_GetAuditLogs_EmptyResult(t *testing.T) {
	lister := &mockAuditLister{logs: []complianceadapter.AuditRecord{}}
	app := newSupervisorFiber(lister)

	req := httptest.NewRequest(http.MethodGet, "/audit/logs", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	payload, _ := io.ReadAll(resp.Body)
	require.NoError(t, json.Unmarshal(payload, &body))

	logs, ok := body["logs"].([]any)
	require.True(t, ok, "logs must be array, not null")
	assert.Len(t, logs, 0)
}

// (d) limit exceeding max is clamped to 100
func TestSupervisorHandler_GetAuditLogs_LimitClamped(t *testing.T) {
	lister := &mockAuditLister{logs: []complianceadapter.AuditRecord{}}
	app := newSupervisorFiber(lister)

	req := httptest.NewRequest(http.MethodGet, "/audit/logs?limit=9999", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]any
	payload, _ := io.ReadAll(resp.Body)
	require.NoError(t, json.Unmarshal(payload, &body))
	assert.EqualValues(t, 100, body["limit"])
}

// (e) negative page is normalized to 1
func TestSupervisorHandler_GetAuditLogs_NegativePage(t *testing.T) {
	lister := &mockAuditLister{logs: []complianceadapter.AuditRecord{}}
	app := newSupervisorFiber(lister)

	req := httptest.NewRequest(http.MethodGet, "/audit/logs?page=-5", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]any
	payload, _ := io.ReadAll(resp.Body)
	require.NoError(t, json.Unmarshal(payload, &body))
	assert.EqualValues(t, 1, body["page"])
}

// (f) upstream error returns 502
func TestSupervisorHandler_GetAuditLogs_UpstreamError(t *testing.T) {
	lister := &mockAuditLister{err: errors.New("compliance service unavailable")}
	app := newSupervisorFiber(lister)

	req := httptest.NewRequest(http.MethodGet, "/audit/logs", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	payload, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(payload), "compliance service unavailable")
}

// --- ZK Pointer Verification tests (T027 / FR-SUP-002 / US2) ---

type mockZKVerifier struct {
	record *handlers.ZKPointerLookup
	err    error
}

func (m *mockZKVerifier) GetZKPointerRecord(_ context.Context, _, _ string) (*handlers.ZKPointerLookup, error) {
	return m.record, m.err
}

func newZKFiber(verifier handlers.ZKPointerVerifier) *fiber.App {
	app := fiber.New()
	h := handlers.NewSupervisorHandler(nil, verifier)
	app.Get("/zk-pointer/verify", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "sup-1", Roles: []string{domain.RoleSupervisor}})
		return h.VerifyZKPointer(c)
	})
	return app
}

// (g) valid pointer returns 200 with VALID state
func TestSupervisorHandler_VerifyZKPointer_Valid(t *testing.T) {
	exp := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	verifier := &mockZKVerifier{
		record: &handlers.ZKPointerLookup{
			PointerID:      "ptr_abc",
			CommitmentHash: "0xabc123",
			State:          "VALID",
			ExpiresAt:      &exp,
		},
	}
	app := newZKFiber(verifier)

	req := httptest.NewRequest(http.MethodGet, "/zk-pointer/verify?bank_id=cb_spoke_a&commitment_hash=0xabc123", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	payload, _ := io.ReadAll(resp.Body)
	require.NoError(t, json.Unmarshal(payload, &body))
	assert.Equal(t, "VALID", body["state"])
	assert.Equal(t, "ptr_abc", body["pointer_id"])
	assert.Equal(t, "cb_spoke_a", body["bank_id"])
}

// (h) not found returns 404
func TestSupervisorHandler_VerifyZKPointer_NotFound(t *testing.T) {
	verifier := &mockZKVerifier{err: handlers.ErrZKPointerNotFound}
	app := newZKFiber(verifier)

	req := httptest.NewRequest(http.MethodGet, "/zk-pointer/verify?bank_id=cb_spoke_a&commitment_hash=0xdeadbeef", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// (i) expired pointer returns 200 with EXPIRED state
func TestSupervisorHandler_VerifyZKPointer_Expired(t *testing.T) {
	exp := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	verifier := &mockZKVerifier{
		record: &handlers.ZKPointerLookup{
			PointerID:      "ptr_old",
			CommitmentHash: "0xold",
			State:          "EXPIRED",
			ExpiresAt:      &exp,
		},
	}
	app := newZKFiber(verifier)

	req := httptest.NewRequest(http.MethodGet, "/zk-pointer/verify?bank_id=cb_spoke_a&commitment_hash=0xold", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	payload, _ := io.ReadAll(resp.Body)
	require.NoError(t, json.Unmarshal(payload, &body))
	assert.Equal(t, "EXPIRED", body["state"])
}

// (j) missing params returns 400
// (SC-SUP-006) ROLE_GOVERNANCE must be rejected from the supervisor audit log endpoint.
func TestSupervisorHandler_GetAuditLogs_ForbiddenForGovernanceRole(t *testing.T) {
	lister := &mockAuditLister{}
	app := fiber.New()
	h := handlers.NewSupervisorHandler(lister, nil)
	app.Get("/audit/logs",
		func(c *fiber.Ctx) error {
			c.Locals("claims", domain.TokenClaims{Subject: "gov-1", Roles: []string{"ROLE_GOVERNANCE"}})
			return c.Next()
		},
		middleware.RequireSupervisorRole(),
		h.GetAuditLogs,
	)

	req := httptest.NewRequest(http.MethodGet, "/audit/logs", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestSupervisorHandler_VerifyZKPointer_MissingParams(t *testing.T) {
	verifier := &mockZKVerifier{}
	app := newZKFiber(verifier)

	req := httptest.NewRequest(http.MethodGet, "/zk-pointer/verify?bank_id=cb_spoke_a", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
