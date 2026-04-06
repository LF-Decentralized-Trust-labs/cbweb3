// This file wires application dependencies and builds the configured Fiber app.
package app

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	authadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/auth"
	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	identityadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/identity"
	paymentadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/payment"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/config"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/router"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// App wraps the Fiber HTTP server and all gRPC connections for lifecycle management.
type App struct {
	Fiber   *fiber.App
	closers []io.Closer
}

// Shutdown gracefully stops the HTTP server and closes all gRPC connections.
func (a *App) Shutdown() error {
	firstErr := a.Fiber.Shutdown()
	for _, c := range a.closers {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// dialGRPC establishes a gRPC client connection with the given timeout.
func dialGRPC(address string, timeout time.Duration) (*grpc.ClientConn, error) {
	dialCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return grpc.DialContext( //nolint:staticcheck
		dialCtx,
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
}

func New(cfg config.Config) (*App, error) {
	var closers []io.Closer

	// Single shared gRPC connection for auth + identity (same AUTH_GRPC_ADDR).
	authConn, err := dialGRPC(cfg.AuthGRPCAddr, cfg.RequestTimeout)
	if err != nil {
		return nil, fmt.Errorf("auth gRPC unavailable at %s: %w", cfg.AuthGRPCAddr, err)
	}
	closers = append(closers, authConn)

	identityGRPCProvider := authadapter.NewIdentityGRPCAuthProviderFromConn(authConn)
	identityManager := identityadapter.NewIdentityGRPCManagerFromConn(authConn)

	// Compliance gRPC adapter: governance portal operations (mandatory).
	if cfg.ComplianceGRPCAddr == "" {
		closeAll(closers)
		return nil, fmt.Errorf("COMPLIANCE_GRPC_ADDR is required but not set")
	}
	complianceGRPC, err := complianceadapter.NewGRPCAdapter(cfg.ComplianceGRPCAddr, cfg.RequestTimeout)
	if err != nil {
		closeAll(closers)
		return nil, fmt.Errorf("compliance gRPC unavailable at %s: %w", cfg.ComplianceGRPCAddr, err)
	}
	closers = append(closers, complianceGRPC)
	governanceHandler := handlers.NewGovernanceHandler(complianceGRPC)

	authHandler := handlers.NewAuthHandler(identityGRPCProvider, identityManager, cfg.CookieSecure)
	complianceHandler := handlers.NewComplianceHandler(identityManager, complianceGRPC)

	deps := router.Dependencies{
		AuthHandler:       authHandler,
		ComplianceHandler: complianceHandler,
		GovernanceHandler: governanceHandler,
		AuthProvider:      identityGRPCProvider,
	}

	// Payment orchestrator gRPC adapter (optional; enables HTLC + token endpoints).
	if cfg.PaymentGRPCAddr != "" {
		paymentGRPC, err := paymentadapter.NewGRPCAdapter(cfg.PaymentGRPCAddr, cfg.RequestTimeout)
		if err != nil {
			closeAll(closers)
			return nil, fmt.Errorf("payment gRPC unavailable at %s: %w", cfg.PaymentGRPCAddr, err)
		}
		closers = append(closers, paymentGRPC)
		deps.PaymentHandler = handlers.NewPaymentHandler(paymentGRPC)

		// Commercial bank: wire escrow proxy that forwards to the Central Bank.
		if cfg.CentralBankAPIURL != "" {
			deps.PaymentProxyHandler = handlers.NewPaymentProxyHandler(
				cfg.CentralBankAPIURL,
				paymentGRPC,
				cfg.EntityBesuAddress,
				cfg.PaladinIdentity,
				cfg.CBPaladinIdentity,
			)
		}
	}

	if cfg.CentralBankAPIURL != "" {
		deps.OnboardingProxyHandler = handlers.NewOnboardingProxyHandler(
			cfg.CentralBankAPIURL,
			cfg.PKIDir,
			cfg.BankCode,
			identityManager,
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

	router.Setup(fiberApp, deps)

	return &App{Fiber: fiberApp, closers: closers}, nil
}

func closeAll(closers []io.Closer) {
	for _, c := range closers {
		if err := c.Close(); err != nil {
			log.Printf("warning: failed to close resource: %v", err)
		}
	}
}
