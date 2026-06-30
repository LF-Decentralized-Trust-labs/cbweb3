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

// ErrAwaitingGovernanceApproval signals that the credential request was accepted
// by the central bank but the certificate cannot be issued until a governance
// officer approves the bank's KYC. Approval is a human action performed in the
// Governance Portal (ROLE_GOVERNANCE); once approved, re-running apply resumes
// the join. The full Phase 2.5/3 handshake (poll pop_nonce → sign → complete) is
// owned by the Governance Portal milestone.
var ErrAwaitingGovernanceApproval = errors.New(
	"awaiting central bank governance KYC approval (Governance Portal); re-run apply after the bank is approved")

// isDeferredOnboardingStep reports whether a mode:join step runs in the deferred
// tail, off the critical path, where a failure is non-fatal because the bank is
// already fully provisioned. Two groups:
//   - governance-gated identity: proof-of-possession (participant registration,
//     done by the CB on KYC approval) and receive-cert (CB-signed cert, issued
//     after approval in the Governance Portal);
//   - bilateral privacy (US3): create-pente-context + deploy-fxa-pente, blocked by
//     a Paladin cross-node registry-resolution behaviour and not consumed by the
//     backend — tracked for Paladin follow-up.
func isDeferredOnboardingStep(name string) bool {
	switch name {
	case StepProofPossession, StepReceiveCert, StepCreatePenteJoin, StepDeployFXAJoin:
		return true
	default:
		return false
	}
}

// receiveCertStep finalizes the certificate acquisition. If the request step
// already obtained the cert synchronously (pending-cert staged), it stores it.
// Otherwise the request is awaiting governance approval: the step reports that
// clearly rather than polling, since cert issuance is gated on a human decision.
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

	// Async path: the request was accepted (.cert-requested marker) but the cert is
	// not yet issued. Issuance is gated on a governance KYC approval performed in
	// the Governance Portal, so there is nothing to poll autonomously here.
	if _, err := os.Stat(filepath.Join(tlsDir, certRequestedMark)); err == nil {
		return ErrAwaitingGovernanceApproval
	}
	return fmt.Errorf("receive-cert: no staged certificate and no pending request marker for %s", s.bankCode)
}
