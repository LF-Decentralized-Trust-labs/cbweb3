// Package app wires application dependencies and builds the configured Fiber app.
// Scenario B v2 services are wired here (T020).
package app

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	authadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/auth"
	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	identityadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/identity"
	paymentadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/payment"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/config"
	dbinit "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/db/init"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/router"
	v2router "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/router/v2"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	ammclient "github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/amm"
	tcebmclient "github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/tcebm"
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

	// Wire ZK pointer gate for supervisor verification (D-02 — gate was nil at runtime before this).
	// Gracefully disabled when DATABASE_URL is absent (non-CB entities without local compliance DB).
	var zkVerifier handlers.ZKPointerVerifier
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		if zkDB, zkErr := gorm.Open(postgres.Open(dbURL), &gorm.Config{}); zkErr == nil {
			zkVerifier = newZKPointerAdapter(services.NewZKPointerGate(zkDB))
		} else {
			log.Printf("warning: ZK pointer gate unavailable (%v), /zk-pointer/verify disabled", zkErr)
		}
	}
	supervisorHandler := handlers.NewSupervisorHandler(complianceGRPC, zkVerifier)
	authHandler := handlers.NewAuthHandler(identityGRPCProvider, identityManager, cfg.CookieSecure)
	complianceHandler := handlers.NewComplianceHandler(identityManager, complianceGRPC)

	// --- Scenario B v2 service wiring (T020) ---
	v2Deps := buildV2Dependencies(cfg, identityGRPCProvider, identityManager)

	deps := router.Dependencies{
		AuthHandler:       authHandler,
		ComplianceHandler: complianceHandler,
		GovernanceHandler: governanceHandler,
		SupervisorHandler: supervisorHandler,
		AuthProvider:      identityGRPCProvider,
		V2Deps:            v2Deps,
	}

	if cfg.CentralBankAPIURL != "" {
		deps.OnboardingProxyHandler = handlers.NewOnboardingProxyHandler(
			cfg.CentralBankAPIURL,
			cfg.PKIDir,
			cfg.BankCode,
			identityManager,
		)
		// Payment proxy: /api/v1/payments/{deposits,escrows,redeems} → Central Bank.
		// Requires PAYMENT_GRPC_ADDR to also be set (Zeto transfer in RequestRedeem).
		if cfg.PaymentGRPCAddr != "" {
			paymentGRPC, err := paymentadapter.NewGRPCAdapter(cfg.PaymentGRPCAddr, cfg.RequestTimeout)
			if err != nil {
				log.Printf("warning: payment gRPC unavailable at %s, payment proxy disabled: %v", cfg.PaymentGRPCAddr, err)
			} else {
				closers = append(closers, paymentGRPC)
				deps.PaymentProxyHandler = handlers.NewPaymentProxyHandler(
					cfg.CentralBankAPIURL,
					cfg.EntityBesuAddress,
					cfg.RelayAuthSecret,
				)
				deps.PaymentHandler = handlers.NewPaymentHandler(paymentGRPC)
			}
		}
	} else {
		deps.OnboardingHandler = handlers.NewOnboardingHandler(identityManager)
		// Payment handler (token balance): /api/v1/token/balance via gRPC.
		// Active even without CentralBankAPIURL when PAYMENT_GRPC_ADDR is set (Central Bank gateway).
		if cfg.PaymentGRPCAddr != "" {
			paymentGRPC, err := paymentadapter.NewGRPCAdapter(cfg.PaymentGRPCAddr, cfg.RequestTimeout)
			if err != nil {
				log.Printf("warning: payment gRPC unavailable at %s, token balance disabled: %v", cfg.PaymentGRPCAddr, err)
			} else {
				closers = append(closers, paymentGRPC)
				deps.PaymentHandler = handlers.NewPaymentHandler(paymentGRPC)
			}
		}
	}

	// Wire PaymentHandler and (for commercial banks) PaymentProxyHandler.
	if cfg.PaymentGRPCAddr != "" {
		paymentGRPC, err := paymentadapter.NewGRPCAdapter(cfg.PaymentGRPCAddr, cfg.RequestTimeout)
		if err != nil {
			log.Printf("warning: payment gRPC unavailable at %s: %v (payment endpoints disabled)", cfg.PaymentGRPCAddr, err)
		} else {
			closers = append(closers, paymentGRPC)
			deps.PaymentHandler = handlers.NewPaymentHandler(paymentGRPC)
			if cfg.CentralBankAPIURL != "" {
				deps.PaymentProxyHandler = handlers.NewPaymentProxyHandler(
					cfg.CentralBankAPIURL,
					cfg.EntityBesuAddress,
					cfg.RelayAuthSecret,
				)
			}
		}
	}

	fiberApp := fiber.New(
		fiber.Config{
			BodyLimit: 10 * 1024 * 1024,
			AppName:   "api-gateway",
		},
	)

	// CORS middleware: only enable if explicitly configured to avoid security issues.
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

