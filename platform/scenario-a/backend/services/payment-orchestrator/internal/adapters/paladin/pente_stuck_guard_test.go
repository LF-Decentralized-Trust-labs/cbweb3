// SPDX-License-Identifier: Apache-2.0

package paladin

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

// A private transaction that cannot complete must not be joined by more of them.
//
// SUP-152 / the stuck-transaction card: three FX proposals were submitted from a commercial bank
// while its central bank node was unreachable. None completed. The two that stayed pending
// re-assembled about 32 times a second on Paladin v1.0.0, held every database connection on the
// node, and flooded the central bank node with ~400 messages a second once it came back. Paladin
// cannot cancel them (upstream LFDT-Paladin/paladin#1356, fixed only after v1.0.0).
//
// What this repository controls is whether it keeps submitting. The orchestrator used to report a
// 30-second receipt timeout as an ordinary failure; the caller retried under a new trade id, and
// each retry became another pending transaction. These tests pin the two changes: a pending Pente
// transaction older than the threshold blocks new submissions from this node, and a timeout is
// reported as "submitted, not confirmed" rather than as a failure to retry.

func pendingTx(id string, age time.Duration) map[string]any {
	return map[string]any{
		"id":       id,
		"domain":   "pente",
		"function": "propose(bytes32,address,address,address,address,uint256,uint256,bytes32,bytes32,uint256,uint256)",
		"created":  time.Now().Add(-age).UTC().Format(time.RFC3339Nano),
	}
}

func TestSendTx_RefusesWhileAPenteTransactionIsStuck(t *testing.T) {
	f := &fakePaladin{receiptSuccess: true, pending: []any{pendingTx("stuck-1", 10*time.Minute)}}
	c, done := newTestClient(t, f)
	defer done()

	_, err := c.Propose(proposeCtx(), sampleParams())
	if !errors.Is(err, ports.ErrPrivacyLayerStuck) {
		t.Fatalf("Propose with a 10-minute-old pending Pente transaction: got %v, want ErrPrivacyLayerStuck", err)
	}
	if !strings.Contains(err.Error(), "stuck-1") {
		t.Errorf("the error must name the pending transaction so the operator can find it, got %q", err)
	}
	if f.called("pgroup_sendTransaction") {
		t.Fatal("a new private transaction was submitted while another one is stuck")
	}
}

// The guard sits in sendTx, so every lifecycle call is covered, not only propose.
func TestSendTx_GuardCoversEveryLifecycleCall(t *testing.T) {
	f := &fakePaladin{receiptSuccess: true, pending: []any{pendingTx("stuck-1", 10*time.Minute)}}
	c, done := newTestClient(t, f)
	defer done()

	var trade [32]byte
	if _, err := c.Accept(proposeCtx(), trade); !errors.Is(err, ports.ErrPrivacyLayerStuck) {
		t.Fatalf("Accept: got %v, want ErrPrivacyLayerStuck", err)
	}
	if _, err := c.Settle(proposeCtx(), trade); !errors.Is(err, ports.ErrPrivacyLayerStuck) {
		t.Fatalf("Settle: got %v, want ErrPrivacyLayerStuck", err)
	}
	if f.called("pgroup_sendTransaction") {
		t.Fatal("a lifecycle transaction was submitted while another one is stuck")
	}
}

// A transaction that is merely in flight is normal. Concurrent proposals must keep working.
func TestSendTx_SubmitsWhenThePendingTransactionIsRecent(t *testing.T) {
	f := &fakePaladin{receiptSuccess: true, pending: []any{pendingTx("in-flight", 5*time.Second)}}
	c, done := newTestClient(t, f)
	defer done()

	if _, err := c.Propose(proposeCtx(), sampleParams()); err != nil {
		t.Fatalf("Propose with a 5-second-old pending transaction: %v", err)
	}
	if !f.called("pgroup_sendTransaction") {
		t.Fatal("the transaction was not submitted")
	}
}

