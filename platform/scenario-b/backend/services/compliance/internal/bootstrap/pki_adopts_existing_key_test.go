// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	pki "github.com/LACNetNetworks/cbweb3-platform/backend/shared/identity"
)

// TestEnsurePKIFilesAdoptsAKeyItDidNotCreate reproduces the state the toolkit leaves.
//
// gen-tls writes {bankCode}.crt and {bankCode}.key and no CSR. EnsurePKIFiles then found a
// key with no CSR beside it, treated that as "nothing here", and generated a fresh pair —
// replacing a private key it did not own while ensureCert left the old certificate in
// place. The two files then held different keys, and the mismatch surfaced weeks later as
// "x509: provided PrivateKey doesn't match parent's PublicKey" at the first bank's
// onboarding, naming neither file.
func TestEnsurePKIFilesAdoptsAKeyItDidNotCreate(t *testing.T) {
	dir := t.TempDir()
	const bankCode = "central-bank"

	// What gen-tls leaves: a coherent cert+key pair, no CSR.
	certPEM, keyPEM, err := pki.GenerateSelfSignedCA("seeded-by-gen-tls", "org", 1)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	keyPath := filepath.Join(dir, bankCode+".key")
	certPath := filepath.Join(dir, bankCode+".crt")
	mustWrite(t, keyPath, keyPEM)
	mustWrite(t, certPath, certPEM)

	if err := EnsurePKIFiles(dir, bankCode, "", "", ""); err != nil {
		t.Fatalf("EnsurePKIFiles: %v", err)
	}

	if got := mustRead(t, keyPath); got != keyPEM {
		t.Error("the pre-existing private key was replaced; the certificate beside it is now unusable, " +
			"and nothing will notice until this CA is asked to sign")
	}
	if got := mustRead(t, certPath); got != certPEM {
		t.Error("the pre-existing certificate was replaced")
	}
	if _, err := os.Stat(filepath.Join(dir, bankCode+".csr")); err != nil {
		t.Errorf("no CSR was written for the adopted key: %v", err)
	}
}

// TestEnsurePKIFilesProducesAUsableCA is the property the outage needed: whatever the
// starting state, the CA this bootstrap leaves behind can actually sign.
func TestEnsurePKIFilesProducesAUsableCA(t *testing.T) {
	for _, tc := range []struct {
		name string
		seed func(t *testing.T, dir, bankCode string)
	}{
		{"empty directory", func(*testing.T, string, string) {}},
		{"key seeded by gen-tls, no CSR", func(t *testing.T, dir, bankCode string) {
			certPEM, keyPEM, err := pki.GenerateSelfSignedCA("seeded", "org", 1)
			if err != nil {
				t.Fatalf("seed: %v", err)
			}
			mustWrite(t, filepath.Join(dir, bankCode+".key"), keyPEM)
			mustWrite(t, filepath.Join(dir, bankCode+".crt"), certPEM)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			const bankCode = "central-bank"
			tc.seed(t, dir, bankCode)

			if err := EnsurePKIFiles(dir, bankCode, "", "", ""); err != nil {
				t.Fatalf("EnsurePKIFiles: %v", err)
			}

			// The pair the toolkit points CA_CERT_FILE/CA_KEY_FILE at must be able to issue.
			caCert := mustRead(t, filepath.Join(dir, bankCode+"-ca.crt"))
			caKey := mustRead(t, filepath.Join(dir, bankCode+"-ca.key"))
			csr, _, err := pki.GenerateCSR("some-bank", "org", "ROLE_COMMERCIAL_BANK", "BR")
			if err != nil {
				t.Fatalf("csr: %v", err)
			}
			if _, err := pki.SignCSR(caCert, caKey, csr, 1); err != nil {
				t.Fatalf("the CA this bootstrap left cannot sign a CSR: %v", err)
			}
		})
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// TestEnsurePKIFilesAdoptsAPKCS8Key covers the key a human leaves, not the one gen-tls
// leaves.
//
// Everything in this repository writes SEC1 ("EC PRIVATE KEY"), but `openssl genpkey` has
// defaulted to PKCS#8 ("PRIVATE KEY") since OpenSSL 3, and hand repair is precisely the
// situation in which this code meets a key it did not create. Refusing that encoding would
// abandon the adoption path exactly where it is needed and fall back to replacing the key —
// the behaviour this whole change exists to stop.
func TestEnsurePKIFilesAdoptsAPKCS8Key(t *testing.T) {
	dir := t.TempDir()
	const bankCode = "central-bank"

	certPEM, sec1PEM, err := pki.GenerateSelfSignedCA("repaired-by-hand", "org", 1)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	block, _ := pem.Decode([]byte(sec1PEM))
	if block == nil {
		t.Fatal("no PEM block in the seeded key")
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse SEC1: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal PKCS#8: %v", err)
	}
	pkcs8PEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))

	keyPath := filepath.Join(dir, bankCode+".key")
	mustWrite(t, keyPath, pkcs8PEM)
	mustWrite(t, filepath.Join(dir, bankCode+".crt"), certPEM)

	if err := EnsurePKIFiles(dir, bankCode, "", "", ""); err != nil {
		t.Fatalf("EnsurePKIFiles: %v", err)
	}
	if got := mustRead(t, keyPath); got != pkcs8PEM {
		t.Error("a PKCS#8 key was replaced instead of adopted; the certificate beside it is now unusable")
	}
	if _, err := os.Stat(filepath.Join(dir, bankCode+".csr")); err != nil {
		t.Errorf("no CSR was derived from the adopted PKCS#8 key: %v", err)
	}
}
