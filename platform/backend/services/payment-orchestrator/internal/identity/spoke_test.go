package identity
package identity_test

import (
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/identity"
)

func TestSpokePrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"standard spoke-a", "funded_operator@spoke-a-bank-a", "spoke-a"},
		{"standard spoke-b", "funded_operator@spoke-b-bank-d", "spoke-b"},
		{"no @ separator", "bank-b", ""},
		{"empty string", "", ""},
		{"no dash after @", "user@nodash", ""},
		{"multi-segment spoke", "user@spoke-alpha-1-bank-x", "spoke-alpha"},
		{"only @ no content", "user@", ""},
		{"@ with single segment", "user@spoke", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := identity.SpokePrefix(tt.input)
			if got != tt.expected {
				t.Errorf("SpokePrefix(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
