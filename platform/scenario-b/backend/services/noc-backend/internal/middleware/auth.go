// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/backend/services/noc-backend/internal/keycloak"
	"github.com/gofiber/fiber/v2"
)

const claimsKey = "noc_claims"

// SessionCookieName is the HttpOnly cookie the session now lives in.
//
// It replaced localStorage, where any script on the page could read the token. A cookie the
// browser cannot read can only be set by a server, which is why the login moved into this
// service at the same time.
const SessionCookieName = "access_token"

// machineBearerPaths are the ONLY routes where a Bearer token authenticates.
//
// The toolkit posts here with a Bearer token while provisioning every entity's NOC agent
// (toolkit/engine/orchestrator/noc.go, and scenario-a's apply/observe.go). It is not a
// browser: it has no cookie to send, and removing this would break the deploy.
//
// Everything else is cookie-only on purpose. Accepting Bearer everywhere would keep open
// the self-selected exemption the api-gateways just closed — a caller could waive the
// session cookie, and with it the CSRF check that only fires when a cookie is present, and
// still be served.
//
// Named here, in one place, rather than wired group by group. That mirrors the CSRF guard's
// own rule: mounted broadly, with every exemption a visible edit in a single file. It also
// has to be one place for a Fiber reason — the portal group's "/api/v1" prefix also covers
// "/api/v1/admin", so an admin request runs BOTH groups' middleware, and per-group auth
// policies would contradict each other.
var machineBearerPaths = map[string]bool{
	"/api/v1/admin/agents/provision-key": true,
	// register-noc-spoke posts here, and re-runs GET it under /spokes/<uuid> to decide
	// whether registration already happened. Both are the toolkit, never a browser.
	"/api/v1/admin/spokes": true,
}

// machineBearerPrefixes covers the toolkit's parameterised admin reads
// (/api/v1/admin/spokes/<uuid>), which an exact map cannot express.
//
// Kept deliberately short and adjacent to the map above: this is the whole machine surface,
// and anything added here widens where a Bearer token authenticates.
var machineBearerPrefixes = []string{
	"/api/v1/admin/spokes/",
}

// allowsMachineBearer reports whether a path is part of the toolkit's provisioning surface.
func allowsMachineBearer(path string) bool {
	if machineBearerPaths[path] {
		return true
	}
	for _, prefix := range machineBearerPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// RequireAuth authenticates from the session cookie, falling back to a Bearer token only on
// the machine paths above. On success it stores TokenClaims in Locals for downstream
// handlers.
//
// The cookie is tried FIRST even on a machine path. A browser operator using the admin
// screens sends one, and it must be the credential that counts — otherwise attaching a
// header of their own could downgrade a real session.
func RequireAuth(kc keycloak.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if cookie := strings.TrimSpace(c.Cookies(SessionCookieName)); cookie != "" {
			claims, err := kc.ValidateToken(c.Context(), cookie)
			if err != nil {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired session"})
			}
			c.Locals(claimsKey, claims)
			return c.Next()
		}

		header := c.Get("Authorization")
		if strings.HasPrefix(header, "Bearer ") && allowsMachineBearer(c.Path()) {
			claims, err := kc.ValidateToken(c.Context(), strings.TrimPrefix(header, "Bearer "))
			if err != nil {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired token"})
			}
			c.Locals(claimsKey, claims)
			return c.Next()
		}

		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "no session"})
	}
}

// ClaimsFrom returns the claims RequireAuth stored, and whether a session was authenticated
// at all.
//
// Exported because the profile endpoint needs them and lives in another package. The Locals
// key stays unexported: one accessor is what keeps the key from being spelled by hand
// somewhere and drifting.
func ClaimsFrom(c *fiber.Ctx) (keycloak.TokenClaims, bool) {
	claims, ok := c.Locals(claimsKey).(keycloak.TokenClaims)
	return claims, ok
}

// RequireRole rejects requests where the authenticated user does not hold roleName.
func RequireRole(role string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, ok := c.Locals(claimsKey).(keycloak.TokenClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthenticated"})
		}
		for _, r := range claims.Roles {
			if r == role {
				return c.Next()
			}
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "insufficient role"})
	}
}

// NOCPortalRoles are the roles that may read the NOC portal. The portal-read routes used to
// take RequireAuth alone: their comment said "any NOC role", but nothing checked for one, so
// every authenticated token — a treasury operator's included — was served the dashboard.
// Local stacks run with NOC_SKIP_AUTH=true and hide this, because the no-op Keycloak client
// hands back all three roles for any token.
var NOCPortalRoles = []string{"ROLE_NOC_ADMIN", "ROLE_NOC_OPERATOR", "ROLE_NOC_VIEWER"}

// RequireAnyRole rejects requests where the authenticated user holds none of roles. It is the
// multi-role form of RequireRole, for a group open to a family of roles rather than one.
func RequireAnyRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, ok := c.Locals(claimsKey).(keycloak.TokenClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthenticated"})
		}
		for _, want := range roles {
			for _, held := range claims.Roles {
				if held == want {
					return c.Next()
				}
			}
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "insufficient role"})
	}
}

// GetClaims retrieves TokenClaims stored by RequireAuth.
func GetClaims(c *fiber.Ctx) (keycloak.TokenClaims, bool) {
	claims, ok := c.Locals(claimsKey).(keycloak.TokenClaims)
	return claims, ok
}
