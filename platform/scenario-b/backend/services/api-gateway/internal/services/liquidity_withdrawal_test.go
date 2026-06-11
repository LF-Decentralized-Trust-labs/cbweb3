package services

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	apidomain "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// errStopBeforeDB halts removeShares right after the AMM call so the test can assert the
// on-chain wiring (share resolution, owner fallback, home-side selection) without a DB.
var errStopBeforeDB = errors.New("stop-before-db")

// withdrawalMockAMM records the arguments removeShares passes to the AMM layer.
type withdrawalMockAMM struct {
	lpBalance       *big.Int
	lpBalanceHolder *string // captures the holder passed to LPBalanceOf

	gotShares       *big.Int
	gotHomeIsTokenA *bool
}

func (m *withdrawalMockAMM) AddLiquidity(ctx context.Context, pair, providerID, a, b string) (string, error) {
	return "", errors.New("not used")
}

func (m *withdrawalMockAMM) RemoveLiquidityShares(ctx context.Context, shares *big.Int, homeIsTokenA bool, minAmountOut *big.Int) (string, error) {
	m.gotShares = new(big.Int).Set(shares)
	m.gotHomeIsTokenA = &homeIsTokenA
	return "", errStopBeforeDB
}

func (m *withdrawalMockAMM) LPBalanceOf(ctx context.Context, holder string) (*big.Int, error) {
	h := holder
	m.lpBalanceHolder = &h
	if m.lpBalance == nil {
		return big.NewInt(0), nil
	}
	return new(big.Int).Set(m.lpBalance), nil
}

func (m *withdrawalMockAMM) GetPoolReserves(ctx context.Context, pair string) (string, string, float64, error) {
	return "0", "0", 0, errors.New("not used")
}

// TestRemoveShares_UsesRecordedShares asserts that a position with persisted LPShares burns
// exactly that amount, selecting the provider's home side (deposit_side=B → tokenOut=B).
func TestRemoveShares_UsesRecordedShares(t *testing.T) {
	mock := &withdrawalMockAMM{}
	svc := NewLiquidityProvisionService(nil, mock)

	pos := &apidomain.LiquidityPosition{
		LPID:        "lp-1",
		PoolPair:    "W-BRL-ARS",
		LPShares:    "12345",
		DepositSide: apidomain.DepositSideB,
	}
	_, err := svc.removeShares(context.Background(), pos)
	if !errors.Is(err, errStopBeforeDB) && (err == nil || !strings.Contains(err.Error(), errStopBeforeDB.Error())) {
		t.Fatalf("expected stop-before-db sentinel, got %v", err)
	}
	if mock.gotShares == nil || mock.gotShares.String() != "12345" {
		t.Errorf("RemoveLiquidityShares shares = %v; want 12345 (the persisted position shares)", mock.gotShares)
	}
	if mock.gotHomeIsTokenA == nil || *mock.gotHomeIsTokenA != false {
		t.Errorf("homeIsTokenA = %v; want false (deposit_side=B → home currency is TOKEN_B)", mock.gotHomeIsTokenA)
	}
	if mock.lpBalanceHolder != nil {
		t.Errorf("LPBalanceOf must not be called when the position records shares")
	}
}

// TestRemoveShares_FallbackToSignerBalance asserts that a legacy position (LPShares="0") resolves
// the owner's on-chain balance with an EMPTY holder — i.e. the configured CB signer (D4/D7) —
// never the provider_bank_id name string (the prior bug).
func TestRemoveShares_FallbackToSignerBalance(t *testing.T) {
	mock := &withdrawalMockAMM{lpBalance: big.NewInt(777)}
	svc := NewLiquidityProvisionService(nil, mock)

	pos := &apidomain.LiquidityPosition{
		LPID:           "lp-2",
		PoolPair:       "W-BRL-ARS",
		ProviderBankID: "central-bank-a", // must NOT be used as an address
		LPShares:       "0",
		DepositSide:    apidomain.DepositSideA,
	}
	_, err := svc.removeShares(context.Background(), pos)
	if err == nil || !strings.Contains(err.Error(), errStopBeforeDB.Error()) {
		t.Fatalf("expected stop-before-db sentinel, got %v", err)
	}
	if mock.lpBalanceHolder == nil {
		t.Fatal("expected LPBalanceOf fallback to be called")
	}
	if *mock.lpBalanceHolder != "" {
		t.Errorf("LPBalanceOf holder = %q; want \"\" (empty = configured CB signer, not the bank-id string)", *mock.lpBalanceHolder)
	}
	if mock.gotShares == nil || mock.gotShares.String() != "777" {
		t.Errorf("RemoveLiquidityShares shares = %v; want 777 (full on-chain balance)", mock.gotShares)
	}
	if mock.gotHomeIsTokenA == nil || *mock.gotHomeIsTokenA != true {
		t.Errorf("homeIsTokenA = %v; want true (deposit_side=A)", mock.gotHomeIsTokenA)
	}
}

// TestRemoveShares_NoSharesAnywhere asserts a clear error when neither the position nor the
// signer's on-chain balance holds any shares.
func TestRemoveShares_NoSharesAnywhere(t *testing.T) {
	mock := &withdrawalMockAMM{lpBalance: big.NewInt(0)}
	svc := NewLiquidityProvisionService(nil, mock)

	pos := &apidomain.LiquidityPosition{LPID: "lp-3", LPShares: "0", DepositSide: apidomain.DepositSideA}
	_, err := svc.removeShares(context.Background(), pos)
	if err == nil || !strings.Contains(err.Error(), "no LP shares") {
		t.Fatalf("expected 'no LP shares' error, got %v", err)
	}
	if mock.gotShares != nil {
		t.Error("RemoveLiquidityShares must not be called with zero shares")
	}
}
