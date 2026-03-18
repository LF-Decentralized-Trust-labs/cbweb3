// This file registers API Gateway routes and attaches required dependencies.
package router

import (
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

// Dependencies groups handlers and validators required by route registration.
type Dependencies struct {
	AuthHandler       *handlers.AuthHandler
	ComplianceHandler *handlers.ComplianceHandler
	AuthProvider      interfaces.IAuthProvider
}

// Setup registers all gateway HTTP routes and middleware.
func Setup(app *fiber.App, deps Dependencies) {
	// X-Correlation-Id: generated/propagated on ALL requests (Complemento D / NFR-OPS-001).
	app.Use(middleware.CorrelationID())

	app.Get("/openapi.yaml", handlers.OpenAPIYAML)
	app.Get("/swagger", handlers.SwaggerUI)
	app.Get("/healthz", handlers.Health)

	// --- Auth ---
	authGroup := app.Group("/auth")
	authGroup.Post("/login", deps.AuthHandler.Login)
	authGroup.Post("/refresh", deps.AuthHandler.Refresh)
	authGroup.Post("/logout", middleware.RequireBearerToken(deps.AuthProvider), deps.AuthHandler.Logout)
	authGroup.Post("/onboarding",
		middleware.RequireBearerToken(deps.AuthProvider),
		deps.AuthHandler.Onboarding,
	)
	authGroup.Post("/wallet/bind", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusGone).JSON(fiber.Map{
			"error": "wallet bind endpoint has been removed",
			"code":  "GONE",
		})
	})

	// --- Compliance ---
	complianceGroup := app.Group("/compliance", middleware.RequireBearerToken(deps.AuthProvider))

	// KYC read — any authenticated user
	complianceGroup.Get("/kyc/status/:subject", deps.ComplianceHandler.GetKYCStatus)
	complianceGroup.Post("/kyc/verify-proof", deps.ComplianceHandler.VerifyKYCProof)
	complianceGroup.Post("/aml/screen", deps.ComplianceHandler.AMLScreen)

	// KYC write — CENTRAL_BANK only (Fase 5 — RBAC middleware)
	centralBankRoutes := complianceGroup.Group("", middleware.RequireRole(domain.RoleCentralBank))
	centralBankRoutes.Post("/kyc/issue-credential", deps.ComplianceHandler.IssueKYCCredential)
	centralBankRoutes.Post("/participants/provision", deps.ComplianceHandler.ProvisionParticipant)
	centralBankRoutes.Post("/accounts/freeze", deps.ComplianceHandler.FreezeAccount)
	centralBankRoutes.Post("/accounts/unfreeze", deps.ComplianceHandler.UnfreezeAccount)

	// Administrative participant registration — CENTRAL_BANK only.
	// The CB uses this endpoint to onboard Commercial Banks and Treasury users.
	centralBankRoutes.Post("/register", deps.ComplianceHandler.RegisterParticipant)
}