// resolveHubChainIDStr reads HUB_CHAIN_ID from the environment, defaulting to "1337".
// Emits a warning via warnLogger when the variable is absent so operators can detect
// misconfigured environments without a service failure (FR-009 / Constitution VI).
func resolveHubChainIDStr(warnLogger *log.Logger) string {
	if v := os.Getenv("HUB_CHAIN_ID"); v != "" {
		return v
	}
	warnLogger.Printf("warning: HUB_CHAIN_ID not set; defaulting to 1337 (set HUB_CHAIN_ID to suppress this warning)")
	return "1337"
}

func closeAll(closers []io.Closer) {
	for _, c := range closers {
		if err := c.Close(); err != nil {
			log.Printf("warning: failed to close resource: %v", err)
		}
	}
}

// buildV2Dependencies wires Scenario B v2 services from environment variables.
// If DATABASE_URL or AMM_CONTRACT_ADDRESS are missing, returns partial deps
// (services that depend on DB/AMM are nil, and v2 routes that need them are skipped).
func buildV2Dependencies(cfg config.Config, authProvider interfaces.IAuthProvider, userManager interfaces.UserManager) v2router.Dependencies {
	// Ensure this CB's Hub signer is registered as LiquidityProvider in the Hub
	// IdentityRegistry (007-bridge-based-cb-liquidity / FR-004). Idempotent.
	bootstrapLiquidityProviderRole(context.Background())

	deps := v2router.Dependencies{
		AuthProvider: authProvider,
	}

	// FR-004 / T016 — Anti-G5-cross: wire CentralBankChecker.
	// Prefer on-chain checker (Hub IdentityRegistry) when available — it has cross-institution
	// visibility and blocks minting to any CB signer regardless of local DB state.
	// Falls back to the identity gRPC checker for non-CB environments.
	if identityRegAddr := os.Getenv("HUB_IDENTITY_REGISTRY_ADDRESS"); identityRegAddr != "" && os.Getenv("HUB_BESU_RPC_URL") != "" {
		onchain, err := newOnchainCBChecker(os.Getenv("HUB_BESU_RPC_URL"), identityRegAddr)
		if err != nil {
			log.Printf("warning: onchain CB checker unavailable (%v), falling back to identity gRPC", err)
			if userManager != nil {
				deps.CBChecker = newIdentityCBChecker(userManager)
			}
		} else {
			deps.CBChecker = onchain
			log.Printf("anti-G5-cross guard: using on-chain IdentityRegistry at %s", identityRegAddr)
		}
	} else if userManager != nil {
		deps.CBChecker = newIdentityCBChecker(userManager)
	}

	dbURL := os.Getenv("DATABASE_URL")
	ammAddr := os.Getenv("AMM_CONTRACT_ADDRESS")
	// Prefer SOVEREIGN_AMM_ADDRESS when set — CB entities use the sovereign AMM for
	// pool status (PoolStatusService reads reserves from the correct on-chain pool).
	// Commercial bank env files do not set SOVEREIGN_AMM_ADDRESS, so their behaviour
	// is unchanged (they continue to use the regular Hub AMM).
	if sovAMMAddr := os.Getenv("SOVEREIGN_AMM_ADDRESS"); sovAMMAddr != "" {
		ammAddr = sovAMMAddr
	}
	hubRPC := os.Getenv("HUB_BESU_RPC_URL")
	signerKey := os.Getenv("SIGNER_PRIVATE_KEY")
	chainIDStr := resolveHubChainIDStr(log.New(os.Stderr, "", 0))
	// When SOVEREIGN_HUB_TOKEN_A/B_ADDRESS is set, use it for the token preparer
	// (mint+approve). Falls back to HUB_TOKEN_A/B_ADDRESS for regular pairs.
	hubTokenAAddr := os.Getenv("HUB_TOKEN_A_ADDRESS")
	hubTokenBAddr := os.Getenv("HUB_TOKEN_B_ADDRESS")
	if sovTokA := os.Getenv("SOVEREIGN_HUB_TOKEN_A_ADDRESS"); sovTokA != "" {
		hubTokenAAddr = sovTokA
	}
	if sovTokB := os.Getenv("SOVEREIGN_HUB_TOKEN_B_ADDRESS"); sovTokB != "" {
		hubTokenBAddr = sovTokB
	}

	// Commercial banks: resolve sovereign Hub AMM + W-tokens from spoke CB (T054 option B).
	var cbPoolClient *services.CentralBankPoolClient
	var resolvedHubCfg *services.HubLiquidityConfig
	if cfg.CentralBankAPIURL != "" {
		cbPoolClient = services.NewCentralBankPoolClient(cfg.CentralBankAPIURL, cfg.RequestTimeout)
		hubCfg, err := cbPoolClient.GetHubLiquidityConfig(context.Background())
		if err != nil {
			log.Printf("warning: could not resolve hub liquidity from CB at %s: %v", cfg.CentralBankAPIURL, err)
		} else {
			resolvedHubCfg = hubCfg
			ammAddr = hubCfg.SovereignAMMAddress
			if hubCfg.SovereignHubTokenAAddress != "" {
				hubTokenAAddr = hubCfg.SovereignHubTokenAAddress
			}
			if hubCfg.SovereignHubTokenBAddress != "" {
				hubTokenBAddr = hubCfg.SovereignHubTokenBAddress
			}
			log.Printf("hub swap client: sovereign AMM %s resolved from CB (%s)", ammAddr, cfg.CentralBankAPIURL)
		}
	}

	pairRegistryAddr := os.Getenv("PAIR_REGISTRY_CONTRACT_ADDRESS")
	currencyRegistryAddr := os.Getenv("CURRENCY_REGISTRY_CONTRACT_ADDRESS")

	// If no DATABASE_URL, skip DB-dependent services gracefully.
	var db *gorm.DB
	if dbURL != "" {
		var err error
		db, err = gorm.Open(postgres.Open(dbURL), &gorm.Config{})
		if err != nil {
			log.Printf("warning: GORM connection failed (%s), v2 DB services disabled: %v", dbURL, err)
		} else {
			// Run GORM schema initialization (FR-055).
			if err := dbinit.RunAutoMigrate(db); err != nil {
				log.Printf("warning: GORM auto-migrate: %v", err)
			}
			if err := dbinit.CreateAppendOnlyTriggers(db); err != nil {
				log.Printf("warning: append-only triggers: %v", err)
			}
			if err := dbinit.CreatePartitions(db); err != nil {
				log.Printf("warning: partition creation: %v", err)
			}
		}
	}

	// Wire AMM client when Hub EVM config is present.
	var ammClient *ammclient.Client
	if ammAddr != "" && hubRPC != "" {
		chainID := int64(0)
		if chainIDStr != "" {
			bid := new(big.Int)
			if _, ok := bid.SetString(chainIDStr, 10); ok {
				chainID = bid.Int64()
			}
		}
		c, err := ammclient.NewClient(context.Background(), ammclient.Config{
			RPCURL:          hubRPC,
			ContractAddress: ammAddr,
			ChainID:         chainID,
			PrivateKeyHex:   signerKey,
			Timeout:         15 * time.Second,
			TokenAAddress:   hubTokenAAddr,
			TokenBAddress:   hubTokenBAddr,
		})
		if err != nil {
			log.Printf("warning: AMM client init failed: %v", err)
		} else {
			ammClient = c
		}
	}

	// US1 services: Quote, Swap, Pool Status
	var swapSvc *services.SwapService
	var poolGate services.PoolStatusGate
	// Captured for later attachment of the on-chain counterpart source, which requires
	// the LiquidityCommitRegistry client constructed further below.
	var poolStatusSvc *services.PoolStatusService
	// Commercial banks (CENTRAL_BANK_API_URL set) read pool status from spoke CB (FR-012).
	if cbPoolClient != nil {
		deps.PoolStatusService = cbPoolClient
		poolGate = cbPoolClient
		log.Printf("pool status: using Central Bank API at %s", cfg.CentralBankAPIURL)
	}
	if ammClient != nil {
		adapter := &ammAdapter{c: ammClient}
		deps.QuoteService = services.NewQuoteService(adapter)
		if cbPoolClient == nil {
			poolSvc := services.NewPoolStatusService(adapter)
			// Wire DB enricher for total_lp_count and pending_commits[] (T018).
			if db != nil {
				poolSvc.WithEnricher(newPoolStatusEnricher(db))
			}
			deps.PoolStatusService = poolSvc
			poolStatusSvc = poolSvc
			poolGate = services.NewPoolStatusGate(adapter)
		}

		var swapGates services.SwapGates
		if db != nil {
			swapGates.CircuitBreakerGate = services.NewCircuitBreakerGate(db)
		}
		// Pool status gate: blocks swaps when pool is EMPTY or PENDING_COUNTERPART (FR-011).
		if poolGate != nil {
			swapGates.PoolStatusGate = poolGate
		}
		swapSvc = services.NewSwapService(adapter, swapGates)
		swapSvc.WithFeeReader(adapter) // T058: adapter implements AMMFeeReader (GetFeeBps)
		deps.SwapService = swapSvc
	}

	// Hub token clients for mint+approve (prerequisite for central-bank liquidity provision).
	if hubTokenAAddr != "" && hubTokenBAddr != "" && hubRPC != "" && signerKey != "" && ammAddr != "" {
		chainID := int64(0)
		if chainIDStr != "" {
			bid := new(big.Int)
			if _, ok := bid.SetString(chainIDStr, 10); ok {
				chainID = bid.Int64()
			}
		}
		tokCfg := tcebmclient.Config{
			RPCURL:        hubRPC,
			ChainID:       chainID,
			PrivateKeyHex: signerKey,
			Timeout:       15 * time.Second,
		}
		tokCfgA := tokCfg
		tokCfgA.ContractAddress = hubTokenAAddr
		tokCfgB := tokCfg
		tokCfgB.ContractAddress = hubTokenBAddr
		tA, errA := tcebmclient.NewClient(context.Background(), tokCfgA)
		tB, errB := tcebmclient.NewClient(context.Background(), tokCfgB)
		if errA != nil || errB != nil {
			log.Printf("warning: Hub tCeBM client init failed: tokenA=%v tokenB=%v", errA, errB)
		} else {
			tp, errTP := NewTokenPrepareAdapter(context.Background(), tA, tB, ammAddr)
			if errTP != nil {
				log.Printf("warning: Hub tokenPrepareAdapter init failed (hasRole check): %v", errTP)
			} else {
				deps.TokenPreparer = tp
			}
		}
	}

	// US2 services: Bridge + Liquidity Provision
	var bridgeLockMintSvc *services.BridgeLockMintService
	var bridgeBurnUnlockSvc *services.BridgeBurnUnlockService
	if db != nil {
		bridgeLockMintSvc = services.NewBridgeLockMintService(db)
		bridgeBurnUnlockSvc = services.NewBridgeBurnUnlockService(db)
		// Wrap services for v2 router (strips correlation_id for backward compat)
		deps.BridgeLockMintService = &bridgeLockMintServiceWrapper{svc: bridgeLockMintSvc}
		deps.BridgeBurnUnlockService = &bridgeBurnUnlockServiceWrapper{svc: bridgeBurnUnlockSvc}
		deps.BridgePositionReader = services.NewBridgePositionReader(db)
	}
	if db != nil && ammClient != nil {
		adapter := &ammAdapter{c: ammClient}
		commitRepo := NewPoolCommitRepository(db)
		feeRepo := NewLPFeeEventRepository(db)
		liquiditySvc := services.NewLiquidityProvisionServiceWithRepos(db, adapter, commitRepo, feeRepo)
		deps.LiquidityService = liquiditySvc
		// 013-amm-lp-shares: expose the CB's live on-chain LP-share position to the governance portal.
		deps.LPBalanceReader = adapter
		// T058 / FR-006: Wire fee recorder so swap fees are distributed to LPs synchronously.
		if swapSvc, ok := deps.SwapService.(*services.SwapService); ok {
			swapSvc.WithFeeRecorder(liquiditySvc)
		}

		// Start the commit expiry worker as a background goroutine (FR-013 / D6).
		expiryWorker := services.NewPoolCommitExpiryWorker(commitRepo, 0)
		go expiryWorker.Run(context.Background())
	}

	// US3 services: Circuit Breaker + Oversight
	if db != nil && ammClient != nil {
		adapter := &ammAdapter{c: ammClient}
		deps.CircuitBreakerService = services.NewCircuitBreakerService(db, adapter)
	}
	if db != nil {
		deps.OversightService = services.NewOversightService(db)
	}

	// 009-commercial-cross-currency-swap: Wire orchestrator for cross-currency swaps (T014).
	if db != nil && swapSvc != nil && bridgeLockMintSvc != nil && bridgeBurnUnlockSvc != nil && ammClient != nil {
		adapter := &ammAdapter{c: ammClient}
		swapRepo := newCrossCurrencySwapRepository(db)
		quoteRepo := newSwapQuoteRepository(db)
		rollbackRepo := newSwapRollbackLogRepository(db)

		// Create adapters to match orchestrator interfaces
		bridgeLockMintAdapter := &bridgeLockMintAdapter{svc: bridgeLockMintSvc}
		bridgeBurnUnlockAdapter := &bridgeBurnUnlockAdapter{svc: bridgeBurnUnlockSvc}
		swapServiceAdapter := &swapServiceAdapter{svc: swapSvc}

		rollbackCoordinator := services.NewSwapRollbackCoordinator(bridgeBurnUnlockAdapter, rollbackRepo)

		var poolStatusChecker services.PoolStatusChecker
		if poolGate != nil {
			poolStatusChecker = poolGate
		} else {
			poolStatusChecker = services.NewPoolStatusGate(adapter)
		}
		circuitBreakerChecker := services.NewCircuitBreakerGate(db)

		var bridgeAssets *services.CrossCurrencyBridgeAssets
		if resolvedHubCfg != nil {
			bridgeAssets = services.CrossCurrencyBridgeAssetsFromHub(
				resolvedHubCfg,
				"BRL", "ARS",
				os.Getenv("TOKEN_ADDRESS"),
				"spoke-a",
			)
		}
		var bridgePoller services.BridgePositionPoller
		if deps.BridgePositionReader != nil {
			if r, ok := deps.BridgePositionReader.(*services.BridgePositionReader); ok {
				bridgePoller = r
			}
		}

		orchestrator := services.NewCrossCurrencySwapOrchestrator(
			swapRepo,
			quoteRepo,
			bridgeLockMintAdapter,
			bridgeBurnUnlockAdapter,
			swapServiceAdapter,
			poolStatusChecker,
			circuitBreakerChecker,
			rollbackCoordinator,
			bridgeAssets,
			bridgePoller,
		)

		// 009 sovereign model: attach Cacti relay when CACTI_API_URL and
		// INTERNAL_RELAY_AUTH_SECRET are set on this gateway (CB-A side).
		if cactiURL := os.Getenv("CACTI_API_URL"); cactiURL != "" {
			relaySecret := os.Getenv("INTERNAL_RELAY_AUTH_SECRET")
			if relaySecret != "" {
				cactiRelay := services.NewCactiCrossCurrencyRelay(cactiURL, relaySecret)
				orchestrator = orchestrator.WithCactiRelay(cactiRelay)
				log.Printf("[app] CrossCurrencySwapOrchestrator: Cacti relay wired (%s)", cactiURL)
			}
		}

		// 009 sovereign model: on commercial gateways (CENTRAL_BANK_API_URL set), delegate the
		// Step 1 bridge-in lock-mint to the spoke's CB. Minting W-<source> on the Hub requires
		// CENTRAL_BANK_ROLE, which a commercial bank must not hold — the CB does it instead.
		if cbURL := cfg.CentralBankAPIURL; cbURL != "" {
			relaySecret := os.Getenv("INTERNAL_RELAY_AUTH_SECRET")
			if relaySecret != "" {
				bridgeInRelay := services.NewCrossCurrencyBridgeInRelay(cbURL, relaySecret)
				orchestrator = orchestrator.WithBridgeInRelay(bridgeInRelay)
				log.Printf("[app] CrossCurrencySwapOrchestrator: bridge-in relay wired (CB %s)", cbURL)
			} else {
				log.Printf("[app] WARNING: CENTRAL_BANK_API_URL set but INTERNAL_RELAY_AUTH_SECRET empty — bridge-in cannot be delegated to the CB; commercial lock-mint will fail (no CENTRAL_BANK_ROLE)")
			}
		}

		// Derive the Hub signer address from SIGNER_PRIVATE_KEY so CB-B knows where
		// W-ARS landed after the AMM swap.
		if signerKey != "" {
			rawKey := strings.TrimPrefix(signerKey, "0x")
			if privKey, keyErr := crypto.HexToECDSA(rawKey); keyErr == nil {
				addr := crypto.PubkeyToAddress(privKey.PublicKey)
				orchestrator = orchestrator.WithHubSignerAddress(addr.Hex())
				log.Printf("[app] CrossCurrencySwapOrchestrator: hub signer address = %s", addr.Hex())
			}
		}

		deps.CrossCurrencySwapOrchestrator = orchestrator

		// 009-commercial-cross-currency-swap: Wire quote generator with 15s TTL (T030/T031).
		var quoteReserve services.AMMReserveReader
		if cbPoolClient != nil {
			quoteReserve = cbPoolClient
		} else {
			quoteReserve = &ammQuoteAdapter{adapter: adapter}
		}
		quoteGen := services.NewSwapQuoteGenerator(quoteReserve, quoteRepo)
		deps.SwapQuoteGenerator = quoteGen
	}

	// Phase 8: PairRegistry multi-pair service (FR-017 / D9-D11 / 005-cooperative-liquidity).
	// PAIR_REGISTRY_CONTRACT_ADDRESS enables propose/confirm; read-only ListActivePairs from DB only.
	if db != nil && pairRegistryAddr != "" && hubRPC != "" && signerKey != "" {
		chainID := int64(0)
		if chainIDStr != "" {
			bid := new(big.Int)
			if _, ok := bid.SetString(chainIDStr, 10); ok {
				chainID = bid.Int64()
			}
		}
		prClient, err := NewPairRegistryClient(context.Background(), PairRegistryConfig{
			RPCURL:          hubRPC,
			ContractAddress: pairRegistryAddr,
			ChainID:         chainID,
			PrivateKeyHex:   signerKey,
			Timeout:         15 * time.Second,
		})
		if err != nil {
			log.Printf("warning: PairRegistry client init failed: %v", err)
		} else {
			pairRepo := NewPairRepository(db)
			deps.PairService = services.NewPairService(prClient, pairRepo)
		}
	} else if db != nil {
		// Read-only mode: ListActivePairs only (no on-chain calls).
		pairRepo := NewPairRepository(db)
		deps.PairService = services.NewPairService(nil, pairRepo)
	}

	// 006-hub-currency-registry: CurrencyRegistry client (read/write on-chain, no DB).
	if currencyRegistryAddr != "" && hubRPC != "" && signerKey != "" {
		chainID := int64(0)
		if chainIDStr != "" {
			bid := new(big.Int)
			if _, ok := bid.SetString(chainIDStr, 10); ok {
				chainID = bid.Int64()
			}
		}
		crClient, err := NewCurrencyRegistryClient(context.Background(), CurrencyRegistryConfig{
			RPCURL:          hubRPC,
			ContractAddress: currencyRegistryAddr,
			ChainID:         chainID,
			PrivateKeyHex:   signerKey,
			Timeout:         15 * time.Second,
		})
		if err != nil {
			log.Printf("warning: CurrencyRegistry client init failed: %v", err)
		} else {
			deps.CurrencyService = services.NewCurrencyService(crClient)
		}
	}

	// 007-bridge-based-cb-liquidity: Sovereign CB liquidity services.
	// Requires LIQUIDITY_COMMIT_REGISTRY_ADDRESS and INTERNAL_RELAY_AUTH_SECRET.
	deps.InternalRelayAuthSecret = os.Getenv("INTERNAL_RELAY_AUTH_SECRET")
	lcrAddr := os.Getenv("LIQUIDITY_COMMIT_REGISTRY_ADDRESS")
	if lcrAddr != "" && hubRPC != "" && signerKey != "" && db != nil {
		chainID := int64(0)
		if chainIDStr != "" {
			bid := new(big.Int)
			if _, ok := bid.SetString(chainIDStr, 10); ok {
				chainID = bid.Int64()
			}
		}
		lcrClient, err := NewLiquidityCommitRegistryClient(context.Background(), LiquidityCommitRegistryConfig{
			RPCURL:          hubRPC,
			ContractAddress: lcrAddr,
			ChainID:         chainID,
			PrivateKeyHex:   signerKey,
			Timeout:         15 * time.Second,
		})
		if err != nil {
			log.Printf("warning: LiquidityCommitRegistry client init failed: %v", err)
		} else {
			commitRepo := NewPoolCommitRepository(db)
			lpRepo := newLPPositionRepository(db)
			var sovereignAmm *ammAdapter
			if ammClient != nil {
				sovereignAmm = &ammAdapter{c: ammClient}
			}
			sovereignSvc, errSov := services.NewSovereignLiquidityServiceFromEnv(db, sovereignAmm, commitRepo, lpRepo)
			if errSov != nil {
				log.Printf("warning: SovereignLiquidityService init failed: %v", errSov)
			} else {
				deps.SovereignLiquidityService = sovereignSvc
				deps.SovereignBridgeChecker = sovereignSvc
				deps.LCRRegistrar = &lcrHandlerAdapter{c: lcrClient}
				deps.LPPositionRepo = lpRepo // 008-fix-cb-liquidity: enable GET /liquidity/positions
			}
			// Surface a counterpart CB's on-chain PENDING commit on the opposite side
			// (cross-CB discovery). Only meaningful for CB gateways serving pool status directly.
			if poolStatusSvc != nil {
				counterpartSrc := newOnChainCounterpartSource(lcrClient, cfg.CommitSide)

				// Enable FX-suggested match amount via the Hub ManualOracle (optional —
				// degrades to no suggestion if the oracle is unset/unreachable). ownToken is
				// this gateway's own W-token; counterpartToken is the opposite side's.
				if oracleAddr := os.Getenv("ORACLE_ADDRESS"); oracleAddr != "" && hubRPC != "" {
					tokenA := os.Getenv("SOVEREIGN_HUB_TOKEN_A_ADDRESS")
					tokenB := os.Getenv("SOVEREIGN_HUB_TOKEN_B_ADDRESS")
					ownToken, counterpartToken := tokenA, tokenB
					if cfg.CommitSide == "B" {
						ownToken, counterpartToken = tokenB, tokenA
					}
					if ownToken != "" && counterpartToken != "" {
						oracleClient, oErr := NewManualOracleClient(context.Background(), ManualOracleConfig{
							RPCURL:          hubRPC,
							ContractAddress: oracleAddr,
							Timeout:         15 * time.Second,
						})
						if oErr != nil {
							log.Printf("warning: ManualOracle client init failed (FX suggestion disabled): %v", oErr)
						} else {
							counterpartSrc.withFXSuggestion(oracleClient, common.HexToAddress(ownToken), common.HexToAddress(counterpartToken))
							log.Printf("[counterpart] FX suggestion enabled via oracle %s (own=%s counterpart=%s)", oracleAddr, ownToken, counterpartToken)
						}
					}
				}

				poolStatusSvc.WithCounterpartSource(counterpartSrc)
			}
		}
	}

	// 008-fix-cb-liquidity: Populate simplified API config fields from cfg.
	deps.SpokeNetwork = cfg.SpokeNetwork
	deps.NativeAssetSymbol = cfg.NativeAssetSymbol
	deps.WTokenAddress = cfg.WTokenAddress
	deps.BankCode = cfg.BankCode
	deps.CommitSide = cfg.CommitSide
	// FR-013: approve-amm side auto-detection from BANK_CODE.
	deps.ApproveSide = cfg.ApproveSide
	// Fix: populate LOCAL_CB_HUB_SIGNER for balance checks (recipient of lock-mint tokens).
	deps.LocalCBHubSigner = strings.ToLower(os.Getenv("LOCAL_CB_HUB_SIGNER"))

	// 009-commercial-cross-currency-swap: cross-currency bridge-out receiver (CB-B side).
	// Register the internal Cacti relay endpoint when this gateway has a BridgeBurnUnlockService
	// and a fiat token address configured (i.e. is a sovereign CB with a spoke).
	deps.FiatTokenAddress = os.Getenv("TOKEN_ADDRESS")
	if bridgeBurnUnlockSvc != nil && deps.FiatTokenAddress != "" {
		deps.CrossCurrencyBurnEnqueuer = &bridgeBurnUnlockAdapter{svc: bridgeBurnUnlockSvc}
		// Resolver: looks up beneficiary bank on-chain address from the local participants table.
		// CB-B is the sovereign authority — it knows its member banks' wallet addresses.
		if db != nil {
			deps.CrossCurrencyBeneficiaryResolver = services.NewParticipantResolver(db)
		}
	}

	// 009 sovereign model: cross-currency bridge-in issuer (CB-A side). Registered only on CB
	// gateways — a commercial gateway (CENTRAL_BANK_API_URL set) delegates TO a CB and never
	// receives delegations. The CB performs the W-<source> lock-mint with its own relayer
	// (which holds CENTRAL_BANK_ROLE) on behalf of the payer bank.
	if cfg.CentralBankAPIURL == "" && db != nil && bridgeLockMintSvc != nil &&
		deps.FiatTokenAddress != "" && deps.WTokenAddress != "" {
		deps.CrossCurrencyLockMintEnqueuer = &bridgeLockMintAdapter{svc: bridgeLockMintSvc}
		deps.CrossCurrencyBridgeStateReader = services.NewBridgePositionReader(db)
		// Reserve Tokenisation enforcement: verify the payer bank holds tCeBM before lock-mint.
		// Requires PAYMENT_GRPC_ADDR (reads tCeBM.balanceOf) and DB (resolves bank wallet).
		if payGRPCAddr := cfg.PaymentGRPCAddr; payGRPCAddr != "" {
			bridgeInPayGRPC, payErr := paymentadapter.NewGRPCAdapter(payGRPCAddr, cfg.RequestTimeout)
			if payErr != nil {
				log.Printf("warning: payment gRPC for bridge-in balance check unavailable (%s): %v — Reserve Tokenisation enforcement disabled", payGRPCAddr, payErr)
			} else {
				deps.CrossCurrencyPayerBalanceChecker = bridgeInPayGRPC
				deps.CrossCurrencyPayerWalletResolver = services.NewParticipantResolver(db)
				log.Printf("[app] bridge-in Reserve Tokenisation enforcement enabled (payment gRPC %s)", payGRPCAddr)
			}
		} else {
			log.Printf("[app] WARNING: PAYMENT_GRPC_ADDR not set — Reserve Tokenisation balance enforcement disabled on bridge-in handler")
		}
	}

	return deps
}

