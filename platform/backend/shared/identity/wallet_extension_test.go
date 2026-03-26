package pki

import (
	"encoding/asn1"
	"encoding/hex"
	"strings"
	"testing"
)

func TestWalletExtension_ValidAddress(t *testing.T) {
	addr := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	ext, err := WalletExtension(addr)
	if err != nil {
		t.Fatalf("WalletExtension: %v", err)
	}
	if !ext.Id.Equal(OIDBlockchainWallet) {
		t.Fatalf("extension OID = %v, want %v", ext.Id, OIDBlockchainWallet)
	}
	if ext.Critical {
		t.Fatalf("extension should not be critical")
	}
	var decoded []byte
	if _, err := asn1.Unmarshal(ext.Value, &decoded); err != nil {
		t.Fatalf("asn1.Unmarshal extension value: %v", err)
	}
	want, err := hex.DecodeString(strings.TrimPrefix(addr, "0x"))
	if err != nil {
		t.Fatalf("hex decode want: %v", err)
	}
	if len(decoded) != 20 {
		t.Fatalf("decoded length = %d, want 20", len(decoded))
	}
	if string(decoded) != string(want) {
		t.Fatalf("decoded = %x, want %x", decoded, want)
	}
}

func TestWalletExtension_InvalidAddress(t *testing.T) {
	_, err := WalletExtension("0xdeadbeef")
	if err == nil {
		t.Fatalf("expected error for short address")
	}
}

func TestExtractWalletFromCert_Present(t *testing.T) {
	caCert, caKey, err := GenerateSelfSignedCA("Test CA", "Test Org", 1)
	if err != nil {
		t.Fatalf("GenerateSelfSignedCA: %v", err)
	}
	want := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	issued, err := IssueCertificate(caCert, caKey, CertRequest{
		Subject:       "user-1",
		Org:           "Bank",
		Role:          "participant",
		ValidYears:    1,
		WalletAddress: want,
	})
	if err != nil {
		t.Fatalf("IssueCertificate: %v", err)
	}
	got, err := ExtractWalletFromCert(issued.CertPEM)
	if err != nil {
		t.Fatalf("ExtractWalletFromCert: %v", err)
	}
	if !strings.EqualFold(got, want) {
		t.Fatalf("wallet = %q, want %q (case-insensitive match)", got, want)
	}
}

func TestExtractWalletFromCert_Absent(t *testing.T) {
	caCert, caKey, err := GenerateSelfSignedCA("Test CA 2", "Test Org", 1)
	if err != nil {
		t.Fatalf("GenerateSelfSignedCA: %v", err)
	}
	issued, err := IssueCertificate(caCert, caKey, CertRequest{
		Subject:       "user-2",
		Org:           "Bank",
		Role:          "participant",
		ValidYears:    1,
		WalletAddress: "",
	})
	if err != nil {
		t.Fatalf("IssueCertificate: %v", err)
	}
	got, err := ExtractWalletFromCert(issued.CertPEM)
	if err != nil {
		t.Fatalf("ExtractWalletFromCert: %v", err)
	}
	if got != "" {
		t.Fatalf("wallet = %q, want empty string", got)
	}
}

func TestCertFingerprint_Consistency(t *testing.T) {
	caCert, caKey, err := GenerateSelfSignedCA("FP CA", "Org", 1)
	if err != nil {
		t.Fatalf("GenerateSelfSignedCA: %v", err)
	}
	issued, err := IssueCertificate(caCert, caKey, CertRequest{
		Subject:    "fp-user",
		Org:        "Bank",
		Role:       "participant",
		ValidYears: 1,
	})
	if err != nil {
		t.Fatalf("IssueCertificate: %v", err)
	}
	pem := issued.CertPEM
	fp1, err := CertFingerprint(pem)
	if err != nil {
		t.Fatalf("CertFingerprint first: %v", err)
	}
	fp2, err := CertFingerprint(pem)
	if err != nil {
		t.Fatalf("CertFingerprint second: %v", err)
	}
	if fp1 != fp2 {
		t.Fatalf("same PEM produced different fingerprints: %x vs %x", fp1, fp2)
	}
	other, err := IssueCertificate(caCert, caKey, CertRequest{
		Subject:    "fp-user-other",
		Org:        "Bank",
		Role:       "participant",
		ValidYears: 1,
	})
	if err != nil {
		t.Fatalf("IssueCertificate other: %v", err)
	}
	fpOther, err := CertFingerprint(other.CertPEM)
	if err != nil {
		t.Fatalf("CertFingerprint other: %v", err)
	}
	if fpOther == fp1 {
		t.Fatalf("different certs produced same fingerprint %x", fp1)
	}
}
