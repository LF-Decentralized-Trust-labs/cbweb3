// SPDX-License-Identifier: Apache-2.0

// Package pki implements the PKI flow for the Scenario A provisioning toolkit
// (TK-9 commercial-bank join). Commercial banks generate their own keypair and
// CSR and receive a certificate signed by the Central Bank CA; no commercial
// bank ever generates or holds a CA keypair.
//
// CONTRACT: no function in this package may create a file matching *-ca.key or
// *-ca.crt. Only the Central Bank CA signs CSRs.
package pki

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	// ErrCBUnreachable is returned when the central bank endpoint cannot be reached.
	ErrCBUnreachable = errors.New("pki: central bank credential-request endpoint unreachable")
	// ErrCBRejected is returned when the central bank returns a non-2xx status.
	// The flow MUST NOT fall back to self-signing under any error condition.
	ErrCBRejected = errors.New("pki: central bank rejected the credential request")
	// ErrCertPending is returned by SubmitCSRToCB when the CB accepted the request
	// (HTTP 202) but will sign asynchronously. Callers should poll by re-submitting;
	// the CB endpoint is idempotent on (bank_code, csr).
	ErrCertPending = errors.New("pki: certificate issuance pending at central bank")
	// ErrInvalidInput is returned for empty bankCode/outputDir or malformed inputs.
	ErrInvalidInput = errors.New("pki: invalid input")
)

// GenerateBankCSR generates an ECDSA P-256 keypair and a PKCS#10 CSR for the
// given commercial bank and writes them to outputDir as:
//
//	{bankCode}.key  — private key (PEM, EC PRIVATE KEY, mode 0600)
//	{bankCode}.csr  — certificate signing request (PEM, CERTIFICATE REQUEST)
//
// The CSR subject is: CN={bankCode}, O={institution}, OU=ROLE_COMMERCIAL_BANK, C=BR.
// CONTRACT: never creates *-ca.key or *-ca.crt.
func GenerateBankCSR(bankCode, institution, outputDir string) error {
	if bankCode == "" {
		return fmt.Errorf("%w: bankCode is empty", ErrInvalidInput)
	}
	if outputDir == "" {
		return fmt.Errorf("%w: outputDir is empty", ErrInvalidInput)
	}
	if strings.HasSuffix(bankCode, "-ca") {
		return fmt.Errorf("%w: bankCode %q must not end with -ca (commercial banks never hold CA material)", ErrInvalidInput, bankCode)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate EC key: %w", err)
	}

	tmpl := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:         bankCode,
			Organization:       []string{institution},
			OrganizationalUnit: []string{"ROLE_COMMERCIAL_BANK"},
			Country:            []string{"BR"},
		},
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, tmpl, priv)
	if err != nil {
		return fmt.Errorf("create CSR: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("marshal EC key: %w", err)
	}
	if err := writePEM(filepath.Join(outputDir, bankCode+".key"), "EC PRIVATE KEY", keyDER, 0o600); err != nil {
		return err
	}
	if err := writePEM(filepath.Join(outputDir, bankCode+".csr"), "CERTIFICATE REQUEST", csrDER, 0o644); err != nil {
		return err
	}
	return nil
}

// SubmitCSRToCB reads the CSR at csrPath and submits it to the Central Bank's
// credential-request endpoint. It mirrors onboarding_proxy.go (smart mode):
//
//	POST {cbURL}
//	  body: { csr_pem, bank_code }   (bank_code derived from the CSR CN)
//
// HTTP 200 → returns the signed certificate PEM.
// HTTP 202 → returns ErrCertPending (caller polls by re-submitting; CB is idempotent).
// HTTP 4xx/5xx → ErrCBRejected. Unreachable → ErrCBUnreachable.
// MUST NOT fall back to self-signing under any error condition.
func SubmitCSRToCB(csrPath, cbURL string, timeout time.Duration) (certPEM string, err error) {
	return SubmitCSRToCBWithPubkey(csrPath, cbURL, "", timeout)
}

// SubmitCSRToCBWithPubkey is SubmitCSRToCB plus the participant's blockchain
// public key (uncompressed secp256k1 hex). When blockchainPubkey is non-empty it
// is included in the POST body as "blockchain_pubkey", per the credential-request
// contract (FR-010). An empty pubkey omits the field.
func SubmitCSRToCBWithPubkey(csrPath, cbURL, blockchainPubkey string, timeout time.Duration) (certPEM string, err error) {
	if cbURL == "" {
		return "", fmt.Errorf("%w: cbURL is empty", ErrInvalidInput)
	}
	csrBytes, err := os.ReadFile(csrPath)
	if err != nil {
		return "", fmt.Errorf("read CSR %q: %w", csrPath, err)
	}

	bankCode := bankCodeFromCSR(csrBytes)
	payload := map[string]string{
		"csr_pem":   string(csrBytes),
		"bank_code": bankCode,
	}
	if blockchainPubkey != "" {
		payload["blockchain_pubkey"] = blockchainPubkey
	}
	return postCredentialRequest(cbURL, payload, timeout)
}

