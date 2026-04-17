// Package identity provides utilities for parsing Paladin identity strings.
package identity

import "strings"

// SpokePrefix extracts the spoke identifier from a Paladin identity string.
// e.g. "funded_operator@spoke-a-bank-a" → "spoke-a"
// Returns empty string if the format is unexpected.
func SpokePrefix(paladinIdentity string) string {
	parts := strings.SplitN(paladinIdentity, "@", 2)
	if len(parts) < 2 {
		return ""
	}
	segs := strings.SplitN(parts[1], "-", 3)
	if len(segs) < 2 {
		return ""
	}
	return segs[0] + "-" + segs[1]
}
