// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/ethereum/go-ethereum/crypto"
)

// ---- fakes ----

type fakeSovereignAMM struct {
	depositErr  error
	finalizeErr error
	sharesA     *big.Int
	sharesB     *big.Int
	balance     *big.Int
	balErr      error
}

func (f *fakeSovereignAMM) DepositForCommitAt(_ context.Context, _ string, _ [32]byte, _ bool, _ *big.Int, _ string) error {
	return f.depositErr
}
func (f *fakeSovereignAMM) FinalizeCommitAt(_ context.Context, _ string, _ [32]byte) (*big.Int, *big.Int, error) {
	return f.sharesA, f.sharesB, f.finalizeErr
}
func (f *fakeSovereignAMM) TokenBalanceAt(_ context.Context, _ string, _ bool, _ string) (*big.Int, error) {
	if f.balErr != nil {
		return nil, f.balErr
	}
	if f.balance == nil {
		return big.NewInt(0), nil
	}
	return f.balance, nil
}

// memCommitRepo2 supports lookups by on-chain commit ID for sovereign tests.
type memCommitRepo2 struct {
	memCommitRepo
	byOnChain map[string]*domain.PoolCommit
	updated   map[string]domain.CommitStatus
}

func newMemCommitRepo2() *memCommitRepo2 {
	r := &memCommitRepo2{
		byOnChain: map[string]*domain.PoolCommit{},
		updated:   map[string]domain.CommitStatus{},
	}
	r.commits = map[string]*domain.PoolCommit{}
	r.byPair = map[string]*domain.PoolCommit{}
	r.byProv = map[string]*domain.PoolCommit{}
	return r
}
func (m *memCommitRepo2) FindByOnChainCommitID(_ context.Context, id string) (*domain.PoolCommit, error) {
	if c, ok := m.byOnChain[id]; ok {
		return c, nil
	}
	return nil, errors.New("not found")
}
func (m *memCommitRepo2) UpdateStatusByOnChainCommitID(_ context.Context, id string, status domain.CommitStatus) error {
	m.updated[id] = status
	if c, ok := m.byOnChain[id]; ok {
		c.Status = status
	}
	return nil
}

type memLPRepo struct{ created []*domain.LiquidityPosition }

func (m *memLPRepo) Create(_ context.Context, p *domain.LiquidityPosition) error {
	m.created = append(m.created, p)
	return nil
}
func (m *memLPRepo) FindByPoolPair(_ context.Context, _ string) ([]domain.LiquidityPosition, error) {
	return nil, nil
}
func (m *memLPRepo) FindByProviderAndPoolPair(_ context.Context, _, _ string) ([]domain.LiquidityPosition, error) {
	return nil, nil
}

func newSovereignSvc(t *testing.T, amm sovereignAMMAdder, commitRepo PoolCommitRepo, lpRepo LPPositionRepo, signer string) *SovereignLiquidityService {
	db := newTestDB(t, &domain.BridgedAssetPosition{})
	svc, err := NewSovereignLiquidityService(db, amm, commitRepo, lpRepo, SovereignLiquidityServiceConfig{
		LocalCBHubSigner:    signer,
		SovereignPairAMMMap: `{"W-BRL-ARS":"0xAMM"}`,
		ReconcileTimeoutSec: 5,
	})
	if err != nil {
		t.Fatalf("construct sovereign service: %v", err)
	}
	return svc
}

// ---- tests ----

func TestSovereign_NewFromBadJSON(t *testing.T) {
	_, err := NewSovereignLiquidityService(newTestDB(t), &fakeSovereignAMM{}, nil, nil, SovereignLiquidityServiceConfig{
		SovereignPairAMMMap: `{bad json`,
	})
	if err == nil {
		t.Fatal("expected JSON parse error")
	}
}

func TestSovereign_IsSovereignPair(t *testing.T) {
	svc := newSovereignSvc(t, &fakeSovereignAMM{}, newMemCommitRepo2(), &memLPRepo{}, "0xme")
	if !svc.IsSovereignPair("W-BRL-ARS") {
		t.Fatal("expected sovereign pair")
	}
	if svc.IsSovereignPair("W-USD-EUR") {
		t.Fatal("did not expect unknown pair to be sovereign")
	}
}

