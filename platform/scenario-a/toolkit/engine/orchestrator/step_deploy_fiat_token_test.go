// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDeployFiatTokenStep_Check_False_NoAddr(t *testing.T) {
	dir := t.TempDir()
	step := newDeployFiatTokenStep("spoke-test", dir, "http://localhost:8645", FiatTokenMetadata{Currency: "BRL"}, nil, "/artifact.json", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should be false when FIAT_TOKEN_ADDRESS is absent")
	}
}

func TestDeployFiatTokenStep_Check_True_AddrPresent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("FIAT_TOKEN_ADDRESS=0xF1A7\n"), 0o644)
	step := newDeployFiatTokenStep("spoke-test", dir, "http://localhost:8645", FiatTokenMetadata{Currency: "BRL"}, nil, "/artifact.json", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("check should be true when FIAT_TOKEN_ADDRESS is present")
	}
}

func TestDeployFiatTokenStep_Name(t *testing.T) {
	step := newDeployFiatTokenStep("spoke-test", t.TempDir(), "", FiatTokenMetadata{Currency: "BRL"}, nil, "", 0)
	if step.Name() != StepDeployFiatToken {
		t.Errorf("Name() = %q, want %q", step.Name(), StepDeployFiatToken)
	}
}

// R1-10.3: the fCeBM identity is per spoke. Absent overrides it derives from the
// manifest currency, keeping the "<prefix>_<ISO>" shape every spoke token follows;
// a manifest override wins verbatim.
func TestFiatTokenMetadata(t *testing.T) {
	for _, tc := range []struct {
		label      string
		meta       FiatTokenMetadata
		wantName   string
		wantSymbol string
	}{
		{"derived BRL", FiatTokenMetadata{Currency: "BRL"}, "Fiat BRL", "fCeBM_BRL"},
		{"derived COP", FiatTokenMetadata{Currency: "COP"}, "Fiat COP", "fCeBM_COP"},
		{
			"manifest override",
			FiatTokenMetadata{Currency: "BRL", Name: "Real Digital", Symbol: "fRD_BRL"},
			"Real Digital", "fRD_BRL",
		},
		{
			"partial override falls back per field",
			FiatTokenMetadata{Currency: "ARS", Symbol: "fARSx_ARS"},
			"Fiat ARS", "fARSx_ARS",
		},
	} {
		t.Run(tc.label, func(t *testing.T) {
			if got := tc.meta.name(); got != tc.wantName {
				t.Errorf("name() = %q, want %q", got, tc.wantName)
			}
			if got := tc.meta.symbol(); got != tc.wantSymbol {
				t.Errorf("symbol() = %q, want %q", got, tc.wantSymbol)
			}
		})
	}
}
