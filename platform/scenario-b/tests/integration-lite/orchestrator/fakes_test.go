// SPDX-License-Identifier: Apache-2.0

//go:build integration_lite

package integrationlite

import (
	"context"
	"sync"

	podomain "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

type tokenOp struct {
	addr   string
	amount string
}

// spyToken implements ports.TCeBMPort, recording mint/burn calls.
type spyToken struct {
	mu      sync.Mutex
	balance string
	mints   []tokenOp
	burns   []tokenOp
	mintErr error
	burnErr error
}

func (m *spyToken) Mint(_ context.Context, to, amount string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.mintErr != nil {
		return "", m.mintErr
	}
	m.mints = append(m.mints, tokenOp{to, amount})
	return "0xtoken-mint", nil
}
func (m *spyToken) Burn(_ context.Context, from, amount string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.burnErr != nil {
		return "", m.burnErr
	}
	m.burns = append(m.burns, tokenOp{from, amount})
	return "0xtoken-burn", nil
}
func (m *spyToken) BalanceOf(context.Context, string) (string, error) { return m.balance, nil }
func (m *spyToken) GetBalance(context.Context) (string, error)        { return m.balance, nil }
func (m *spyToken) Decimals(context.Context) (uint8, error)           { return 18, nil }
func (m *spyToken) Symbol(context.Context) (string, error)            { return "tCeBM_BRL", nil }

func (m *spyToken) mintCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.mints)
}

var _ ports.TCeBMPort = (*spyToken)(nil)

// spyFiat implements ports.FiatTokenPort, recording mint/burn calls.
type spyFiat struct {
	mu      sync.Mutex
	balance string
	mints   []tokenOp
	burns   []tokenOp
	mintErr error
	burnErr error
}

func (m *spyFiat) Mint(_ context.Context, to, amount string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.mintErr != nil {
		return "", m.mintErr
	}
	m.mints = append(m.mints, tokenOp{to, amount})
	return "0xfiat-mint", nil
}
func (m *spyFiat) Burn(_ context.Context, from, amount string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.burnErr != nil {
		return "", m.burnErr
	}
	m.burns = append(m.burns, tokenOp{from, amount})
	return "0xfiat-burn", nil
}
func (m *spyFiat) BalanceOf(context.Context, string) (string, error) { return m.balance, nil }
func (m *spyFiat) GetFiatBalance(context.Context) (string, error)    { return m.balance, nil }
func (m *spyFiat) Decimals(context.Context) (uint8, error)           { return 18, nil }
func (m *spyFiat) Symbol(context.Context) (string, error)            { return "fCeBM_BRL", nil }

func (m *spyFiat) burnCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.burns)
}

var _ ports.FiatTokenPort = (*spyFiat)(nil)

// spyRelay implements ports.InteroperabilityPort, recording relayed proofs.
type spyRelay struct {
	mu      sync.Mutex
	relayed []ports.InteroperabilityProof
}

func (r *spyRelay) SubscribeLockEvents(context.Context, func(ports.InteroperabilityProof) error) error {
	return nil
}
func (r *spyRelay) SubscribeSettleEvents(context.Context, func(ports.InteroperabilityProof) error) error {
	return nil
}
func (r *spyRelay) RelayProof(_ context.Context, p ports.InteroperabilityProof) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.relayed = append(r.relayed, p)
	return "0xrelay-tx", nil
}
func (r *spyRelay) VerifyProof(context.Context, ports.InteroperabilityProof) (bool, error) {
	return true, nil
}

var _ ports.InteroperabilityPort = (*spyRelay)(nil)

// inMemEscrowRepo implements ports.EscrowRepository (deposits/redeems/escrows).
type inMemEscrowRepo struct {
	mu       sync.Mutex
	deposits map[string]podomain.DepositRecord
	redeems  map[string]podomain.RedeemRecord
	escrows  map[string]podomain.EscrowRecord
}

func newInMemEscrowRepo() *inMemEscrowRepo {
	return &inMemEscrowRepo{
		deposits: map[string]podomain.DepositRecord{},
		redeems:  map[string]podomain.RedeemRecord{},
		escrows:  map[string]podomain.EscrowRecord{},
	}
}

func (r *inMemEscrowRepo) CreateDeposit(_ context.Context, rec podomain.DepositRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deposits[rec.ID] = rec
	return nil
}
func (r *inMemEscrowRepo) GetDeposit(_ context.Context, id string) (podomain.DepositRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.deposits[id]
	return rec, ok, nil
}
func (r *inMemEscrowRepo) UpdateDeposit(_ context.Context, rec podomain.DepositRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deposits[rec.ID] = rec
	return nil
}
func (r *inMemEscrowRepo) ListDeposits(_ context.Context, requesterID string) ([]podomain.DepositRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []podomain.DepositRecord
	for _, rec := range r.deposits {
		if requesterID == "" || rec.RequesterID == requesterID {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (r *inMemEscrowRepo) CreateRedeem(_ context.Context, rec podomain.RedeemRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.redeems[rec.ID] = rec
	return nil
}
func (r *inMemEscrowRepo) GetRedeem(_ context.Context, id string) (podomain.RedeemRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.redeems[id]
	return rec, ok, nil
}
func (r *inMemEscrowRepo) UpdateRedeem(_ context.Context, rec podomain.RedeemRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.redeems[rec.ID] = rec
	return nil
}
func (r *inMemEscrowRepo) ListRedeems(_ context.Context, requesterID string) ([]podomain.RedeemRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []podomain.RedeemRecord
	for _, rec := range r.redeems {
		if requesterID == "" || rec.RequesterID == requesterID {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (r *inMemEscrowRepo) CreateEscrow(_ context.Context, rec podomain.EscrowRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.escrows[rec.ID] = rec
	return nil
}
func (r *inMemEscrowRepo) GetEscrow(_ context.Context, id string) (podomain.EscrowRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.escrows[id]
	return rec, ok, nil
}
func (r *inMemEscrowRepo) UpdateEscrow(_ context.Context, rec podomain.EscrowRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.escrows[rec.ID] = rec
	return nil
}
func (r *inMemEscrowRepo) ListEscrows(_ context.Context, requesterID string) ([]podomain.EscrowRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []podomain.EscrowRecord
	for _, rec := range r.escrows {
		if requesterID == "" || rec.RequesterID == requesterID {
			out = append(out, rec)
		}
	}
	return out, nil
}

var _ ports.EscrowRepository = (*inMemEscrowRepo)(nil)
