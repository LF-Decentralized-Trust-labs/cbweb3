// This file registers API Gateway routes and attaches required dependencies.
package router

import (
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

// Dependencies groups handlers and validators required by route registration.
type Dependencies struct {
	AuthHandler        *handlers.AuthHandler
	ComplianceHandler  *handlers.ComplianceHandler
	TokenValidator     interfaces.TokenValidator
}

// Setup registers all gateway HTTP routes and middleware.
func Setup(app *fiber.App, deps Dependencies) {
	app.Get("/openapi.yaml", handlers.OpenAPIYAML)
	app.Get("/swagger", handlers.SwaggerUI)

	app.Get("/healthz", handlers.Health)

	authGroup := app.Group("/auth")
	authGroup.Post("/login", deps.AuthHandler.Login)
	authGroup.Post("/wallet/bind", middleware.RequireBearerToken(deps.TokenValidator), deps.AuthHandler.WalletBind)

	complianceGroup := app.Group("/compliance")
	complianceGroup.Get("/kyc/status/:subject", deps.ComplianceHandler.GetKYCStatus)
}