// CredentialRequest is the full credential-request payload the central bank's
// api-gateway (POST /api/v1/onboarding/credential-request) requires. The private
// key never leaves the caller; only the CSR and the public key are sent.
type CredentialRequest struct {
	CSRPath          string
	BlockchainPubKey string // hex, 0x-prefixed
	InstitutionName  string
	Role             string // e.g. ROLE_COMMERCIAL_BANK
	Username         string
	Email            string
	Country          string // optional
}

// SubmitCredentialRequest reads the CSR and POSTs the full credential-request
// payload to the central bank. Field names match the api-gateway contract
// (csr_pem, blockchain_pub_key_hex, institution_name, role, username, email).
func SubmitCredentialRequest(req CredentialRequest, cbURL string, timeout time.Duration) (certPEM string, err error) {
	if cbURL == "" {
		return "", fmt.Errorf("%w: cbURL is empty", ErrInvalidInput)
	}
	csrBytes, err := os.ReadFile(req.CSRPath)
	if err != nil {
		return "", fmt.Errorf("read CSR %q: %w", req.CSRPath, err)
	}
	payload := map[string]string{
		"csr_pem":                string(csrBytes),
		"blockchain_pub_key_hex": req.BlockchainPubKey,
		"institution_name":       req.InstitutionName,
		"role":                   req.Role,
		"username":               req.Username,
		"email":                  req.Email,
		"bank_code":              bankCodeFromCSR(csrBytes),
	}
	if req.Country != "" {
		payload["country"] = req.Country
	}
	return postCredentialRequest(cbURL, payload, timeout)
}

// postCredentialRequest marshals payload as JSON, POSTs it to cbURL, and maps the
// response: 200 → cert_pem, 202 → ErrCertPending, other → ErrCBRejected.
func postCredentialRequest(cbURL string, payload map[string]string, timeout time.Duration) (string, error) {
	reqBody, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal credential request: %w", err)
	}

	client := &http.Client{Timeout: timeout}
	httpReq, err := http.NewRequest(http.MethodPost, cbURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrCBUnreachable, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		var parsed struct {
			CertPEM string `json:"cert_pem"`
		}
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			return "", fmt.Errorf("decode 200 response: %w", err)
		}
		if parsed.CertPEM == "" {
			return "", fmt.Errorf("%w: 200 response has empty cert_pem", ErrCBRejected)
		}
		return parsed.CertPEM, nil
	case http.StatusCreated, http.StatusAccepted, http.StatusConflict:
		// 201 Created / 202 Accepted: the CB registered the request for async
		// issuance (e.g. pending governance approval). 409 Conflict: the request
		// already exists (idempotent re-run). All await governance approval.
		return "", ErrCertPending
	default:
		return "", fmt.Errorf("%w: HTTP %d: %s", ErrCBRejected, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
}

// StoreCertificate writes certPEM to outputDir/{bankCode}.crt (mode 0600).
func StoreCertificate(certPEM, bankCode, outputDir string) error {
	if bankCode == "" || outputDir == "" {
		return fmt.Errorf("%w: bankCode and outputDir are required", ErrInvalidInput)
	}
	if strings.Contains(certPEM, "PRIVATE KEY") {
		return fmt.Errorf("%w: cert PEM contains private key material", ErrInvalidInput)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	certPath := filepath.Join(outputDir, bankCode+".crt")
	if err := os.WriteFile(certPath, []byte(certPEM), 0o600); err != nil {
		return fmt.Errorf("write certificate: %w", err)
	}
	return nil
}

// bankCodeFromCSR extracts the CommonName from a PEM CSR; returns "" if unparsable.
func bankCodeFromCSR(csrPEM []byte) string {
	block, _ := pem.Decode(csrPEM)
	if block == nil {
		return ""
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return ""
	}
	return csr.Subject.CommonName
}

// writePEM encodes der as a PEM block of the given type and writes it to path.
func writePEM(path, blockType string, der []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create %s: %w", filepath.Base(path), err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: blockType, Bytes: der}); err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	return nil
}
