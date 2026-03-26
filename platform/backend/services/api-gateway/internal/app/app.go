// This file wires application dependencies and builds the configured Fiber app.
package app

import (
	"fmt"
	"os"

	authadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/auth"
	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	identityadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/identity"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/config"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/router"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func New(cfg config.Config) (*fiber.App, error) {
	// Auth gRPC provider: auth + PKI nonce/verify methods.
	identityGRPCProvider, err := authadapter.NewIdentityGRPCAuthProvider(cfg.AuthGRPCAddr, cfg.RequestTimeout)
	if err != nil {
		return nil, err
	}

	// AuthGRPCManager handles KYC + participant onboarding.
	identityManager, err := identityadapter.NewIdentityGRPCManager(cfg.AuthGRPCAddr, cfg.RequestTimeout)
	if err != nil {
		return nil, err
	}

	// Compliance gRPC adapter: governance portal operations (mandatory).
	if cfg.ComplianceGRPCAddr == "" {
		return nil, fmt.Errorf("COMPLIANCE_GRPC_ADDR is required but not set")
	}
	complianceGRPC, err := complianceadapter.NewGRPCAdapter(cfg.ComplianceGRPCAddr, cfg.RequestTimeout)
	if err != nil {
		return nil, fmt.Errorf("compliance gRPC unavailable at %s: %w", cfg.ComplianceGRPCAddr, err)
	}
	governanceHandler := handlers.NewGovernanceHandler(complianceGRPC)

	authHandler := handlers.NewAuthHandler(identityGRPCProvider, identityManager, cfg.CookieSecure)
	complianceHandler := handlers.NewComplianceHandler(identityManager)

	deps := router.Dependencies{
		AuthHandler:       authHandler,
		ComplianceHandler: complianceHandler,
		GovernanceHandler: governanceHandler,
		AuthProvider:      identityGRPCProvider,
	}

	if cfg.CentralBankAPIURL != "" {
		deps.OnboardingProxyHandler = handlers.NewOnboardingProxyHandler(
			cfg.CentralBankAPIURL,
			cfg.PKIDir,
			cfg.BankCode,
			identityManager, // implements OnboardingKeyManager
		)
	} else {
		deps.OnboardingHandler = handlers.NewOnboardingHandler(identityManager)
	}

	fiberApp := fiber.New(
		fiber.Config{
			BodyLimit: 10 * 1024 * 1024,
			AppName:   "api-gateway",
		},
	)

	fiberApp.Use(cors.New(cors.Config{
		AllowOrigins:     os.Getenv("CORS_ALLOW_ORIGINS"),
		AllowHeaders:     "Authorization, Content-Type, X-Requested-With, Accept",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowCredentials: true,
	}))

	router.Setup(fiberApp, router.Dependencies{
		AuthHandler:       authHandler,
		ComplianceHandler: complianceHandler,
		GovernanceHandler: governanceHandler,
		AuthProvider:      identityGRPCProvider,
	})

	return fiberApp, nil
}