func TestSovereign_CheckSufficientWTokenBalance(t *testing.T) {
	svc := newSovereignSvc(t, &fakeSovereignAMM{balance: big.NewInt(1000)}, newMemCommitRepo2(), &memLPRepo{}, "0xme")
	if err := svc.CheckSufficientWTokenBalance(context.Background(), "W-BRL-ARS", true, "0xholder", big.NewInt(500)); err != nil {
		t.Fatalf("expected sufficient balance, got %v", err)
	}

	err := svc.CheckSufficientWTokenBalance(context.Background(), "W-BRL-ARS", true, "0xholder", big.NewInt(2000))
	if !IsInsufficientBalance(err) {
		t.Fatalf("expected insufficient balance error, got %v", err)
	}

	if err := svc.CheckSufficientWTokenBalance(context.Background(), "UNKNOWN", true, "0xh", big.NewInt(1)); err == nil {
		t.Fatal("expected unknown-pair error")
	}

	svcErr := newSovereignSvc(t, &fakeSovereignAMM{balErr: errors.New("rpc")}, newMemCommitRepo2(), &memLPRepo{}, "0xme")
	if err := svcErr.CheckSufficientWTokenBalance(context.Background(), "W-BRL-ARS", true, "0xh", big.NewInt(1)); err == nil {
		t.Fatal("expected balance-check error")
	}
}

func TestSovereign_HasActiveBridgePosition(t *testing.T) {
	svc := newSovereignSvc(t, &fakeSovereignAMM{}, newMemCommitRepo2(), &memLPRepo{}, "0xme")
	svc.db.Create(&domain.BridgedAssetPosition{PositionID: "p1", OwnerBankID: "cb-a", BridgeState: domain.BridgeStateActive})
	has, err := svc.HasActiveBridgePosition(context.Background(), "cb-a")
	if err != nil || !has {
		t.Fatalf("expected active position, got %v err %v", has, err)
	}
	has, _ = svc.HasActiveBridgePosition(context.Background(), "cb-z")
	if has {
		t.Fatal("expected no active position for cb-z")
	}
}

func TestSovereign_ExecuteMatchedCommit_UnknownPair(t *testing.T) {
	svc := newSovereignSvc(t, &fakeSovereignAMM{}, newMemCommitRepo2(), &memLPRepo{}, "0xme")
	err := svc.ExecuteMatchedCommit(context.Background(), SovereignExecuteRequest{PoolPair: "NOPE"})
	if err == nil {
		t.Fatal("expected unknown-pair error")
	}
}

func TestSovereign_ExecuteMatchedCommit_NotOurSigner(t *testing.T) {
	svc := newSovereignSvc(t, &fakeSovereignAMM{}, newMemCommitRepo2(), &memLPRepo{}, "0xme")
	// Neither signer matches → silently ignored (nil).
	err := svc.ExecuteMatchedCommit(context.Background(), SovereignExecuteRequest{
		PoolPair: "W-BRL-ARS",
		SignerA:  "0xother1", AmountA: "100", CommitIDA: "aa",
		SignerB: "0xother2", AmountB: "100", CommitIDB: "bb",
	})
	if err != nil {
		t.Fatalf("expected nil (ignored), got %v", err)
	}
}

func TestSovereign_ExecuteMatchedCommit_InvalidAmount(t *testing.T) {
	svc := newSovereignSvc(t, &fakeSovereignAMM{}, newMemCommitRepo2(), &memLPRepo{}, "0xme")
	err := svc.ExecuteMatchedCommit(context.Background(), SovereignExecuteRequest{
		PoolPair: "W-BRL-ARS",
		SignerA:  "0xme", AmountA: "0", CommitIDA: "aabb",
		SignerB: "0xother", AmountB: "100", CommitIDB: "ccdd",
	})
	if err == nil {
		t.Fatal("expected invalid-amount error")
	}
}

