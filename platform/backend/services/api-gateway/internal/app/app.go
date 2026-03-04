// This file wires application dependencies and builds the configured Fiber app.
package app

import (
	"errors"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/auth"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/identity"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/application/compliance"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/config"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/router"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

func New(cfg config.Config) (*fiber.App, error) {
	// Build the available auth providers and validators.
	mockProvider, err := auth.NewMockAuthProvider(cfg.MockClients, cfg.TokenIssuer, cfg.TokenAudience, cfg.TokenTTL)
	if err != nil {
		return nil, err
	}

	keycloakProvider := auth.NewKeycloakAuthProvider(cfg.KeycloakTokenURL, cfg.RequestTimeout)
	keycloakValidator := auth.NewKeycloakTokenValidator(
		cfg.KeycloakIntrospect,
		cfg.KeycloakClientID,
		cfg.KeycloakSecret,
		cfg.RequestTimeout,
	)
	mockValidator := auth.NewMockTokenValidator(mockProvider.PublicKey(), cfg.TokenIssuer, cfg.TokenAudience)

	// Select the active provider/validator strategy based on AUTH_MODE.
	authProvider, tokenValidator, err := chooseProviders(cfg.AuthMode, mockProvider, keycloakProvider, mockValidator, keycloakValidator)
	if err != nil {
		return nil, err
	}

	// Initialize in-memory services for identity and compliance.
	identityManager := identity.NewMemoryIdentityManager()
	complianceService := compliance.NewService(map[string]domain.KYCStatus{
		"bank-a": domain.KYCApproved,
		"bank-b": domain.KYCApproved,
		"bank-z": domain.KYCRejected,
	})

	authHandler := handlers.NewAuthHandler(authProvider, identityManager, complianceService)
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

// chooseProviders returns the auth provider and token validator for a given mode.
// In hybrid mode, it returns fallback chains (Keycloak first, mock second).
func chooseProviders(
	mode string,
	mockProvider interfaces.IAuthProvider,
	keycloakProvider interfaces.IAuthProvider,
	mockValidator interfaces.TokenValidator,
	keycloakValidator interfaces.TokenValidator,
) (interfaces.IAuthProvider, interfaces.TokenValidator, error) {
	switch mode {
	case config.ModeMock:
		return mockProvider, mockValidator, nil
	case config.ModeKeycloak:
		return keycloakProvider, keycloakValidator, nil
	case config.ModeHybrid:
		return auth.NewProviderChain(keycloakProvider, mockProvider), auth.NewValidatorChain(keycloakValidator, mockValidator), nil
	default:
		return nil, nil, errors.New("invalid AUTH_MODE, expected mock|keycloak|hybrid")
	}
}
