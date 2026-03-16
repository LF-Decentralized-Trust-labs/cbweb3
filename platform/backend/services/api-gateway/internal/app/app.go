// This file wires application dependencies and builds the configured Fiber app.
package app

import (
	"errors"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/auth"
	identityadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/identity"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/config"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/router"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

func New(cfg config.Config) (*fiber.App, error) {
	// All identity + auth operations delegate to the identity gRPC service.
	identityGRPCProvider, err := auth.NewIdentityGRPCAuthProvider(cfg.IdentityGRPCAddr, cfg.RequestTimeout)
	if err != nil {
		return nil, err
	}
	tokenValidator, ok := any(identityGRPCProvider).(interfaces.TokenValidator)
	if !ok {
		return nil, errors.New("identity gRPC provider does not implement token validator")
	}

	// IdentityGRPCManager handles wallet binding + KYC operations.
	// The in-memory compliance service is REMOVED — all KYC is delegated to identity gRPC.
	identityManager, err := identityadapter.NewIdentityGRPCManager(cfg.IdentityGRPCAddr, cfg.RequestTimeout)
	if err != nil {
		return nil, err
	}

	authHandler := handlers.NewAuthHandler(identityGRPCProvider, identityManager, identityManager)
	complianceHandler := handlers.NewComplianceHandler(identityManager)

	fiberApp := fiber.New(
		fiber.Config{
			BodyLimit: 10 * 1024 * 1024, // 10MB limit
			AppName:   "api-gateway",
		},
	)
	router.Setup(fiberApp, router.Dependencies{
		AuthHandler:       authHandler,
		ComplianceHandler: complianceHandler,
		TokenValidator:    tokenValidator,
	})

	return fiberApp, nil
}
