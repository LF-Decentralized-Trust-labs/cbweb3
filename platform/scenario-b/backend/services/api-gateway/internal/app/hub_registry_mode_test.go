// SPDX-License-Identifier: Apache-2.0

package app

import "testing"

// TestResolveHubRegistryMode_ReadOnlyWithoutASigningKey is the regression this helper exists for.
//
// A commercial bank deliberately holds no hub signing key: every hub act is delegated to its central
// bank. Discovery is not an act — listing the pairs and currencies registered on the hub is a view
// call. Gating the client on a signing key made the bank fall back to reading pairs from its own
// database, which never receives them (they are proposed and confirmed by the central banks), so a
// funded corridor showed up as no pools at all.
func TestResolveHubRegistryMode_ReadOnlyWithoutASigningKey(t *testing.T) {
	got := resolveHubRegistryMode("0xregistry", "http://hub:8545", "")

	if got != hubRegistryReadOnly {
		t.Fatalf("resolveHubRegistryMode(addr, rpc, no key) = %v; want %v (discovery needs no key)", got, hubRegistryReadOnly)
	}
}

func TestResolveHubRegistryMode_ReadWriteWithASigningKey(t *testing.T) {
	got := resolveHubRegistryMode("0xregistry", "http://hub:8545", "0xabc")

	if got != hubRegistryReadWrite {
		t.Fatalf("resolveHubRegistryMode(addr, rpc, key) = %v; want %v", got, hubRegistryReadWrite)
	}
}

func TestResolveHubRegistryMode_DisabledWithoutTheContractOrTheRPC(t *testing.T) {
	cases := []struct {
		name         string
		contractAddr string
		rpc          string
		key          string
	}{
		{"no contract address", "", "http://hub:8545", "0xabc"},
		{"no hub rpc", "0xregistry", "", "0xabc"},
		{"neither", "", "", ""},
		// Whitespace-only values come from an unset compose variable that still interpolated.
		{"blank contract address", "   ", "http://hub:8545", "0xabc"},
		{"blank hub rpc", "0xregistry", "  ", "0xabc"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveHubRegistryMode(tc.contractAddr, tc.rpc, tc.key); got != hubRegistryDisabled {
				t.Fatalf("resolveHubRegistryMode(%q, %q, %q) = %v; want %v",
					tc.contractAddr, tc.rpc, tc.key, got, hubRegistryDisabled)
			}
		})
	}
}

// A key that is present but blank is the same as absent: compose writes "" for a bank.
func TestResolveHubRegistryMode_BlankKeyIsReadOnly(t *testing.T) {
	if got := resolveHubRegistryMode("0xregistry", "http://hub:8545", "   "); got != hubRegistryReadOnly {
		t.Fatalf("blank signing key = %v; want %v", got, hubRegistryReadOnly)
	}
}
