// SPDX-License-Identifier: Apache-2.0

//go:build integration_lite

package integration

import (
	"context"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	compliancev1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/compliance/v1"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ── fakeZeto: in-memory ZetoOperator (private-token side-effects) ──────────────

type fakeZeto struct {
	mu                   sync.Mutex
	mintCalled           int
	transferCalled       int
	lockCalled           int
	unlockCalled         int
	transferLockedCalled int

	lockErr           error
	unlockErr         error
	transferLockedErr error
}

func (z *fakeZeto) Mint(context.Context, string, string) (string, error) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.mintCalled++
	return "fake-mint-tx", nil
}
func (z *fakeZeto) Burn(context.Context, string, string) (string, error) {
	return "fake-burn-tx", nil
}
func (z *fakeZeto) Transfer(context.Context, string, string) (string, error) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.transferCalled++
	return "fake-transfer-tx", nil
}
func (z *fakeZeto) Lock(context.Context, string, string) (*ports.ZetoLockResult, error) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.lockCalled++
	if z.lockErr != nil {
		return nil, z.lockErr
	}
	return &ports.ZetoLockResult{
		TxHash:         "fake-lock-tx",
		ZetoLockRef:    "fake-lock-ref",
		LockedStateIDs: []string{"0xstate1"},
	}, nil
}
func (z *fakeZeto) Unlock(context.Context, string) (string, error) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.unlockCalled++
	if z.unlockErr != nil {
		return "", z.unlockErr
	}
	return "fake-unlock-tx", nil
}
func (z *fakeZeto) TransferLocked(context.Context, string, string, string) (string, error) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.transferLockedCalled++
	if z.transferLockedErr != nil {
		return "", z.transferLockedErr
	}
	return "fake-transfer-locked-tx", nil
}
func (z *fakeZeto) Balance(context.Context) (string, error) { return "1000000", nil }
func (z *fakeZeto) ResolveIdentity(_ context.Context, id string) (string, error) {
	if id == "" {
		return "", nil
	}
	return "0x1111111111111111111111111111111111111111", nil
}

// counters reads the side-effect counters under lock.
func (z *fakeZeto) counters() (mint, transfer, lock, unlock, transferLocked int) {
	z.mu.Lock()
	defer z.mu.Unlock()
	return z.mintCalled, z.transferCalled, z.lockCalled, z.unlockCalled, z.transferLockedCalled
}

// ── fakeHTLC: in-memory on-chain HTLC coordination port ────────────────────────

type fakeHTLC struct {
	mu           sync.Mutex
	lockCalled   int
	settleCalled int
	refundCalled int
}

func (h *fakeHTLC) Lock(context.Context, ports.HTLCLockParams) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lockCalled++
	return "fake-htlc-lock-tx", nil
}
func (h *fakeHTLC) Settle(context.Context, [32]byte, [32]byte) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.settleCalled++
	return "fake-htlc-settle-tx", nil
}
func (h *fakeHTLC) Refund(context.Context, [32]byte) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.refundCalled++
	return "fake-htlc-refund-tx", nil
}
func (h *fakeHTLC) RegisterAgreementCommitment(context.Context, [32]byte) (string, error) {
	return "fake-htlc-commit-tx", nil
}

func (h *fakeHTLC) counters() (lock, settle, refund int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lockCalled, h.settleCalled, h.refundCalled
}

// ── captureRelay: records the lock/settle handlers so a suite can fire events ──

type captureRelay struct {
	mu     sync.Mutex
	lockH  func(ports.InteroperabilityProof) error
	settle func(ports.InteroperabilityProof) error
}

func (r *captureRelay) SubscribeLockEvents(_ context.Context, h func(ports.InteroperabilityProof) error) error {
	r.mu.Lock()
	r.lockH = h
	r.mu.Unlock()
	return nil
}
func (r *captureRelay) SubscribeSettleEvents(_ context.Context, h func(ports.InteroperabilityProof) error) error {
	r.mu.Lock()
	r.settle = h
	r.mu.Unlock()
	return nil
}
func (r *captureRelay) RelayProof(context.Context, ports.InteroperabilityProof) (string, error) {
	return "fake-relay-tx", nil
}
func (r *captureRelay) VerifyProof(context.Context, ports.InteroperabilityProof) (bool, error) {
	return true, nil
}

func (r *captureRelay) lockHandler() func(ports.InteroperabilityProof) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lockH
}
func (r *captureRelay) settleHandler() func(ports.InteroperabilityProof) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.settle
}

// ── memFXRepo: in-memory FXAgreementRepository ─────────────────────────────────

type memFXRepo struct {
	mu         sync.Mutex
	agreements map[string]*domain.FXAgreementRecord
	events     []*domain.FXAgreementEvent
}

func newMemFXRepo() *memFXRepo {
	return &memFXRepo{agreements: make(map[string]*domain.FXAgreementRecord)}
}

func (m *memFXRepo) CreateAgreement(_ context.Context, r *domain.FXAgreementRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *r
	m.agreements[r.TradeID] = &cp
	return nil
}
func (m *memFXRepo) GetAgreement(_ context.Context, id string) (*domain.FXAgreementRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.agreements[id]
	if !ok {
		return nil, nil
	}
	cp := *r
	return &cp, nil
}
func (m *memFXRepo) UpdateAgreement(_ context.Context, r *domain.FXAgreementRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *r
	m.agreements[r.TradeID] = &cp
	return nil
}
func (m *memFXRepo) ListAgreements(_ context.Context, f ports.FXAgreementFilter) ([]*domain.FXAgreementRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*domain.FXAgreementRecord
	for _, r := range m.agreements {
		if f.State != "" && r.State != f.State {
			continue
		}
		if f.Counterparty != "" && r.CounterpartyB != f.Counterparty {
			continue
		}
		cp := *r
		out = append(out, &cp)
	}
	return out, nil
}
func (m *memFXRepo) CreateAuditEvent(_ context.Context, e *domain.FXAgreementEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
	return nil
}
func (m *memFXRepo) ListAuditEvents(_ context.Context, tradeID string) ([]*domain.FXAgreementEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*domain.FXAgreementEvent
	for _, e := range m.events {
		if e.TradeID == tradeID {
			out = append(out, e)
		}
	}
	return out, nil
}
func (m *memFXRepo) ListExpiredNonTerminal(_ context.Context, _ int64) ([]*domain.FXAgreementRecord, error) {
	return nil, nil
}

