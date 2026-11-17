// SPDX-License-Identifier: Apache-2.0

package paladin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

// A private transaction that cannot complete must not be joined by more of them.
//
// A Pente transaction stayed pending for days in production: FX proposals were submitted from a
// node while a peer node was unreachable, and the ones that could not complete re-assembled in a
// tight loop on Paladin v1.0.0, held every database connection on the node, and flooded the peer
// once it came back. Paladin cannot cancel them (upstream LFDT-Paladin/paladin#1356, fixed only
// after v1.0.0).
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

// The guard runs in Propose, ProposeOnBehalf and every trade-id lifecycle call.
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

// The guard must run before any other node RPC: resolving party addresses alone is up to five
// calls to the node the guard is protecting.
func TestPropose_GuardRunsBeforeResolvingParties(t *testing.T) {
	f := &fakePaladin{receiptSuccess: true, pending: []any{pendingTx("stuck-1", 10*time.Minute)}}
	c, done := newTestClient(t, f)
	defer done()

	params := sampleParams()
	params.Routing.CounterpartyID = "funded_operator@spoke-brl-bank-itau"
	params.Routing.SettlementAgentID = "funded_operator@spoke-brl-cb"
	for _, propose := range []func(context.Context, ports.FXProposalParams) (string, error){c.Propose, c.ProposeOnBehalf} {
		if _, err := propose(proposeCtx(), params); !errors.Is(err, ports.ErrPrivacyLayerStuck) {
			t.Fatalf("got %v, want ErrPrivacyLayerStuck", err)
		}
	}
	if got := f.firstMethod(); got != "ptx_queryPendingTransactions" {
		t.Errorf("first RPC = %q, want the guard's ptx_queryPendingTransactions", got)
	}
	if f.called("ptx_resolveVerifier") {
		t.Error("party addresses were resolved against a node with a stuck transaction")
	}
}

// One query answers for every submission within the cache window, including concurrent ones.
func TestGuard_SharesOneQueryAcrossSubmissions(t *testing.T) {
	f := &fakePaladin{receiptSuccess: true, pendingDelay: 50 * time.Millisecond}
	c, done := newTestClient(t, f)
	defer done()

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := c.Propose(proposeCtx(), sampleParams())
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Propose: %v", err)
		}
	}
	if _, err := c.Accept(proposeCtx(), [32]byte{}); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if n := f.count("ptx_queryPendingTransactions"); n != 1 {
		t.Errorf("pending-transaction queries = %d for 9 submissions in the cache window, want 1", n)
	}
	if n := f.count("pgroup_sendTransaction"); n != 9 {
		t.Errorf("submissions = %d, want 9", n)
	}
}

// After the cache window the node is asked again: a transaction that became stuck is noticed.
func TestGuard_AsksAgainAfterTheCacheWindow(t *testing.T) {
	f := &fakePaladin{receiptSuccess: true}
	c, done := newTestClient(t, f)
	defer done()
	c.guardCacheTTL = 20 * time.Millisecond

	if _, err := c.Propose(proposeCtx(), sampleParams()); err != nil {
		t.Fatalf("first Propose: %v", err)
	}
	f.mu.Lock()
	f.pending = []any{pendingTx("stuck-1", 10*time.Minute)}
	f.mu.Unlock()
	time.Sleep(30 * time.Millisecond)
	if _, err := c.Propose(proposeCtx(), sampleParams()); !errors.Is(err, ports.ErrPrivacyLayerStuck) {
		t.Fatalf("second Propose after the window: got %v, want ErrPrivacyLayerStuck", err)
	}
}

// One caller giving up must not fail the shared check for the others.
func TestGuard_CallerCancellationDoesNotPoisonTheSharedCheck(t *testing.T) {
	f := &fakePaladin{receiptSuccess: true, pendingDelay: 80 * time.Millisecond}
	c, done := newTestClient(t, f)
	defer done()

	ctx, cancel := context.WithCancel(proposeCtx())
	first := make(chan error, 1)
	go func() { first <- c.ensureNoStuckPending(ctx) }()
	time.Sleep(10 * time.Millisecond)
	second := make(chan error, 1)
	go func() { second <- c.ensureNoStuckPending(proposeCtx()) }()
	time.Sleep(10 * time.Millisecond)
	cancel()

	if err := <-first; err != nil {
		t.Fatalf("the check ran detached from the first caller, so it should still answer: %v", err)
	}
	if err := <-second; err != nil {
		t.Fatalf("the waiting caller got %v, want the shared answer (nil)", err)
	}
}

