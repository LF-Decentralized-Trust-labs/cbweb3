// SPDX-License-Identifier: Apache-2.0

package apply

import "testing"

// provisioning/templates/vars/NAMING.md documents how the entity prefixes are derived. It used to
// say they come from ENTITY with "hífens → underscores", and the direction was backwards: this
// function turns underscores INTO hyphens, and the source is metadata.name, not ENTITY.
//
// The doc had been wrong long enough for someone to copy its example. This pins the direction so
// the code cannot drift away from the corrected text without saying so.
func TestSanitizePrefix_MatchesTheDocumentedRule(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"bank_a", "bank-a"},                           // underscore -> hyphen, NOT the reverse
		{"bank-a", "bank-a"},                           // hyphens are kept
		{"Central Bank/Brazil", "central-bank-brazil"}, // lowercase; space and slash -> hyphen
		{"  padded  ", "padded"},
		{"", "cbweb3"}, // the documented fallback
	} {
		if got := sanitizePrefix(tc.in); got != tc.want {
			t.Errorf("sanitizePrefix(%q) = %q, want %q — NAMING.md documents this mapping; change "+
				"both or neither", tc.in, got, tc.want)
		}
	}
}
