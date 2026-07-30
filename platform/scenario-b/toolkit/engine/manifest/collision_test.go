// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"strings"
	"testing"
)

// findErrContaining reports whether any error message contains all substrings.
func findErrContaining(r Result, field string, substrs ...string) bool {
	for _, f := range r.Errors {
		if f.Field != field {
			continue
		}
		ok := true
		for _, s := range substrs {
			if !strings.Contains(f.Message, s) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// SC-003: a valid founder+joiner pair (same spoke, same chainId) is NOT a collision.
func TestValidSetFounderAndJoiner(t *testing.T) {
	set := []*ParticipantDeployment{
		mustLoad(t, "found-spoke.yaml"),
		mustLoad(t, "join.yaml"),
	}
	res := ValidateSet(set)
	if !res.Valid() {
		t.Fatalf("founder+joiner on same spoke must not collide, got: %+v", res.Errors)
	}
}

// SC-003: two different spokes with the same chainId collide, naming both.
func TestChainIDCollision(t *testing.T) {
	a := mustLoad(t, "found-spoke.yaml") // spoke-a, chainId 1338, name central-bank-a
	b := mustLoad(t, "found-spoke.yaml")
	b.Metadata.Name = "central-bank-b"
	b.Spec.Spoke.ID = "spoke-b"
	// keep b's chainId equal to a's (1338) → collision across different networks
	// give b distinct ports so only chainId collides
	b.Spec.Node.RPC.Port = 9645
	b.Spec.Node.WS.Port = 9655
	b.Spec.Node.P2P.Port = 41303

	res := ValidateSet([]*ParticipantDeployment{a, b})
	if res.Valid() {
		t.Fatal("expected chainId collision")
	}
	if !findErrContaining(res, "chainId", "central-bank-a", "central-bank-b") {
		t.Errorf("expected chainId error naming both manifests, got %+v", res.Errors)
	}
}

// SC-003: overlapping declared node ports collide, naming both.
func TestPortCollision(t *testing.T) {
	a := mustLoad(t, "found-spoke.yaml")
	b := mustLoad(t, "found-spoke.yaml")
	b.Metadata.Name = "central-bank-b"
	b.Spec.Spoke.ID = "spoke-b"
	b.Spec.Spoke.ChainID = 1339 // distinct network so only port collides
	// b reuses a's rpc port → collision
	b.Spec.Node.RPC.Port = a.Spec.Node.RPC.Port
	b.Spec.Node.WS.Port = 9655
	b.Spec.Node.P2P.Port = 41303

	res := ValidateSet([]*ParticipantDeployment{a, b})
	if res.Valid() {
		t.Fatal("expected port collision")
	}
	if !findErrContaining(res, "node.port", "central-bank-a", "central-bank-b") {
		t.Errorf("expected port error naming both manifests, got %+v", res.Errors)
	}
}

// SC-003: two founders of the same spoke.id collide, naming both.
func TestSpokeIDFounderCollision(t *testing.T) {
	a := mustLoad(t, "found-spoke.yaml") // spoke-a
	b := mustLoad(t, "found-spoke.yaml") // also spoke-a
	b.Metadata.Name = "central-bank-b"
	b.Spec.Spoke.ChainID = 1339
	b.Spec.Node.RPC.Port = 9645
	b.Spec.Node.WS.Port = 9655
	b.Spec.Node.P2P.Port = 41303

	res := ValidateSet([]*ParticipantDeployment{a, b})
	if res.Valid() {
		t.Fatal("expected spoke.id founder collision")
	}
	if !findErrContaining(res, "spoke.id", "central-bank-a", "central-bank-b") {
		t.Errorf("expected spoke.id error naming both manifests, got %+v", res.Errors)
	}
}

// SC-003: duplicate metadata.name collides, naming the shared name.
func TestMetadataNameCollision(t *testing.T) {
	a := mustLoad(t, "found-hub.yaml") // hub-cbweb3
	b := mustLoad(t, "found-spoke.yaml")
	b.Metadata.Name = a.Metadata.Name // same name

	res := ValidateSet([]*ParticipantDeployment{a, b})
	if res.Valid() {
		t.Fatal("expected metadata.name collision")
	}
	if !findErrContaining(res, "metadata.name", a.Metadata.Name) {
		t.Errorf("expected metadata.name error, got %+v", res.Errors)
	}
}
