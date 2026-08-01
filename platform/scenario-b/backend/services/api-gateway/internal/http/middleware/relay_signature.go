// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"crypto/subtle"
	"fmt"
	"log"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/relayauth"
	"github.com/gofiber/fiber/v2"
)

// RelayAuthConfig configures authentication for internal relay endpoints during the
// migration from a shared symmetric secret to per-CB asymmetric signatures (R2-CR-6).
type RelayAuthConfig struct {
	// Registry holds the pinned peer verifying keys. A nil store, or a store holding no keys,
	// disables signature checking on this gateway (e.g. PKI not provisioned yet).
	//
	// A store rather than a registry because the set of peers changes at runtime: a central bank's
	// peers are its onboarded banks, and one onboarded after boot must become verifiable without a
	// restart. The store swaps immutable registries atomically, so verification never takes a lock.
	Registry *relayauth.Store
	// LegacySecret is the old shared INTERNAL_RELAY_AUTH_SECRET, accepted as a fallback
	// for callers that do not yet sign (notably the Cacti TS relay). Empty disables it.
	LegacySecret string
	// RequireSignature, when true, rejects requests that carry no signature even if a
	// legacy secret is configured — used to enforce the cutover once all callers sign.
	RequireSignature bool
}

// Validate refuses the one combination that turns a single boolean into an outage:
// RequireSignature with no pinned key to verify against.
//
// In that state RequireRelayAuthMigrating never attempts verification — hasRegistry is false — so it
// falls to the "no verifiable signature" branch and answers 401 to EVERY internal request,
// including a correctly signed one. Bridge-in, the delegated hub swap and the residue return all
// live behind those routes, so the deployment stops settling payments while looking configured.
// Refusing to start names the cause once, at boot, instead of surfacing it as a wave of 401s.
//
// Deliberately NOT a startup failure: neither signatures nor a secret configured. That already
// fails closed per request (503), and a gateway which never receives internal calls is legitimately
// in that state — refusing it would block entities that have nothing to authenticate.
func (c RelayAuthConfig) Validate() error {
	if !c.RequireSignature {
		return nil
	}
	if reg := c.Registry.Get(); reg == nil || reg.Len() == 0 {
		return fmt.Errorf(
			"RELAY_REQUIRE_SIGNATURE is set but no peer verifying keys are pinned from PKI_DIR: " +
				"every internal relay request would be rejected with 401, including correctly signed ones — " +
				"provision the peer certificates (PKI_DIR/<entity>.crt) or unset RELAY_REQUIRE_SIGNATURE")
	}
	return nil
}

// RequireRelayAuthMigrating enforces relay authentication with a signature-preferred,
// secret-fallback policy:
//
//   - If a request carries signature headers and a registry is configured, the
//     signature is verified strictly: a bad signature is rejected (no downgrade to
//     the shared secret).
//   - If no signature is present, the legacy shared secret is accepted — unless
//     RequireSignature is set, in which case the request is rejected.
//   - If neither a registry nor a legacy secret is configured, the endpoint fails
//     closed (503), preserving the original RequireRelayAuth safety property.
//
// This lets the bridge-in path (pure Go→Go) authenticate by signature today while the
// Cacti-fronted endpoints keep working on the secret until Cacti forwards signatures.
func RequireRelayAuthMigrating(cfg RelayAuthConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Read the registry ONCE per request: a concurrent reload must not make the checks below
		// disagree with each other.
		registry := cfg.Registry.Get()
		hasRegistry := registry != nil && registry.Len() > 0
		keyID := c.Get(relayauth.HeaderKeyID)

		if hasRegistry && keyID != "" {
			err := registry.VerifyRequest(
				keyID,
				c.Get(relayauth.HeaderTimestamp),
				c.Get(relayauth.HeaderSignature),
				c.Method(),
				c.Path(),
				c.Body(),
				time.Now(),
			)
			if err != nil {
				log.Printf("[relay-auth] signature rejected for key-id=%q path=%s: %v", keyID, c.Path(), err)
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "relay signature verification failed",
					"code":  "RELAY_SIGNATURE_INVALID",
				})
			}
			return c.Next()
		}

		// No (verifiable) signature on the request.
		if cfg.RequireSignature {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "per-CB relay signature required",
				"code":  "RELAY_SIGNATURE_REQUIRED",
			})
		}

		if cfg.LegacySecret == "" {
			// Neither signatures nor a secret configured — fail closed.
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "relay auth not configured on server",
			})
		}
		provided := c.Get("X-Relay-Auth")
		if provided == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "relay authentication required (signature or X-Relay-Auth)",
			})
		}
		if subtle.ConstantTimeCompare([]byte(provided), []byte(cfg.LegacySecret)) != 1 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid relay auth secret",
			})
		}
		return c.Next()
	}
}
