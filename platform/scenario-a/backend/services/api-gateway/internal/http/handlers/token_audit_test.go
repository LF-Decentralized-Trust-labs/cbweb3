// SPDX-License-Identifier: Apache-2.0

// Tests for the audit trail and the mandatory justification on the two routes
// that create and destroy money (finding R2-M-8).
//
// Two things were true before these tests existed. First, the treasury screen
// forced the operator to type a redemption reason and then dropped it in the
// browser: the request body carried only `from` and `amount`, and the handler
// accepted only those. So the stated reason for destroying central-bank money
// existed nowhere on the server. Second, neither mint nor burn wrote an audit
// entry at all — which is also why the treasury history screen, which reads
// `/governance/audit/logs?category=TREASURY`, had nothing to show.
//
// What these tests pin:
//   - a burn without a reason is rejected, so the justification cannot be lost again;
//   - a successful burn/mint writes an audit entry carrying actor, amount and reason;
//   - a failure to write that entry does NOT fail the operation (best effort), because
//     blocking a money operation on the availability of the compliance service would
//     break a flow that works today;
//   - and neither does a compliance service that HANGS. That is the half best effort
//     actually turns on: an error returns, an outage does not, so only a bounded call
//     keeps the promise. The first version of this feature bounded nothing;
//   - a handler with no audit writer wired behaves exactly as before (regression guard);
//   - mint does NOT require a request id. Three pieces of tooling mint as scenario
//     setup with no deposit to reference (the livehappy E2E, the performance
//     provisioning script and the FX tryout); requiring it would break them.
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeAuditWriter records the entries a handler writes, and can be made to fail.
type fakeAuditWriter struct {
	entries []complianceadapter.AuditEntry
	err     error
}

func (f *fakeAuditWriter) CreateAuditLog(_ context.Context, entry complianceadapter.AuditEntry) error {
	f.entries = append(f.entries, entry)
	return f.err
}

// tokenAuditApp wires a handler with the given audit writer over the fake backend.
func tokenAuditApp(t *testing.T, writer AuditLogWriter) (*fiber.App, *fakePaymentServer) {
	t.Helper()
	fake := &fakePaymentServer{token: &pb.MintTokenResponse{TxHash: "tx"}}
	h := startFakePaymentBackend(t, fake)
	if writer != nil {
		h = h.WithAuditLogger(writer)
	}
	app := fiber.New()
	app.Use(authedClaims("central-bank-a"))
	app.Post("/mint", h.MintToken)
	app.Post("/burn", h.BurnToken)
	return app, fake
}

// A burn with no reason must be rejected. The reason is the only record of WHY
// money was destroyed; accepting the request without it recreates the defect.
func TestBurnToken_RequiresReason(t *testing.T) {
	t.Parallel()
	app, _ := tokenAuditApp(t, &fakeAuditWriter{})

	resp := postJSON(t, app, "/burn", map[string]any{"from": "vault-01", "amount": "50000"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("burn without reason: want 400, got %d", resp.StatusCode)
	}

	// Whitespace is not a reason.
	resp = postJSON(t, app, "/burn", map[string]any{"from": "vault-01", "amount": "50000", "reason": "   "})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("burn with blank reason: want 400, got %d", resp.StatusCode)
	}
}

