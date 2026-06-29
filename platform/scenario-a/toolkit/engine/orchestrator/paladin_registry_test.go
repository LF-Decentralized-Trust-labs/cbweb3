// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestCBNodeName_DerivedFromSpoke(t *testing.T) {
	if got := cbNodeName("spoke-brl"); got != "spoke-brl-cb" {
		t.Errorf("cbNodeName = %q; want spoke-brl-cb", got)
	}
	if got := cbNodeName("spoke-cop"); got != "spoke-cop-cb" {
		t.Errorf("cbNodeName = %q; want spoke-cop-cb", got)
	}
	// No hardcoded spoke-a fallback.
	if got := cbNodeName("spoke-pathtest"); got != "spoke-pathtest-cb" {
		t.Errorf("cbNodeName = %q; want spoke-pathtest-cb (parametric, not spoke-a)", got)
	}
}

func TestCBGrpcHostname_DerivedFromSpoke(t *testing.T) {
	if got := cbGrpcHostname("spoke-brl"); got != "paladin-spoke-brl-cb" {
		t.Errorf("cbGrpcHostname = %q; want paladin-spoke-brl-cb", got)
	}
}

func TestDevKeys_DecodeToFundedAddresses(t *testing.T) {
	// The local-profile bootstrap keys must decode and match their documented,
	// genesis-funded addresses.
	cases := map[string]string{
		registryDeployerKey: "0xFE3B557E8Fb62b89F4916B721be55cEb828dBd73",
		cbNodeOwnerKey:      "0x627306090abaB3A6e1400e9345bC60c78a8BEf57",
	}
	for keyHex, wantAddr := range cases {
		k, err := devKey(keyHex)
		if err != nil {
			t.Fatalf("devKey: %v", err)
		}
		got := crypto.PubkeyToAddress(k.PublicKey).Hex()
		if !strings.EqualFold(got, wantAddr) {
			t.Errorf("addr = %s; want %s", got, wantAddr)
		}
	}
}

func TestIdentityRegistryABI_Parses(t *testing.T) {
	parsed, err := abi.JSON(strings.NewReader(identityRegistryABIJSON))
	if err != nil {
		t.Fatalf("parse ABI: %v", err)
	}
	for _, fn := range []string{"registerIdentity", "setIdentityProperty"} {
		if _, ok := parsed.Methods[fn]; !ok {
			t.Errorf("ABI missing method %q", fn)
		}
	}
	if _, ok := parsed.Events["IdentityRegistered"]; !ok {
		t.Error("ABI missing event IdentityRegistered")
	}
}
