// SPDX-License-Identifier: Apache-2.0

package handlers_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/relayauth"
	"github.com/gofiber/fiber/v2"
)

// The payment proxy carries a commercial bank's deposits, escrows and redeems to its central bank
// over /internal/v1/payments/*. Those were authenticated by the shared X-Relay-Auth secret alone —
// and that secret is identical in every entity, so any entity could forge them as any other bank:
// registering a deposit in another bank's name, or driving its tokenisation and redemption.
//
// The subtle part is WHAT gets signed. proxyWithEntityEnrichment rewrites the body (it injects
// requester_besu_address) before forwarding, so a signature computed over the ORIGINAL body would not
// match what the receiver verifies, and every enriched request would be rejected. The test below
// verifies at the receiver, exactly as the CB does, rather than merely asserting that headers exist.

func newProxySigner(t *testing.T, keyID string) *relayauth.Signer {
	t.Helper()
	dir := t.TempDir()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, keyID+".key"),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	signer, err := relayauth.LoadSigner(dir, keyID)
	if err != nil {
		t.Fatalf("load signer: %v", err)
	}
	return signer
}

func TestPaymentProxy_SignsTheENRICHEDBody(t *testing.T) {
	signer := newProxySigner(t, "bank-itau")
	reg := relayauth.NewRegistry()
	reg.Add("bank-itau", signer.PublicKey())

	var verifyErr error
	var seenBody map[string]interface{}
	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		verifyErr = reg.VerifyRequest(
			r.Header.Get(relayauth.HeaderKeyID),
			r.Header.Get(relayauth.HeaderTimestamp),
			r.Header.Get(relayauth.HeaderSignature),
			r.Method, r.URL.Path, body, time.Now())
		_ = json.Unmarshal(body, &seenBody)
		w.WriteHeader(http.StatusCreated)
	}))
	defer cb.Close()

	h := handlers.NewPaymentProxyHandler(cb.URL, "0xBANKADDR", "shh").WithSigner(signer)
	app := fiber.New()
	app.Post("/payments/deposits", h.RegisterDeposit)

	req := httptest.NewRequest(http.MethodPost, "/payments/deposits",
		strings.NewReader(`{"amount":"100","currency":"BRL"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("proxy: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	// The enrichment happened...
	if seenBody["requester_besu_address"] != "0xBANKADDR" {
		t.Fatalf("the CB did not receive the enriched body: %+v", seenBody)
	}
	// ...and the signature covers that enriched body, not the original one.
	if verifyErr != nil {
		t.Fatalf("the signature did not verify over the body the CB received: %v", verifyErr)
	}
}

// The legacy secret must survive the migration: a CB that has not pinned this bank yet still
// authenticates it, instead of a payment failing closed over authentication.
func TestPaymentProxy_KeepsTheSharedSecretAlongsideTheSignature(t *testing.T) {
	signer := newProxySigner(t, "bank-itau")

	var gotSecret, gotKeyID string
	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSecret = r.Header.Get("X-Relay-Auth")
		gotKeyID = r.Header.Get(relayauth.HeaderKeyID)
		w.WriteHeader(http.StatusOK)
	}))
	defer cb.Close()

	h := handlers.NewPaymentProxyHandler(cb.URL, "0xBANKADDR", "shh").WithSigner(signer)
	app := fiber.New()
	app.Get("/payments/deposits", h.ListDeposits)

	if _, err := app.Test(httptest.NewRequest(http.MethodGet, "/payments/deposits", nil), -1); err != nil {
		t.Fatalf("proxy: %v", err)
	}
	if gotSecret != "shh" {
		t.Fatal("the shared secret was dropped before the receiver is guaranteed to verify signatures")
	}
	if gotKeyID != "bank-itau" {
		t.Fatalf("a GET was not signed (key-id=%q) — reads are attributable too", gotKeyID)
	}
}

// Without a signer the proxy must behave exactly as before: secret only, no signature headers.
func TestPaymentProxy_UnsignedWhenNoSignerIsAttached(t *testing.T) {
	var gotKeyID, gotSecret string
	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKeyID = r.Header.Get(relayauth.HeaderKeyID)
		gotSecret = r.Header.Get("X-Relay-Auth")
		w.WriteHeader(http.StatusOK)
	}))
	defer cb.Close()

	h := handlers.NewPaymentProxyHandler(cb.URL, "0xBANKADDR", "shh")
	app := fiber.New()
	app.Get("/payments/deposits", h.ListDeposits)
	if _, err := app.Test(httptest.NewRequest(http.MethodGet, "/payments/deposits", nil), -1); err != nil {
		t.Fatalf("proxy: %v", err)
	}
	if gotKeyID != "" {
		t.Fatalf("signature headers were sent without a signer: %q", gotKeyID)
	}
	if gotSecret != "shh" {
		t.Fatal("the shared secret must still be sent")
	}
}
