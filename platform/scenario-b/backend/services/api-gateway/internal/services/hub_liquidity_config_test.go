// SPDX-License-Identifier: Apache-2.0

package services

import (
	"testing"
)

func TestHubLiquidityConfig_AMMAddressForPair(t *testing.T) {
	cfg := &HubLiquidityConfig{
		SovereignAMMAddress: "0xdefault",
		SovereignPairAMMMap: map[string]string{
			"W-BRL-ARS": "0xsov",
		},
	}
	if got := cfg.AMMAddressForPair("W-BRL-W-ARS"); got != "0xsov" {
		t.Fatalf("AMMAddressForPair = %q, want 0xsov", got)
	}
	if got := cfg.AMMAddressForPair("BRL-USD"); got != "0xdefault" {
		t.Fatalf("AMMAddressForPair fallback = %q, want 0xdefault", got)
	}
}
