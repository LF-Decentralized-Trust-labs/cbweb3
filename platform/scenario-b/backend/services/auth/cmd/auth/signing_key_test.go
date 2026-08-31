// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

// This service used to sign the spoke IdentityRegistry with CB_PRIVATE_KEY, the same account
// the payment-orchestrator and the sibling service write with. Three processes on one account
// share one nonce, and a per-process counter cannot serialise across containers: concurrent
// writes replaced each other in the mempool and their callers waited on receipts that were
// never written.
//
// It now prefers its own derived identity. The fallback is not a nicety — a spoke provisioned
// before the split never granted the services address GOVERNANCE_ROLE or VERIFIER_ROLE, so
// preferring an unset key there would revert every registration.
func TestSigningKeySelection(t *testing.T) {
	const own, shared = "0xServicesKey", "0xDeployerKey"

	for _, tc := range []struct {
		name            string
		services, cbKey string
		want            string
	}{
		{"prefers its own identity when provisioned", own, shared, own},
		{"falls back to the shared key on a spoke provisioned before the split", "", shared, shared},
		{"ignores an empty services key rather than signing with nothing", "   ", shared, shared},
		{"reports no key when neither is set, so the caller can disable on-chain writes", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CB_SERVICES_PRIVATE_KEY", tc.services)
			t.Setenv("CB_PRIVATE_KEY", tc.cbKey)
			if got := firstNonEmptyEnv("CB_SERVICES_PRIVATE_KEY", "CB_PRIVATE_KEY"); got != tc.want {
				t.Errorf("selected %q, want %q", got, tc.want)
			}
		})
	}
}
