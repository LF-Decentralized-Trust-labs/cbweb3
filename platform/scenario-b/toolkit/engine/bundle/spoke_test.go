package bundle

import (
	"strings"
	"testing"
)

func validSpoke() SpokeBundle {
	return SpokeBundle{
		Version:  SpokeBundleVersion,
		SpokeID:  "spoke-a",
		ChainID:  1338,
		Enode:    "enode://abcd@host:30303",
		SpokeRPC: "http://host:8645",
		Genesis:  `{"config":{"chainId":1338}}`,
		Contracts: map[string]string{
			"identityRegistry": "0x01", "tCeBM": "0x02", "spokeBridge": "0x03", "fCeBM": "0x04",
		},
	}
}

func TestEmitLoadSpokeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p, err := EmitSpoke(validSpoke(), dir)
	if err != nil {
		t.Fatalf("EmitSpoke: %v", err)
	}
	if !strings.HasSuffix(p, "spoke-spoke-a.bundle.yaml") {
		t.Fatalf("path = %s", p)
	}
	got, err := LoadSpoke(p)
	if err != nil {
		t.Fatalf("LoadSpoke: %v", err)
	}
	if got.ChainID != 1338 || got.Enode != "enode://abcd@host:30303" || got.Contracts["spokeBridge"] != "0x03" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestValidateSpokeRejectsMissing(t *testing.T) {
	b := validSpoke()
	b.Enode = ""
	if err := ValidateSpoke(b); err == nil {
		t.Fatal("expected error for missing enode")
	}
	b = validSpoke()
	b.Genesis = ""
	if err := ValidateSpoke(b); err == nil {
		t.Fatal("expected error for missing genesis")
	}
	b = validSpoke()
	delete(b.Contracts, "fCeBM")
	if err := ValidateSpoke(b); err == nil {
		t.Fatal("expected error for missing contract")
	}
}

func TestValidateSpokeRejectsSecrets(t *testing.T) {
	b := validSpoke()
	b.Genesis = "----BEGIN PRIVATE KEY---- leaked"
	if err := ValidateSpoke(b); err == nil || !strings.Contains(err.Error(), "no-secrets") {
		t.Fatalf("expected no-secrets rejection, got %v", err)
	}
}
