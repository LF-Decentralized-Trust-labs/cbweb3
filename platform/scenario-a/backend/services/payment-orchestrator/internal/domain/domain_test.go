// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
)

// TestFXState_IsTerminal documents which FX states are terminal (cannot
// transition further). Terminal states must never be re-opened by any flow.
func TestFXState_IsTerminal(t *testing.T) {
	cases := []struct {
		state    domain.FXState
		terminal bool
	}{
		{domain.FXStateInvalid, false},
		{domain.FXStateProposed, false},
		{domain.FXStateAccepted, false},
		{domain.FXStateRejected, true},
		{domain.FXStateCancelled, true},
		{domain.FXStateSettled, true},
		{domain.FXState("UNKNOWN"), false},
	}
	for _, c := range cases {
		t.Run(string(c.state), func(t *testing.T) {
			if got := c.state.IsTerminal(); got != c.terminal {
				t.Errorf("FXState(%q).IsTerminal() = %v, want %v", c.state, got, c.terminal)
			}
		})
	}
}
