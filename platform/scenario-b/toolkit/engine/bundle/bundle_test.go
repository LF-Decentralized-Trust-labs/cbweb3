package bundle

import (
	"strings"
	"testing"
)

func validHub() HubBundle {
	return HubBundle{
		Version: HubBundleVersion,
		ChainID: 1337,
		HubRPC:  "http://host.docker.internal:8845",
		HubWS:   "ws://host.docker.internal:8846",
		Contracts: map[string]string{
			"identityRegistry": "0x01",
			"fxAgreement":      "0x04", "pairRegistry": "0x05", "currencyRegistry": "0x06",
			"manualOracle": "0x07",
		},
	}
}

func TestEmitLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p, err := EmitHub(validHub(), dir)
	if err != nil {
		t.Fatalf("EmitHub: %v", err)
	}
	got, err := LoadHub(p)
	if err != nil {
		t.Fatalf("LoadHub: %v", err)
	}
	if got.ChainID != 1337 || got.Contracts["identityRegistry"] != "0x01" || got.Version == "" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if len(got.Contracts) != len(RequiredContracts) {
		t.Fatalf("contracts = %d, want %d", len(got.Contracts), len(RequiredContracts))
	}
}

func TestValidateRejectsMissingFields(t *testing.T) {
	b := validHub()
	b.HubRPC = ""
	if err := ValidateHub(b); err == nil {
		t.Fatal("expected error for missing hubRpc")
	}
	b = validHub()
	delete(b.Contracts, "manualOracle")
	if err := ValidateHub(b); err == nil {
		t.Fatal("expected error for missing contract")
	}
	b = validHub()
	b.ChainID = 0
	if err := ValidateHub(b); err == nil {
		t.Fatal("expected error for missing chainId")
	}
}

// SC-007: no secrets.
func TestValidateRejectsPrivateKeyMaterial(t *testing.T) {
	b := validHub()
	b.Contracts["leak"] = "-----BEGIN PRIVATE KEY----- oops"
	err := ValidateHub(b)
	if err == nil || !strings.Contains(err.Error(), "no-secrets") {
		t.Fatalf("expected no-secrets rejection, got %v", err)
	}
}
