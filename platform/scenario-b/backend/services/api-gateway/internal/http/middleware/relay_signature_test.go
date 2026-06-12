package middleware_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/relayauth"
	"github.com/gofiber/fiber/v2"
)

const sigPath = "/internal/amm/cross-currency-bridge-in"

func newApp(cfg middleware.RelayAuthConfig) *fiber.App {
	app := fiber.New()
	app.Post(sigPath, middleware.RequireRelayAuthMigrating(cfg), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	return app
}

func signedReq(t *testing.T, signer *relayauth.Signer, body string, at time.Time) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, sigPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h, err := signer.HeadersFor(http.MethodPost, sigPath, []byte(body), at)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	for k, v := range h {
		req.Header.Set(k, v)
	}
	return req
}

func do(t *testing.T, app *fiber.App, req *http.Request) int {
	t.Helper()
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	_, _ = io.ReadAll(resp.Body)
	return resp.StatusCode
}

func keyAndReg(t *testing.T, keyID string) (*relayauth.Signer, *relayauth.Registry) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	reg := relayauth.NewRegistry()
	reg.Add(keyID, &key.PublicKey)
	return relayauth.NewSigner(keyID, key), reg
}

// A valid per-CB signature is accepted.
func TestMigrating_ValidSignatureAccepted(t *testing.T) {
	signer, reg := keyAndReg(t, "central-bank-a")
	app := newApp(middleware.RelayAuthConfig{Registry: reg, LegacySecret: "shh"})

	body := `{"correlation_id":"c1"}`
	if code := do(t, app, signedReq(t, signer, body, time.Now())); code != http.StatusOK {
		t.Fatalf("expected 200 for valid signature, got %d", code)
	}
}

// A present-but-invalid signature is rejected and does NOT fall back to the secret
// (no downgrade attack when a signer is in play).
func TestMigrating_InvalidSignatureNotDowngraded(t *testing.T) {
	signer, reg := keyAndReg(t, "central-bank-a")
	app := newApp(middleware.RelayAuthConfig{Registry: reg, LegacySecret: "shh"})

	// Sign one body, send a different one under the same headers + the valid secret.
	req := signedReq(t, signer, `{"amount":"1"}`, time.Now())
	req.Body = io.NopCloser(strings.NewReader(`{"amount":"999"}`))
	req.ContentLength = int64(len(`{"amount":"999"}`))
	req.Header.Set("X-Relay-Auth", "shh")

	if code := do(t, app, req); code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for tampered signature even with valid secret, got %d", code)
	}
}

// With no signature, the legacy secret is accepted (Cacti compatibility).
func TestMigrating_LegacySecretFallback(t *testing.T) {
	_, reg := keyAndReg(t, "central-bank-a")
	app := newApp(middleware.RelayAuthConfig{Registry: reg, LegacySecret: "shh"})

	req := httptest.NewRequest(http.MethodPost, sigPath, strings.NewReader(`{}`))
	req.Header.Set("X-Relay-Auth", "shh")
	if code := do(t, app, req); code != http.StatusOK {
		t.Fatalf("expected 200 for valid legacy secret, got %d", code)
	}
}

// With RequireSignature, a secret-only request is rejected (post-cutover enforcement).
func TestMigrating_RequireSignatureRejectsSecretOnly(t *testing.T) {
	_, reg := keyAndReg(t, "central-bank-a")
	app := newApp(middleware.RelayAuthConfig{Registry: reg, LegacySecret: "shh", RequireSignature: true})

	req := httptest.NewRequest(http.MethodPost, sigPath, strings.NewReader(`{}`))
	req.Header.Set("X-Relay-Auth", "shh")
	if code := do(t, app, req); code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when signature required but only secret provided, got %d", code)
	}
}

// Neither signature nor secret configured → fail closed (503).
func TestMigrating_FailsClosedWhenUnconfigured(t *testing.T) {
	app := newApp(middleware.RelayAuthConfig{})

	req := httptest.NewRequest(http.MethodPost, sigPath, strings.NewReader(`{}`))
	if code := do(t, app, req); code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when unconfigured, got %d", code)
	}
}

// A wrong secret with no signature is rejected.
func TestMigrating_WrongSecretRejected(t *testing.T) {
	app := newApp(middleware.RelayAuthConfig{LegacySecret: "shh"})

	req := httptest.NewRequest(http.MethodPost, sigPath, strings.NewReader(`{}`))
	req.Header.Set("X-Relay-Auth", "nope")
	if code := do(t, app, req); code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong secret, got %d", code)
	}
}
