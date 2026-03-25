// This file wires application dependencies and builds the configured Fiber app.
package app

import (
	"fmt"

	authadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/auth"
	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	identityadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/identity"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/config"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/router"
	"github.com/gofiber/fiber/v2"
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
	onboardingHandler := handlers.NewOnboardingHandler(identityManager)

	fiberApp := fiber.New(
		fiber.Config{
			BodyLimit: 10 * 1024 * 1024,
			AppName:   "api-gateway",
		},
	)
	router.Setup(fiberApp, router.Dependencies{
		AuthHandler:       authHandler,
		ComplianceHandler: complianceHandler,
		GovernanceHandler: governanceHandler,
		OnboardingHandler: onboardingHandler,
		AuthProvider:      identityGRPCProvider,
	})

	return fiberApp, nil
}
