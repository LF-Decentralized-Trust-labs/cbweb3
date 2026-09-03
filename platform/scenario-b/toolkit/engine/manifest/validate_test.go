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

// SC-001: the roadmap examples validate without errors.
func TestValidExamples(t *testing.T) {
	for _, name := range []string{"found-hub.yaml", "found-spoke.yaml", "join.yaml", "observe.yaml"} {
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

// observe: node-provisioning fields are NOT required (no Besu node), but the
// discriminating fields of other modes are forbidden.
func TestObserveMode(t *testing.T) {
	t.Run("valid without node/keyProvider/relay", func(t *testing.T) {
		pd := mustLoad(t, "observe.yaml")
		res := Validate(pd)
		if !res.Valid() {
			t.Fatalf("expected valid observe manifest, got errors: %+v", res.Errors)
		}
		// The fixture deliberately omits node/image/keyProvider/certSource/relay.
		for _, f := range []string{"spec.node", "spec.image", "spec.keyProvider", "spec.certSource", "spec.relay"} {
			if findErr(res, f) {
				t.Errorf("observe must not require %s, got error", f)
			}
		}
	})
	t.Run("missing nocBundleRef", func(t *testing.T) {
		pd := mustLoad(t, "observe.yaml")
		pd.Spec.NOCBundleRef = ""
		res := Validate(pd)
		if !findErr(res, "spec.nocBundleRef") {
			t.Errorf("expected spec.nocBundleRef required error, got %+v", res.Errors)
		}
	})
	t.Run("forbidden spoke", func(t *testing.T) {
		pd := mustLoad(t, "observe.yaml")
		pd.Spec.Spoke = &Spoke{ID: "spoke-a", ChainID: 1338, Currency: "BRL"}
		res := Validate(pd)
		if !findErr(res, "spec.spoke") {
			t.Errorf("expected spec.spoke forbidden error, got %+v", res.Errors)
		}
	})
	t.Run("still requires frontendHost + adminUsers", func(t *testing.T) {
		pd := mustLoad(t, "observe.yaml")
		pd.Spec.FrontendHost = ""
		pd.Spec.AdminUsers = nil
		res := Validate(pd)
		if !findErr(res, "spec.frontendHost") || !findErr(res, "spec.adminUsers") {
			t.Errorf("expected frontendHost + adminUsers required, got %+v", res.Errors)
		}
	})
	t.Run("bad noc component type", func(t *testing.T) {
		pd := mustLoad(t, "observe.yaml")
		pd.Spec.NOC.Components = []string{"BESU", "NONSENSE"}
		res := Validate(pd)
		if !findErr(res, "spec.noc.components[1]") {
			t.Errorf("expected noc.components[1] error, got %+v", res.Errors)
		}
	})
}

// nocBundleRef is forbidden in the node modes.
func TestNOCBundleRefForbiddenInNodeModes(t *testing.T) {
	for _, name := range []string{"found-hub.yaml", "found-spoke.yaml", "join.yaml"} {
		pd := mustLoad(t, name)
		pd.Spec.NOCBundleRef = "./bundles/spoke-a.noc.bundle.yaml"
		res := Validate(pd)
		if !findErr(res, "spec.nocBundleRef") {
			t.Errorf("%s: expected spec.nocBundleRef forbidden error, got %+v", name, res.Errors)
		}
	}
}

// R1-10.3: the per-spoke ERC-20 metadata overrides are optional and accepted
// when they keep the "<prefix>_<ISO>" shape with the spoke's own currency.
func TestSpokeTokenOverridesAccepted(t *testing.T) {
	pd := mustLoad(t, "found-spoke.yaml")
	pd.Spec.Spoke.TokenName = "Real Digital"
	pd.Spec.Spoke.TokenSymbol = "tRD_BRL"
	pd.Spec.Spoke.FiatTokenName = "Real"
	pd.Spec.Spoke.FiatTokenSymbol = "fRD_BRL"
	res := Validate(pd)
	if !res.Valid() {
		t.Fatalf("expected valid token overrides, got errors: %+v", res.Errors)
	}
}

// A symbol without the "_<ISO>" tail is rejected: portals and the api-gateway
// derive the displayed currency code from the segment after the last underscore.
func TestRejectSpokeTokenSymbolWithoutCurrencySegment(t *testing.T) {
	for _, tc := range []struct{ field, symbol string }{
		{"spec.spoke.tokenSymbol", "tBRL"},
		{"spec.spoke.fiatTokenSymbol", "fBRL"},
		{"spec.spoke.tokenSymbol", "tCeBM_"},
	} {
		t.Run(tc.field+"/"+tc.symbol, func(t *testing.T) {
			pd := mustLoad(t, "found-spoke.yaml")
			if tc.field == "spec.spoke.tokenSymbol" {
				pd.Spec.Spoke.TokenSymbol = tc.symbol
			} else {
				pd.Spec.Spoke.FiatTokenSymbol = tc.symbol
			}
			res := Validate(pd)
			if !findErr(res, tc.field) {
				t.Errorf("expected %s error for %q, got %+v", tc.field, tc.symbol, res.Errors)
			}
		})
	}
}

// The currency code is the routing key, so a symbol may not smuggle in a
// different one than spec.spoke.currency.
func TestRejectSpokeTokenSymbolCurrencyMismatch(t *testing.T) {
	pd := mustLoad(t, "found-spoke.yaml") // currency: BRL
	pd.Spec.Spoke.TokenSymbol = "tCeBM_COP"
	res := Validate(pd)
	if !findErr(res, "spec.spoke.tokenSymbol") {
		t.Errorf("expected spec.spoke.tokenSymbol error, got %+v", res.Errors)
	}
}

// Whitespace is never valid in an ERC-20 symbol; a blank name is rejected too
// (omit the field to derive it from the currency).
func TestRejectSpokeTokenBlankAndWhitespace(t *testing.T) {
	pd := mustLoad(t, "found-spoke.yaml")
	pd.Spec.Spoke.TokenSymbol = "tCeBM BRL"
	pd.Spec.Spoke.TokenName = "   "
	res := Validate(pd)
	if !findErr(res, "spec.spoke.tokenSymbol") {
		t.Errorf("expected spec.spoke.tokenSymbol error, got %+v", res.Errors)
	}
	if !findErr(res, "spec.spoke.tokenName") {
		t.Errorf("expected spec.spoke.tokenName error, got %+v", res.Errors)
	}
}

// Only the founding CB deploys the spoke tokens, so overrides in a join manifest
// are inert — warned about, never a hard error.
func TestJoinTokenOverridesWarn(t *testing.T) {
	pd := mustLoad(t, "join.yaml")
	pd.Spec.Spoke.TokenSymbol = "tCeBM_" + pd.Spec.Spoke.Currency
	res := Validate(pd)
	if !res.Valid() {
		t.Fatalf("expected valid (warning only), got errors: %+v", res.Errors)
	}
	if !findWarn(res, "spec.spoke") {
		t.Errorf("expected spec.spoke warning, got %+v", res.Warnings)
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

// A keyProvider the toolkit cannot honour must be REJECTED, not accepted and ignored.
//
// Before this, the field was validated for shape and then never read by anything: the only
// references to spec.KeyProvider were in this validator. A manifest carrying
// `keyProvider: kms://aws-prod` therefore validated successfully and the deployment ran on keys
// derived from a public salt — production custody declared, development custody delivered, with no
// error anywhere. Accepting the declaration is the dangerous part, so validation is where it stops.
func TestRejectUnsupportedKeyProviderHost(t *testing.T) {
	for _, uri := range []string{
		"kms://aws-prod",
		"kms://prod",
		"kms://vault.internal",
		"kms://azure-keyvault?vault=cbdc",
	} {
		t.Run(uri, func(t *testing.T) {
			pd := mustLoad(t, "found-spoke.yaml")
			pd.Spec.KeyProvider = uri
			res := Validate(pd)
			if !findErr(res, "spec.keyProvider") {
				t.Fatalf("expected %q to be refused: it would silently deliver development keys", uri)
			}
			// The message has to say WHY, or an operator reads it as a typo.
			var msg string
			for _, f := range res.Errors {
				if f.Field == "spec.keyProvider" {
					msg = f.Message
				}
			}
			if !strings.Contains(msg, "local-emulator") {
				t.Fatalf("message must name the only supported provider, got %q", msg)
			}
		})
	}
}

// The local emulator stays valid, with or without a seed — it is what every sample uses.
func TestAcceptLocalEmulatorKeyProvider(t *testing.T) {
	for _, uri := range []string{
		"kms://local-emulator",
		"kms://local-emulator?seed=cbweb3-scenario-b-cb-hub-key:",
	} {
		t.Run(uri, func(t *testing.T) {
			pd := mustLoad(t, "found-spoke.yaml")
			pd.Spec.KeyProvider = uri
			if res := Validate(pd); findErr(res, "spec.keyProvider") {
				t.Fatalf("%q must remain valid: %+v", uri, res.Errors)
			}
		})
	}
}

// spec.noc.portalOrigins is embedded in a JSON array inside a single-quoted `bash -c`
// argument for kcadm, so only a plain scheme://host[:port] survives both layers. Checking it
// at manifest level turns a mid-deploy step failure into an error before anything is
// provisioned — and rejects the wildcard that finding R1-10.7 removed from this client.
func TestNOCPortalOriginsMustBePlainOrigins(t *testing.T) {
	for _, bad := range []string{
		"*",
		"http://localhost:3030/",
		"http://localhost:3030/noc",
		"localhost:3030",
		"http://*.example.org:3030",
	} {
		pd := mustLoad(t, "found-spoke.yaml")
		if pd.Spec.NOC == nil {
			pd.Spec.NOC = &NOC{}
		}
		pd.Spec.NOC.PortalOrigins = []string{bad}
		if res := Validate(pd); !findErr(res, "spec.noc.portalOrigins[0]") {
			t.Errorf("portalOrigins %q was accepted; it cannot be embedded safely: %+v", bad, res.Errors)
		}
	}
}

func TestNOCPortalOriginsAcceptsPlainOrigins(t *testing.T) {
	pd := mustLoad(t, "found-spoke.yaml")
	if pd.Spec.NOC == nil {
		pd.Spec.NOC = &NOC{}
	}
	pd.Spec.NOC.PortalOrigins = []string{"http://localhost:3030", "https://noc.example.org", "http://10.0.0.9:3030"}
	if res := Validate(pd); findErr(res, "spec.noc.portalOrigins[0]") ||
		findErr(res, "spec.noc.portalOrigins[1]") || findErr(res, "spec.noc.portalOrigins[2]") {
		t.Fatalf("valid portal origins were rejected: %+v", res.Errors)
	}
}
