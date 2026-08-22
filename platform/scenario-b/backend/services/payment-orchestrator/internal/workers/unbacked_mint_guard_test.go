// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"strings"
	"testing"
)

// planLockMint is the gate on Hub minting. The case that matters is the third one: a position
// that reaches the Hub mint with no spoke-side leg behind it. That is legitimate in a local lab
// (a CB minting itself liquidity with no spoke wired) and is unbacked wrapped supply anywhere
// else, so the decision has to depend on the declared environment and not on a default.
func TestPlanLockMint(t *testing.T) {
	const bankWallet = "0x27187c765127eC4e957A0Be20Bfd66C4C6d75bE6"
	tests := []struct {
		name            string
		environment     string
		spokeConfigured bool
		skipSpokeLock   bool
		bankWallet      string
		nativeAsset     string
		want            lockMintAction
	}{
		// Commercial bank bridge-in: burns the bank's own tCeBM. Never gated by SkipSpokeLock,
		// and backed by construction.
		{"bank bridge-in burns", "local", true, false, bankWallet, "0xTOKEN", lockMintBurnFromBank},
		{"bank bridge-in burns even when skipping the lock", "prod", true, true, bankWallet, "0xTOKEN", lockMintBurnFromBank},
		{"bank bridge-in without a spoke cannot burn", "prod", false, false, bankWallet, "0xTOKEN", lockMintNoSpokeForBurn},

		// Sovereign self-service: fund then lock on the spoke.
		{"sovereign locks on the spoke", "local", true, false, "", "0xTOKEN", lockMintSovereignLock},
		{"sovereign locks on the spoke outside local too", "prod", true, false, "", "0xTOKEN", lockMintSovereignLock},

		// The escape hatches. Allowed only where the environment is explicitly local.
		{"skip-lock is a local-only shortcut", "local", true, true, "", "0xTOKEN", lockMintUnbackedAllowed},
		{"no spoke configured is a local-only shortcut", "local", false, false, "", "0xTOKEN", lockMintUnbackedAllowed},
		{"no native asset is a local-only shortcut", "local", true, false, "", "", lockMintUnbackedAllowed},
		{"skip-lock is refused outside local", "prod", true, true, "", "0xTOKEN", lockMintUnbackedRefused},
		{"no spoke configured is refused outside local", "staging", false, false, "", "0xTOKEN", lockMintUnbackedRefused},
		{"no native asset is refused outside local", "prod", true, false, "", "", lockMintUnbackedRefused},

		// A forgotten environment must harden, not open — the same rule the Keycloak
		// sslRequired gate applies (R1-10.7).
		{"an unset environment is not local", "", true, true, "", "0xTOKEN", lockMintUnbackedRefused},
		{"whitespace is not local", "  ", false, false, "", "0xTOKEN", lockMintUnbackedRefused},
		{"case matters: Local is not local", "Local", true, true, "", "0xTOKEN", lockMintUnbackedRefused},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := planLockMint(tc.environment, tc.spokeConfigured, tc.skipSpokeLock, tc.bankWallet, tc.nativeAsset)
			if got != tc.want {
				t.Fatalf("planLockMint(%q, spoke=%v, skip=%v, bank=%q, asset=%q) = %v, want %v",
					tc.environment, tc.spokeConfigured, tc.skipSpokeLock, tc.bankWallet, tc.nativeAsset, got, tc.want)
			}
		})
	}
}

// The boot check refuses the explicit dev flag outside a local environment. It is separate from
// the per-position decision above: BRIDGE_SKIP_SPOKE_LOCK is a deliberate opt-in with no
// legitimate non-local use, so it should fail at startup rather than at the first mint.
func TestValidateBridgeProfile(t *testing.T) {
	tests := []struct {
		name          string
		environment   string
		skipSpokeLock bool
		wantErr       bool
	}{
		{"local may skip the spoke lock", "local", true, false},
		{"local without the flag", "local", false, false},
		{"production without the flag boots", "prod", false, false},
		{"production with the flag refuses", "prod", true, true},
		{"unset environment with the flag refuses", "", true, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateBridgeProfile(tc.environment, tc.skipSpokeLock)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateBridgeProfile(%q, %v) error = %v, wantErr %v",
					tc.environment, tc.skipSpokeLock, err, tc.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "BRIDGE_SKIP_SPOKE_LOCK") {
				t.Fatalf("the error must name the flag an operator has to change, got: %v", err)
			}
		})
	}
}

// The boot check must run before anything dials, so a misconfigured stack fails on
// configuration rather than on connectivity. The RPC URL below is unreachable on purpose: if
// the constructor returned a dial error instead of the profile error, this would catch it.
func TestNewBesuRelayerExecutor_RefusesTheDevFlagOutsideLocal(t *testing.T) {
	_, err := NewBesuRelayerExecutor(context.Background(), nil, BesuRelayerConfig{
		HubRPCURL:     "http://127.0.0.1:1/never-dialled",
		HubSignerKey:  "0000000000000000000000000000000000000000000000000000000000000001",
		SkipSpokeLock: true,
		Environment:   "prod",
	})
	if err == nil {
		t.Fatal("expected the constructor to refuse BRIDGE_SKIP_SPOKE_LOCK outside local")
	}
	if !strings.Contains(err.Error(), "BRIDGE_SKIP_SPOKE_LOCK") {
		t.Fatalf("error must name the flag, got: %v", err)
	}
	if strings.Contains(err.Error(), "dial") {
		t.Fatalf("the profile check must precede dialling, got: %v", err)
	}
}
