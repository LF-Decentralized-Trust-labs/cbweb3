// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"math/big"
	"testing"

	apidomain "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// ---- fakes ----

type fakeLiquidityAMM struct {
	addShares string
	addErr    error
	removeOut string
	removeErr error
	balance   *big.Int
	balErr    error
}

func (f *fakeLiquidityAMM) AddLiquidity(_ context.Context, _, _, _, _ string) (string, error) {
	return f.addShares, f.addErr
}
func (f *fakeLiquidityAMM) RemoveLiquidityShares(_ context.Context, _ *big.Int, _ bool, _ *big.Int) (string, error) {
	return f.removeOut, f.removeErr
}
func (f *fakeLiquidityAMM) LPBalanceOf(_ context.Context, _ string) (*big.Int, error) {
	if f.balErr != nil {
		return nil, f.balErr
	}
	if f.balance == nil {
		return big.NewInt(0), nil
	}
	return f.balance, nil
}
func (f *fakeLiquidityAMM) GetPoolReserves(_ context.Context, _ string) (string, string, float64, error) {
	return "0", "0", 0, nil
}

type memCommitRepo struct {
	commits map[string]*apidomain.PoolCommit
	byPair  map[string]*apidomain.PoolCommit // key pair|side
	byProv  map[string]*apidomain.PoolCommit // key provider|pair
}

func newMemCommitRepo() *memCommitRepo {
	return &memCommitRepo{
		commits: map[string]*apidomain.PoolCommit{},
		byPair:  map[string]*apidomain.PoolCommit{},
		byProv:  map[string]*apidomain.PoolCommit{},
	}
}
func (m *memCommitRepo) Create(_ context.Context, c *apidomain.PoolCommit) error {
	m.commits[c.CommitID] = c
	m.byPair[c.PoolPair+"|"+string(c.Side)] = c
	m.byProv[c.ProviderID+"|"+c.PoolPair] = c
	return nil
}
func (m *memCommitRepo) FindActiveByPairAndSide(_ context.Context, pair string, side apidomain.CommitSide) (*apidomain.PoolCommit, error) {
	if c, ok := m.byPair[pair+"|"+string(side)]; ok {
		return c, nil
	}
	return nil, errors.New("not found")
}
func (m *memCommitRepo) FindByProviderAndPair(_ context.Context, prov, pair string) (*apidomain.PoolCommit, error) {
	if c, ok := m.byProv[prov+"|"+pair]; ok {
		return c, nil
	}
	return nil, errors.New("not found")
}
func (m *memCommitRepo) UpdateStatus(_ context.Context, id string, status apidomain.CommitStatus) error {
	if c, ok := m.commits[id]; ok {
		c.Status = status
		return nil
	}
	return errors.New("not found")
}
func (m *memCommitRepo) FindByID(_ context.Context, id string) (*apidomain.PoolCommit, error) {
	if c, ok := m.commits[id]; ok {
		return c, nil
	}
	return nil, errors.New("not found")
}
func (m *memCommitRepo) ListByPair(_ context.Context, pair, _ string) ([]apidomain.PoolCommit, error) {
	var out []apidomain.PoolCommit
	for _, c := range m.commits {
		if c.PoolPair == pair {
			out = append(out, *c)
		}
	}
	return out, nil
}
func (m *memCommitRepo) UpdateStatusByOnChainCommitID(_ context.Context, _ string, _ apidomain.CommitStatus) error {
	return nil
}
func (m *memCommitRepo) FindByOnChainCommitID(_ context.Context, _ string) (*apidomain.PoolCommit, error) {
	return nil, errors.New("not found")
}

type memFeeRepo struct{ created []*apidomain.LPFeeEvent }

func (m *memFeeRepo) Create(_ context.Context, e *apidomain.LPFeeEvent) error {
	m.created = append(m.created, e)
	return nil
}
func (m *memFeeRepo) FindBySwapOrderID(_ context.Context, _ string) (*apidomain.LPFeeEvent, error) {
	return nil, errors.New("not found")
}

// ---- tests ----

