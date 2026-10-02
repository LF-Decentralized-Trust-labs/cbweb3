// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"testing"

	sharedpki "github.com/LACNetNetworks/cbweb3-platform/backend/shared/identity"
)

// TestBootstrapAndLoadCAOnAnEmptyDir pins the order, not the statements.
//
// CA_CERT_FILE names {bankCode}-ca.crt, which the PKI bootstrap CREATES. Loading the CA
// before running the bootstrap therefore reads a file that does not exist yet and takes
// the process down with log.Fatalf — on every first boot, on a fresh deploy, which is the
// most expensive place to discover an ordering mistake.
//
// The previous order survived only because CA_CERT_FILE used to name a file the toolkit's
// gen-tls step had already written. Nothing about that was deliberate.
func TestBootstrapAndLoadCAOnAnEmptyDir(t *testing.T) {
	dir := t.TempDir()
	const bankCode = "central-bank"

	t.Setenv("BANK_CODE", bankCode)
	t.Setenv("PKI_DIR", dir)
	t.Setenv("CA_CERT_FILE", filepath.Join(dir, bankCode+"-ca.crt"))
	t.Setenv("CA_KEY_FILE", filepath.Join(dir, bankCode+"-ca.key"))

	ca, err := bootstrapAndLoadCA()
	if err != nil {
		t.Fatalf("first boot on an empty PKI dir failed: %v", err)
	}
	if ca == nil {
		t.Fatal("no CA was loaded, so this entity cannot issue a single credential")
	}

	// Loading is not the property that matters — signing is.
	csr, _, err := sharedpki.GenerateCSR("some-bank", "org", "ROLE_COMMERCIAL_BANK", "BR")
	if err != nil {
		t.Fatalf("csr: %v", err)
	}
	if _, err := ca.SignCSR(csr); err != nil {
		t.Fatalf("the CA loaded at boot cannot sign: %v", err)
	}
}

// TestBootstrapAndLoadCADisabledWithoutCACertFile keeps the check above from being
// satisfied by a function that always succeeds: with no CA_CERT_FILE, issuance is off and
// that is not an error.
func TestBootstrapAndLoadCADisabledWithoutCACertFile(t *testing.T) {
	t.Setenv("BANK_CODE", "")
	t.Setenv("PKI_DIR", "")
	t.Setenv("CA_CERT_FILE", "")

	ca, err := bootstrapAndLoadCA()
	if err != nil {
		t.Fatalf("dev mode should not be an error: %v", err)
	}
	if ca != nil {
		t.Error("a CA was loaded with no CA_CERT_FILE set")
	}
}
