// SPDX-License-Identifier: Apache-2.0

// Package certsource defines the PKI certificate issuance abstraction for the
// Scenario A provisioning toolkit. The Central Bank CA signs CSRs from commercial
// banks; no commercial bank ever holds a CA keypair.
//
// The local implementation generates a self-signed CA per spoke in process memory.
// The CA private key is never written to disk, logs, or environment variables.
// The trust anchor (CA certificate) is the only material that leaves the process,
// embedded in the join bundle (TK-6).
//
// Integration points:
//   - IssueLeafCert is called by the orchestration engine in mode:join (TK-9) after
//     the commercial bank submits its CSR via the CB credential-request endpoint.
//   - GetTrustAnchor is called by the join-bundle emitter (TK-6) to include the
//     spoke CA cert in the bundle distributed to joining banks.
package certsource

import (
	"context"
	"errors"
)

// CertSource is the PKI boundary for spoke certificate issuance.
type CertSource interface {
	// IssueLeafCert signs csrPEM (PKCS#10, PEM-encoded) with the CA of spokeID
	// and returns the signed certificate (PEM-encoded, X.509).
	// The local implementation generates the spoke CA on first use (idempotent).
	// Returns ErrForbiddenRole if the CSR subject OU does not contain ROLE_COMMERCIAL_BANK.
	// Returns ErrUnsupportedKeyAlgorithm if the CSR public key is not ECDSA P-256.
	// Returns ErrInvalidCSR if the CSR PEM is malformed or the signature is invalid.
	IssueLeafCert(ctx context.Context, csrPEM []byte, spokeID string) (certPEM []byte, err error)

	// GetTrustAnchor returns the CA certificate (PEM-encoded) for spokeID.
	// Returns ErrTrustAnchorNotFound if no CA has been generated for spokeID yet.
	// GetTrustAnchor never initialises a spoke CA — only IssueLeafCert does.
	GetTrustAnchor(ctx context.Context, spokeID string) (caCertPEM []byte, err error)
}

var (
	// ErrTrustAnchorNotFound is returned by GetTrustAnchor when no CA exists for the spoke.
	ErrTrustAnchorNotFound = errors.New("certsource: trust anchor not found for spoke")
	// ErrForbiddenRole is returned when the CSR OU does not contain ROLE_COMMERCIAL_BANK.
	ErrForbiddenRole = errors.New("certsource: CSR OU must be ROLE_COMMERCIAL_BANK")
	// ErrUnsupportedKeyAlgorithm is returned when the CSR public key is not ECDSA P-256.
	ErrUnsupportedKeyAlgorithm = errors.New("certsource: CSR public key must be ECDSA P-256")
	// ErrInvalidCSR is returned when the CSR PEM is malformed or has an invalid signature.
	ErrInvalidCSR = errors.New("certsource: CSR is malformed or has invalid signature")
	// ErrNotImplemented is returned by the production stub until PR-2 (Phase 4) is complete.
	ErrNotImplemented = errors.New("certsource: not implemented")
)