// bridgeLockMintServiceWrapper wraps BridgeLockMintService to remove correlation_id for backward compat with v2 router.
type bridgeLockMintServiceWrapper struct {
	svc *services.BridgeLockMintService
}

func (w *bridgeLockMintServiceWrapper) LockAndEnqueue(ctx context.Context, ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount string) (*services.BridgePositionResult, error) {
	return w.svc.LockAndEnqueue(ctx, ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount, "")
}

// bridgeBurnUnlockServiceWrapper wraps BridgeBurnUnlockService to remove correlation_id for backward compat with v2 router.
type bridgeBurnUnlockServiceWrapper struct {
	svc *services.BridgeBurnUnlockService
}

func (w *bridgeBurnUnlockServiceWrapper) BurnAndEnqueue(ctx context.Context, positionID string) (*services.BridgePositionResult, error) {
	return w.svc.BurnAndEnqueue(ctx, positionID, "")
}

// bridgeLockMintAdapter adapts BridgeLockMintService to add correlation_id parameter for orchestrator (009).
type bridgeLockMintAdapter struct {
	svc *services.BridgeLockMintService
}

func (a *bridgeLockMintAdapter) LockAndEnqueue(ctx context.Context, ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount, correlationID string, mintToHubAddress ...string) (*services.BridgePositionResult, error) {
	return a.svc.LockAndEnqueue(ctx, ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount, correlationID, mintToHubAddress...)
}

