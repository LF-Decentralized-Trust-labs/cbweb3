// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	"github.com/gofiber/fiber/v2"
)

// The treasury dashboard's operations table reads the audit trail, but the whole
// /api/v1/governance group is ROLE_GOVERNANCE, so the operator who PERFORMS mint and burn
// saw an empty table — a 403, not an absence of entries. PR #143 made those operations write
// audit rows and fixed the field mapping; the screen was still not fixed end to end.
//
// The narrow answer is a treasury-owned read pinned to its own category. "Pinned" is the whole
// point: if the category came from the query, a treasury token could read GOVERNANCE or
// COMPLIANCE rows by asking, which is the widening this route exists to avoid.

// categoryCapturingStub records what the handler asked the compliance service for.
type categoryCapturingStub struct {
	governanceComplianceStub
	gotCategory string
	gotSeverity string
	gotPage     int
	gotLimit    int
}

func (s *categoryCapturingStub) GetAuditLogs(_ context.Context, category, severity, _, _ string, page, limit int) ([]complianceadapter.AuditRecord, error) {
	s.gotCategory = category
	s.gotSeverity = severity
	s.gotPage = page
	s.gotLimit = limit
	return []complianceadapter.AuditRecord{}, nil
}

func treasuryAuditApp(stub *categoryCapturingStub) *fiber.App {
	app := fiber.New()
	h := NewGovernanceHandler(stub)
	app.Get("/api/v1/treasury/audit/logs", h.GetTreasuryAuditLogs)
	return app
}

func TestGetTreasuryAuditLogs_PinsTheCategoryToTreasury(t *testing.T) {
	stub := &categoryCapturingStub{}
	resp, err := treasuryAuditApp(stub).Test(httptest.NewRequest(http.MethodGet, "/api/v1/treasury/audit/logs", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; want 200", resp.StatusCode)
	}
	if stub.gotCategory != "TREASURY" {
		t.Errorf("category = %q; want TREASURY", stub.gotCategory)
	}
}

// The security property of this route: the caller cannot widen it.
func TestGetTreasuryAuditLogs_IgnoresACategoryFromTheQuery(t *testing.T) {
	for _, asked := range []string{"GOVERNANCE", "COMPLIANCE", "", "treasury"} {
		stub := &categoryCapturingStub{}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/treasury/audit/logs?category="+asked, nil)
		if _, err := treasuryAuditApp(stub).Test(req); err != nil {
			t.Fatal(err)
		}
		if stub.gotCategory != "TREASURY" {
			t.Errorf("category=%q in the query produced %q; the route must stay pinned to TREASURY",
				asked, stub.gotCategory)
		}
	}
}

// The other filters are the operator's to use, and the same bounds the governance route
// applies must apply here (finding R2-M-14: an unbounded limit becomes an unbounded query).
func TestGetTreasuryAuditLogs_KeepsTheOtherFiltersAndBounds(t *testing.T) {
	stub := &categoryCapturingStub{}
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/treasury/audit/logs?severity=HIGH&page=2&limit=999999", nil)
	if _, err := treasuryAuditApp(stub).Test(req); err != nil {
		t.Fatal(err)
	}
	if stub.gotSeverity != "HIGH" {
		t.Errorf("severity = %q; want HIGH", stub.gotSeverity)
	}
	if stub.gotPage != 2 {
		t.Errorf("page = %d; want 2", stub.gotPage)
	}
	if stub.gotLimit == 999999 {
		t.Errorf("limit = %d; an unbounded limit must be capped, as the governance route caps it", stub.gotLimit)
	}
}
