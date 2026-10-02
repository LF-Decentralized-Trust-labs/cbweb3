// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/gofiber/fiber/v2"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/backend/services/noc-backend/internal/keycloak"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/backend/services/noc-backend/internal/middleware"
)

// routeRegistrar is the single method every api handler exposes to mount itself on a
// router. Depending on the method rather than the concrete handlers is what lets the
// wiring below be tested without a database.
type routeRegistrar interface {
	Register(fiber.Router)
}

// protectedRegistrar is the optional half of a registrar whose routes require a session.
// Only the auth handler implements it — its /me sits in a group that no other middleware
// covers, so the guard has to be handed to it explicitly.
type protectedRegistrar interface {
	RegisterProtected(fiber.Router, fiber.Handler)
}

// nocRoutes groups the handlers by the authorization their group requires.
type nocRoutes struct {
	// Push is the agent push endpoint: API key auth, no Keycloak.
	Push routeRegistrar
	// Admin requires ROLE_NOC_ADMIN.
	Admin []routeRegistrar
	// Portal is the NOC portal read surface: any NOC role.
	Portal []routeRegistrar
	// Auth is the login/refresh/logout group. It takes no authorization — it is what
	// produces the session the other groups require.
	Auth routeRegistrar
}

// mountRoutes wires every route group and the authorization it requires.
//
// This lives apart from main() so a test can build the SAME wiring with fake handlers.
// That matters more here than the usual argument for testability: the authorization
// defect this guards against was never in the middleware — RequireRole existed and
// worked. What was wrong is that the portal group did not USE one, while the comment
// above it said "any NOC role", so every authenticated token was served the dashboard.
// A test of the middleware alone cannot see that returning; only a test of the wiring
// can, which is what routes_test.go does.
//
// Registration order is load-bearing and matches what main() did before this was
// extracted: push, then admin, then portal. Fiber applies group middleware by prefix, so
// the portal group's "/api/v1" also covers "/api/v1/admin" — an admin route therefore
// clears the admin role check and then a portal one. That is consistent (ROLE_NOC_ADMIN
// is itself a portal role) and it is why the order must not be shuffled.
func mountRoutes(app *fiber.App, kc keycloak.Client, r nocRoutes, csrfSecret []byte) {
	// The CSRF guard, mounted APP-WIDE and before any route.
	//
	// App-wide rather than group by group, copying the gateways' rule: per-group wiring is
	// how a tree came to hold 27 mutating routes with no CSRF at all — the guard was
	// attached to the groups that existed when it was written, and nothing failed when new
	// ones appeared. Mounted here, a new route is protected by default.
	//
	// The session id it binds to is the session COOKIE, so a request without one carries no
	// ambient credential and is let through. That is what lets the toolkit's cookieless
	// Bearer call reach provision-key while a browser — which always sends its cookie —
	// stays covered.
	app.Use(middleware.CSRF(middleware.CSRFConfig{
		Secret: csrfSecret,
		SessionID: func(c *fiber.Ctx) string {
			return c.Cookies(middleware.SessionCookieName)
		},
	}))

	// Routes — login, refresh, logout. Deliberately NOT behind RequireAuth: this is where a
	// browser with no session obtains one, so guarding it would make the cookie
	// unobtainable. CSRF above still covers refresh and logout once a session exists.
	if r.Auth != nil {
		authGroup := app.Group("/api/v1/auth")
		r.Auth.Register(authGroup)
		// /me needs a session, and the guard is applied HERE rather than inherited: this
		// group sits outside the portal group's prefix on purpose, so nothing else would
		// protect it.
		if p, ok := r.Auth.(protectedRegistrar); ok {
			p.RegisterProtected(authGroup, middleware.RequireAuth(kc))
		}
	}

	// Routes — agent push (API key auth, no Keycloak)
	if r.Push != nil {
		r.Push.Register(app)
	}

	// Routes — admin (Keycloak JWT + ROLE_NOC_ADMIN role required)
	admin := app.Group("/api/v1/admin",
		middleware.RequireAuth(kc),
		middleware.RequireRole("ROLE_NOC_ADMIN"),
	)
	for _, h := range r.Admin {
		h.Register(admin)
	}

	// Routes — NOC portal read (Keycloak JWT, any NOC role)
	portal := app.Group("/api/v1",
		middleware.RequireAuth(kc),
		middleware.RequireAnyRole(middleware.NOCPortalRoles...),
	)
	for _, h := range r.Portal {
		h.Register(portal)
	}
}
