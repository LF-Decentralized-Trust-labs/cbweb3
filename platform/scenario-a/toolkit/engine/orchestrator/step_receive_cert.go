// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/pki"
)

// receiveCertStep finalizes the certificate acquisition. If the request step
// already obtained the cert synchronously (pending-cert staged), it stores it.
// Otherwise it polls the central bank (by resubmitting the CSR; the CB endpoint
// is idempotent) until the signed cert is issued, then stores it.
type receiveCertStep struct {
	bankCode   string
	dataDir    string
	cbEndpoint string
	timeout    time.Duration // overall deadline for the polling loop
	interval   time.Duration // sleep between poll attempts
	reqTimeout time.Duration // per-request HTTP timeout for each CB submission
}

func newReceiveCertStep(bankCode, dataDir, cbEndpoint string, timeout, interval, reqTimeout time.Duration) Step {
	return &receiveCertStep{bankCode: bankCode, dataDir: dataDir, cbEndpoint: cbEndpoint, timeout: timeout, interval: interval, reqTimeout: reqTimeout}
}

func (s *receiveCertStep) Name() string { return StepReceiveCert }

func (s *receiveCertStep) Check(_ context.Context) (bool, error) {
	_, err := os.Stat(filepath.Join(s.dataDir, "tls", s.bankCode+".crt"))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *receiveCertStep) Run(ctx context.Context) error {
	tlsDir := filepath.Join(s.dataDir, "tls")

	// Fast path: cert already staged synchronously by the request step.
	pendingPath := filepath.Join(tlsDir, pendingCertFile)
	if data, err := os.ReadFile(pendingPath); err == nil {
		if err := pki.StoreCertificate(string(data), s.bankCode, tlsDir); err != nil {
			return err
		}
		_ = os.Remove(pendingPath)
		_ = os.Remove(filepath.Join(tlsDir, certRequestedMark))
		return nil
	}

	// Async path: poll the CB by resubmitting the CSR until issued.
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	csrPath := filepath.Join(s.dataDir, "pki", s.bankCode+".csr")

	for {
		certPEM, err := pki.SubmitCSRToCB(csrPath, s.cbEndpoint, s.reqTimeout)
		if err == nil {
			if err := pki.StoreCertificate(certPEM, s.bankCode, tlsDir); err != nil {
				return err
			}
			_ = os.Remove(filepath.Join(tlsDir, certRequestedMark))
			return nil
		}
		if !errors.Is(err, pki.ErrCertPending) {
			return err // hard rejection — fail fast
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("receive-cert: timed out after %s waiting for CB to issue certificate", s.timeout)
		case <-time.After(s.interval):
		}
	}
}
