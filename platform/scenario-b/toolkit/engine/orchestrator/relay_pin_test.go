// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// The Cacti relay is the one peer that will never be an onboarded participant: it calls a central
// bank's internal bridge-out endpoint but has no CSR and no entry in the participants table. So it is
// pinned by FILE, which is exactly the case the union of pin sources keeps files for.
//
// Both halves live in the toolkit because it is the only component that can write to both named
// volumes: the relay's data volume (where its private key must land, never on the host tree) and each
// CB's PKI volume (where the certificate must be pinned). A host path would put the relay's private
// key in the repository working tree, and an HTTP fetch would make the pin trust-on-first-use — a weak
// trust path inside the mechanism that exists to strengthen trust.

func TestEnsureRelayIdentityWritesKeyAndCert(t *testing.T) {
	r := &exec.FakeRunner{}
	if err := ensureRelayIdentity(context.Background(), r, "cbweb3-relay_data", "cacti-relay"); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	var wroteKey, wroteCert bool
	var keyMode string
	for _, c := range r.Calls {
		joined := strings.Join(c.Args, " ")
		if strings.Contains(joined, "cacti-relay.key") && strings.Contains(joined, "base64 -d") {
			wroteKey = true
			if strings.Contains(joined, "chmod 0600") {
				keyMode = "0600"
			}
		}
		if strings.Contains(joined, "cacti-relay.crt") && strings.Contains(joined, "base64 -d") {
			wroteCert = true
		}
	}
	if !wroteKey || !wroteCert {
		t.Fatalf("expected both key and certificate to be written: key=%v cert=%v", wroteKey, wroteCert)
	}
	// The private key must not be world-readable inside the volume.
	if keyMode != "0600" {
		t.Fatalf("the private key was not written 0600")
	}
}

// Copying the certificate must move ONLY the certificate. Carrying the key across would put the
// relay's private key inside every central bank's volume — every CB could then impersonate the relay.
func TestPinPeerCertCopiesOnlyTheCertificate(t *testing.T) {
	r := &exec.FakeRunner{Outputs: map[string][]byte{
		"docker": []byte("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"),
	}}
	if err := pinPeerCert(context.Background(), r, "cbweb3-relay_data", "cb_tls", "cacti-relay"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	for _, c := range r.Calls {
		joined := strings.Join(c.Args, " ")
		if strings.Contains(joined, "cacti-relay.key") {
			t.Fatalf("the relay's PRIVATE KEY was touched while distributing its certificate: %v", c.Args)
		}
	}
}

// A missing source certificate must fail loudly rather than pin an empty file: an empty pin would
// make the registry non-empty and reject every signed request from that peer.
func TestPinPeerCertRefusesAnEmptySource(t *testing.T) {
	r := &exec.FakeRunner{Outputs: map[string][]byte{"docker": []byte("   ")}}
	if err := pinPeerCert(context.Background(), r, "src", "dst", "cacti-relay"); err == nil {
		t.Fatal("expected an empty source certificate to be refused")
	}
}

// A peer that is not a central bank has no CA in its own volume — the relay is the case in point,
// since the CA lives in each CB's volume. That is the NORMAL path, and it must be distinguishable from
// a CA that exists and cannot issue: reporting both the same way sent an operator looking for a
// problem that did not exist during the first clean deploy.
func TestIssueFromVolumeCADistinguishesAbsentFromUnusable(t *testing.T) {
	// Absent: every read fails.
	absent := &exec.FakeRunner{Errs: map[string]error{"docker": errors.New("cat: can't open")}}
	if _, _, err := issueFromVolumeCA(context.Background(), absent, "cbweb3-relay_data", "cacti-relay"); !errors.Is(err, errNoCAMaterial) {
		t.Fatalf("an absent CA must report errNoCAMaterial, got %v", err)
	}

	// Present but unusable: the read succeeds and returns something that is not a usable CA pair.
	unusable := &exec.FakeRunner{Outputs: map[string][]byte{"docker": []byte("not a certificate")}}
	_, _, err := issueFromVolumeCA(context.Background(), unusable, "cb_tls", "central-bank-brazil")
	if err == nil {
		t.Fatal("an unusable CA must surface an error")
	}
	if errors.Is(err, errNoCAMaterial) {
		t.Fatalf("an unusable CA must NOT be reported as absent — that hides an inconsistency worth a warning: %v", err)
	}
}
