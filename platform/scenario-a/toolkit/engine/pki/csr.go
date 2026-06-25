// SPDX-License-Identifier: Apache-2.0

// Package pki defines the PKI contract for the Scenario A provisioning toolkit.
// Commercial banks MUST receive certificates signed by the Central Bank CA;
// no commercial bank may generate or hold a CA keypair.
//
// Implementation is provided by the TK-9 commercial-bank join flow PR.
// The function signatures here define the contract; bodies return "not implemented".
package pki

import (
	"errors"
	"time"
)

// GenerateBankCSR generates an ECDSA P-256 keypair and a PKCS#10 CSR for the
// given commercial bank and writes them to outputDir as:
//
//	{bankCode}.key  — private key (PEM, EC PRIVATE KEY)
//	{bankCode}.csr  — certificate signing request (PEM, CERTIFICATE REQUEST)
//
// The CSR subject is: CN={bankCode}, O={institution}, OU=ROLE_COMMERCIAL_BANK, C=BR.
//
// CONTRACT: this function MUST NOT create any file matching *-ca.key or *-ca.crt.
// Commercial banks never hold CA credentials; only the Central Bank CA signs CSRs.
func GenerateBankCSR(bankCode, institution, outputDir string) error {
	return errors.New("not implemented: GenerateBankCSR — see TK-9 for implementation")
}

// SubmitCSRToCB submits the CSR at csrPath to the Central Bank's
// credential-request endpoint and returns the signed certificate PEM.
// It mirrors the flow implemented in onboarding_proxy.go (smart mode):
//
//	POST {cbCredentialRequestURL}
//	  body: { csr_pem: <contents of csrPath>, bank_code: <bankCode>, ... }
//
// Returns a non-nil error if the CB is unreachable or returns a non-2xx status.
// MUST NOT fall back to self-signing under any error condition.
func SubmitCSRToCB(csrPath, cbCredentialRequestURL string, timeout time.Duration) (certPEM string, err error) {
	return "", errors.New("not implemented: SubmitCSRToCB — see TK-9 for implementation")
}

// StoreCertificate writes certPEM to outputDir/{bankCode}.crt (mode 0600).
func StoreCertificate(certPEM, bankCode, outputDir string) error {
	return errors.New("not implemented: StoreCertificate — see TK-9 for implementation")
}
