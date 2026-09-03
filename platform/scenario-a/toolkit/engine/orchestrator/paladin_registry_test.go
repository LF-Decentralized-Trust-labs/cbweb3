// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
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

func TestLocalKeyProvider_SeedsFundedOperator(t *testing.T) {
	// The orchestrator holds no raw keys: the funded operator lives in the key
	// provider under LocalOperatorKeyID and resolves to the genesis-funded address.
	p := keyprovider.NewLocalKeyProviderSeeded()
	addr, err := keyProviderAddress(context.Background(), p, keyprovider.LocalOperatorKeyID)
	if err != nil {
		t.Fatalf("keyProviderAddress: %v", err)
	}
	if !strings.EqualFold(addr.Hex(), "0xFE3B557E8Fb62b89F4916B721be55cEb828dBd73") {
		t.Errorf("operator addr = %s; want 0xFE3B557E8Fb62b89F4916B721be55cEb828dBd73", addr.Hex())
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

func TestLookupIdentityHashByName_EventLayout(t *testing.T) {
	// Guard the IdentityRegistered unpack indices used by lookupIdentityHashByName:
	// data = (parentIdentityHash, identityHash, name, owner) — name at [2], hash at [1].
	parsed, err := abi.JSON(strings.NewReader(identityRegistryABIJSON))
	if err != nil {
		t.Fatalf("parse ABI: %v", err)
	}
	evt := parsed.Events["IdentityRegistered"]
	var parent, idHash [32]byte
	idHash[0] = 0xab
	packed, err := evt.Inputs.Pack(parent, idHash, "spoke-brazil-cb1", common.HexToAddress("0xFE3B557E8Fb62b89F4916B721be55cEb828dBd73"))
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	ed, err := evt.Inputs.Unpack(packed)
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}
	if len(ed) < 3 {
		t.Fatalf("want >=3 fields, got %d", len(ed))
	}
	h, ok := ed[1].([32]byte)
	if !ok || h[0] != 0xab {
		t.Errorf("identityHash at [1] = %v; want 0xab…", ed[1])
	}
	name, ok := ed[2].(string)
	if !ok || name != "spoke-brazil-cb1" {
		t.Errorf("name at [2] = %v; want spoke-brazil-cb1", ed[2])
	}
}
