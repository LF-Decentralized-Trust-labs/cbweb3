// SPDX-License-Identifier: Apache-2.0

package manifest_test

import (
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
)

func TestDisplayNameOr(t *testing.T) {
	tests := []struct {
		name     string
		display  string
		fallback string
		want     string
	}{
		{"uses displayName when set", "Itaú", "bank-itau", "Itaú"},
		{"falls back when empty", "", "bank-itau", "bank-itau"},
		{"falls back to entity name", "", "central-bank-brazil", "central-bank-brazil"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &manifest.Manifest{Spec: manifest.Spec{DisplayName: tt.display}}
			if got := m.DisplayNameOr(tt.fallback); got != tt.want {
				t.Errorf("DisplayNameOr(%q) with displayName %q = %q, want %q",
					tt.fallback, tt.display, got, tt.want)
			}
		})
	}
}
