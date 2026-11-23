// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

// An FX operation whose private transaction was submitted but not confirmed is recorded as
// pending. On Paladin v1.0.0 it may commit much later, or never. These tests pin how the
// reconciler brings the record back in line with the chain.

type pendingStore struct {
	mu      sync.Mutex
	records map[string]*domain.FXAgreementRecord
	events  []*domain.FXAgreementEvent
}

func newPendingStore(recs ...*domain.FXAgreementRecord) *pendingStore {
	s := &pendingStore{records: map[string]*domain.FXAgreementRecord{}}
	for _, r := range recs {
		cp := *r
		s.records[r.TradeID] = &cp
	}
	return s
}

func (s *pendingStore) ListPendingAgreements(context.Context, int) ([]*domain.FXAgreementRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*domain.FXAgreementRecord
	for _, r := range s.records {
		if r.Pending.IsSet() {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *pendingStore) UpdateAgreement(_ context.Context, r *domain.FXAgreementRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *r
	s.records[r.TradeID] = &cp
	return nil
}

func (s *pendingStore) CreateAuditEvent(_ context.Context, e *domain.FXAgreementEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	return nil
}

func (s *pendingStore) get(id string) domain.FXAgreementRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.records[id]
}

// pendingChain answers receipt outcomes by Paladin transaction id and agreement states by
// on-chain trade id, and records the group each read was bound to.
type pendingChain struct {
	outcomes   map[string]ports.PrivateTxOutcome
	outcomeErr error
	states     map[string]string // 0x bytes32 trade id -> FX_STATE_*
	readErr    error
	targets    []ports.PenteFXTarget
}

func (c *pendingChain) PrivateTxOutcome(_ context.Context, txID string) (ports.PrivateTxOutcome, error) {
	if c.outcomeErr != nil {
		return ports.PrivateTxPending, c.outcomeErr
	}
	return c.outcomes[txID], nil
}

func (c *pendingChain) GetFXAgreement(ctx context.Context, tradeID string) (*ports.PenteFXAgreementState, error) {
	if t, ok := ports.PenteFXTargetFromContext(ctx); ok {
		c.targets = append(c.targets, t)
	}
	if c.readErr != nil {
		return nil, c.readErr
	}
	st, ok := c.states[tradeID]
	if !ok {
		return nil, nil
	}
	return &ports.PenteFXAgreementState{TradeID: tradeID, State: st}, nil
}

func chainID(tradeID string) string {
	b := domain.OnChainTradeID(tradeID)
	return "0x" + hex.EncodeToString(b[:])
}

func pendingRec(id string, st domain.FXState, op domain.FXPendingOp, txID string, since time.Time) *domain.FXAgreementRecord {
	return &domain.FXAgreementRecord{
		TradeID: id, State: st, GroupID: "grp-1", ContractAddress: "0xfxa",
		Pending: domain.FXPending{Op: op, TxID: txID, Since: since},
	}
}

func newTestReconciler(store PendingFXStore, chain ports.FXPendingChainPort, logs *bytes.Buffer) *FXPendingReconciler {
	var logger *slog.Logger
	if logs != nil {
		logger = slog.New(slog.NewTextHandler(logs, nil))
	}
	return NewFXPendingReconciler(store, chain, time.Second, 10*time.Minute, logger)
}

func TestReconciler_ConfirmedProposeBecomesProposed(t *testing.T) {
	store := newPendingStore(pendingRec("T1", domain.FXStateInvalid, domain.FXPendingPropose, "ptx-1", time.Now()))
	chain := &pendingChain{
		outcomes: map[string]ports.PrivateTxOutcome{"ptx-1": ports.PrivateTxSucceeded},
		states:   map[string]string{chainID("T1"): "FX_STATE_PROPOSED"},
	}
	newTestReconciler(store, chain, nil).RunOnce(context.Background())

	got := store.get("T1")
	if got.State != domain.FXStateProposed || got.Pending.IsSet() {
		t.Fatalf("after a confirmed propose: state=%s pending=%+v, want PROPOSED and nothing pending", got.State, got.Pending)
	}
	if len(store.events) != 1 || store.events[0].FromState != domain.FXStateInvalid || store.events[0].ToState != domain.FXStateProposed ||
		store.events[0].Source != domain.EventSourceSystemJob || store.events[0].TxHash != "ptx-1" {
		t.Errorf("audit events = %+v", store.events)
	}
	if len(chain.targets) != 1 || chain.targets[0].GroupID != "grp-1" || chain.targets[0].ContractAddress != "0xfxa" {
		t.Errorf("the chain read was not bound to the agreement's group: %+v", chain.targets)
	}
}

// A lifecycle operation that committed later: the record follows the chain, so a following
// settle is no longer refused because the database still says PROPOSED.
func TestReconciler_ConfirmedAcceptAdvancesTheRecord(t *testing.T) {
	store := newPendingStore(pendingRec("T1", domain.FXStateProposed, domain.FXPendingAccept, "ptx-2", time.Now()))
	chain := &pendingChain{
		outcomes: map[string]ports.PrivateTxOutcome{"ptx-2": ports.PrivateTxSucceeded},
		states:   map[string]string{chainID("T1"): "FX_STATE_ACCEPTED"},
	}
	newTestReconciler(store, chain, nil).RunOnce(context.Background())

	if got := store.get("T1"); got.State != domain.FXStateAccepted || got.Pending.IsSet() {
		t.Fatalf("state=%s pending=%+v, want ACCEPTED and nothing pending", got.State, got.Pending)
	}
}

// A failed receipt means the operation did not happen: the state stays, and the agreement can
// be operated on again.
func TestReconciler_FailedOperationClearsWithoutChangingState(t *testing.T) {
	store := newPendingStore(pendingRec("T1", domain.FXStateProposed, domain.FXPendingAccept, "ptx-3", time.Now()))
	chain := &pendingChain{outcomes: map[string]ports.PrivateTxOutcome{"ptx-3": ports.PrivateTxFailed}}
	newTestReconciler(store, chain, nil).RunOnce(context.Background())

	got := store.get("T1")
	if got.State != domain.FXStateProposed || got.Pending.IsSet() {
		t.Fatalf("state=%s pending=%+v, want PROPOSED and nothing pending", got.State, got.Pending)
	}
	if len(store.events) != 1 || !strings.Contains(store.events[0].Notes, "failed") {
		t.Errorf("expected one audit event noting the failure, got %+v", store.events)
	}
}

func TestReconciler_StillPendingIsLeftAloneAndReportedWhenOld(t *testing.T) {
	old := time.Now().Add(-time.Hour)
	store := newPendingStore(pendingRec("T1", domain.FXStateProposed, domain.FXPendingAccept, "ptx-4", old))
	chain := &pendingChain{outcomes: map[string]ports.PrivateTxOutcome{}} // no receipt yet
	var logs bytes.Buffer
	w := newTestReconciler(store, chain, &logs)
	w.RunOnce(context.Background())
	w.RunOnce(context.Background())

	if got := store.get("T1"); !got.Pending.IsSet() || got.State != domain.FXStateProposed {
		t.Fatalf("a still-pending operation was changed: state=%s pending=%+v", got.State, got.Pending)
	}
	if n := strings.Count(logs.String(), "pending for a long time"); n != 1 {
		t.Errorf("long-pending warning logged %d times over two cycles, want once per warn interval:\n%s", n, logs.String())
	}
}

// With no transaction id (the submission got no answer), only the chain can tell.
func TestReconciler_UnknownTxIDResolvesFromTheChain(t *testing.T) {
	store := newPendingStore(
		pendingRec("ON", domain.FXStateInvalid, domain.FXPendingPropose, "", time.Now()),
		pendingRec("OFF", domain.FXStateInvalid, domain.FXPendingPropose, "", time.Now()),
	)
	chain := &pendingChain{states: map[string]string{chainID("ON"): "FX_STATE_PROPOSED"}}
	newTestReconciler(store, chain, nil).RunOnce(context.Background())

	if got := store.get("ON"); got.State != domain.FXStateProposed || got.Pending.IsSet() {
		t.Errorf("ON: state=%s pending=%+v, want PROPOSED and nothing pending", got.State, got.Pending)
	}
	if got := store.get("OFF"); got.State != domain.FXStateInvalid || !got.Pending.IsSet() {
		t.Errorf("OFF: state=%s pending=%+v, want unchanged", got.State, got.Pending)
	}
}

// Read failures change nothing; the next cycle retries.
func TestReconciler_ReadFailuresChangeNothing(t *testing.T) {
	for name, chain := range map[string]*pendingChain{
		"outcome read fails":   {outcomeErr: errors.New("connection refused")},
		"agreement read fails": {outcomes: map[string]ports.PrivateTxOutcome{"ptx-5": ports.PrivateTxSucceeded}, readErr: errors.New("timeout")},
	} {
		t.Run(name, func(t *testing.T) {
			store := newPendingStore(pendingRec("T1", domain.FXStateProposed, domain.FXPendingAccept, "ptx-5", time.Now()))
			newTestReconciler(store, chain, nil).RunOnce(context.Background())
			if got := store.get("T1"); !got.Pending.IsSet() || got.State != domain.FXStateProposed {
				t.Fatalf("state=%s pending=%+v, want unchanged", got.State, got.Pending)
			}
		})
	}
}

// A terminal record is never overwritten; its pending operation is cleared, loudly.
func TestReconciler_TerminalRecordIsNotOverwritten(t *testing.T) {
	store := newPendingStore(pendingRec("T1", domain.FXStateCancelled, domain.FXPendingAccept, "ptx-6", time.Now()))
	chain := &pendingChain{
		outcomes: map[string]ports.PrivateTxOutcome{"ptx-6": ports.PrivateTxSucceeded},
		states:   map[string]string{chainID("T1"): "FX_STATE_ACCEPTED"},
	}
	var logs bytes.Buffer
	newTestReconciler(store, chain, &logs).RunOnce(context.Background())

	if got := store.get("T1"); got.State != domain.FXStateCancelled || got.Pending.IsSet() {
		t.Fatalf("state=%s pending=%+v, want CANCELLED and nothing pending", got.State, got.Pending)
	}
	if !strings.Contains(logs.String(), "level=WARN") {
		t.Errorf("expected a WARN, got:\n%s", logs.String())
	}
}
