// SPDX-License-Identifier: Apache-2.0

// Tests for the guard that refuses a Zeto lock whose locked-state ID cannot be
// settled afterwards.
//
// The defect being guarded against was reproduced live on 2026-08-20 (scenario A,
// Paladin v0.15.0-rc.1): transferLocked fails with
//
//	PD011814: Domain reverted transaction on assemble:
//	PD210134: Failed to query states by IDs. Wanted: 1, Found: 0
//
// permanently — five retries over 50s all failed — for a locked-state ID whose
// first BYTE is 0x00. Controls in the same run settled fine: 0x0a…, 0x06…, 0x29…,
// 0x2f…. That contrast is what defines the rule: a zero high *nibble* is harmless,
// a whole zero *byte* is not. The ID travels through the `uint256[]` lockedInputs
// parameter and comes back padded to whole bytes but not to the full 32-byte width,
// so the domain's state lookup misses.
//
// The ID in TestUnsettleableLockedStateID/real_world_failure is the actual one that
// failed, kept as a fixture rather than a synthetic value.
//
// Why a guard and not a fix: passing lockedInputs as bytes32[] was tried and the
// domain rejects it outright — "PD210016: Unexpected signature for function
// 'transferLocked': expected='function transferLocked(uint256[] memory ...'" — which
// would turn a 1-in-256 failure into every failure. The real fix lives in Paladin.
package paladin

import "testing"

func TestUnsettleableLockedStateID(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		ids       []string
		wantBad   string
		wantFound bool
	}{
		"real world failure": {
			ids:       []string{"0x00758ab137c47984ef49b1b7c3b073fb698f7366a8ad8ea98d8b915c4eb723b6"},
			wantBad:   "0x00758ab137c47984ef49b1b7c3b073fb698f7366a8ad8ea98d8b915c4eb723b6",
			wantFound: true,
		},
		// Zero high nibble is fine — these four settled in the same live run.
		"zero nibble 0a settles": {ids: []string{"0x0a6166eca978d3a555321234567890abcdef1234567890abcdef1234567890ab"}},
		"zero nibble 06 settles": {ids: []string{"0x0629df3404e2caeb14ed1234567890abcdef1234567890abcdef1234567890ab"}},
		"ordinary 29":            {ids: []string{"0x29e168c590cb81c6aedb1234567890abcdef1234567890abcdef1234567890ab"}},
		"ordinary 2f":            {ids: []string{"0x2f69fa562c7230f1d7f01234567890abcdef1234567890abcdef1234567890ab"}},
		"no 0x prefix is still checked": {
			ids:       []string{"00758ab137c47984ef49b1b7c3b073fb698f7366a8ad8ea98d8b915c4eb723b6"},
			wantBad:   "00758ab137c47984ef49b1b7c3b073fb698f7366a8ad8ea98d8b915c4eb723b6",
			wantFound: true,
		},
		"uppercase prefix is still checked": {
			ids:       []string{"0X00758AB137C47984EF49B1B7C3B073FB698F7366A8AD8EA98D8B915C4EB723B6"},
			wantBad:   "0X00758AB137C47984EF49B1B7C3B073FB698F7366A8AD8EA98D8B915C4EB723B6",
			wantFound: true,
		},
		// A lock can produce several states; one bad one poisons the settle, so the
		// guard must look at all of them and name the offender.
		"one bad among several": {
			ids: []string{
				"0x2f69fa562c7230f1d7f01234567890abcdef1234567890abcdef1234567890ab",
				"0x00758ab137c47984ef49b1b7c3b073fb698f7366a8ad8ea98d8b915c4eb723b6",
				"0x0a6166eca978d3a555321234567890abcdef1234567890abcdef1234567890ab",
			},
			wantBad:   "0x00758ab137c47984ef49b1b7c3b073fb698f7366a8ad8ea98d8b915c4eb723b6",
			wantFound: true,
		},
		"all good among several": {
			ids: []string{
				"0x2f69fa562c7230f1d7f01234567890abcdef1234567890abcdef1234567890ab",
				"0x0a6166eca978d3a555321234567890abcdef1234567890abcdef1234567890ab",
			},
		},
		"empty list": {ids: nil},
		// Do not invent a verdict for something that is not a 32-byte id: a short or
		// malformed value is not the failure this guard knows about, and blocking a
		// lock on a guess would be worse than letting the settle report the truth.
		"too short to judge":  {ids: []string{"0x00"}},
		"empty string":        {ids: []string{""}},
		"just the prefix":     {ids: []string{"0x"}},
		"not hex":             {ids: []string{"lock-reference-not-an-id"}},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, found := unsettleableLockedStateID(tc.ids)
			if found != tc.wantFound {
				t.Fatalf("found = %v, want %v (ids=%v)", found, tc.wantFound, tc.ids)
			}
			if got != tc.wantBad {
				t.Errorf("offending id = %q, want %q", got, tc.wantBad)
			}
		})
	}
}

// The error must name the id and say why, because the operator's next question is
// "which lock, and is my money gone" — and for this id both settle and refund are
// impossible, since the rollback path calls the same transferLocked.
func TestErrUnsettleableLockMessage(t *testing.T) {
	t.Parallel()
	id := "0x00758ab137c47984ef49b1b7c3b073fb698f7366a8ad8ea98d8b915c4eb723b6"
	err := errUnsettleableLock(id)
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	for _, want := range []string{id, "PD210134", "retry"} {
		if !contains(msg, want) {
			t.Errorf("error message %q does not mention %q", msg, want)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