// ── fakeCompliance: in-process ComplianceService over the real shared proto ────
//
// Replicates the transfer-limit decision in compliance's CheckAndDeductTransferLimit:
//   - resolve payer bank -> central bank by "-a"/"-b" suffix (no CB => allow),
//   - find the applicable limit (exact participant+currency, else CB-wide),
//   - compare accumulated daily volume + this amount against the limit (in wei),
//   - deny with TRANSFER_LIMIT_EXCEEDED, else deduct (accumulate) and allow.
// RestoreTransferLimit reverses an accumulation (refund path).
type fakeCompliance struct {
	compliancev1.UnimplementedComplianceServiceServer
	mu      sync.Mutex
	limits  map[string]string // centralBankID -> max amount (human decimal)
	volumes map[string]string // centralBankID|currency|day -> accumulated wei
}

func newFakeCompliance() *fakeCompliance {
	return &fakeCompliance{
		limits:  make(map[string]string),
		volumes: make(map[string]string),
	}
}

// SetLimit seeds a central-bank-wide transfer limit (human decimal, e.g. "1000").
func (s *fakeCompliance) SetLimit(centralBankID, maxAmountHuman string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.limits[centralBankID] = maxAmountHuman
}

func centralBankForPayer(payer string) string {
	lower := strings.ToLower(strings.TrimSpace(payer))
	switch {
	case strings.HasSuffix(lower, "-a"):
		return "central-bank-a"
	case strings.HasSuffix(lower, "-b"):
		return "central-bank-b"
	default:
		return ""
	}
}

func volKey(cb, currency string, day time.Time) string {
	return cb + "|" + currency + "|" + day.Format("2006-01-02")
}

const tokenDecimals = 18

func humanToWei(human string) (*big.Int, bool) {
	human = strings.TrimSpace(human)
	mult := new(big.Int).Exp(big.NewInt(10), big.NewInt(tokenDecimals), nil)
	parts := strings.SplitN(human, ".", 2)
	intPart, ok := new(big.Int).SetString(parts[0], 10)
	if !ok {
		return nil, false
	}
	res := new(big.Int).Mul(intPart, mult)
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) > tokenDecimals {
			frac = frac[:tokenDecimals]
		} else {
			frac += strings.Repeat("0", tokenDecimals-len(frac))
		}
		fi, ok := new(big.Int).SetString(frac, 10)
		if !ok {
			return nil, false
		}
		res.Add(res, fi)
	}
	return res, true
}

func (s *fakeCompliance) CheckAndDeductTransferLimit(_ context.Context, req *compliancev1.CheckAndDeductTransferLimitRequest) (*compliancev1.CheckAndDeductTransferLimitResponse, error) {
	cb := centralBankForPayer(req.PayerBankId)
	if cb == "" {
		return &compliancev1.CheckAndDeductTransferLimitResponse{Allowed: true}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	maxHuman, ok := s.limits[cb]
	if !ok {
		// No limit configured for this central bank => allow.
		return &compliancev1.CheckAndDeductTransferLimitResponse{Allowed: true}, nil
	}

	amountWei, ok := humanToWei(req.AmountHuman)
	if !ok {
		return &compliancev1.CheckAndDeductTransferLimitResponse{Allowed: true}, nil
	}
	maxWei, _ := humanToWei(maxHuman)

	day := time.Now().UTC().Truncate(24 * time.Hour)
	k := volKey(cb, req.Currency, day)
	accWei, _ := new(big.Int).SetString(s.volumes[k], 10)
	if accWei == nil {
		accWei = new(big.Int)
	}

	if new(big.Int).Add(accWei, amountWei).Cmp(maxWei) > 0 {
		return &compliancev1.CheckAndDeductTransferLimitResponse{
			Allowed:   false,
			ErrorCode: "TRANSFER_LIMIT_EXCEEDED",
			MaxAmount: maxHuman,
		}, nil
	}

	s.volumes[k] = new(big.Int).Add(accWei, amountWei).String()
	return &compliancev1.CheckAndDeductTransferLimitResponse{Allowed: true}, nil
}

func (s *fakeCompliance) RestoreTransferLimit(_ context.Context, req *compliancev1.RestoreTransferLimitRequest) (*emptypb.Empty, error) {
	cb := centralBankForPayer(req.PayerBankId)
	if cb == "" {
		return &emptypb.Empty{}, nil
	}
	amountWei, ok := humanToWei(req.AmountHuman)
	if !ok {
		return &emptypb.Empty{}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	day := time.Now().UTC().Truncate(24 * time.Hour)
	k := volKey(cb, req.Currency, day)
	accWei, _ := new(big.Int).SetString(s.volumes[k], 10)
	if accWei == nil {
		accWei = new(big.Int)
	}
	res := new(big.Int).Sub(accWei, amountWei)
	if res.Sign() < 0 {
		res = new(big.Int)
	}
	s.volumes[k] = res.String()
	return &emptypb.Empty{}, nil
}
