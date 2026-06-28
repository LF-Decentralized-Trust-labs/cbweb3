// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	kp "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/pki"
)

// Intermediate artifact names written under SPOKE_DATA_DIR/tls/.
const (
	pendingCertFile    = ".pending-cert.pem" // synchronous (HTTP 200) cert awaiting StoreCertificate
	certRequestedMark  = ".cert-requested"   // marker that the CSR was accepted (HTTP 202) for async signing
)

// requestCertStep submits the CSR to the central bank's credential-request
// endpoint. On synchronous issuance (200) it stages the cert for the receive
// step; on async acceptance (202) it writes a marker so a restart does not
// resubmit redundantly. The receive step finalizes either path.
type requestCertStep struct {
	bankCode    string
	dataDir     string
	cbEndpoint  string
	keyProvider kp.KeyProvider
	timeout     time.Duration
}

func newRequestCertStep(bankCode, dataDir, cbEndpoint string, keyProvider kp.KeyProvider, timeout time.Duration) Step {
	return &requestCertStep{bankCode: bankCode, dataDir: dataDir, cbEndpoint: cbEndpoint, keyProvider: keyProvider, timeout: timeout}
}

func (s *requestCertStep) Name() string { return StepRequestCert }

// Check returns true if the final cert already exists, or a pending/requested
// artifact is already staged (the POST was already made).
func (s *requestCertStep) Check(_ context.Context) (bool, error) {
	tlsDir := filepath.Join(s.dataDir, "tls")
	for _, name := range []string{s.bankCode + ".crt", pendingCertFile, certRequestedMark} {
		if _, err := os.Stat(filepath.Join(tlsDir, name)); err == nil {
			return true, nil
		}
	}
	return false, nil
}

func (s *requestCertStep) Run(ctx context.Context) error {
	tlsDir := filepath.Join(s.dataDir, "tls")
	if err := os.MkdirAll(tlsDir, 0o755); err != nil {
		return fmt.Errorf("create tls dir: %w", err)
	}
	csrPath := filepath.Join(s.dataDir, "pki", s.bankCode+".csr")

	// Resolve the bank's blockchain public key (generate idempotently if needed)
	// so it can be sent alongside the CSR per FR-010. The private key never leaves
	// the key provider.
	pubkeyHex := ""
	if s.keyProvider != nil {
		pub, err := s.keyProvider.GetPublicKey(ctx, s.bankCode)
		if errors.Is(err, kp.ErrKeyNotFound) {
			pub, err = s.keyProvider.GenerateKey(ctx, s.bankCode)
		}
		if err != nil {
			return fmt.Errorf("resolve blockchain pubkey: %w", err)
		}
		pubkeyHex = "0x" + hex.EncodeToString(pub)
	}

	certPEM, err := pki.SubmitCSRToCBWithPubkey(csrPath, s.cbEndpoint, pubkeyHex, s.timeout)
	if err != nil {
		if errors.Is(err, pki.ErrCertPending) {
			// CB accepted asynchronously — mark so receive step polls.
			return os.WriteFile(filepath.Join(tlsDir, certRequestedMark), []byte("pending\n"), 0o644)
		}
		return err // ErrCBRejected / ErrCBUnreachable — fail fast, no fallback
	}
	// Synchronous issuance: stage the cert for the receive step.
	return os.WriteFile(filepath.Join(tlsDir, pendingCertFile), []byte(certPEM), 0o600)
}
