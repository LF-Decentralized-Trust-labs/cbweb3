// SPDX-License-Identifier: Apache-2.0

// Package identity provides utilities for parsing spoke identity strings.
package identity

import "strings"

// SpokePrefix extracts the spoke identifier from an identity string.
// e.g. "funded_operator@spoke-a-bank-a" → "spoke-a"
// Returns empty string if the format is unexpected.
func SpokePrefix(identity string) string {
	parts := strings.SplitN(identity, "@", 2)
	if len(parts) < 2 {
		return ""
	}
	segs := strings.SplitN(parts[1], "-", 3)
	if len(segs) < 2 {
		return ""
	}
	return segs[0] + "-" + segs[1]
}
