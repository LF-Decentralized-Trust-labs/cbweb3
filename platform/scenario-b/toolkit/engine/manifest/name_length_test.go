// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"strings"
	"testing"
)

// PR #226 fixed the 63-octet DNS label overflow (RFC 1035) by moving the proxy's upstreams to
// short network aliases built from the entity's network prefix, and described the result as
// "short whatever the entity is called".
//
// That was not true by construction. The network prefix is sanitizePrefix(metadata.name), and
// nothing rejected a long name — the only protection was a spot-check of four names chosen by
// hand. A 60-character name passed validation, produced an alias past the limit, and Docker's
// resolver refused it: back to the 502 that PR closed, with no guard saying so.
//
// The bound belongs here rather than in a test of the rendered names, because this is the last
// place it can be caught BEFORE anything is created. An apply that fails halfway leaves
// containers behind; a manifest rejected at validation leaves nothing.

func manifestWithName(name string) *ParticipantDeployment {
	pd := &ParticipantDeployment{}
	pd.APIVersion = wantAPIVersion
	pd.Kind = wantKind
	pd.Metadata.Name = name
	return pd
}

// findingFor returns the error message recorded against field, or "".
func findingFor(r Result, field string) string {
	for _, e := range r.Errors {
		if e.Field == field {
			return e.Message
		}
	}
	return ""
}

func TestMetadataName_AtTheLimitIsAccepted(t *testing.T) {
	name := strings.Repeat("a", MaxMetadataNameLen)
	if msg := findingFor(Validate(manifestWithName(name)), "metadata.name"); msg != "" {
		t.Errorf("a name of exactly %d octets was rejected: %s", MaxMetadataNameLen, msg)
	}
}

func TestMetadataName_OneOctetOverIsRejected(t *testing.T) {
	name := strings.Repeat("a", MaxMetadataNameLen+1)
	msg := findingFor(Validate(manifestWithName(name)), "metadata.name")
	if msg == "" {
		t.Fatalf("a name of %d octets was accepted; it produces a network alias past the "+
			"63-octet DNS label limit, which Docker's resolver refuses", len(name))
	}
	// The message has to teach, not just refuse. An operator who reads "invalid" learns nothing
	// and shortens the name by guesswork.
	for _, want := range []string{"63", "DNS"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal does not mention %q, so it does not say WHY: %s", want, msg)
		}
	}
	if !strings.Contains(msg, "alias") && !strings.Contains(msg, "proxy") {
		t.Errorf("the refusal does not name what breaks: %s", msg)
	}
}

// The check must not fire on the names actually in use, or it is a broken gate rather than a
// guard. The samples are the deployment names this project ships.
func TestMetadataName_DeployedNamesFitComfortably(t *testing.T) {
	for _, name := range []string{
		"central-bank-brazil", "central-bank-colombia", "central-bank-argentina",
		"bank-itau", "bank-bradesco", "bank-galicia", "bank-macro",
		"bank-bancolombia", "bank-davivienda", "hub-cbweb3",
	} {
		if msg := findingFor(Validate(manifestWithName(name)), "metadata.name"); msg != "" {
			t.Errorf("%q is a name this project deploys and validation rejected it: %s", name, msg)
		}
	}
}

// An empty name is already "required field is missing"; the length check must not replace that
// with something less useful.
func TestMetadataName_EmptyStillReportsMissing(t *testing.T) {
	if msg := findingFor(Validate(manifestWithName("")), "metadata.name"); !strings.Contains(msg, "required") {
		t.Errorf("an empty name should report the missing-field error, got: %q", msg)
	}
}
