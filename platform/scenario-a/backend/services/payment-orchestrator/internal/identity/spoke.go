// Package identity provides utilities for parsing Paladin identity strings.
package identity

import (
	"fmt"
	"strings"
)

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

// BankID extracts the institution identifier from a Paladin identity string.
// e.g. "funded_operator@spoke-a-bank-a" → "bank-a"
//
// The Paladin identity format "{name}@{spoke-word}-{spoke-letter}-{bankID}" is
// structural to this function: the bankID is the third dash-delimited segment
// after the "@". If Paladin ever changes this naming convention, this function
// will start returning errors and all callers will need to be updated.
//
// During testing, parse errors here signal a format drift that requires updating
// the implementation before authorization logic can be trusted.
func BankID(paladinIdentity string) (string, error) {
	parts := strings.SplitN(paladinIdentity, "@", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("identity.BankID: missing '@' in %q — expected format {name}@{spoke-word}-{letter}-{bankID}", paladinIdentity)
	}
	segs := strings.SplitN(parts[1], "-", 3)
	if len(segs) < 3 {
		return "", fmt.Errorf("identity.BankID: fewer than three dash-segments in %q — expected format spoke-{letter}-{bankID}", parts[1])
	}
	return segs[2], nil
}
