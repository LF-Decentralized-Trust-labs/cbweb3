// SPDX-License-Identifier: Apache-2.0

package pki

import (
	"crypto/x509"
	"encoding/pem"
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

// toPKCS8 re-encodes a SEC1 ("EC PRIVATE KEY") PEM as PKCS#8 ("PRIVATE KEY"), which is what
// `openssl genpkey` has produced by default since OpenSSL 3 — and therefore what a key
// placed by hand most likely is.
func toPKCS8(t *testing.T, sec1PEM string) string {
	t.Helper()
	block, _ := pem.Decode([]byte(sec1PEM))
	if block == nil {
		t.Fatal("no PEM block in the SEC1 key")
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse SEC1: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal PKCS#8: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

// TestNewCAFromEnvAcceptsAPKCS8Key: the encoding says nothing about whether the key is the
// right one, and this check is fatal — refusing PKCS#8 would turn a correctly repaired CA
// into a crash loop at boot. At least one central bank's material was repaired by hand
// before this validation existed, which is exactly where a PKCS#8 key comes from.
func TestNewCAFromEnvAcceptsAPKCS8Key(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM, err := sharedpki.GenerateSelfSignedCA("cb", "org", 1)
	if err != nil {
		t.Fatalf("ca: %v", err)
	}

	certPath := filepath.Join(dir, "cb.crt")
	keyPath := filepath.Join(dir, "cb.key")
	write(t, certPath, certPEM)
	write(t, keyPath, toPKCS8(t, keyPEM))

	t.Setenv("CA_CERT_FILE", certPath)
	t.Setenv("CA_KEY_FILE", keyPath)

	ca, err := NewCAFromEnv()
	if err != nil {
		t.Fatalf("a PKCS#8 copy of the CORRECT key was refused: %v", err)
	}
	if ca == nil {
		t.Fatal("no CA returned")
	}
}

// TestNewCAFromEnvStillRefusesAMismatchedPKCS8Key: accepting the encoding must not weaken
// the check the encoding is orthogonal to.
func TestNewCAFromEnvStillRefusesAMismatchedPKCS8Key(t *testing.T) {
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
	write(t, keyPath, toPKCS8(t, otherKeyPEM))

	t.Setenv("CA_CERT_FILE", certPath)
	t.Setenv("CA_KEY_FILE", keyPath)

	if _, err := NewCAFromEnv(); err == nil {
		t.Fatal("a mismatched key loaded as a CA because it was PKCS#8-encoded")
	} else if !strings.Contains(err.Error(), "not a pair") {
		t.Errorf("a mismatch should be reported as a mismatch, got: %v", err)
	}
}

// TestNewCAFromEnvSeparatesUnreadableFromMismatched: a key this loader cannot decode has a
// different remedy from one that decodes and belongs elsewhere. Reporting the first as
// "not a pair" sends an operator to regenerate a key that may be the correct one.
func TestNewCAFromEnvSeparatesUnreadableFromMismatched(t *testing.T) {
	dir := t.TempDir()
	certPEM, _, err := sharedpki.GenerateSelfSignedCA("cb", "org", 1)
	if err != nil {
		t.Fatalf("cert: %v", err)
	}

	certPath := filepath.Join(dir, "cb.crt")
	keyPath := filepath.Join(dir, "cb.key")
	write(t, certPath, certPEM)
	write(t, keyPath, "-----BEGIN PRIVATE KEY-----\nbm90IGEga2V5\n-----END PRIVATE KEY-----\n")

	t.Setenv("CA_CERT_FILE", certPath)
	t.Setenv("CA_KEY_FILE", keyPath)

	_, err = NewCAFromEnv()
	if err == nil {
		t.Fatal("an undecodable key loaded as a CA")
	}
	if strings.Contains(err.Error(), "not a pair") {
		t.Errorf("an unreadable key is reported as a mismatch, which points at the wrong fix: %v", err)
	}
	if !strings.Contains(err.Error(), keyPath) {
		t.Errorf("the error does not name the key file: %v", err)
	}
}

// TestNewCAFromEnvRefusesALeafAsTheIssuer closes the case the toolkit's filename assertion
// only names.
//
// A matched pair passes every other check here and still cannot be an issuer: a certificate
// with CA:FALSE signs nothing a verifier will accept. Nothing catches that at runtime today
// — CA_CERT_PEM is unset, so auth skips VerifyChain — so certificates issued from a leaf
// would be accepted at login and rejected the day that gate is turned on, retroactively,
// for every credential issued in between.
func TestNewCAFromEnvRefusesALeafAsTheIssuer(t *testing.T) {
	dir := t.TempDir()

	caCertPEM, caKeyPEM, err := sharedpki.GenerateSelfSignedCA("cb", "org", 1)
	if err != nil {
		t.Fatalf("ca: %v", err)
	}
	csrPEM, leafKeyPEM, err := sharedpki.GenerateCSR("central-bank", "org", "ROLE_CENTRAL_BANK", "BR")
	if err != nil {
		t.Fatalf("csr: %v", err)
	}
	issued, err := sharedpki.SignCSR(caCertPEM, caKeyPEM, csrPEM, 1)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	// A leaf and ITS OWN key: the pair check has nothing to object to.
	certPath := filepath.Join(dir, "central-bank.crt")
	keyPath := filepath.Join(dir, "central-bank.key")
	write(t, certPath, issued.CertPEM)
	write(t, keyPath, leafKeyPEM)

	t.Setenv("CA_CERT_FILE", certPath)
	t.Setenv("CA_KEY_FILE", keyPath)

	ca, err := NewCAFromEnv()
	if err == nil {
		t.Fatal("a participant certificate loaded as the CA; this service would issue " +
			"credentials from it that no verifier can chain")
	}
	if ca != nil {
		t.Error("a CA was returned alongside the error")
	}
	if !strings.Contains(err.Error(), certPath) {
		t.Errorf("the error does not name the offending file: %v", err)
	}
	if strings.Contains(err.Error(), "not a pair") {
		t.Errorf("a leaf is not a pairing fault; reporting it as one points at the wrong fix: %v", err)
	}
}

// TestNewCAFromEnvAcceptsTheBootstrapCA keeps the check above from being satisfied by
// refusing everything: the pair EnsurePKIFiles actually creates must load.
func TestNewCAFromEnvAcceptsTheBootstrapCA(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM, err := sharedpki.GenerateSelfSignedCA("central-bank-CA", "org", 10)
	if err != nil {
		t.Fatalf("ca: %v", err)
	}
	certPath := filepath.Join(dir, "central-bank-ca.crt")
	keyPath := filepath.Join(dir, "central-bank-ca.key")
	write(t, certPath, certPEM)
	write(t, keyPath, keyPEM)

	t.Setenv("CA_CERT_FILE", certPath)
	t.Setenv("CA_KEY_FILE", keyPath)

	ca, err := NewCAFromEnv()
	if err != nil {
		t.Fatalf("the pair the compliance bootstrap creates was refused: %v", err)
	}
	if _, err := ca.IssueParticipantCert("someone", "org", "ROLE_COMMERCIAL_BANK"); err != nil {
		t.Fatalf("the loaded CA cannot issue: %v", err)
	}
}