// The query must be the one verified against the Paladin v1.0.0 source: oldest pending Pente
// transaction first, one row, summary form.
func TestSendTx_QueriesOldestPendingPenteTransaction(t *testing.T) {
	f := &fakePaladin{receiptSuccess: true}
	c, done := newTestClient(t, f)
	defer done()

	if _, err := c.Propose(proposeCtx(), sampleParams()); err != nil {
		t.Fatalf("Propose with nothing pending: %v", err)
	}
	f.mu.Lock()
	params := f.pendingParams
	f.mu.Unlock()
	if len(params) != 2 {
		t.Fatalf("ptx_queryPendingTransactions params = %d, want 2 (query, full)", len(params))
	}
	var q struct {
		Eq []struct {
			Field string `json:"field"`
			Value string `json:"value"`
		} `json:"eq"`
		Sort  []string `json:"sort"`
		Limit int      `json:"limit"`
	}
	if err := json.Unmarshal(params[0], &q); err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(q.Eq) != 1 || q.Eq[0].Field != "domain" || q.Eq[0].Value != "pente" {
		t.Errorf("query filter = %+v, want domain == pente", q.Eq)
	}
	if len(q.Sort) != 1 || q.Sort[0] != "created" || q.Limit != 1 {
		t.Errorf("query sort/limit = %v/%d, want [created]/1", q.Sort, q.Limit)
	}
	if string(params[1]) != "false" {
		t.Errorf("full = %s, want false", params[1])
	}
}

// A node that cannot answer a pending-transaction query within seconds is the wedged node of the
// incident. Submitting to it adds to the load; refuse instead.
func TestSendTx_RefusesWhenPaladinDoesNotAnswerTheQuery(t *testing.T) {
	f := &fakePaladin{receiptSuccess: true, pendingDelay: 300 * time.Millisecond}
	c, done := newTestClient(t, f)
	defer done()
	c.guardTimeout = 50 * time.Millisecond

	_, err := c.Propose(proposeCtx(), sampleParams())
	if !errors.Is(err, ports.ErrPrivacyLayerUnavailable) {
		t.Fatalf("got %v, want ErrPrivacyLayerUnavailable", err)
	}
	if f.called("pgroup_sendTransaction") {
		t.Fatal("submitted to a node that could not answer a query")
	}
}

// If Paladin answers but rejects the query itself, the guard cannot judge. It must not turn into
// a refusal of every FX transaction, and it must not be silent either.
func TestSendTx_GuardStepsAsideLoudlyWhenPaladinRejectsTheQuery(t *testing.T) {
	f := &fakePaladin{receiptSuccess: true, pendingErr: &rpcError{Code: -32602, Message: "PD020000: invalid query"}}
	c, done := newTestClient(t, f)
	defer done()
	var logs bytes.Buffer
	c.logger = slog.New(slog.NewTextHandler(&logs, nil))

	if _, err := c.Propose(proposeCtx(), sampleParams()); err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if !f.called("pgroup_sendTransaction") {
		t.Fatal("the transaction was not submitted")
	}
	if !strings.Contains(logs.String(), "level=WARN") || !strings.Contains(logs.String(), "PD020000") {
		t.Errorf("expected a WARN naming the Paladin error, got logs:\n%s", logs.String())
	}
}

// A receipt that does not arrive in time is not a failure: the transaction is still pending in
// Paladin, and a caller that resubmits creates another one.
func TestWaitReceipt_TimeoutIsReportedAsUnconfirmed(t *testing.T) {
	f := &fakePaladin{noReceipt: true}
	c, done := newTestClient(t, f)
	defer done()
	c.receiptTimeout = 20 * time.Millisecond

	_, err := c.Propose(proposeCtx(), sampleParams())
	if !errors.Is(err, ports.ErrPrivateTxUnconfirmed) {
		t.Fatalf("got %v, want ErrPrivateTxUnconfirmed", err)
	}
	if !strings.Contains(err.Error(), "tx-uuid-123") {
		t.Errorf("the error must carry the Paladin transaction id, got %q", err)
	}
}

func TestStuckThreshold_DefaultsAndOverride(t *testing.T) {
	if got := NewPenteClient(PenteClientConfig{BaseURL: "http://x"}).stuckAfter; got != DefaultStuckPendingAfter {
		t.Errorf("default stuckAfter = %s, want %s", got, DefaultStuckPendingAfter)
	}
	if got := NewPenteClient(PenteClientConfig{BaseURL: "http://x", StuckPendingAfter: time.Minute}).stuckAfter; got != time.Minute {
		t.Errorf("override stuckAfter = %s, want 1m", got)
	}
}
