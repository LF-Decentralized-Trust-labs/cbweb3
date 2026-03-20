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
	GovernanceHandler *handlers.GovernanceHandler
	AuthProvider      interfaces.IAuthProvider
}

// Setup registers all gateway HTTP routes and middleware.
func Setup(app *fiber.App, deps Dependencies) {
	// X-Correlation-Id: generated/propagated on ALL requests (Complemento D / NFR-OPS-001).
	app.Use(middleware.CorrelationID())

	app.Get("/openapi.yaml", handlers.OpenAPIYAML)
	app.Get("/docs", handlers.SwaggerUI)
	app.Get("/healthz", handlers.Health)

	// --- Auth ---
	authGroup := app.Group("/api/v1/auth")
	authGroup.Post("/login", deps.AuthHandler.Login)
	authGroup.Post("/refresh", deps.AuthHandler.Refresh)
	authGroup.Post("/logout", middleware.RequireBearerToken(deps.AuthProvider), deps.AuthHandler.Logout)
	// PKI login step 2: submit signed nonce + X.509 certificate
	authGroup.Post("/wallet/bind", deps.AuthHandler.WalletBind)

	// --- Compliance (KYC status, AML gate, onboarding, provisioning) ---
	complianceGroup := app.Group("/api/v1/compliance", middleware.RequireBearerToken(deps.AuthProvider))
	complianceGroup.Get("/kyc/status/:subject", deps.ComplianceHandler.GetKYCStatus)
	complianceGroup.Post("/aml/screen", deps.ComplianceHandler.AMLScreen)

	centralBankRoutes := complianceGroup.Group("", middleware.RequireRole(domain.RoleGovernance))
	centralBankRoutes.Post("/approve-kyc", deps.GovernanceHandler.ApproveKYC)
	centralBankRoutes.Post("/participants/provision", deps.ComplianceHandler.ProvisionParticipant)
	centralBankRoutes.Post("/accounts/freeze", deps.ComplianceHandler.FreezeAccount)
	centralBankRoutes.Post("/accounts/unfreeze", deps.ComplianceHandler.UnfreezeAccount)
	centralBankRoutes.Post("/register", deps.ComplianceHandler.RegisterParticipant)

	// --- Governance Portal (PKI / ROLE_GOVERNANCE only) ---
	govGroup := app.Group("/api/v1/governance",
		middleware.RequireBearerToken(deps.AuthProvider),
		middleware.RequireRole(domain.RoleGovernance),
	)
	// Participant registration and registry
	govGroup.Post("/participants", deps.GovernanceHandler.RegisterParticipant)
	govGroup.Get("/registry", deps.GovernanceHandler.GetRegistry)
	govGroup.Post("/registry/csr", deps.GovernanceHandler.SubmitCSR)

	// Account management
	govGroup.Get("/accounts", deps.GovernanceHandler.GetAccounts)
	govGroup.Post("/accounts/freeze", deps.GovernanceHandler.FreezeAccount)
	govGroup.Post("/accounts/unfreeze", deps.GovernanceHandler.UnfreezeAccount)

	// Circuit breaker
	govGroup.Get("/circuit-breaker/status", deps.GovernanceHandler.GetCircuitBreakerStatus)
	govGroup.Post("/circuit-breaker/toggle", deps.GovernanceHandler.ToggleCircuitBreaker)

	// Global parameters
	govGroup.Get("/parameters", deps.GovernanceHandler.GetParameters)
	govGroup.Put("/parameters", deps.GovernanceHandler.UpdateParameters)

	// Audit logs
	govGroup.Get("/audit/logs", deps.GovernanceHandler.GetAuditLogs)
}
