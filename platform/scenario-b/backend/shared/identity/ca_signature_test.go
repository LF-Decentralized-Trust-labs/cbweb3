// SPDX-License-Identifier: Apache-2.0

package pki

import (
	"encoding/hex"
	"strings"
	"testing"
)

// caFixture builds a self-signed CA and an end-entity cert (via CSR signing)
// together with the end-entity's P-256 private key PEM.
type caFixture struct {
	caCertPEM string
	caKeyPEM  string
	certPEM   string
	keyPEM    string
}

func newCAFixture(t *testing.T) caFixture {
	t.Helper()
	caCert, caKey, err := GenerateSelfSignedCA("Central Bank CA", "BCB", 5)
	if err != nil {
		t.Fatalf("GenerateSelfSignedCA: %v", err)
	}
	csrPEM, keyPEM, err := GenerateCSR("participant-1", "Bank Inc", "ROLE_COMMERCIAL_BANK", "BR")
	if err != nil {
		t.Fatalf("GenerateCSR: %v", err)
	}
	issued, err := SignCSR(caCert, caKey, csrPEM, 1)
	if err != nil {
		t.Fatalf("SignCSR: %v", err)
	}
	return caFixture{caCertPEM: caCert, caKeyPEM: caKey, certPEM: issued.CertPEM, keyPEM: keyPEM}
}

func TestGenerateSelfSignedCA(t *testing.T) {
	cert, key, err := GenerateSelfSignedCA("Root", "Org", 0) // 0 -> default 10y
	if err != nil {
		t.Fatalf("GenerateSelfSignedCA: %v", err)
	}
	if !strings.Contains(cert, "BEGIN CERTIFICATE") || !strings.Contains(key, "EC PRIVATE KEY") {
		t.Fatalf("unexpected PEM output")
	}
	meta, err := ExtractMetadata(cert)
	if err != nil {
		t.Fatalf("ExtractMetadata: %v", err)
	}
	if meta.CommonName != "Root" || meta.Organization != "Org" {
		t.Fatalf("metadata=%+v", meta)
	}
}

func TestGenerateCSR(t *testing.T) {
	csr, key, err := GenerateCSR("cn", "org", "ou", "BR")
	if err != nil {
		t.Fatalf("GenerateCSR: %v", err)
	}
	if !strings.Contains(csr, "CERTIFICATE REQUEST") || !strings.Contains(key, "EC PRIVATE KEY") {
		t.Fatalf("unexpected output")
	}
}

func TestSignCSR(t *testing.T) {
	fix := newCAFixture(t)

	t.Run("metadata_preserved", func(t *testing.T) {
		meta, err := ExtractMetadata(fix.certPEM)
		if err != nil {
			t.Fatalf("ExtractMetadata: %v", err)
		}
		if meta.CommonName != "participant-1" || meta.Organization != "Bank Inc" || meta.Role != "ROLE_COMMERCIAL_BANK" {
			t.Fatalf("metadata=%+v", meta)
		}
	})

	t.Run("with_wallet_extension", func(t *testing.T) {
		csrPEM, _, _ := GenerateCSR("p", "o", "ou", "BR")
		wallet := "0x52908400098527886E0F7030069857D2E4169EE7"
		issued, err := SignCSR(fix.caCertPEM, fix.caKeyPEM, csrPEM, 1, SignCSROptions{WalletAddress: wallet})
		if err != nil {
			t.Fatalf("SignCSR: %v", err)
		}
		got, err := ExtractWalletFromCert(issued.CertPEM)
		if err != nil {
			t.Fatalf("ExtractWalletFromCert: %v", err)
		}
		if !strings.EqualFold(got, wallet) {
			t.Fatalf("wallet=%q want %q", got, wallet)
		}
	})

	t.Run("bad_ca_cert", func(t *testing.T) {
		if _, err := SignCSR("bad", fix.caKeyPEM, "x", 1); err == nil {
			t.Fatal("expected error for bad CA cert")
		}
	})
	t.Run("bad_ca_key", func(t *testing.T) {
		if _, err := SignCSR(fix.caCertPEM, "bad", "x", 1); err == nil {
			t.Fatal("expected error for bad CA key")
		}
	})
	t.Run("bad_csr_pem", func(t *testing.T) {
		if _, err := SignCSR(fix.caCertPEM, fix.caKeyPEM, "not-pem", 1); err == nil {
			t.Fatal("expected error for bad CSR PEM")
		}
	})
	t.Run("default_valid_years", func(t *testing.T) {
		csrPEM, _, _ := GenerateCSR("p", "o", "ou", "BR")
		if _, err := SignCSR(fix.caCertPEM, fix.caKeyPEM, csrPEM, 0); err != nil {
			t.Fatalf("SignCSR with 0 years: %v", err)
		}
	})
}

