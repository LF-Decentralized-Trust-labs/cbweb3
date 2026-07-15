// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"testing"

	"gorm.io/gorm"
)

// deriveSpokeTxID must be deterministic (same positionID → same txId) for retry
// idempotency, and collision-free across distinct positionIDs.
func TestDeriveSpokeTxID_DeterministicAndUnique(t *testing.T) {
	a1 := deriveSpokeTxID("pos-A")
	a2 := deriveSpokeTxID("pos-A")
	b := deriveSpokeTxID("pos-B")

	if a1 != a2 {
		t.Errorf("expected deterministic txId for same positionID")
	}
	if a1 == b {
		t.Errorf("expected distinct txId for distinct positionIDs")
	}
	if a1 == ([32]byte{}) {
		t.Errorf("expected non-zero txId")
	}
}

// planSpokeDelivery encodes the bridge-out gating rule. The load-bearing case is
// (skipSpokeLock=true, beneficiary set) → MINT: cross-currency delivery must NOT be
// suppressed by SkipSpokeLock, otherwise the hub W-token is burned but the beneficiary is
// never credited (the half-settlement this fix closes). The legacy release path stays gated.
func TestPlanSpokeDelivery(t *testing.T) {
	const bene = "0x27187c765127eC4e957A0Be20Bfd66C4C6d75bE6"
	tests := []struct {
		name           string
		spokeConfigured bool
		skipSpokeLock  bool
		beneficiary    string
		want           spokeDeliveryAction
	}{
		{"cross-currency mints even when skipping lock", true, true, bene, spokeDeliveryMint},
		{"cross-currency mints when not skipping", true, false, bene, spokeDeliveryMint},
		{"cross-currency mint ignores blank-padded beneficiary", true, true, "  " + bene + " ", spokeDeliveryMint},
		{"standard release runs when not skipping", true, false, "", spokeDeliveryRelease},
		{"standard release suppressed by skip", true, true, "", spokeDeliveryNone},
		{"no spoke configured → none even with beneficiary", false, false, bene, spokeDeliveryNone},
		{"no spoke configured → none", false, true, "", spokeDeliveryNone},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := planSpokeDelivery(tc.spokeConfigured, tc.skipSpokeLock, tc.beneficiary); got != tc.want {
				t.Errorf("planSpokeDelivery(spoke=%v, skip=%v, bene=%q) = %d, want %d",
					tc.spokeConfigured, tc.skipSpokeLock, tc.beneficiary, got, tc.want)
			}
		})
	}
}

// NewBesuRelayerExecutor rejects missing required Hub configuration before any dial.
func TestNewBesuRelayerExecutor_RequiresHubConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  BesuRelayerConfig
	}{
		{"missing rpc", BesuRelayerConfig{HubSignerKey: "0xkey"}},
		{"missing signer", BesuRelayerConfig{HubRPCURL: "http://localhost:8545"}},
		{"both missing", BesuRelayerConfig{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewBesuRelayerExecutor(context.Background(), (*gorm.DB)(nil), tc.cfg)
			if err == nil {
				t.Fatal("expected config validation error")
			}
		})
	}
}
