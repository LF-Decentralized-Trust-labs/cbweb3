// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"strings"
	"testing"
)

// mustLoad loads a fixture from testdata and fails the test on parse error.
func mustLoad(t *testing.T, name string) *ParticipantDeployment {
	t.Helper()
	pd, err := Load("testdata/" + name)
	if err != nil {
		t.Fatalf("Load(%s): %v", name, err)
	}
	return pd
}

// findErr reports whether any error finding is anchored on the given field.
func findErr(r Result, field string) bool {
	for _, f := range r.Errors {
		if f.Field == field {
			return true
		}
	}
	return false
}

func findWarn(r Result, field string) bool {
	for _, f := range r.Warnings {
		if f.Field == field {
			return true
		}
	}
	return false
}

func boolPtr(b bool) *bool { return &b }

// SC-001: the three roadmap examples validate without errors.
func TestValidExamples(t *testing.T) {
	for _, name := range []string{"found-hub.yaml", "found-spoke.yaml", "join.yaml"} {
		t.Run(name, func(t *testing.T) {
			pd := mustLoad(t, name)
			res := Validate(pd)
			if !res.Valid() {
				t.Fatalf("expected valid, got errors: %+v", res.Errors)
			}
		})
	}
}

// SC-002: wrong apiVersion / kind are rejected.
func TestRejectAPIVersionKind(t *testing.T) {
	pd := mustLoad(t, "found-hub.yaml")
	pd.APIVersion = "cbweb3/v1"
	pd.Kind = "Deployment"
	res := Validate(pd)
	if res.Valid() {
		t.Fatal("expected invalid manifest")
	}
	if !findErr(res, "apiVersion") {
		t.Errorf("expected apiVersion error, got %+v", res.Errors)
	}
	if !findErr(res, "kind") {
		t.Errorf("expected kind error, got %+v", res.Errors)
	}
}

// SC-002 edge: non-local environment is rejected.
func TestRejectNonLocalEnvironment(t *testing.T) {
	for _, env := range []string{"staging", "prod"} {
		pd := mustLoad(t, "found-hub.yaml")
		pd.Spec.Environment = env
		res := Validate(pd)
		if res.Valid() {
			t.Fatalf("env=%s: expected invalid", env)
		}
		if !findErr(res, "spec.environment") {
			t.Errorf("env=%s: expected spec.environment error, got %+v", env, res.Errors)
		}
	}
}

// SC-005: embedded private-key material is rejected.
func TestRejectEmbeddedSecrets(t *testing.T) {
	t.Run("PEM block", func(t *testing.T) {
		pd := mustLoad(t, "found-hub.yaml")
		pd.Spec.DisplayName = "-----BEGIN EC PRIVATE KEY-----\nMHcCAQ\n-----END EC PRIVATE KEY-----"
		res := Validate(pd)
		if res.Valid() {
			t.Fatal("expected invalid: PEM private key present")
		}
		if !findErr(res, "spec.displayName") {
			t.Errorf("expected spec.displayName secret error, got %+v", res.Errors)
		}
	})
	t.Run("hex key", func(t *testing.T) {
		pd := mustLoad(t, "join.yaml")
		pd.Spec.AdminUsers[0].Password = "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318"
		res := Validate(pd)
		if res.Valid() {
			t.Fatal("expected invalid: hex private key present")
		}
		if !findErr(res, "spec.adminUsers[0].password") {
			t.Errorf("expected spec.adminUsers[0].password secret error, got %+v", res.Errors)
		}
	})
}

// SC-002 / FR-007: invalid keyProvider / certSource are rejected.
func TestRejectKeyProviderCertSource(t *testing.T) {
	pd := mustLoad(t, "found-hub.yaml")
	pd.Spec.KeyProvider = "vault://foo"
	pd.Spec.CertSource = "letsencrypt"
	res := Validate(pd)
	if !findErr(res, "spec.keyProvider") {
		t.Errorf("expected spec.keyProvider error, got %+v", res.Errors)
	}
	if !findErr(res, "spec.certSource") {
		t.Errorf("expected spec.certSource error, got %+v", res.Errors)
	}
}

// FR-004: node addressing must be present and valid.
func TestRejectBadNode(t *testing.T) {
	pd := mustLoad(t, "found-hub.yaml")
	pd.Spec.Node.AdvertisedHost = ""
	pd.Spec.Node.RPC.Port = 0
	pd.Spec.Node.P2P = nil
	res := Validate(pd)
	if !findErr(res, "spec.node.advertisedHost") {
		t.Errorf("expected advertisedHost error, got %+v", res.Errors)
	}
	if !findErr(res, "spec.node.rpc.port") {
		t.Errorf("expected rpc.port error, got %+v", res.Errors)
	}
	if !findErr(res, "spec.node.p2p") {
		t.Errorf("expected p2p missing error, got %+v", res.Errors)
	}
}