func TestLiquidityProvision_AddLiquidity(t *testing.T) {
	db := newTestDB(t, &apidomain.LiquidityPosition{})
	amm := &fakeLiquidityAMM{addShares: "1000"}
	svc := NewLiquidityProvisionService(db, amm)

	res, err := svc.AddLiquidity(context.Background(), LiquidityProvisionRequest{
		PoolPair: "W-BRL-W-ARS", ProviderBankID: "cb-a", TokenAAmount: "500", TokenBAmount: "500",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.LPShares != "1000" || res.DepositSide != string(apidomain.DepositSideBoth) {
		t.Fatalf("unexpected result: %+v", res)
	}
	// recalculateSharesInTx writes shares_percentage to the DB row (not the in-memory
	// result struct), so verify the persisted value instead.
	var stored apidomain.LiquidityPosition
	db.Where("lp_id = ?", res.LPID).First(&stored)
	if stored.SharesPercentage == nil || *stored.SharesPercentage <= 0 {
		t.Fatal("expected shares percentage to be recalculated and persisted")
	}
}

func TestLiquidityProvision_AddLiquidity_Validation(t *testing.T) {
	svc := NewLiquidityProvisionService(newTestDB(t, &apidomain.LiquidityPosition{}), &fakeLiquidityAMM{})
	if _, err := svc.AddLiquidity(context.Background(), LiquidityProvisionRequest{}); err == nil {
		t.Fatal("expected missing-field error")
	}
	if _, err := svc.AddLiquidity(context.Background(), LiquidityProvisionRequest{PoolPair: "p", ProviderBankID: "cb", TokenAAmount: "bad", TokenBAmount: "1"}); err == nil {
		t.Fatal("expected invalid token_a error")
	}
	if _, err := svc.AddLiquidity(context.Background(), LiquidityProvisionRequest{PoolPair: "p", ProviderBankID: "cb", TokenAAmount: "1", TokenBAmount: "bad"}); err == nil {
		t.Fatal("expected invalid token_b error")
	}
}

func TestLiquidityProvision_AddLiquidity_AMMError(t *testing.T) {
	svc := NewLiquidityProvisionService(newTestDB(t, &apidomain.LiquidityPosition{}), &fakeLiquidityAMM{addErr: errors.New("revert")})
	if _, err := svc.AddLiquidity(context.Background(), LiquidityProvisionRequest{PoolPair: "p", ProviderBankID: "cb", TokenAAmount: "1", TokenBAmount: "1"}); err == nil {
		t.Fatal("expected amm error")
	}
}

func TestLiquidityProvision_RemoveLiquidity_Full(t *testing.T) {
	db := newTestDB(t, &apidomain.LiquidityPosition{})
	amm := &fakeLiquidityAMM{removeOut: "1000"}
	svc := NewLiquidityProvisionService(db, amm)

	pos := &apidomain.LiquidityPosition{
		LPID: "lp-1", ProviderBankID: "cb-a", PoolPair: "W-BRL-W-ARS",
		TokenAContributed: "500", TokenBContributed: "500", LPShares: "1000",
		Status: apidomain.LPStatusActive, DepositSide: apidomain.DepositSideBoth, FeeClaimAccumulated: "0",
	}
	db.Create(pos)

	res, err := svc.RemoveLiquidity(context.Background(), LiquidityRemoveRequest{
		PoolPair: "W-BRL-W-ARS", ProviderBankID: "cb-a", LPID: "lp-1", FractionBps: 10000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.WithdrawalMode != "SHARES_HOME_CURRENCY" {
		t.Fatalf("expected full withdrawal mode, got %s", res.WithdrawalMode)
	}

	var updated apidomain.LiquidityPosition
	db.Where("lp_id = ?", "lp-1").First(&updated)
	if updated.Status != apidomain.LPStatusWithdrawn {
		t.Fatalf("expected WITHDRAWN, got %s", updated.Status)
	}
}

func TestLiquidityProvision_RemoveLiquidity_Partial(t *testing.T) {
	db := newTestDB(t, &apidomain.LiquidityPosition{})
	amm := &fakeLiquidityAMM{removeOut: "500"}
	svc := NewLiquidityProvisionService(db, amm)

	db.Create(&apidomain.LiquidityPosition{
		LPID: "lp-2", ProviderBankID: "cb-a", PoolPair: "P",
		TokenAContributed: "1000", TokenBContributed: "0", LPShares: "1000",
		Status: apidomain.LPStatusActive, DepositSide: apidomain.DepositSideA, FeeClaimAccumulated: "0",
	})

	res, err := svc.RemoveLiquidity(context.Background(), LiquidityRemoveRequest{
		PoolPair: "P", ProviderBankID: "cb-a", LPID: "lp-2", FractionBps: 5000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.WithdrawalMode != "SHARES_HOME_CURRENCY_PARTIAL" {
		t.Fatalf("expected partial mode, got %s", res.WithdrawalMode)
	}
	var updated apidomain.LiquidityPosition
	db.Where("lp_id = ?", "lp-2").First(&updated)
	if updated.Status != apidomain.LPStatusActive || updated.LPShares != "500" {
		t.Fatalf("expected ACTIVE with 500 shares, got %s / %s", updated.Status, updated.LPShares)
	}
}

func TestLiquidityProvision_RemoveLiquidity_Validation(t *testing.T) {
	svc := NewLiquidityProvisionService(newTestDB(t, &apidomain.LiquidityPosition{}), &fakeLiquidityAMM{})
	if _, err := svc.RemoveLiquidity(context.Background(), LiquidityRemoveRequest{}); err == nil {
		t.Fatal("expected missing-field error")
	}
	if _, err := svc.RemoveLiquidity(context.Background(), LiquidityRemoveRequest{PoolPair: "p", ProviderBankID: "c", LPID: "l", FractionBps: 20000}); err == nil {
		t.Fatal("expected out-of-range fraction error")
	}
}

func TestLiquidityProvision_RemoveLiquidity_NotFound(t *testing.T) {
	svc := NewLiquidityProvisionService(newTestDB(t, &apidomain.LiquidityPosition{}), &fakeLiquidityAMM{})
	if _, err := svc.RemoveLiquidity(context.Background(), LiquidityRemoveRequest{PoolPair: "p", ProviderBankID: "c", LPID: "l", FractionBps: 10000}); err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestLiquidityProvision_RegisterCommit(t *testing.T) {
	db := newTestDB(t, &apidomain.LiquidityPosition{})
	repo := newMemCommitRepo()
	svc := NewLiquidityProvisionServiceWithRepos(db, &fakeLiquidityAMM{}, repo, &memFeeRepo{})

	res, err := svc.RegisterCommit(context.Background(), CommitRequest{
		PoolPair: "P", ProviderID: "cb-a", Side: apidomain.CommitSideA, Amount: "1000",
		OnChainCommitID: make([]byte, 32),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != apidomain.CommitStatusPending {
		t.Fatalf("expected PENDING, got %s", res.Status)
	}
	if res.OnChainCommitID == "" {
		t.Fatal("expected on-chain commit id to be set")
	}
}

func TestLiquidityProvision_RegisterCommit_Validation(t *testing.T) {
	repo := newMemCommitRepo()
	svc := NewLiquidityProvisionServiceWithRepos(newTestDB(t), &fakeLiquidityAMM{}, repo, &memFeeRepo{})
	if _, err := svc.RegisterCommit(context.Background(), CommitRequest{}); err == nil {
		t.Fatal("expected missing-field error")
	}
	if _, err := svc.RegisterCommit(context.Background(), CommitRequest{PoolPair: "P", ProviderID: "c", Side: "X", Amount: "1"}); err == nil {
		t.Fatal("expected invalid side error")
	}
	if _, err := svc.RegisterCommit(context.Background(), CommitRequest{PoolPair: "P", ProviderID: "c", Side: apidomain.CommitSideA, Amount: "bad"}); err == nil {
		t.Fatal("expected invalid amount error")
	}
}

func TestLiquidityProvision_RegisterCommit_NoRepo(t *testing.T) {
	svc := NewLiquidityProvisionService(newTestDB(t), &fakeLiquidityAMM{})
	if _, err := svc.RegisterCommit(context.Background(), CommitRequest{PoolPair: "P", ProviderID: "c", Side: apidomain.CommitSideA, Amount: "1"}); err == nil {
		t.Fatal("expected repo-not-configured error")
	}
}

func TestLiquidityProvision_RegisterCommit_DuplicateSameSide(t *testing.T) {
	repo := newMemCommitRepo()
	svc := NewLiquidityProvisionServiceWithRepos(newTestDB(t), &fakeLiquidityAMM{}, repo, &memFeeRepo{})
	_, _ = svc.RegisterCommit(context.Background(), CommitRequest{PoolPair: "P", ProviderID: "cb-a", Side: apidomain.CommitSideA, Amount: "1"})
	_, err := svc.RegisterCommit(context.Background(), CommitRequest{PoolPair: "P", ProviderID: "cb-a", Side: apidomain.CommitSideA, Amount: "1"})
	var ce *CommitError
	if !errors.As(err, &ce) || ce.Code != "COMMIT_ALREADY_EXISTS" {
		t.Fatalf("expected COMMIT_ALREADY_EXISTS, got %v", err)
	}
}

func TestLiquidityProvision_RegisterCommit_BothSides(t *testing.T) {
	repo := newMemCommitRepo()
	svc := NewLiquidityProvisionServiceWithRepos(newTestDB(t), &fakeLiquidityAMM{}, repo, &memFeeRepo{})
	_, _ = svc.RegisterCommit(context.Background(), CommitRequest{PoolPair: "P", ProviderID: "cb-a", Side: apidomain.CommitSideA, Amount: "1"})
	_, err := svc.RegisterCommit(context.Background(), CommitRequest{PoolPair: "P", ProviderID: "cb-a", Side: apidomain.CommitSideB, Amount: "1"})
	var ce *CommitError
	if !errors.As(err, &ce) || ce.Code != "SAME_PROVIDER_BOTH_SIDES" {
		t.Fatalf("expected SAME_PROVIDER_BOTH_SIDES, got %v", err)
	}
}

func TestLiquidityProvision_GetListCancelCommit(t *testing.T) {
	repo := newMemCommitRepo()
	svc := NewLiquidityProvisionServiceWithRepos(newTestDB(t), &fakeLiquidityAMM{}, repo, &memFeeRepo{})
	res, _ := svc.RegisterCommit(context.Background(), CommitRequest{PoolPair: "P", ProviderID: "cb-a", Side: apidomain.CommitSideA, Amount: "1"})

	got, err := svc.GetCommit(context.Background(), res.CommitID)
	if err != nil || got.CommitID != res.CommitID {
		t.Fatalf("GetCommit failed: %v", err)
	}

	list, err := svc.ListCommits(context.Background(), "P", "")
	if err != nil || len(list) != 1 {
		t.Fatalf("ListCommits expected 1, got %d err %v", len(list), err)
	}

	// Wrong provider → NOT_AUTHORIZED_LP
	if err := svc.CancelCommit(context.Background(), res.CommitID, "cb-other"); err == nil {
		t.Fatal("expected not-authorized error")
	}
	// Correct provider → expires
	if err := svc.CancelCommit(context.Background(), res.CommitID, "cb-a"); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}
}

func TestLiquidityProvision_CancelCommit_NotFound(t *testing.T) {
	repo := newMemCommitRepo()
	svc := NewLiquidityProvisionServiceWithRepos(newTestDB(t), &fakeLiquidityAMM{}, repo, &memFeeRepo{})
	if err := svc.CancelCommit(context.Background(), "missing", "cb"); err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestLiquidityProvision_RecordSwapFee(t *testing.T) {
	db := newTestDB(t, &apidomain.LiquidityPosition{})
	pct := 100.0
	db.Create(&apidomain.LiquidityPosition{
		LPID: "lp-1", PoolPair: "P", Status: apidomain.LPStatusActive,
		SharesPercentage: &pct, FeeClaimAccumulated: "0",
		TokenAContributed: "1", TokenBContributed: "1", LPShares: "1",
	})
	feeRepo := &memFeeRepo{}
	svc := NewLiquidityProvisionServiceWithRepos(db, &fakeLiquidityAMM{}, newMemCommitRepo(), feeRepo)

	err := svc.RecordSwapFee(context.Background(), "P", "ord-1", big.NewInt(100), big.NewInt(0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(feeRepo.created) != 1 {
		t.Fatalf("expected 1 fee event, got %d", len(feeRepo.created))
	}
	var updated apidomain.LiquidityPosition
	db.Where("lp_id = ?", "lp-1").First(&updated)
	if updated.FeeClaimAccumulated != "100" {
		t.Fatalf("expected 100 accumulated, got %s", updated.FeeClaimAccumulated)
	}
}

func TestLiquidityProvision_RecordSwapFee_NoRepo(t *testing.T) {
	svc := NewLiquidityProvisionService(newTestDB(t, &apidomain.LiquidityPosition{}), &fakeLiquidityAMM{})
	if err := svc.RecordSwapFee(context.Background(), "P", "ord", big.NewInt(1), big.NewInt(0)); err != nil {
		t.Fatalf("expected graceful nil when fee repo unset, got %v", err)
	}
}

func TestCommitError_Error(t *testing.T) {
	e := &CommitError{Code: "X", Message: "y"}
	if e.Error() != "X: y" {
		t.Fatalf("unexpected error string: %s", e.Error())
	}
}