// bridgeBurnUnlockAdapter adapts BridgeBurnUnlockService to add correlation_id parameter for orchestrator (009).
type bridgeBurnUnlockAdapter struct {
	svc *services.BridgeBurnUnlockService
}

func (a *bridgeBurnUnlockAdapter) BurnAndEnqueue(ctx context.Context, positionID, correlationID string) (*services.BridgePositionResult, error) {
	return a.svc.BurnAndEnqueue(ctx, positionID, correlationID)
}

func (a *bridgeBurnUnlockAdapter) EnqueueBurnAfterSwap(ctx context.Context, ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount, correlationID string, extras ...string) (*services.BridgePositionResult, error) {
	return a.svc.EnqueueBurnAfterSwap(ctx, ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount, correlationID, extras...)
}

// swapServiceAdapter adapts SwapService to match orchestrator interface (009).
type swapServiceAdapter struct {
	svc *services.SwapService
}

func (a *swapServiceAdapter) Execute(ctx context.Context, req services.SwapRequest) (*services.SwapResult, error) {
	return a.svc.Execute(ctx, req)
}

// ammQuoteAdapter adapts ammAdapter to match SwapQuoteGenerator's AMMReserveReader interface (009).
type ammQuoteAdapter struct {
	adapter *ammAdapter
}

func (a *ammQuoteAdapter) GetPoolReserves(ctx context.Context, pair string) (string, string, float64, error) {
	return a.adapter.GetPoolReserves(ctx, pair)
}

func (a *ammQuoteAdapter) GetFeeBps(ctx context.Context, pair string) (uint16, error) {
	return a.adapter.GetFeeBpsForPair(ctx, pair)
}
