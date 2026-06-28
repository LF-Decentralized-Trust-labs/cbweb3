// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
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

func TestReceiveCertStep_PollsUntilReady(t *testing.T) {
	dir := t.TempDir()
	writeFakeCSR(t, dir, "bank-x")
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt64(&calls, 1) < 2 {
			w.WriteHeader(http.StatusAccepted) // pending on first call
			w.Write([]byte(`{"status":"pending"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"cert_pem":"-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"}`))
	}))
	defer srv.Close()

	step := newReceiveCertStep("bank-x", dir, srv.URL, 5*time.Second, 10*time.Millisecond, 5*time.Second)
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "tls", "bank-x.crt")); err != nil {
		t.Errorf("final cert not stored after polling: %v", err)
	}
}
