// SPDX-License-Identifier: Apache-2.0

package pki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	sharedpki "github.com/LACNetNetworks/cbweb3-platform/backend/shared/identity"
)

// TestNewCAFromEnvRefusesAMismatchedPair is the check whose absence cost weeks.
//
// Two producers have owned {bankCode}.crt and {bankCode}.key — the toolkit's gen-tls step
// and this service's PKI bootstrap. When they disagreed, the process started, logged
// "CA loaded from disk", and failed only at the first bank's onboarding with an x509 error
// that named neither file and pointed at neither producer.
func TestNewCAFromEnvRefusesAMismatchedPair(t *testing.T) {
	dir := t.TempDir()

	certPEM, _, err := sharedpki.GenerateSelfSignedCA("cb", "org", 1)
	if err != nil {
		t.Fatalf("cert: %v", err)
	}
	_, otherKeyPEM, err := sharedpki.GenerateSelfSignedCA("cb", "org", 1)
	if err != nil {
		t.Fatalf("key: %v", err)
	}

	certPath := filepath.Join(dir, "cb.crt")
	keyPath := filepath.Join(dir, "cb.key")
	write(t, certPath, certPEM)
	write(t, keyPath, otherKeyPEM)

	t.Setenv("CA_CERT_FILE", certPath)
	t.Setenv("CA_KEY_FILE", keyPath)

	ca, err := NewCAFromEnv()
	if err == nil {
		t.Fatal("a certificate and a key that are not each other's loaded as a CA; " +
			"the process would report success and fail at the first issuance instead")
	}
	if ca != nil {
		t.Error("a CA was returned alongside the error")
	}
	// The operator has to know WHICH files, or the message sends them reading x509 internals.
	for _, want := range []string{certPath, keyPath} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not name %s: %v", want, err)
		}
	}
}

// TestNewCAFromEnvAcceptsAMatchedPair keeps the check from being satisfied by refusing
// everything.
func TestNewCAFromEnvAcceptsAMatchedPair(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM, err := sharedpki.GenerateSelfSignedCA("cb", "org", 1)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	certPath := filepath.Join(dir, "cb.crt")
	keyPath := filepath.Join(dir, "cb.key")
	write(t, certPath, certPEM)
	write(t, keyPath, keyPEM)

	t.Setenv("CA_CERT_FILE", certPath)
	t.Setenv("CA_KEY_FILE", keyPath)

	ca, err := NewCAFromEnv()
	if err != nil {
		t.Fatalf("a matched pair was refused: %v", err)
	}
	// It must also actually sign — the pairing check is a proxy for this.
	csr, _, err := sharedpki.GenerateCSR("bank", "org", "ROLE_COMMERCIAL_BANK", "BR")
	if err != nil {
		t.Fatalf("csr: %v", err)
	}
	if _, err := ca.SignCSR(csr); err != nil {
		t.Fatalf("the accepted CA cannot sign: %v", err)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
