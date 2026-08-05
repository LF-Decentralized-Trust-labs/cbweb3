// SPDX-License-Identifier: Apache-2.0

// Package certsource is the spoke-CA boundary of the toolkit (CB-as-CA). The
// central bank is the CA of its spoke: it issues a commercial bank's leaf
// certificate from a CSR (the bank holds its own key) and exposes the trust
// anchor (the CA cert). The CA private key never leaves the process — it is
// never written to disk, log, env, manifest, state, or bundle.
//
// Two implementations sit behind the URI factory (New): a local, in-memory CA
// per spoke (self-signed) and a production stub (ca://...).
package certsource

import (
	"context"
	"errors"
)

// CertSource issues leaf certs from CSRs and exposes the spoke trust anchor.
type CertSource interface {
	// IssueLeafCert validates the PKCS#10 CSR (PEM) and returns a leaf cert
	// (PEM) signed by spokeID's CA, creating that CA on demand.
	IssueLeafCert(ctx context.Context, csrPEM []byte, spokeID string) ([]byte, error)
	// GetTrustAnchor returns spokeID's CA cert (PEM). It does NOT create a CA.
	GetTrustAnchor(ctx context.Context, spokeID string) ([]byte, error)
}

// Typed, distinguishable errors (no silent failures — Constitution VI).
var (
	ErrNotImplemented          = errors.New("certsource: not implemented in production stub")
	ErrInvalidCSR              = errors.New("certsource: invalid or unparseable CSR")
	ErrUnsupportedKeyAlgorithm = errors.New("certsource: CSR key algorithm must be ECDSA P-256")
	ErrForbiddenRole           = errors.New("certsource: CSR OU must contain ROLE_COMMERCIAL_BANK")
	ErrTrustAnchorNotFound     = errors.New("certsource: no CA for spoke id")
	ErrUnsupportedURI          = errors.New("certsource: unsupported factory URI")
)
