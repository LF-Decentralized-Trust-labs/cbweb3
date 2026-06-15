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
