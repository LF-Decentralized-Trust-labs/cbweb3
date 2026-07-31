// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"math/big"
	"testing"
)

// TestOutputIsTokenA covers the pure pair-direction derivation used for bank-side
// quote display: the target outputs TOKEN_A whenever it differs from the pair's
// trailing TOKEN_B currency code.
func TestOutputIsTokenA(t *testing.T) {
	c := &CentralBankPoolClient{}
	ctx := context.Background()

	// Trailing code is the TOKEN_B currency; target == TOKEN_B → not TOKEN_A.
	if got, err := c.OutputIsTokenA(ctx, "W-BRL_ARS", "ARS"); err != nil || got {
		t.Fatalf("OutputIsTokenA(ARS) = %v, err %v; want false", got, err)
	}
	// Target differs from TOKEN_B → outputs TOKEN_A.
	if got, err := c.OutputIsTokenA(ctx, "W-BRL_ARS", "BRL"); err != nil || !got {
		t.Fatalf("OutputIsTokenA(BRL) = %v, err %v; want true", got, err)
	}
}

// TestErrorMessages covers the Error() methods of the service error types.
func TestErrorMessages(t *testing.T) {
	if got := (&ZKValidationError{Msg: "zk failed"}).Error(); got != "zk failed" {
		t.Errorf("ZKValidationError.Error() = %q; want %q", got, "zk failed")
	}
	if got := (&insufficientBalanceError{have: big.NewInt(1), need: big.NewInt(2)}).Error(); got == "" {
		t.Error("insufficientBalanceError.Error() must be non-empty")
	}
	if got := (&ErrTransferLimitExceeded{PayerBankID: "bank-a", Currency: "BRL", MaxAmount: "100"}).Error(); got == "" {
		t.Error("ErrTransferLimitExceeded.Error() must be non-empty")
	}
}

// TestHubLiquidityConfigFromEnv covers env-driven config construction and the
// per-pair AMM address resolution (map lookup, default fallback, nil receiver).
func TestHubLiquidityConfigFromEnv(t *testing.T) {
	// Commercial gateway: SOVEREIGN_AMM_ADDRESS unset → nil config.
	t.Setenv("SOVEREIGN_AMM_ADDRESS", "")
	if HubLiquidityConfigFromEnv() != nil {
		t.Fatal("want nil config when SOVEREIGN_AMM_ADDRESS is empty")
	}

	// CB gateway with sovereign liquidity wired.
	t.Setenv("SOVEREIGN_AMM_ADDRESS", "0xamm")
	t.Setenv("SOVEREIGN_HUB_TOKEN_A_ADDRESS", "0xA")
	t.Setenv("SOVEREIGN_HUB_TOKEN_B_ADDRESS", "0xB")
	t.Setenv("SOVEREIGN_PAIR_AMM_MAP", `{"W-BRL-ARS":"0xpair"}`)

	cfg := HubLiquidityConfigFromEnv()
	if cfg == nil || cfg.SovereignAMMAddress != "0xamm" {
		t.Fatalf("cfg = %+v; want SovereignAMMAddress=0xamm", cfg)
	}
	if cfg.DefaultSovereignPoolPair != "W-BRL-ARS" {
		t.Errorf("DefaultSovereignPoolPair = %q; want default W-BRL-ARS", cfg.DefaultSovereignPoolPair)
	}
	// Cross-currency quote pair normalizes to the Hub pair id, then hits the map.
	if got := cfg.AMMAddressForPair("W-BRL-W-ARS"); got != "0xpair" {
		t.Errorf("AMMAddressForPair(map hit) = %q; want 0xpair", got)
	}
	// Unmapped pair falls back to the default sovereign AMM.
	if got := cfg.AMMAddressForPair("W-XXX-YYY"); got != "0xamm" {
		t.Errorf("AMMAddressForPair(default) = %q; want 0xamm", got)
	}
	// Nil receiver is safe.
	var nilCfg *HubLiquidityConfig
	if got := nilCfg.AMMAddressForPair("anything"); got != "" {
		t.Errorf("nil.AMMAddressForPair = %q; want empty", got)
	}
}
