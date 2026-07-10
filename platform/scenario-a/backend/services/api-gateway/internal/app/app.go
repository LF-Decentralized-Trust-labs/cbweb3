// SPDX-License-Identifier: Apache-2.0

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
	besuscanner "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/besu"
	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	identityadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/identity"
	paladinadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/paladin"
	paymentadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/payment"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/config"
	dbinit "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/db/init"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/router"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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
	// Resolve the actor's institution name for audit log entries (Auditor Portal)
	// via the compliance participant registry.
	supervisorHandler := handlers.NewSupervisorHandler(complianceGRPC).WithParticipantResolver(complianceGRPC)

	authHandler := handlers.NewAuthHandler(identityGRPCProvider, identityManager, cfg.CookieSecure, cfg.BankCode)
	complianceHandler := handlers.NewComplianceHandler(identityManager, complianceGRPC)

	// Investigation Module: open a separate DB connection for the OversightService (optional).
	var oversightHandler *handlers.OversightHandler
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		if oversightDB, dbErr := gorm.Open(postgres.Open(dbURL), &gorm.Config{}); dbErr == nil {
			if migrateErr := dbinit.RunAutoMigrate(oversightDB); migrateErr != nil {
				log.Printf("warning: oversight schema migration failed (%v); disclosure endpoints disabled", migrateErr)
			} else {
				oversightSvc := services.NewOversightService(oversightDB)
				oversightHandler = handlers.NewOversightHandler(oversightSvc)

				// Decrypt endpoint: wire Paladin client and oversight quorum gate into SupervisorHandler.
				if cfg.PaladinURL != "" {
					paladinClient := paladinadapter.NewClient(cfg.PaladinURL)
					supervisorHandler.SetDecryptDeps(paladinClient, oversightSvc)
					log.Printf("supervisor decrypt endpoint enabled (paladin: %s)", cfg.PaladinURL)
				}
			}
		} else {
			log.Printf("warning: oversight DB unavailable (%v); disclosure endpoints disabled", dbErr)
		}
	}

	deps := router.Dependencies{
		AuthHandler:       authHandler,
		ComplianceHandler: complianceHandler,
		GovernanceHandler: governanceHandler,
		SupervisorHandler: supervisorHandler,
		OversightHandler:  oversightHandler,
		IdentityHandler:   handlers.NewIdentityHandler(cfg.PaladinIdentities),
		AuthProvider:      identityGRPCProvider,
	}

	// Transfer Limits (R1-10.1): CB only — commercial banks do not manage limits.
	if cfg.CentralBankAPIURL == "" {
		deps.TransferLimitHandler = handlers.NewTransferLimitHandler(complianceGRPC)
	}

	// Payment orchestrator gRPC adapter (optional; enables HTLC + token endpoints).
	if cfg.PaymentGRPCAddr != "" {
		paymentGRPC, err := paymentadapter.NewGRPCAdapter(cfg.PaymentGRPCAddr, cfg.RequestTimeout)
		if err != nil {
			closeAll(closers)
			return nil, fmt.Errorf("payment gRPC unavailable at %s: %w", cfg.PaymentGRPCAddr, err)
		}
		closers = append(closers, paymentGRPC)
		ph := handlers.NewPaymentHandler(paymentGRPC, cfg.BankCode)
		// Resolve requester institution names for deposit/escrow/redeem listings
		// (Treasury portal auditing) via the compliance participant registry.
		ph = ph.WithParticipantResolver(complianceGRPC)
		if cfg.FiatSymbol != "" {
			limitComplianceGRPC := complianceGRPC
			// Commercial banks point their limit checks at the central bank's compliance service,
			// since transfer limits are stored there (managed by the CB Treasury portal).
			if cfg.TransferLimitComplianceAddr != "" {
				tlConn, err := complianceadapter.NewGRPCAdapter(cfg.TransferLimitComplianceAddr, cfg.RequestTimeout)
				if err != nil {
					closeAll(closers)
					return nil, fmt.Errorf("transfer limit compliance gRPC unavailable at %s: %w", cfg.TransferLimitComplianceAddr, err)
				}
				closers = append(closers, tlConn)
				limitComplianceGRPC = tlConn
			}
			ph = ph.WithLimitChecker(limitComplianceGRPC, cfg.FiatSymbol)
		}
		deps.PaymentHandler = ph

		// Commercial bank: wire escrow proxy that forwards to the Central Bank.
		if cfg.CentralBankAPIURL != "" {
			deps.PaymentProxyHandler = handlers.NewPaymentProxyHandler(
				cfg.CentralBankAPIURL,
				paymentGRPC,
				cfg.EntityBesuAddress,
				cfg.PaladinIdentity,
				cfg.CBPaladinIdentity,
				cfg.RelayAuthSecret,
			)
		}
	}

	// On-chain HTLC scanner: supervisor searches bypass the payment-orchestrator and
	// read event logs directly from Besu, making all network HTLCs visible.
	if cfg.BesuRPCURL != "" && cfg.HTLCContractAddress != "" && deps.PaymentHandler != nil {
		scanner, scanErr := besuscanner.NewHTLCScanner(cfg.BesuRPCURL, cfg.HTLCContractAddress)
		if scanErr != nil {
			closeAll(closers)
			return nil, fmt.Errorf("besu htlc scanner: %w", scanErr)
		}
		closers = append(closers, scanner)
		deps.PaymentHandler.SetHTLCScanner(scanner)
		supervisorHandler.SetHTLCScanner(scanner)
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

	// CORS middleware: only enable if explicitly configured to avoid security issues.
	// AllowCredentials=true is incompatible with AllowOrigins="*" per RFC 6749.
	if corsOrigins := os.Getenv("CORS_ALLOW_ORIGINS"); corsOrigins != "" {
		fiberApp.Use(cors.New(cors.Config{
			AllowOrigins:     corsOrigins,
			AllowHeaders:     "Authorization, Content-Type, X-Requested-With, Accept, X-Correlation-Id",
			AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
			AllowCredentials: true,
		}))
	}

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
