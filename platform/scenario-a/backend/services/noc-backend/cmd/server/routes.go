// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/gofiber/fiber/v2"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/keycloak"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/middleware"
)

// routeRegistrar is the single method every api handler exposes to mount itself on a
// router. Depending on the method rather than the concrete handlers is what lets the
// wiring below be tested without a database.
type routeRegistrar interface {
	Register(fiber.Router)
}

// nocRoutes groups the handlers by the authorization their group requires.
type nocRoutes struct {
	// Push is the agent push endpoint: API key auth, no Keycloak.
	Push routeRegistrar
	// Admin requires ROLE_NOC_ADMIN.
	Admin []routeRegistrar
	// Portal is the NOC portal read surface: any NOC role.
	Portal []routeRegistrar
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
func mountRoutes(app *fiber.App, kc keycloak.Client, r nocRoutes) {
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
