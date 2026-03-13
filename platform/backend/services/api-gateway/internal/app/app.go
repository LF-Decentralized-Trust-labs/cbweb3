// This file wires application dependencies and builds the configured Fiber app.
package app

import (
	"errors"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/auth"
	identityadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/identity"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/application/compliance"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/config"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/router"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

func New(cfg config.Config) (*fiber.App, error) {
	// The gateway always delegates authentication/token validation to identity gRPC.
	identityGRPCProvider, err := auth.NewIdentityGRPCAuthProvider(cfg.IdentityGRPCAddr, cfg.RequestTimeout)
	if err != nil {
		return nil, err
	}
	tokenValidator, ok := any(identityGRPCProvider).(interfaces.TokenValidator)
	if !ok {
		return nil, errors.New("identity gRPC provider does not implement token validator")
	}

	identityManager, err := identityadapter.NewIdentityGRPCManager(cfg.IdentityGRPCAddr, cfg.RequestTimeout)
	if err != nil {
		return nil, err
	}

	// Initialize compliance service.
	complianceService := compliance.NewService(map[string]domain.KYCStatus{
		"bank-a": domain.KYCApproved,
		"bank-b": domain.KYCApproved,
		"bank-z": domain.KYCRejected,
	})

	authHandler := handlers.NewAuthHandler(identityGRPCProvider, identityManager, complianceService)
	complianceHandler := handlers.NewComplianceHandler(complianceService)

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
