// SPDX-License-Identifier: Apache-2.0

package services_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/relayauth"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
)

func TestRemoteTransferLimitChecker_CheckAndDeduct_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/v2/transfer-limits/check-and-deduct" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-Relay-Auth") != "testsecret" {
			t.Error("missing or wrong X-Relay-Auth header")
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer srv.Close()

	checker := services.NewRemoteTransferLimitChecker(srv.URL, "testsecret", 5*time.Second)
	if err := checker.CheckAndDeduct(context.Background(), "bank-a", "BRL", "1000"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestRemoteTransferLimitChecker_CheckAndDeduct_LimitExceeded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{
			"error_code": "TRANSFER_LIMIT_EXCEEDED",
			"error":      "daily transfer limit exceeded",
		})
	}))
	defer srv.Close()

	checker := services.NewRemoteTransferLimitChecker(srv.URL, "secret", 5*time.Second)
	err := checker.CheckAndDeduct(context.Background(), "bank-a", "BRL", "9999999")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var limitErr *services.ErrTransferLimitExceeded
	if !isErrTransferLimitExceeded(err, &limitErr) {
		t.Fatalf("expected *ErrTransferLimitExceeded, got %T: %v", err, err)
	}
}

func TestRemoteTransferLimitChecker_CheckAndDeduct_CBUnavailable(t *testing.T) {
	checker := services.NewRemoteTransferLimitChecker("http://127.0.0.1:0", "secret", 100*time.Millisecond)
	err := checker.CheckAndDeduct(context.Background(), "bank-a", "BRL", "100")
	if err == nil {
		t.Fatal("expected error when CB is unreachable")
	}
}

func TestRemoteTransferLimitChecker_Restore_BestEffort(t *testing.T) {
	// Restore must not panic even when CB returns error or is unreachable.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	checker := services.NewRemoteTransferLimitChecker(srv.URL, "secret", 5*time.Second)
	// Must not panic or return anything.
	checker.Restore(context.Background(), "bank-a", "BRL", "100")
}

// isErrTransferLimitExceeded checks if err is *ErrTransferLimitExceeded using errors.As semantics.
func isErrTransferLimitExceeded(err error, target **services.ErrTransferLimitExceeded) bool {
	if e, ok := err.(*services.ErrTransferLimitExceeded); ok {
		*target = e
		return true
	}
	return false
}

// These two endpoints were the last bank→CB calls authenticated by the shared symmetric secret
// alone, while every other internal route had moved to per-entity signatures. They are not
// incidental: check-and-deduct enforces the CB's authoritative daily limit, so forging it lets a bank
// spend past its own limit or consume another bank's quota — and the secret is identical in every
// entity, so any entity can forge it as any other.
//
// Signature-preferred, secret-preserved: the receiver accepts either during the migration, so a
// signing client and a not-yet-signing one both work.

func TestRemoteTransferLimitChecker_SignsWhenASignerIsAttached(t *testing.T) {
	dir := t.TempDir()
	writeTestSignerKey(t, dir, "bank-itau")
	signer, err := relayauth.LoadSigner(dir, "bank-itau")
	if err != nil {
		t.Fatalf("load signer: %v", err)
	}

	var gotKeyID, gotSig, gotTS, gotSecret string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKeyID = r.Header.Get(relayauth.HeaderKeyID)
		gotSig = r.Header.Get(relayauth.HeaderSignature)
		gotTS = r.Header.Get(relayauth.HeaderTimestamp)
		gotSecret = r.Header.Get("X-Relay-Auth")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := services.NewRemoteTransferLimitChecker(srv.URL, "shh", 5*time.Second).WithSigner(signer)
	if err := c.CheckAndDeduct(context.Background(), "bank-itau", "BRL", "10"); err != nil {
		t.Fatalf("CheckAndDeduct: %v", err)
	}
	if gotKeyID != "bank-itau" || gotSig == "" || gotTS == "" {
		t.Fatalf("request was not signed: key-id=%q sig=%q ts=%q", gotKeyID, gotSig, gotTS)
	}
	// The secret stays for the migration window: a CB that has not pinned this bank yet still
	// authenticates it, instead of the call failing closed and blocking the payment.
	if gotSecret != "shh" {
		t.Fatalf("the legacy secret was dropped before the receiver is guaranteed to verify signatures")
	}
}

// The signature must cover the path actually called, or the receiver's canonical string will not
// match and every request is rejected. Restore and check-and-deduct are different paths.
func TestRemoteTransferLimitChecker_SignsEachPathDistinctly(t *testing.T) {
	dir := t.TempDir()
	writeTestSignerKey(t, dir, "bank-itau")
	signer, err := relayauth.LoadSigner(dir, "bank-itau")
	if err != nil {
		t.Fatalf("load signer: %v", err)
	}

	reg := relayauth.NewRegistry()
	reg.Add("bank-itau", signer.PublicKey())

	var verifyErr error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		verifyErr = reg.VerifyRequest(
			r.Header.Get(relayauth.HeaderKeyID),
			r.Header.Get(relayauth.HeaderTimestamp),
			r.Header.Get(relayauth.HeaderSignature),
			r.Method, r.URL.Path, body, time.Now())
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := services.NewRemoteTransferLimitChecker(srv.URL, "shh", 5*time.Second).WithSigner(signer)
	if err := c.CheckAndDeduct(context.Background(), "bank-itau", "BRL", "10"); err != nil {
		t.Fatalf("CheckAndDeduct: %v", err)
	}
	if verifyErr != nil {
		t.Fatalf("check-and-deduct signature did not verify at the receiver: %v", verifyErr)
	}
	c.Restore(context.Background(), "bank-itau", "BRL", "10")
	if verifyErr != nil {
		t.Fatalf("restore signature did not verify at the receiver: %v", verifyErr)
	}
}

// writeTestSignerKey writes a PEM EC private key as <id>.key, the layout LoadSigner expects.
func writeTestSignerKey(t *testing.T, dir, id string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".key"),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
}
