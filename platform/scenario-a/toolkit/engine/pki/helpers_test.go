// SPDX-License-Identifier: Apache-2.0

package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"testing"
	"time"
)

// testCBCA creates a self-signed P-256 CA cert that simulates a spoke's
// Central Bank CA. Used to verify that SubmitCSRToCB returns a cert whose
// issuer matches the CB CA, not a bank-self-signed cert.
func testCBCA(t *testing.T) (certPEM, keyPEM string) {
	t.Helper()

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate test CB CA key: %v", err)
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "TestCentralBank-CA", Organization: []string{"TestCentralBank"}},
		NotBefore:             now,
		NotAfter:              now.AddDate(5, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
	if err != nil {
		t.Fatalf("create test CB CA cert: %v", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		t.Fatalf("marshal test CB CA key: %v", err)
	}

	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	return certPEM, keyPEM
}

// newTestCBSigningServer returns an http.Handler that simulates the CB
// credential-request endpoint. It reads the CSR PEM from the request body
// (JSON field "csr_pem"), signs it with the provided CA, and returns a
// JSON response with a "cert_pem" field — matching the production CB API.
func newTestCBSigningServer(t *testing.T, caCertPEM, caKeyPEM string) http.Handler {
	t.Helper()

	caCert := parseCertPEMForTest(t, caCertPEM)
	caKey := parseKeyPEMForTest(t, caKeyPEM)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}

		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		csrPEMStr, _ := req["csr_pem"].(string)
		if csrPEMStr == "" {
			http.Error(w, "missing csr_pem", http.StatusBadRequest)
			return
		}

		// Parse and sign the CSR.
		block, _ := pem.Decode([]byte(csrPEMStr))
		if block == nil {
			http.Error(w, "invalid csr pem", http.StatusBadRequest)
			return
		}
		csr, err := x509.ParseCertificateRequest(block.Bytes)
		if err != nil {
			http.Error(w, "parse csr", http.StatusBadRequest)
			return
		}
		if err := csr.CheckSignature(); err != nil {
			http.Error(w, "invalid csr signature", http.StatusBadRequest)
			return
		}

		serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		now := time.Now().UTC()
		leafTemplate := &x509.Certificate{
			SerialNumber:          serial,
			Subject:               csr.Subject,
			NotBefore:             now,
			NotAfter:              now.AddDate(1, 0, 0),
			KeyUsage:              x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
			IsCA:                  false,
		}

		certDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caCert, csr.PublicKey, caKey)
		if err != nil {
			http.Error(w, "sign csr", http.StatusInternalServerError)
			return
		}

		certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"cert_pem": certPEM}) //nolint:errcheck
	})
}

func parseCertPEMForTest(t *testing.T, certPEM string) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		t.Fatal("parseCertPEMForTest: nil PEM block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parseCertPEMForTest: %v", err)
	}
	return cert
}

func parseKeyPEMForTest(t *testing.T, keyPEM string) *ecdsa.PrivateKey {
	t.Helper()
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		t.Fatal("parseKeyPEMForTest: nil PEM block")
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("parseKeyPEMForTest: %v", err)
	}
	return key
}