func TestSovereign_ExecuteMatchedCommit_HappyPath(t *testing.T) {
	repo := newMemCommitRepo2()
	repo.byOnChain["aabb"] = &domain.PoolCommit{CommitID: "commit-1", ProviderID: "cb-a", Amount: "100", Status: domain.CommitStatusPending}
	lp := &memLPRepo{}
	svc := newSovereignSvc(t, &fakeSovereignAMM{sharesA: big.NewInt(50), sharesB: big.NewInt(50)}, repo, lp, "0xme")

	err := svc.ExecuteMatchedCommit(context.Background(), SovereignExecuteRequest{
		PoolPair: "W-BRL-ARS",
		SignerA:  "0xme", AmountA: "100", CommitIDA: "aabb",
		SignerB: "0xother", AmountB: "100", CommitIDB: "ccdd",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.updated["aabb"] != domain.CommitStatusExecuted {
		t.Fatalf("expected commit EXECUTED, got %s", repo.updated["aabb"])
	}
	if len(lp.created) != 1 {
		t.Fatalf("expected LP position created, got %d", len(lp.created))
	}
}

func TestSovereign_ExecuteMatchedCommit_DepositErrorReconcile(t *testing.T) {
	repo := newMemCommitRepo2()
	repo.byOnChain["aabb"] = &domain.PoolCommit{CommitID: "commit-1", ProviderID: "cb-a", Amount: "100", Status: domain.CommitStatusPending}
	svc := newSovereignSvc(t, &fakeSovereignAMM{depositErr: errors.New("revert")}, repo, &memLPRepo{}, "0xme")

	err := svc.ExecuteMatchedCommit(context.Background(), SovereignExecuteRequest{
		PoolPair: "W-BRL-ARS",
		SignerA:  "0xme", AmountA: "100", CommitIDA: "aabb",
		SignerB: "0xother", AmountB: "100", CommitIDB: "ccdd",
	})
	if err == nil {
		t.Fatal("expected deposit error")
	}
	if repo.updated["aabb"] != domain.CommitStatusReconciliationRequired {
		t.Fatalf("expected RECONCILIATION_REQUIRED, got %s", repo.updated["aabb"])
	}
}

func TestSovereign_ExecuteMatchedCommit_AlreadyExecuted(t *testing.T) {
	repo := newMemCommitRepo2()
	repo.byOnChain["aabb"] = &domain.PoolCommit{CommitID: "c1", ProviderID: "cb-a", Amount: "100", Status: domain.CommitStatusExecuted}
	lp := &memLPRepo{}
	svc := newSovereignSvc(t, &fakeSovereignAMM{}, repo, lp, "0xme")

	err := svc.ExecuteMatchedCommit(context.Background(), SovereignExecuteRequest{
		PoolPair: "W-BRL-ARS",
		SignerA:  "0xme", AmountA: "100", CommitIDA: "aabb",
		SignerB: "0xother", AmountB: "100", CommitIDB: "ccdd",
	})
	if err != nil {
		t.Fatalf("expected idempotent success, got %v", err)
	}
	if len(lp.created) != 1 {
		t.Fatalf("expected LP position ensured, got %d", len(lp.created))
	}
}

func TestSovereign_ExecuteMatchedCommit_SyntheticCommitID(t *testing.T) {
	repo := newMemCommitRepo2()
	svc := newSovereignSvc(t, &fakeSovereignAMM{}, repo, &memLPRepo{}, "0xme")
	// "direct-deposit-..." is not valid hex → synthetic; no DB record, returns nil.
	err := svc.ExecuteMatchedCommit(context.Background(), SovereignExecuteRequest{
		PoolPair: "W-BRL-ARS",
		SignerA:  "0xme", AmountA: "100", CommitIDA: "direct-deposit-cb-a",
		SignerB: "0xother", AmountB: "100", CommitIDB: "direct-deposit-cb-b",
	})
	if err != nil {
		t.Fatalf("expected nil for synthetic commit, got %v", err)
	}
}

func TestSovereign_ResolvePoolPair_ByHash(t *testing.T) {
	svc := newSovereignSvc(t, &fakeSovereignAMM{}, newMemCommitRepo2(), &memLPRepo{}, "0xme")
	hash := crypto.Keccak256Hash([]byte("W-BRL-ARS")).Hex()
	name, addr, ok := svc.resolvePoolPair(hash)
	if !ok || name != "W-BRL-ARS" || addr != "0xamm" {
		t.Fatalf("expected hash resolution, got name=%s addr=%s ok=%v", name, addr, ok)
	}
}

func TestSovereign_Helpers(t *testing.T) {
	if isSyntheticCommitID("aabb") {
		t.Fatal("hex string should not be synthetic")
	}
	if !isSyntheticCommitID("direct-deposit-cb-a") {
		t.Fatal("non-hex string should be synthetic")
	}
	if parseBig("notnum") != nil {
		t.Fatal("expected nil for non-numeric")
	}
	if parseBig("123").Cmp(big.NewInt(123)) != 0 {
		t.Fatal("parseBig failed")
	}
	// deriveEscrowKey is order-independent.
	if deriveEscrowKey("aa", "bb") != deriveEscrowKey("bb", "aa") {
		t.Fatal("escrow key should be order-independent")
	}
}
