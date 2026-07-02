// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
)

// writeFakeCSR creates a minimal CSR file so SubmitCSRToCB can read it.
func writeFakeCSR(t *testing.T, dataDir, bankCode string) {
	t.Helper()
	pkiDir := filepath.Join(dataDir, "pki")
	os.MkdirAll(pkiDir, 0o755)
	os.WriteFile(filepath.Join(pkiDir, bankCode+".csr"),
		[]byte("-----BEGIN CERTIFICATE REQUEST-----\nMIIB\n-----END CERTIFICATE REQUEST-----\n"), 0o644)
}

func TestRequestCertStep_HTTP200_StagesPendingCert(t *testing.T) {
	dir := t.TempDir()
	writeFakeCSR(t, dir, "bank-x")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"cert_pem":"-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"}`))
	}))
	defer srv.Close()

	step := newRequestCertStep("bank-x", "Bank X SA", dir, srv.URL, nil, 5*time.Second)
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "tls", pendingCertFile)); err != nil {
		t.Errorf("pending cert not staged: %v", err)
	}
	done, _ := step.Check(context.Background())
	if !done {
		t.Error("Check should be true after staging pending cert")
	}
}

func TestRequestCertStep_HTTP202_WritesMarker(t *testing.T) {
	dir := t.TempDir()
	writeFakeCSR(t, dir, "bank-x")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"request_id":"req-1","poll_url":"/p/req-1"}`))
	}))
	defer srv.Close()

	step := newRequestCertStep("bank-x", "Bank X SA", dir, srv.URL, nil, 5*time.Second)
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "tls", certRequestedMark)); err != nil {
		t.Errorf("requested marker not written: %v", err)
	}
}

func TestRequestCertStep_HTTP400_FailsFast(t *testing.T) {
	dir := t.TempDir()
	writeFakeCSR(t, dir, "bank-x")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad csr"}`))
	}))
	defer srv.Close()

	step := newRequestCertStep("bank-x", "Bank X SA", dir, srv.URL, nil, 5*time.Second)
	if err := step.Run(context.Background()); err == nil {
		t.Fatal("Run should fail fast on HTTP 400")
	}
}

// T053: the POST body includes blockchain_pub_key_hex when a key provider is set.
func TestRequestCertStep_SendsBlockchainPubkey(t *testing.T) {
	dir := t.TempDir()
	writeFakeCSR(t, dir, "bank-x")

	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"cert_pem":"-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"}`))
	}))
	defer srv.Close()

	kp := keyprovider.NewLocalKeyProvider()
	step := newRequestCertStep("bank-x", "Bank X SA", dir, srv.URL, kp, 5*time.Second)
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(gotBody, "blockchain_pub_key_hex") {
		t.Errorf("POST body must include blockchain_pub_key_hex, got: %s", gotBody)
	}
	if !strings.Contains(gotBody, "0x") {
		t.Errorf("blockchain_pub_key_hex should be hex-encoded with 0x prefix, got: %s", gotBody)
	}
}
