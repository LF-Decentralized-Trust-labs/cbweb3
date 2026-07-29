// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"strings"
	"testing"
)

func validNOC() NOCBundle {
	return NOCBundle{
		Version:      NOCBundleVersion,
		SpokeID:      "spoke-brazil",
		SpokeUUID:    "a1000000-0000-0000-0000-000000000001",
		Name:         "Central Bank Brazil",
		CurrencyCode: "BRL",
		Jurisdiction: "Brazil",
		Components: []NOCComponent{
			{Name: "besu-central-bank", Type: "BESU", Endpoint: "http://host:8645", ContainerName: "cbweb3-central-bank-besu"},
			{Name: "paladin-central-bank", Type: "PALADIN", Endpoint: "http://host:8548"},
		},
	}
}

func TestEmitLoadNOCRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p, err := EmitNOC(validNOC(), dir)
	if err != nil {
		t.Fatalf("EmitNOC: %v", err)
	}
	if !strings.HasSuffix(p, "spoke-brazil.noc.bundle.yaml") {
		t.Fatalf("path = %s", p)
	}
	got, err := LoadNOC(p)
	if err != nil {
		t.Fatalf("LoadNOC: %v", err)
	}
	if got.SpokeUUID != "a1000000-0000-0000-0000-000000000001" || len(got.Components) != 2 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.Components[1].Type != "PALADIN" {
		t.Fatalf("component type mismatch: %+v", got.Components)
	}
}

func TestValidateNOCRejectsMissingAndBad(t *testing.T) {
	cases := map[string]func(*NOCBundle){
		"spokeUuid":    func(b *NOCBundle) { b.SpokeUUID = "" },
		"name":         func(b *NOCBundle) { b.Name = "" },
		"currencyCode": func(b *NOCBundle) { b.CurrencyCode = "" },
		"jurisdiction": func(b *NOCBundle) { b.Jurisdiction = "" },
		"components":   func(b *NOCBundle) { b.Components = nil },
		"badUUID":      func(b *NOCBundle) { b.SpokeUUID = "spoke-brazil" },
		"badType":      func(b *NOCBundle) { b.Components[0].Type = "PAYMENT_ORCHESTRATOR" },
	}
	for name, mut := range cases {
		b := validNOC()
		mut(&b)
		if err := ValidateNOC(b); err == nil {
			t.Errorf("expected error for %s", name)
		}
	}
}

func TestValidateNOCRejectsSecrets(t *testing.T) {
	b := validNOC()
	b.Components[0].Endpoint = "http://x/----BEGIN PRIVATE KEY---- leaked"
	if err := ValidateNOC(b); err == nil || !strings.Contains(err.Error(), "no-secrets") {
		t.Fatalf("expected no-secrets rejection, got %v", err)
	}
}
