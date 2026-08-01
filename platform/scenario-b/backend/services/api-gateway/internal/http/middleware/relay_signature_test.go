// SPDX-License-Identifier: Apache-2.0

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
	app := newApp(middleware.RelayAuthConfig{Registry: relayauth.NewStore(reg), LegacySecret: "shh"})

	body := `{"correlation_id":"c1"}`
	if code := do(t, app, signedReq(t, signer, body, time.Now())); code != http.StatusOK {
		t.Fatalf("expected 200 for valid signature, got %d", code)
	}
}

// A present-but-invalid signature is rejected and does NOT fall back to the secret
// (no downgrade attack when a signer is in play).
func TestMigrating_InvalidSignatureNotDowngraded(t *testing.T) {
	signer, reg := keyAndReg(t, "central-bank-a")
	app := newApp(middleware.RelayAuthConfig{Registry: relayauth.NewStore(reg), LegacySecret: "shh"})

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
	app := newApp(middleware.RelayAuthConfig{Registry: relayauth.NewStore(reg), LegacySecret: "shh"})

	req := httptest.NewRequest(http.MethodPost, sigPath, strings.NewReader(`{}`))
	req.Header.Set("X-Relay-Auth", "shh")
	if code := do(t, app, req); code != http.StatusOK {
		t.Fatalf("expected 200 for valid legacy secret, got %d", code)
	}
}

// With RequireSignature, a secret-only request is rejected (post-cutover enforcement).
func TestMigrating_RequireSignatureRejectsSecretOnly(t *testing.T) {
	_, reg := keyAndReg(t, "central-bank-a")
	app := newApp(middleware.RelayAuthConfig{Registry: relayauth.NewStore(reg), LegacySecret: "shh", RequireSignature: true})

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

// --- startup validation ---
//
// The dangerous combination is RequireSignature with nothing to verify against. In that state
// RequireRelayAuthMigrating never even attempts verification (hasRegistry is false), so it takes the
// "no verifiable signature" branch and answers 401 to EVERY internal request — including a correctly
// signed one. That kills bridge-in, the delegated hub swap and the residue return, so a deployment
// would stop settling payments because of one boolean. It has to be caught before serving traffic.

func TestRelayAuthConfig_Validate_RejectsEnforcementWithNothingToVerify(t *testing.T) {
	cases := map[string]middleware.RelayAuthConfig{
		"nil registry": {
			RequireSignature: true,
			LegacySecret:     "some-secret",
		},
		"empty registry": {
			Registry:         relayauth.NewStore(relayauth.NewRegistry()),
			RequireSignature: true,
			LegacySecret:     "some-secret",
		},
		"empty registry and no secret": {
			Registry:         relayauth.NewStore(relayauth.NewRegistry()),
			RequireSignature: true,
		},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("expected enforcement with no pinned keys to be refused: every internal request would 401")
			}
			// The message has to name the two settings, or an operator cannot act on it.
			if !strings.Contains(err.Error(), "RELAY_REQUIRE_SIGNATURE") || !strings.Contains(err.Error(), "PKI_DIR") {
				t.Fatalf("error must name both settings to be actionable, got: %v", err)
			}
		})
	}
}

// Enforcement with at least one pinned key is exactly what the operator asked for.
func TestRelayAuthConfig_Validate_AcceptsEnforcementWithPinnedKeys(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	reg := relayauth.NewRegistry()
	reg.Add("central-bank-a", &priv.PublicKey)

	cfg := middleware.RelayAuthConfig{Registry: relayauth.NewStore(reg), RequireSignature: true}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("enforcement with a pinned key must be accepted: %v", err)
	}
}

// The migrating state — no keys yet, shared secret in use — is the current normal and must keep
// starting. Refusing it would block every deployment that has not migrated.
func TestRelayAuthConfig_Validate_AcceptsMigratingState(t *testing.T) {
	cases := map[string]middleware.RelayAuthConfig{
		"secret only":               {LegacySecret: "s"},
		"empty registry and secret": {Registry: relayauth.NewStore(relayauth.NewRegistry()), LegacySecret: "s"},
		// Neither configured stays a RUNTIME 503 (fail-closed per request), not a startup failure:
		// a gateway that never receives internal calls is legitimately in this state.
		"neither configured": {},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			if err := cfg.Validate(); err != nil {
				t.Fatalf("must not refuse to start: %v", err)
			}
		})
	}
}
