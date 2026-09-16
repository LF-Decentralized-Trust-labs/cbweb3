// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"sort"
	"testing"
)

// lnetRoster is the identity roster of the deployed LNET environment, verbatim. It is
// the fixture that matters: the defect this guards produced the unroutable spoke id
// "spoke-costa" from exactly these entries, because a two-word country makes a
// positional split return part of the country name.
var lnetRoster = []string{
	"funded_operator@spoke-chile-cb",
	"funded_operator@spoke-chile-cb3",
	"funded_operator@spoke-chile-cb4",
	"funded_operator@spoke-costa-rica-cb",
	"funded_operator@spoke-costa-rica-cb1",
	"funded_operator@spoke-costa-rica-cb2",
	"funded_operator@spoke-peru-cb",
	"funded_operator@spoke-peru-cb5",
	"funded_operator@spoke-peru-cb6",
}

// samplesRoster is the other live convention: hyphenated bank ids under a one-word
// country. Both must yield the same spoke set, or the server repeats the client-side
// mistake in the opposite direction.
var samplesRoster = []string{
	"funded_operator@spoke-brl-cb",
	"funded_operator@spoke-brl-bank-itau",
	"funded_operator@spoke-brl-bank-bradesco",
	"funded_operator@spoke-costa-rica-cb",
	"funded_operator@spoke-costa-rica-cb1",
	"funded_operator@spoke-costa-rica-cb2",
}

func sortedSpokes(roster []string) []string {
	set := spokeIDsFromRoster(roster)
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func TestSpokeIDsFromRoster_BothConventions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		roster []string
		want   []string
	}{
		{"LNET: bare bank ids, one two-word country", lnetRoster, []string{"spoke-chile", "spoke-costa-rica", "spoke-peru"}},
		{"samples: hyphenated bank ids", samplesRoster, []string{"spoke-brl", "spoke-costa-rica"}},
		{"a roster with no central bank yields nothing", []string{"funded_operator@spoke-brl-bank-itau"}, []string{}},
		{"empty roster", nil, []string{}},
	} {
		got := sortedSpokes(tc.roster)
		if len(got) != len(tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
				break
			}
		}
	}
}

// TestUnknownSpokeIDs_RejectsThePrefix is the case the card names explicitly: a valid
// PREFIX of a real spoke id must be rejected. A prefix-tolerant check would accept
// "spoke-costa" — the exact value that reached immutable Pente storage — and would
// still pass a careless test.
func TestUnknownSpokeIDs_RejectsThePrefix(t *testing.T) {
	spokes := spokeIDsFromRoster(lnetRoster)

	if got := unknownSpokeIDs(spokes, "spoke-costa-rica"); len(got) != 0 {
		t.Errorf("the real spoke id must be accepted, got invalid=%v", got)
	}
	got := unknownSpokeIDs(spokes, "spoke-costa")
	if len(got) != 1 || got[0] != "spoke-costa" {
		t.Errorf(`"spoke-costa" is a prefix of a real spoke id and must be rejected, got %v`, got)
	}
	// The other direction: a longer string that merely starts with a real spoke id.
	if got := unknownSpokeIDs(spokes, "spoke-peru-extra"); len(got) != 1 {
		t.Errorf("a value extending a real spoke id must be rejected, got %v", got)
	}
}

func TestUnknownSpokeIDs_Behaviour(t *testing.T) {
	spokes := spokeIDsFromRoster(lnetRoster)

	for _, tc := range []struct {
		name string
		ids  []string
		want []string
	}{
		{"both valid", []string{"spoke-chile", "spoke-peru"}, nil},
		{"one invalid, named", []string{"spoke-chile", "spoke-nowhere"}, []string{"spoke-nowhere"}},
		{"both invalid, order preserved", []string{"spoke-b", "spoke-a"}, []string{"spoke-b", "spoke-a"}},
		{"duplicates reported once", []string{"spoke-x", "spoke-x"}, []string{"spoke-x"}},
		{"empty values skipped", []string{"", "   ", "spoke-chile"}, nil},
		{"whitespace trimmed before comparing", []string{"  spoke-peru  "}, nil},
	} {
		got := unknownSpokeIDs(spokes, tc.ids...)
		if len(got) != len(tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
				break
			}
		}
	}
}

// TestUnknownSpokeIDs_EmptySetFailsOpen pins the documented trade-off rather than
// leaving it to be discovered. A roster carrying no central-bank entry cannot yield a
// spoke set, and rejecting every propose in that case would be a worse outage than the
// defect; the caller logs a warning so it is visible.
func TestUnknownSpokeIDs_EmptySetFailsOpen(t *testing.T) {
	if got := unknownSpokeIDs(map[string]struct{}{}, "spoke-anything"); got != nil {
		t.Errorf("an empty spoke set must validate nothing, got %v", got)
	}
}
