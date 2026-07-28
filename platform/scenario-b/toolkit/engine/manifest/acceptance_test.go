// SPDX-License-Identifier: Apache-2.0

package manifest

import "testing"

// TestAcceptance walks the spec's Success Criteria SC-001..SC-005 end-to-end
// through Load + Validate + ValidateSet, mirroring quickstart.md.
func TestAcceptance(t *testing.T) {
	// SC-001: the three roadmap examples validate without error.
	t.Run("SC-001 examples valid", func(t *testing.T) {
		for _, name := range []string{"found-hub.yaml", "found-spoke.yaml", "join.yaml"} {
			if res := Validate(mustLoad(t, name)); !res.Valid() {
				t.Errorf("%s: expected valid, got %+v", name, res.Errors)
			}
		}
	})

	// SC-002: every listed edge case is rejected with a field-named message.
	t.Run("SC-002 edge cases rejected", func(t *testing.T) {
		cases := []struct {
			name    string
			mutate  func(*ParticipantDeployment)
			base    string
			wantErr string
		}{
			{"invalid mode", func(p *ParticipantDeployment) { p.Spec.Mode = "found-nonsense" }, "found-hub.yaml", "spec.mode"},
			{"missing per-mode field", func(p *ParticipantDeployment) { p.Spec.Hub = nil }, "found-hub.yaml", "spec.hub"},
			{"foreign per-mode field", func(p *ParticipantDeployment) { p.Spec.HubBundleRef = "x" }, "found-hub.yaml", "spec.hubBundleRef"},
			{"non-local env", func(p *ParticipantDeployment) { p.Spec.Environment = "prod" }, "found-hub.yaml", "spec.environment"},
			{"bad certSource", func(p *ParticipantDeployment) { p.Spec.CertSource = "nope" }, "found-hub.yaml", "spec.certSource"},
			{"bad keyProvider", func(p *ParticipantDeployment) { p.Spec.KeyProvider = "nope" }, "found-hub.yaml", "spec.keyProvider"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				pd := mustLoad(t, c.base)
				c.mutate(pd)
				res := Validate(pd)
				if res.Valid() {
					t.Fatalf("expected invalid")
				}
				if !findErr(res, c.wantErr) {
					t.Errorf("expected error on %q, got %+v", c.wantErr, res.Errors)
				}
			})
		}
	})

	// SC-003: a set with a chainId/port/spoke collision fails, naming both.
	t.Run("SC-003 collision in set", func(t *testing.T) {
		a := mustLoad(t, "found-spoke.yaml")
		b := mustLoad(t, "found-spoke.yaml")
		b.Metadata.Name = "central-bank-b"
		b.Spec.Spoke.ID = "spoke-b" // different network, same chainId → collide
		b.Spec.Node.RPC.Port = 9001
		b.Spec.Node.WS.Port = 9002
		b.Spec.Node.P2P.Port = 9003
		res := ValidateSet([]*ParticipantDeployment{a, b})
		if res.Valid() {
			t.Fatal("expected a set collision")
		}
	})

	// SC-004: covered by TestSchemaParity / TestSchemaParityOnFixtures.

	// SC-005: no manifest carrying private-key material is accepted.
	t.Run("SC-005 secrets rejected", func(t *testing.T) {
		pd := mustLoad(t, "found-hub.yaml")
		pd.Spec.DisplayName = "-----BEGIN PRIVATE KEY-----abc-----END PRIVATE KEY-----"
		if Validate(pd).Valid() {
			t.Fatal("expected secret-bearing manifest to be rejected")
		}
	})
}
