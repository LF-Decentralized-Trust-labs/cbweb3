// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"errors"
	"sync"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

// fakeEscrowRepo is an in-memory EscrowRepository.
type fakeEscrowRepo struct {
	mu       sync.Mutex
	deposits map[string]domain.DepositRecord
	redeems  map[string]domain.RedeemRecord
	escrows  map[string]domain.EscrowRecord
	failGet  bool
}

func newFakeEscrowRepo() *fakeEscrowRepo {
	return &fakeEscrowRepo{
		deposits: map[string]domain.DepositRecord{},
		redeems:  map[string]domain.RedeemRecord{},
		escrows:  map[string]domain.EscrowRecord{},
	}
}

func (r *fakeEscrowRepo) CreateDeposit(_ context.Context, rec domain.DepositRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deposits[rec.ID] = rec
	return nil
}
func (r *fakeEscrowRepo) GetDeposit(_ context.Context, id string) (domain.DepositRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failGet {
		return domain.DepositRecord{}, false, errors.New("db error")
	}
	rec, ok := r.deposits[id]
	return rec, ok, nil
}
func (r *fakeEscrowRepo) UpdateDeposit(_ context.Context, rec domain.DepositRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deposits[rec.ID] = rec
	return nil
}
func (r *fakeEscrowRepo) ListDeposits(_ context.Context, requesterID string) ([]domain.DepositRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.DepositRecord
	for _, rec := range r.deposits {
		if requesterID == "" || rec.RequesterID == requesterID {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (r *fakeEscrowRepo) CreateRedeem(_ context.Context, rec domain.RedeemRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.redeems[rec.ID] = rec
	return nil
}
func (r *fakeEscrowRepo) GetRedeem(_ context.Context, id string) (domain.RedeemRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.redeems[id]
	return rec, ok, nil
}
func (r *fakeEscrowRepo) UpdateRedeem(_ context.Context, rec domain.RedeemRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.redeems[rec.ID] = rec
	return nil
}
func (r *fakeEscrowRepo) ListRedeems(_ context.Context, requesterID string) ([]domain.RedeemRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.RedeemRecord
	for _, rec := range r.redeems {
		if requesterID == "" || rec.RequesterID == requesterID {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (r *fakeEscrowRepo) CreateEscrow(_ context.Context, rec domain.EscrowRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.escrows[rec.ID] = rec
	return nil
}
func (r *fakeEscrowRepo) GetEscrow(_ context.Context, id string) (domain.EscrowRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.escrows[id]
	return rec, ok, nil
}
func (r *fakeEscrowRepo) UpdateEscrow(_ context.Context, rec domain.EscrowRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.escrows[rec.ID] = rec
	return nil
}
func (r *fakeEscrowRepo) ListEscrows(_ context.Context, requesterID string) ([]domain.EscrowRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.EscrowRecord
	for _, rec := range r.escrows {
		if requesterID == "" || rec.RequesterID == requesterID {
			out = append(out, rec)
		}
	}
	return out, nil
}

var _ ports.EscrowRepository = (*fakeEscrowRepo)(nil)

// errFiat is a FiatTokenPort whose Mint and Burn always fail — used to exercise
// the mint/burn failure paths (the shared mockFiat always succeeds on write ops).
type errFiat struct{ err error }

func (f errFiat) Decimals(context.Context) (uint8, error)              { return 18, nil }
func (f errFiat) Symbol(context.Context) (string, error)               { return "fCeBM_BRL", nil }
func (f errFiat) Mint(context.Context, string, string) (string, error) { return "", f.err }
func (f errFiat) Burn(context.Context, string, string) (string, error) { return "", f.err }
func (f errFiat) BalanceOf(context.Context, string) (string, error)    { return "0", nil }
func (f errFiat) GetFiatBalance(context.Context) (string, error)       { return "0", nil }

var _ ports.FiatTokenPort = errFiat{}

// fakeFXRepo is an in-memory FXAgreementRepository.
type fakeFXRepo struct {
	mu        sync.Mutex
	agrs      map[string]*domain.FXAgreementRecord
	events    map[string][]*domain.FXAgreementEvent
	createErr error
}

func newFakeFXRepo() *fakeFXRepo {
	return &fakeFXRepo{
		agrs:   map[string]*domain.FXAgreementRecord{},
		events: map[string][]*domain.FXAgreementEvent{},
	}
}

func (r *fakeFXRepo) CreateAgreement(_ context.Context, rec *domain.FXAgreementRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	cp := *rec
	r.agrs[rec.TradeID] = &cp
	return nil
}
func (r *fakeFXRepo) GetAgreement(_ context.Context, tradeID string) (*domain.FXAgreementRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.agrs[tradeID]
	if !ok {
		return nil, nil
	}
	cp := *rec
	return &cp, nil
}
func (r *fakeFXRepo) UpdateAgreement(_ context.Context, rec *domain.FXAgreementRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *rec
	r.agrs[rec.TradeID] = &cp
	return nil
}
func (r *fakeFXRepo) ListAgreements(_ context.Context, f ports.FXAgreementFilter) ([]*domain.FXAgreementRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.FXAgreementRecord
	for _, rec := range r.agrs {
		if f.Counterparty != "" && rec.CounterpartyB != f.Counterparty && rec.Originator != f.Counterparty {
			continue
		}
		if f.State != "" && rec.State != f.State {
			continue
		}
		cp := *rec
		out = append(out, &cp)
	}
	return out, nil
}
func (r *fakeFXRepo) CreateAuditEvent(_ context.Context, e *domain.FXAgreementEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events[e.TradeID] = append(r.events[e.TradeID], e)
	return nil
}
func (r *fakeFXRepo) ListAuditEvents(_ context.Context, tradeID string) ([]*domain.FXAgreementEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.events[tradeID], nil
}
func (r *fakeFXRepo) ListExpiredNonTerminal(context.Context, int64) ([]*domain.FXAgreementRecord, error) {
	return nil, nil
}

var _ ports.FXAgreementRepository = (*fakeFXRepo)(nil)

// fakeFXContract implements ports.FXAgreementContractPort, recording which method
// was invoked and returning a canned tx hash (or a configured error).
type fakeFXContract struct {
	err    error
	called []string
}

func (c *fakeFXContract) record(name string) (string, error) {
	c.called = append(c.called, name)
	if c.err != nil {
		return "", c.err
	}
	return "0xchain-" + name, nil
}
func (c *fakeFXContract) Propose(context.Context, ports.FXProposalParams) (string, error) {
	return c.record("Propose")
}
func (c *fakeFXContract) ProposeOnBehalf(context.Context, ports.FXProposalParams) (string, error) {
	return c.record("ProposeOnBehalf")
}
func (c *fakeFXContract) Accept(context.Context, [32]byte) (string, error) { return c.record("Accept") }
func (c *fakeFXContract) AcceptOnBehalf(context.Context, [32]byte) (string, error) {
	return c.record("AcceptOnBehalf")
}
func (c *fakeFXContract) Reject(context.Context, [32]byte) (string, error) { return c.record("Reject") }
func (c *fakeFXContract) RejectOnBehalf(context.Context, [32]byte) (string, error) {
	return c.record("RejectOnBehalf")
}
func (c *fakeFXContract) Cancel(context.Context, [32]byte) (string, error) { return c.record("Cancel") }
func (c *fakeFXContract) Settle(context.Context, [32]byte) (string, error) { return c.record("Settle") }

var _ ports.FXAgreementContractPort = (*fakeFXContract)(nil)
