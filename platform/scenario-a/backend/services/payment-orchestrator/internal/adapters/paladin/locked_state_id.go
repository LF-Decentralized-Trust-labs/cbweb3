// SPDX-License-Identifier: Apache-2.0

package paladin

import (
	"fmt"
	"strings"
)

// stateIDHexDigits is the width of a Zeto state id in hex digits (32 bytes).
const stateIDHexDigits = 64

// unsettleableLockedStateID returns the first locked-state id that this Paladin
// build cannot settle, and whether one was found.
//
// A Zeto locked-state id whose first BYTE is zero cannot be spent by
// transferLocked on Paladin v0.15.0-rc.1. The id is carried through the
// `uint256[] lockedInputs` parameter, and the domain re-serialises it padded to
// whole bytes but NOT to the full 32-byte width, so a 62-digit value is looked up
// as 62 digits and the state store — which keys on the full width — returns
// nothing:
//
//	PD011814: Domain reverted transaction on assemble:
//	PD210134: Failed to query states by IDs. Wanted: 1, Found: 0
//
// Reproduced live on 2026-08-20 and permanent: five retries over 50s all failed,
// while ids beginning 0x0a, 0x06, 0x29 and 0x2f settled normally in the same run.
// A zero high nibble is therefore harmless; only a whole zero byte is fatal.
//
// Only full-width ids are judged. Anything shorter or non-hex is left alone: it is
// not the failure documented here, and refusing a lock on a guess would be worse
// than letting the settle report the truth.
func unsettleableLockedStateID(ids []string) (string, bool) {
	for _, id := range ids {
		digits := strings.TrimPrefix(strings.TrimPrefix(id, "0x"), "0X")
		if len(digits) != stateIDHexDigits {
			continue
		}
		if digits[0] == '0' && digits[1] == '0' {
			return id, true
		}
	}
	return "", false
}

// errUnsettleableLock explains a lock that was refused, in the terms the operator
// needs: which id, why it cannot be settled, and that retrying is the way out
// (state ids are effectively random, so a fresh lock has ~255/256 odds of being
// usable).
//
// The tokens of the refused lock are already locked on-chain by the time the id is
// known, and they cannot be recovered: the rollback path calls the same
// transferLocked, so it fails identically. That is stated plainly rather than
// hidden, because the alternative — proceeding — creates a cross-spoke commitment
// that can never settle and, worse, halts the relay's event cursor for every other
// trade behind it.
func errUnsettleableLock(id string) error {
	return fmt.Errorf(
		"locked state %s begins with a zero byte, which this Paladin build cannot spend "+
			"via transferLocked (PD210134: state lookup misses because the id is not padded "+
			"back to 32 bytes); refusing the lock instead of creating an HTLC that can never "+
			"settle — retry the lock to obtain a different state id. The amount locked by this "+
			"attempt cannot be released, because the rollback path uses the same call",
		id)
}
