// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReceiveCertStep_AlreadyPresent(t *testing.T) {
	dir := t.TempDir()
	tlsDir := filepath.Join(dir, "tls")
	os.MkdirAll(tlsDir, 0o755)
	os.WriteFile(filepath.Join(tlsDir, "bank-x.crt"), []byte("CERT"), 0o600)

	step := newReceiveCertStep("bank-x", dir, "http://unused", time.Second, 10*time.Millisecond, time.Second)
	done, err := step.Check(context.Background())
	if err != nil || !done {
		t.Errorf("Check: done=%v err=%v, want true,nil", done, err)
	}
}

func TestReceiveCertStep_StoresStagedPendingCert(t *testing.T) {
	dir := t.TempDir()
	tlsDir := filepath.Join(dir, "tls")
	os.MkdirAll(tlsDir, 0o755)
	certPEM := "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"
	os.WriteFile(filepath.Join(tlsDir, pendingCertFile), []byte(certPEM), 0o600)

	step := newReceiveCertStep("bank-x", dir, "http://unused", time.Second, 10*time.Millisecond, time.Second)
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tlsDir, "bank-x.crt")); err != nil {
		t.Errorf("final cert not stored: %v", err)
	}
}

// When the request was accepted but the cert is not yet issued (the .cert-requested
// marker is present), the step reports that it is awaiting governance approval
// rather than polling — cert issuance is gated on a human Governance Portal action.
func TestReceiveCertStep_AwaitsGovernanceApproval(t *testing.T) {
	dir := t.TempDir()
	tlsDir := filepath.Join(dir, "tls")
	os.MkdirAll(tlsDir, 0o755)
	os.WriteFile(filepath.Join(tlsDir, certRequestedMark), []byte("pending\n"), 0o644)

	step := newReceiveCertStep("bank-x", dir, "http://unused", time.Second, 10*time.Millisecond, time.Second)
	err := step.Run(context.Background())
	if !errors.Is(err, ErrAwaitingGovernanceApproval) {
		t.Fatalf("Run err = %v; want ErrAwaitingGovernanceApproval", err)
	}
}