// A successful burn writes one audit entry carrying the actor, the amount and the
// operator's reason.
func TestBurnToken_WritesAuditEntry(t *testing.T) {
	t.Parallel()
	writer := &fakeAuditWriter{}
	app, _ := tokenAuditApp(t, writer)

	resp := postJSON(t, app, "/burn", map[string]any{
		"from":   "vault-01",
		"amount": "50000",
		"reason": "quarterly redemption for bank-a",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("burn: want 201, got %d", resp.StatusCode)
	}
	if len(writer.entries) != 1 {
		t.Fatalf("want 1 audit entry, got %d", len(writer.entries))
	}
	entry := writer.entries[0]
	if entry.ActorSubject != "user-1" {
		t.Errorf("ActorSubject = %q, want the authenticated subject", entry.ActorSubject)
	}
	if entry.ActionType != "TOKEN_BURN" {
		t.Errorf("ActionType = %q, want TOKEN_BURN", entry.ActionType)
	}
	if entry.Category != "TREASURY" {
		// The treasury history screen filters on this category; a different value
		// writes the entry where nothing reads it.
		t.Errorf("Category = %q, want TREASURY", entry.Category)
	}
	if entry.Severity != "HIGH" {
		t.Errorf("Severity = %q, want HIGH", entry.Severity)
	}
	if !strings.Contains(entry.Details, "quarterly redemption for bank-a") {
		t.Errorf("Details %q does not carry the operator reason", entry.Details)
	}
	if !strings.Contains(entry.Details, "50000") {
		t.Errorf("Details %q does not carry the amount", entry.Details)
	}
}

// A mint writes an audit entry too, and records the funding request and reserve
// proof reference when the caller supplies them.
func TestMintToken_WritesAuditEntryWithRequestAndProof(t *testing.T) {
	t.Parallel()
	writer := &fakeAuditWriter{}
	app, _ := tokenAuditApp(t, writer)

	resp := postJSON(t, app, "/mint", map[string]any{
		"to":                "bank-a",
		"amount":            "100000",
		"request_id":        "dep-77",
		"reserve_proof_ref": "proof-2026-08-19",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("mint: want 201, got %d", resp.StatusCode)
	}
	if len(writer.entries) != 1 {
		t.Fatalf("want 1 audit entry, got %d", len(writer.entries))
	}
	entry := writer.entries[0]
	if entry.ActionType != "TOKEN_MINT" {
		t.Errorf("ActionType = %q, want TOKEN_MINT", entry.ActionType)
	}
	if entry.Category != "TREASURY" {
		t.Errorf("Category = %q, want TREASURY", entry.Category)
	}
	for _, want := range []string{"dep-77", "proof-2026-08-19", "100000"} {
		if !strings.Contains(entry.Details, want) {
			t.Errorf("Details %q is missing %q", entry.Details, want)
		}
	}
}

// Mint must NOT require a request id. The livehappy E2E, the performance
// provisioning script and the FX tryout all mint as setup, with no deposit to
// reference; requiring one would break them.
func TestMintToken_DoesNotRequireRequestID(t *testing.T) {
	t.Parallel()
	writer := &fakeAuditWriter{}
	app, _ := tokenAuditApp(t, writer)

	resp := postJSON(t, app, "/mint", map[string]any{"to": "bank-a", "amount": "1"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("mint without request_id: want 201, got %d", resp.StatusCode)
	}
	if len(writer.entries) != 1 {
		t.Fatalf("want 1 audit entry even without a request id, got %d", len(writer.entries))
	}
}

// A failure to write the audit entry must not fail the operation. The burn has
// already happened on-chain and cannot be rolled back; failing the response
// would report a false negative and would make a compliance outage stop money
// operations that work today.
func TestBurnToken_AuditWriteFailureDoesNotBlock(t *testing.T) {
	t.Parallel()
	writer := &fakeAuditWriter{err: errors.New("compliance unreachable")}
	app, _ := tokenAuditApp(t, writer)

	resp := postJSON(t, app, "/burn", map[string]any{
		"from": "vault-01", "amount": "1", "reason": "test",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("burn with failing audit writer: want 201, got %d", resp.StatusCode)
	}
	if len(writer.entries) != 1 {
		t.Errorf("the entry should still have been attempted, got %d attempts", len(writer.entries))
	}
}

// With no audit writer wired the handler must behave exactly as it did before
// this change, so an entity that has not configured compliance is unaffected.
func TestTokenHandlers_NoAuditLoggerWired(t *testing.T) {
	t.Parallel()
	app, _ := tokenAuditApp(t, nil)

	if resp := postJSON(t, app, "/mint", map[string]any{"to": "x", "amount": "1"}); resp.StatusCode != http.StatusCreated {
		t.Errorf("mint with no audit writer: want 201, got %d", resp.StatusCode)
	}
	if resp := postJSON(t, app, "/burn", map[string]any{"from": "x", "amount": "1", "reason": "r"}); resp.StatusCode != http.StatusCreated {
		t.Errorf("burn with no audit writer: want 201, got %d", resp.StatusCode)
	}
}

// The audit Details field must be valid JSON for any operator input.
//
// It was built with fmt.Sprintf and %q, which is Go quoting, not JSON quoting:
// a control byte or invalid UTF-8 in the reason renders as \x7f, which JSON
// rejects. The record created for accountability would be the one that cannot
// be parsed — and an operator types this field by hand.
func TestBurnToken_AuditDetailsAreValidJSON(t *testing.T) {
	t.Parallel()
	for name, reason := range map[string]string{
		"quotes":       `he said "settle it"`,
		"control byte": "reason with \x01 control",
		"del byte":     "reason with \x7f del",
		"invalid utf8": string([]byte{0x66, 0x6f, 0x6f, 0xff}),
		"newline":      "first line\nsecond line",
	} {
		t.Run(name, func(t *testing.T) {
			writer := &fakeAuditWriter{}
			app, _ := tokenAuditApp(t, writer)

			resp := postJSON(t, app, "/burn", map[string]any{
				"from": "vault-01", "amount": "50000", "reason": reason,
			})
			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("burn: want 201, got %d", resp.StatusCode)
			}
			if len(writer.entries) != 1 {
				t.Fatalf("want 1 audit entry, got %d", len(writer.entries))
			}
			var parsed map[string]any
			if err := json.Unmarshal([]byte(writer.entries[0].Details), &parsed); err != nil {
				t.Fatalf("Details is not valid JSON (%v): %s", err, writer.entries[0].Details)
			}
			if parsed["amount"] != "50000" {
				t.Errorf("amount = %v, want 50000", parsed["amount"])
			}
		})
	}
}

// A rejected burn must leave a record too. Auditing only successes means the
// trail cannot answer "did anyone try", which is the question an incident
// review asks first.
func TestBurnToken_FailedAttemptIsAudited(t *testing.T) {
	t.Parallel()
	writer := &fakeAuditWriter{}
	fake := &fakePaymentServer{err: status.Error(codes.Internal, "zeto burn rejected")}
	h := startFakePaymentBackend(t, fake).WithAuditLogger(writer)
	app := fiber.New()
	app.Use(authedClaims("central-bank-a"))
	app.Post("/burn", h.BurnToken)

	resp := postJSON(t, app, "/burn", map[string]any{
		"from": "vault-01", "amount": "50000", "reason": "quarterly redemption",
	})
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("burn against a failing backend: want 500, got %d", resp.StatusCode)
	}
	if len(writer.entries) != 1 {
		t.Fatalf("want 1 audit entry for the failed attempt, got %d", len(writer.entries))
	}
	entry := writer.entries[0]
	if entry.Result != "FAILURE" {
		t.Errorf("Result = %q, want FAILURE", entry.Result)
	}
	if entry.ActionType != "TOKEN_BURN" {
		t.Errorf("ActionType = %q, want TOKEN_BURN", entry.ActionType)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(entry.Details), &parsed); err != nil {
		t.Fatalf("Details is not valid JSON: %v", err)
	}
	if parsed["reason"] != "quarterly redemption" {
		t.Errorf("the operator's reason is missing from the failed-attempt record: %v", parsed)
	}
	if parsed["error"] == nil || parsed["error"] == "" {
		t.Error("the failure record carries no error detail")
	}
}

// The success path must still be marked SUCCESS after the result argument was
// introduced — a mislabelled record is worse than none.
func TestBurnToken_SuccessIsMarkedSuccess(t *testing.T) {
	t.Parallel()
	writer := &fakeAuditWriter{}
	app, _ := tokenAuditApp(t, writer)

	if resp := postJSON(t, app, "/burn", map[string]any{
		"from": "vault-01", "amount": "1", "reason": "r",
	}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("burn: want 201, got %d", resp.StatusCode)
	}
	if got := writer.entries[0].Result; got != "SUCCESS" {
		t.Errorf("Result = %q, want SUCCESS", got)
	}
}

// hangingAuditWriter accepts the call and then stops answering, which is how an
// unhealthy dependency usually behaves — it rarely has the courtesy to return an error.
type hangingAuditWriter struct {
	reached chan struct{}
	ctxErr  chan error
}

func (h *hangingAuditWriter) CreateAuditLog(ctx context.Context, _ complianceadapter.AuditEntry) error {
	close(h.reached)
	<-ctx.Done() // returns only because the caller bounded the call
	h.ctxErr <- ctx.Err()
	return ctx.Err()
}

// A compliance service that hangs must not hang the mint.
//
// The best-effort test above uses a writer that returns an error immediately, which pins
// a property nobody doubted. This pins the one that bites: c.UserContext() carries no
// deadline and the compliance adapter bounds its dial rather than its calls, so without
// auditWriteTimeout this handler never responds at all. Fiber's WriteTimeout does not
// rescue it either — fasthttp applies that to writing the response, not to the handler.
func TestMintToken_HangingAuditWriterDoesNotHangTheRequest(t *testing.T) {
	t.Parallel()
	w := &hangingAuditWriter{reached: make(chan struct{}), ctxErr: make(chan error, 1)}
	app, _ := tokenAuditApp(t, w)

	type outcome struct {
		status int
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		body, _ := json.Marshal(map[string]any{"to": "alice", "amount": "1"})
		req := httptest.NewRequest(http.MethodPost, "/mint", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		// -1 disables the client-side timeout, so the server is the only thing that can
		// end this request. With an unbounded audit write, nothing does.
		resp, err := app.Test(req, -1)
		if err != nil {
			done <- outcome{err: err}
			return
		}
		done <- outcome{status: resp.StatusCode}
	}()

	select {
	case <-w.reached:
	case <-time.After(5 * time.Second):
		t.Fatal("the audit writer was never called")
	}

	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("mint returned an error: %v", got.err)
		}
		if got.status != http.StatusCreated {
			t.Errorf("mint status = %d, want %d: a stalled audit write must not change the outcome", got.status, http.StatusCreated)
		}
	case <-time.After(auditWriteTimeout + 10*time.Second):
		t.Fatalf("mint did not respond within %v of a hung compliance service: the audit write is unbounded",
			auditWriteTimeout+10*time.Second)
	}

	select {
	case err := <-w.ctxErr:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("audit context ended with %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("the audit call context was never cancelled")
	}
}