// A node that answered, with a body the guard cannot read, is alive. Refusing every FX call on it
// because of a response-format change would be a node-wide outage caused by the guard itself.
func TestGuard_StepsAsideLoudlyOnAnUnreadableAnswer(t *testing.T) {
	for name, raw := range map[string]string{
		"proxy error page":   "<html><body>502 Bad Gateway</body></html>",
		"changed field type": `{"jsonrpc":"2.0","id":1,"result":[{"id":"x","created":12345}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			f := &fakePaladin{receiptSuccess: true, pendingRaw: raw}
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
			if !strings.Contains(logs.String(), "level=WARN") || !strings.Contains(logs.String(), "stuck-transaction guard skipped") {
				t.Errorf("expected a WARN that the guard stepped aside, got logs:\n%s", logs.String())
			}
		})
	}
}

// A node that cannot be reached at all is refused, and nothing is submitted.
func TestGuard_RefusesWhenPaladinCannotBeReached(t *testing.T) {
	c := NewPenteClient(PenteClientConfig{BaseURL: closedURL(t), FXAgreementAddress: "0xC0nTract", Identity: "op@node"})
	if _, err := c.Propose(proposeCtx(), sampleParams()); !errors.Is(err, ports.ErrPrivacyLayerUnavailable) {
		t.Fatalf("got %v, want ErrPrivacyLayerUnavailable", err)
	}
}

// A submission that never reached Paladin is known not to have been accepted.
func TestSendTx_UndeliveredSubmissionIsUnavailable(t *testing.T) {
	f := &fakePaladin{receiptSuccess: true}
	c, done := newTestClient(t, f)
	defer done()
	if err := c.ensureNoStuckPending(proposeCtx()); err != nil { // the guard's answer is now cached
		t.Fatal(err)
	}
	c.rpcURL = closedURL(t)

	_, err := c.Propose(proposeCtx(), sampleParams())
	if !errors.Is(err, ports.ErrPrivacyLayerUnavailable) || errors.Is(err, ports.ErrPrivateTxUnconfirmed) {
		t.Fatalf("got %v, want ErrPrivacyLayerUnavailable only", err)
	}
}

// A submission that may have reached Paladin, but got no usable answer, is not retry-safe.
func TestSendTx_AmbiguousSubmissionIsUnconfirmed(t *testing.T) {
	cases := map[string]func(f *fakePaladin, c *PenteClient){
		"unreadable answer": func(f *fakePaladin, _ *PenteClient) { f.sendRaw = "<html>502</html>" },
		"no answer in time": func(f *fakePaladin, c *PenteClient) {
			f.sendDelay = 200 * time.Millisecond
			c.httpClient.Timeout = 50 * time.Millisecond
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := &fakePaladin{receiptSuccess: true}
			c, done := newTestClient(t, f)
			defer done()
			setup(f, c)

			_, err := c.Propose(proposeCtx(), sampleParams())
			if !errors.Is(err, ports.ErrPrivateTxUnconfirmed) {
				t.Fatalf("got %v, want ErrPrivateTxUnconfirmed", err)
			}
		})
	}
}

// Paladin refusing the submission with a JSON-RPC error is a definite answer: nothing was
// accepted, so it stays an ordinary error.
func TestSendTx_RefusedSubmissionStaysAnOrdinaryError(t *testing.T) {
	f := &fakePaladin{receiptSuccess: true, sendErr: &rpcError{Code: -32000, Message: "PD011000: invalid input"}}
	c, done := newTestClient(t, f)
	defer done()

	_, err := c.Propose(proposeCtx(), sampleParams())
	if err == nil || errors.Is(err, ports.ErrPrivateTxUnconfirmed) || errors.Is(err, ports.ErrPrivacyLayerUnavailable) {
		t.Fatalf("got %v, want an ordinary error", err)
	}
}

// Once Paladin returned a transaction id, every exit other than a receipt is unconfirmed and
// carries the id: the transaction exists and may still complete.
func TestWaitReceipt_EveryExitAfterSubmissionIsUnconfirmed(t *testing.T) {
	t.Run("receipt query fails", func(t *testing.T) {
		f := &fakePaladin{receiptErr: &rpcError{Code: -32000, Message: "PD012000: database busy"}}
		c, done := newTestClient(t, f)
		defer done()
		_, err := c.Propose(proposeCtx(), sampleParams())
		if !errors.Is(err, ports.ErrPrivateTxUnconfirmed) || !strings.Contains(err.Error(), "tx-uuid-123") {
			t.Fatalf("got %v, want ErrPrivateTxUnconfirmed naming tx-uuid-123", err)
		}
	})
	t.Run("caller's context ends", func(t *testing.T) {
		f := &fakePaladin{noReceipt: true}
		c, done := newTestClient(t, f)
		defer done()
		ctx, cancel := context.WithTimeout(proposeCtx(), 100*time.Millisecond)
		defer cancel()
		_, err := c.Propose(ctx, sampleParams())
		if !errors.Is(err, ports.ErrPrivateTxUnconfirmed) || !strings.Contains(err.Error(), "tx-uuid-123") {
			t.Fatalf("got %v, want ErrPrivateTxUnconfirmed naming tx-uuid-123", err)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("the context error must stay inspectable, got %v", err)
		}
	})
}

// closedURL is an address where nothing listens, so a dial fails before any request is sent.
func closedURL(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return "http://" + addr
}

func TestStuckThreshold_DefaultsAndOverride(t *testing.T) {
	if got := NewPenteClient(PenteClientConfig{BaseURL: "http://x"}).stuckAfter; got != DefaultStuckPendingAfter {
		t.Errorf("default stuckAfter = %s, want %s", got, DefaultStuckPendingAfter)
	}
	if got := NewPenteClient(PenteClientConfig{BaseURL: "http://x", StuckPendingAfter: time.Minute}).stuckAfter; got != time.Minute {
		t.Errorf("override stuckAfter = %s, want 1m", got)
	}
}
