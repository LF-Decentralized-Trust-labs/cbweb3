// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"strings"
	"testing"
)

func validNOC() NOCBundle {
	return NOCBundle{
		Version:      NOCBundleVersion,
		SpokeID:      "spoke-brl",
		SpokeUUID:    "b1000000-0000-0000-0000-000000000001",
		Name:         "Central Bank Brazil",
		CurrencyCode: "BRL",
		Jurisdiction: "Brazil",
		Components: []NOCComponent{
			{Name: "besu-central-bank", Type: "BESU", Endpoint: "http://host.docker.internal:8645", ContainerName: "sc-b-cbweb3-spoke-brl-central-bank-besu"},
			{Name: "cacti-relay", Type: "CACTI_RELAY", Endpoint: "http://host.docker.internal:4000"},
		},
	}
}

func TestEmitLoadNOCRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p, err := EmitNOC(validNOC(), dir)
	if err != nil {
		t.Fatalf("EmitNOC: %v", err)
	}
	if !strings.HasSuffix(p, "spoke-brl.noc.bundle.yaml") {
		t.Fatalf("path = %s", p)
	}
	got, err := LoadNOC(p)
	if err != nil {
		t.Fatalf("LoadNOC: %v", err)
	}
	if got.SpokeUUID != "b1000000-0000-0000-0000-000000000001" || got.CurrencyCode != "BRL" || len(got.Components) != 2 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.Components[1].Type != "CACTI_RELAY" {
		t.Fatalf("component type mismatch: %+v", got.Components)
	}
}

func TestEmitNOCDefaultsVersion(t *testing.T) {
	b := validNOC()
	b.Version = ""
	dir := t.TempDir()
	p, err := EmitNOC(b, dir)
	if err != nil {
		t.Fatalf("EmitNOC: %v", err)
	}
	got, err := LoadNOC(p)
	if err != nil {
		t.Fatalf("LoadNOC: %v", err)
	}
	if got.Version != NOCBundleVersion {
		t.Fatalf("version not defaulted: %q", got.Version)
	}
}

func TestValidateNOCRejectsMissing(t *testing.T) {
	cases := map[string]func(*NOCBundle){
		"spokeUuid":    func(b *NOCBundle) { b.SpokeUUID = "" },
		"name":         func(b *NOCBundle) { b.Name = "" },
		"currencyCode": func(b *NOCBundle) { b.CurrencyCode = "" },
		"jurisdiction": func(b *NOCBundle) { b.Jurisdiction = "" },
		"components":   func(b *NOCBundle) { b.Components = nil },
	}
	for name, mut := range cases {
		b := validNOC()
		mut(&b)
		if err := ValidateNOC(b); err == nil {
			t.Errorf("expected error for missing %s", name)
		}
	}
}

func TestValidateNOCRejectsBadUUID(t *testing.T) {
	b := validNOC()
	b.SpokeUUID = "spoke-brl" // a name, not a UUID — backend uuid.Parse would 403
	if err := ValidateNOC(b); err == nil || !strings.Contains(err.Error(), "UUID") {
		t.Fatalf("expected UUID rejection, got %v", err)
	}
}

func TestValidateNOCRejectsBadComponentType(t *testing.T) {
	b := validNOC()
	b.Components[0].Type = "PAYMENT_ORCHESTRATOR" // agent has no probe for it
	if err := ValidateNOC(b); err == nil || !strings.Contains(err.Error(), "invalid type") {
		t.Fatalf("expected invalid-type rejection, got %v", err)
	}
}

func TestValidateNOCRejectsSecrets(t *testing.T) {
	b := validNOC()
	b.Components[0].Endpoint = "http://x/----BEGIN PRIVATE KEY---- leaked"
	if err := ValidateNOC(b); err == nil || !strings.Contains(err.Error(), "no-secrets") {
		t.Fatalf("expected no-secrets rejection, got %v", err)
	}
}