// FR-003 required: each mode's required fields must be present.
func TestRejectMissingRequiredPerMode(t *testing.T) {
	t.Run("found-hub missing hub", func(t *testing.T) {
		pd := mustLoad(t, "found-hub.yaml")
		pd.Spec.Hub = nil
		res := Validate(pd)
		if !findErr(res, "spec.hub") {
			t.Errorf("expected spec.hub required error, got %+v", res.Errors)
		}
	})
	t.Run("found-spoke missing hubBundleRef", func(t *testing.T) {
		pd := mustLoad(t, "found-spoke.yaml")
		pd.Spec.HubBundleRef = ""
		res := Validate(pd)
		if !findErr(res, "spec.hubBundleRef") {
			t.Errorf("expected spec.hubBundleRef required error, got %+v", res.Errors)
		}
	})
	t.Run("join missing bankId", func(t *testing.T) {
		pd := mustLoad(t, "join.yaml")
		pd.Spec.BankID = ""
		res := Validate(pd)
		if !findErr(res, "spec.bankId") {
			t.Errorf("expected spec.bankId required error, got %+v", res.Errors)
		}
	})
}

// FR-003 forbidden: fields belonging to another mode are rejected. (SC-002)
func TestRejectForbiddenPerMode(t *testing.T) {
	t.Run("found-hub with hubBundleRef", func(t *testing.T) {
		pd := mustLoad(t, "found-hub.yaml")
		pd.Spec.HubBundleRef = "./bundles/hub.yaml"
		res := Validate(pd)
		if !findErr(res, "spec.hubBundleRef") {
			t.Errorf("expected spec.hubBundleRef forbidden error, got %+v", res.Errors)
		}
	})
	t.Run("join with hub", func(t *testing.T) {
		pd := mustLoad(t, "join.yaml")
		pd.Spec.Hub = &Hub{ChainID: 1337, Currency: []string{"BRL"}}
		res := Validate(pd)
		if !findErr(res, "spec.hub") {
			t.Errorf("expected spec.hub forbidden error, got %+v", res.Errors)
		}
	})
	t.Run("join with cbEndpoint", func(t *testing.T) {
		pd := mustLoad(t, "join.yaml")
		pd.Spec.CBEndpoint = "http://cb:18080"
		res := Validate(pd)
		if !findErr(res, "spec.cbEndpoint") {
			t.Errorf("expected spec.cbEndpoint forbidden error, got %+v", res.Errors)
		}
	})
}

// FR-012: sovereign pair validation.
func TestRejectBadPair(t *testing.T) {
	t.Run("proposer==confirmer", func(t *testing.T) {
		pd := mustLoad(t, "found-spoke.yaml")
		pd.Spec.Pair.ConfirmerCB = pd.Spec.Pair.ProposerCB
		res := Validate(pd)
		if !findErr(res, "spec.pair.confirmerCB") {
			t.Errorf("expected pair.confirmerCB error, got %+v", res.Errors)
		}
	})
	t.Run("symbolA==symbolB", func(t *testing.T) {
		pd := mustLoad(t, "found-spoke.yaml")
		pd.Spec.Pair.SymbolB = pd.Spec.Pair.SymbolA
		res := Validate(pd)
		if !findErr(res, "spec.pair.symbolB") {
			t.Errorf("expected pair.symbolB error, got %+v", res.Errors)
		}
	})
}

// FR-013: join with node.validator: true → warning, not error.
func TestJoinValidatorWarning(t *testing.T) {
	pd := mustLoad(t, "join.yaml")
	pd.Spec.Node.Validator = boolPtr(true)
	res := Validate(pd)
	if !res.Valid() {
		t.Fatalf("expected valid (warning only), got errors: %+v", res.Errors)
	}
	if !findWarn(res, "spec.node.validator") {
		t.Errorf("expected spec.node.validator warning, got %+v", res.Warnings)
	}
}

// FR-011: multiple violations are collected in a single pass.
func TestCollectsAllFindings(t *testing.T) {
	pd := mustLoad(t, "found-hub.yaml")
	pd.APIVersion = "x"
	pd.Spec.Environment = "prod"
	pd.Spec.Image = ""
	res := Validate(pd)
	if len(res.Errors) < 3 {
		t.Errorf("expected >=3 collected errors, got %d: %+v", len(res.Errors), res.Errors)
	}
}

// Parse errors are clear and single-line (no stack traces).
func TestParseErrorClarity(t *testing.T) {
	_, err := Load("testdata/does-not-exist.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if strings.Contains(err.Error(), "\n") {
		t.Errorf("expected single-line error, got: %q", err.Error())
	}
}
