// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"path/filepath"
	"strings"
	"testing"
)

// A manifest's node.dataDir is relative, so it resolves against the working directory. Running apply
// from a different directory therefore points the whole entity at a fresh data dir — where gen-csr's
// check (does the key file exist?) finds nothing and mints a NEW keypair. Compose then rebinds the
// running gateway to that directory, and the bank starts signing with a key its central bank has no
// certificate for: every internal call fails with "signature does not verify", and the old key is
// still sitting in the original directory, untouched and unused.
//
// Nothing in that sequence errors. This guard makes it error, before the key is written.

func TestIdentityDirConflict_RefusesADifferentDirectoryThanTheRunningGateway(t *testing.T) {
	err := identityDirConflict("bank-itau",
		"/repo/cbweb3-data/bank-itau/pki",
		"/repo/scenario-b/samples/cbweb3-data/bank-itau/pki")
	if err == nil {
		t.Fatal("no error; minting a second identity for a bank that is already running must not be silent")
	}
	// The operator has to be able to act on it, which means seeing both paths.
	for _, want := range []string{"bank-itau", "/repo/cbweb3-data/bank-itau/pki", "/repo/scenario-b/samples/cbweb3-data/bank-itau/pki"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q: %v", want, err)
		}
	}
}

func TestIdentityDirConflict_AllowsTheSameDirectory(t *testing.T) {
	if err := identityDirConflict("bank-itau", "/repo/data/bank-itau/pki", "/repo/data/bank-itau/pki"); err != nil {
		t.Fatalf("same directory reported a conflict: %v", err)
	}
}

// Path spelling is not identity: the same directory written differently must not trip the guard.
func TestIdentityDirConflict_NormalizesPathSpelling(t *testing.T) {
	cases := [][2]string{
		{"/repo/data/bank-itau/pki/", "/repo/data/bank-itau/pki"},
		{"/repo/data/./bank-itau/pki", "/repo/data/bank-itau/pki"},
		{"/repo/data/other/../bank-itau/pki", "/repo/data/bank-itau/pki"},
	}
	for _, tc := range cases {
		if err := identityDirConflict("bank-itau", tc[0], tc[1]); err != nil {
			t.Errorf("identityDirConflict(%q, %q) = %v; want nil", tc[0], tc[1], err)
		}
	}
}

// A relative resolved dir is the very shape that causes the bug, so it must be compared as the
// absolute path it will actually become.
func TestIdentityDirConflict_ResolvesARelativeDirectoryBeforeComparing(t *testing.T) {
	abs, err := filepath.Abs(filepath.Join("cbweb3-data", "bank-itau", "pki"))
	if err != nil {
		t.Fatal(err)
	}
	if err := identityDirConflict("bank-itau", filepath.Join("cbweb3-data", "bank-itau", "pki"), abs); err != nil {
		t.Fatalf("a relative path was not resolved before comparing: %v", err)
	}
}

// Nothing bound means nothing is running yet: a first join must not be blocked.
func TestIdentityDirConflict_AllowsAFirstJoin(t *testing.T) {
	if err := identityDirConflict("bank-itau", "/repo/data/bank-itau/pki", ""); err != nil {
		t.Fatalf("a first join was blocked: %v", err)
	}
	if err := identityDirConflict("bank-itau", "/repo/data/bank-itau/pki", "   "); err != nil {
		t.Fatalf("a blank bound path was treated as a conflict: %v", err)
	}
}

func TestParsePKIMountSource_PicksTheBindForThePKIDestination(t *testing.T) {
	// One line per mount, as emitted by the inspect format the guard asks for.
	out := "/host/paladin\x00/workspace/paladin\n" +
		"/host/samples/cbweb3-data/bank-itau/pki\x00/workspace/backend/config/pki\n" +
		"/host/tls\x00/workspace/tls\n"

	got := parsePKIMountSource([]byte(out))

	if got != "/host/samples/cbweb3-data/bank-itau/pki" {
		t.Fatalf("parsePKIMountSource() = %q; want the pki bind source", got)
	}
}

func TestParsePKIMountSource_EmptyWhenAbsent(t *testing.T) {
	if got := parsePKIMountSource([]byte("/host/paladin\x00/workspace/paladin\n")); got != "" {
		t.Fatalf("parsePKIMountSource() = %q; want empty", got)
	}
	if got := parsePKIMountSource(nil); got != "" {
		t.Fatalf("parsePKIMountSource(nil) = %q; want empty", got)
	}
}