func TestIssueCertificate_WithWallet(t *testing.T) {
	caCert, caKey, _ := GenerateSelfSignedCA("CA", "Org", 2)
	wallet := "0x52908400098527886E0F7030069857D2E4169EE7"
	issued, err := IssueCertificate(caCert, caKey, CertRequest{
		Subject: "u", Org: "O", Role: "ROLE_TREASURY", WalletAddress: wallet, ValidYears: 0,
	})
	if err != nil {
		t.Fatalf("IssueCertificate: %v", err)
	}
	got, err := ExtractWalletFromCert(issued.CertPEM)
	if err != nil {
		t.Fatalf("ExtractWalletFromCert: %v", err)
	}
	if !strings.EqualFold(got, wallet) {
		t.Fatalf("wallet=%q want %q", got, wallet)
	}
}

func TestIssueCertificate_Errors(t *testing.T) {
	if _, err := IssueCertificate("bad", "bad", CertRequest{}); err == nil {
		t.Fatal("expected error for bad CA cert")
	}
	caCert, _, _ := GenerateSelfSignedCA("CA", "Org", 1)
	if _, err := IssueCertificate(caCert, "bad-key", CertRequest{}); err == nil {
		t.Fatal("expected error for bad CA key")
	}
}

// ---------------------------------------------------------------------------
// Signature: ValidateSignature / SignMessage
// ---------------------------------------------------------------------------

func TestSignAndValidate_RoundTrip(t *testing.T) {
	fix := newCAFixture(t)
	msg := hex.EncodeToString([]byte("hello world"))

	sig, err := SignMessage(fix.keyPEM, msg)
	if err != nil {
		t.Fatalf("SignMessage: %v", err)
	}
	if err := ValidateSignature(fix.certPEM, msg, sig); err != nil {
		t.Fatalf("ValidateSignature: %v", err)
	}
}

func TestValidateSignature_Errors(t *testing.T) {
	fix := newCAFixture(t)
	msg := hex.EncodeToString([]byte("m"))
	sig, _ := SignMessage(fix.keyPEM, msg)

	t.Run("bad_cert", func(t *testing.T) {
		if err := ValidateSignature("bad", msg, sig); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("bad_message_hex", func(t *testing.T) {
		if err := ValidateSignature(fix.certPEM, "zz", sig); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("bad_signature_hex", func(t *testing.T) {
		if err := ValidateSignature(fix.certPEM, msg, "zz"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("bad_der", func(t *testing.T) {
		if err := ValidateSignature(fix.certPEM, msg, hex.EncodeToString([]byte{0x01, 0x02})); err == nil {
			t.Fatal("expected error for malformed DER")
		}
	})
	t.Run("wrong_signature", func(t *testing.T) {
		// Sign a different message; verification against msg should fail.
		other, _ := SignMessage(fix.keyPEM, hex.EncodeToString([]byte("different")))
		if err := ValidateSignature(fix.certPEM, msg, other); err == nil {
			t.Fatal("expected verification failure")
		}
	})
	t.Run("non_ecdsa_handled", func(t *testing.T) {
		// A valid cert with an ECDSA key already covers the happy path; this just
		// ensures bad message hex doesn't panic when the key type is fine.
		if err := ValidateSignature(fix.certPEM, "0g", sig); err == nil {
			t.Fatal("expected error for invalid hex digit")
		}
	})
}

func TestSignMessage_Errors(t *testing.T) {
	fix := newCAFixture(t)
	if _, err := SignMessage("bad-key", "00"); err == nil {
		t.Fatal("expected error for bad key")
	}
	if _, err := SignMessage(fix.keyPEM, "zz"); err == nil {
		t.Fatal("expected error for bad message hex")
	}
}

// ---------------------------------------------------------------------------
// Verifier: VerifyChain / ExtractMetadata
// ---------------------------------------------------------------------------

func TestVerifyChain(t *testing.T) {
	fix := newCAFixture(t)

	t.Run("valid", func(t *testing.T) {
		if err := VerifyChain(fix.certPEM, fix.caCertPEM); err != nil {
			t.Fatalf("VerifyChain: %v", err)
		}
	})
	t.Run("wrong_ca", func(t *testing.T) {
		otherCA, _, _ := GenerateSelfSignedCA("Other", "O", 1)
		if err := VerifyChain(fix.certPEM, otherCA); err == nil {
			t.Fatal("expected chain verification failure")
		}
	})
	t.Run("bad_cert", func(t *testing.T) {
		if err := VerifyChain("bad", fix.caCertPEM); err == nil {
			t.Fatal("expected error for bad cert")
		}
	})
	t.Run("bad_ca_pool", func(t *testing.T) {
		if err := VerifyChain(fix.certPEM, "not-a-pem"); err == nil {
			t.Fatal("expected error for unparseable CA pool")
		}
	})
}

func TestExtractMetadata_Errors(t *testing.T) {
	if _, err := ExtractMetadata("bad"); err == nil {
		t.Fatal("expected error for bad cert PEM")
	}
}
