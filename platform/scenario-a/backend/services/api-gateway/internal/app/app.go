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
	relayadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/relay"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/config"
	dbinit "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/db/init"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/router"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/authz"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"google.golang.org/grpc"
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

// dialGRPC establishes a gRPC client connection with the given timeout. serverName
// is the logical name the peer certificate must present under mTLS (R2-H-8); it is
// ignored when client mTLS is not configured (plaintext transitional default).
func dialGRPC(address, serverName string, timeout time.Duration) (*grpc.ClientConn, error) {
	dialCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	credOpt, err := authz.ClientDialOptionFromEnv(serverName)
	if err != nil {
		return nil, err
	}
	return grpc.DialContext( //nolint:staticcheck
		dialCtx,
		address,
		credOpt,
	)
}

func New(cfg config.Config) (*App, error) {
	var closers []io.Closer

	// Single shared gRPC connection for auth + identity (same AUTH_GRPC_ADDR).
	authConn, err := dialGRPC(cfg.AuthGRPCAddr, "auth", cfg.RequestTimeout)
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

	// Shared api-gateway DB connection (optional). Backs the Investigation Module
	// (OversightService) and the Central Bank PvP ledger. AutoMigrate covers all
	// api-gateway-owned tables.
	var appDB *gorm.DB
	var oversightHandler *handlers.OversightHandler
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		if db, dbErr := gorm.Open(postgres.Open(dbURL), &gorm.Config{}); dbErr == nil {
			if migrateErr := dbinit.RunAutoMigrate(db); migrateErr != nil {
				log.Printf("warning: api-gateway schema migration failed (%v); DB-backed endpoints disabled", migrateErr)
			} else {
				appDB = db
				oversightSvc := services.NewOversightService(db)
				oversightHandler = handlers.NewOversightHandler(oversightSvc)

				// Decrypt endpoint: wire Paladin client and oversight quorum gate into SupervisorHandler.
				if cfg.PaladinURL != "" {
					paladinClient := paladinadapter.NewClient(cfg.PaladinURL)
					supervisorHandler.SetDecryptDeps(paladinClient, oversightSvc)
					log.Printf("supervisor decrypt endpoint enabled (paladin: %s)", cfg.PaladinURL)
				}
			}
		} else {
			log.Printf("warning: api-gateway DB unavailable (%v); DB-backed endpoints disabled", dbErr)
		}
	}

	deps := router.Dependencies{
		AuthHandler:       authHandler,
		ComplianceHandler: complianceHandler,
		GovernanceHandler: governanceHandler,
		SupervisorHandler: supervisorHandler,
		OversightHandler:  oversightHandler,
		// Default roster: static override only (no live Pente source). When a
		// payment-orchestrator is wired below, this is replaced with a
		// membership-backed roster.
		IdentityHandler: handlers.NewIdentityHandler(handlers.NewIdentityRoster(cfg.PaladinIdentities, nil)),
		AuthProvider:    identityGRPCProvider,
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
		// FX party roster sourced from real Pente membership (via the orchestrator),
		// with PALADIN_IDENTITIES as an optional static override. Shared by the
		// identities endpoint and propose-time validation, so an identity that is
		// not a real member is rejected with a clear 400 instead of a cryptic
		// on-chain Pente failure.
		//
		// localRoster is this spoke's own view (local Pente membership ∪ override);
		// it answers GET /identities?scope=local and gates propose-time validation
		// against the LOCAL leg. When RELAY_URL is set, the default (network-wide)
		// roster is federated across every spoke the relay registry knows: each
		// spoke's CB gateway is queried for its local roster, so a new spoke that
		// registers with the relay appears in every portal with no manifest edit.
		// Without RELAY_URL the two rosters are identical (pre-federation behaviour).
		localRoster := handlers.NewIdentityRoster(cfg.PaladinIdentities, paymentGRPC)
		roster := localRoster
		if cfg.RelayURL != "" {
			relayClient := relayadapter.NewClient(cfg.RelayURL, cfg.RelayAuthSecret, cfg.RequestTimeout)
			federated := relayadapter.NewFederatedRoster(paymentGRPC, relayClient, relayClient, 0)
			roster = handlers.NewIdentityRoster(cfg.PaladinIdentities, federated)
			log.Printf("FX party roster federated across the relay's spoke registry (relay: %s)", cfg.RelayURL)
		}
		deps.IdentityHandler = handlers.NewFederatedIdentityHandler(roster, localRoster)
		ph = ph.WithIdentityRoster(roster)
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
		// Central Bank gateway (no proxy): wire the PvP ledger so settling
		// orchestrators can report settled legs and receiving banks can read their
		// incoming credits. Requires DATABASE_URL for durable storage.
		if cfg.CentralBankAPIURL == "" && appDB != nil {
			ph = ph.WithPvPLedger(services.NewPvPLedgerService(appDB))
		}
		deps.PaymentHandler = ph

		// Commercial bank: wire escrow proxy that forwards to the Central Bank.
		if cfg.CentralBankAPIURL != "" {
			proxy := handlers.NewPaymentProxyHandler(
				cfg.CentralBankAPIURL,
				paymentGRPC,
				cfg.EntityBesuAddress,
				cfg.PaladinIdentity,
				cfg.CBPaladinIdentity,
				cfg.RelayAuthSecret,
			)
			deps.PaymentProxyHandler = proxy
			// Statement (extrato) consolidates this bank's deposit/tokenisation/redeem
			// records (sourced from the Central Bank via the same proxy), its sent
			// inter-bank PvP legs as debits (from this entity's orchestrator), and its
			// received PvP legs as credits (derived at the Central Bank from settled FX
			// agreements — the receiving side is never on the local orchestrator).
			deps.StatementHandler = handlers.NewStatementHandler(proxy).
				WithHTLCSource(paymentGRPC, cfg.BankCode).
				WithPvPCreditSource(proxy, cfg.BankCode)
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

	fiberApp := fiber.New(serverConfig())

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

// serverConfig is the api-gateway's Fiber configuration, including the connection
// timeouts (finding R2-LOW).
//
// WHY THESE THREE. Without ReadTimeout a connection can open, dribble its headers and hold
// a server slot for as long as it likes — the slowloris shape, and nothing in the handler
// chain can bound it because it happens before any handler runs. IdleTimeout does the same
// for keep-alive connections that stop sending anything.
//
// WHY A TIGHT WriteTimeout IS SAFE HERE, which is the non-obvious part. A cross-currency
// swap is documented as taking up to 180s and this gateway's own internal deadlines already
// reach 150s, so the reflex worry is that a 30s write timeout would cut a legitimate swap.
// It does not: fasthttp applies WriteTimeout to writing the response, not to the handler's
// duration. Measured, not assumed — a 5s handler completes under a 2s WriteTimeout, and
// TestServerTimeouts_SlowHandlerStillCompletes keeps that true if anyone retunes this.
func serverConfig() fiber.Config {
	return serverConfigWith(30*time.Second, 30*time.Second, 120*time.Second)
}

// serverConfigWith is serverConfig with the timeouts supplied, so the behaviour tests can
// exercise the same configuration on one-second bounds. Waiting on the production 30s
// ReadTimeout to fire cost 62 seconds per scenario, which is not a price a unit suite
// should pay to assert something a short timeout proves identically.
func serverConfigWith(read, write, idle time.Duration) fiber.Config {
	return fiber.Config{
		// BodyLimit and ReadTimeout are coupled: in fasthttp the read deadline covers the
		// headers AND the body, so the largest accepted body must be uploadable within
		// ReadTimeout. 10MB in 30s needs roughly 2.7 Mbit/s. Today's payloads are small
		// JSON and PEMs, so there is slack to spare — but move either number and check the
		// other still fits.
		BodyLimit:    10 * 1024 * 1024,
		AppName:      "api-gateway",
		ReadTimeout:  read,
		WriteTimeout: write,
		IdleTimeout:  idle,
	}
}
