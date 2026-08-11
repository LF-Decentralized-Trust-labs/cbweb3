// SPDX-License-Identifier: Apache-2.0

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

	authadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/auth"
	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	identityadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/identity"
	paymentadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/payment"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/config"
	dbinit "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/db/init"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/router"
	v2router "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/router/v2"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/relayauth"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	tcebmclient "github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/tcebm"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/authz"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// newSignedPaymentProxy builds the bank-side payment proxy with this entity's signer attached.
//
// One constructor for both wiring paths: they differ only in when the payment gRPC connection is
// available, and having each build the proxy on its own is how one of them ends up not signing.
func newSignedPaymentProxy(cfg config.Config) *handlers.PaymentProxyHandler {
	proxy := handlers.NewPaymentProxyHandler(cfg.CentralBankAPIURL, cfg.EntityBesuAddress, cfg.RelayAuthSecret)
	if s, err := relayauth.LoadSigner(cfg.PKIDir, cfg.RelayKeyID); err == nil {
		log.Printf("[app] payment proxy: per-entity signature enabled (key-id=%s)", cfg.RelayKeyID)
		return proxy.WithSigner(s)
	} else if cfg.PKIDir != "" {
		log.Printf("[app] payment proxy: signing unavailable (%v) — deposits/escrows/redeems fall back to the shared secret", err)
	}
	return proxy
}

// relayAuthConfigFor builds the internal-relay auth configuration from the environment: peer
// verifying keys pinned from PKI_DIR/<entity>.crt, the legacy shared secret, and whether signatures
// are mandatory.
//
// Silent by design — it is called twice, once by New to validate before any side effect and once by
// the dependency wiring — so the logging lives with the caller that reports the outcome. Loading is
// file reads only, which is why calling it twice is cheap enough to prefer over threading the
// result through construction.
func relayAuthConfigFor(cfg config.Config) middleware.RelayAuthConfig {
	registry, _ := relayauth.LoadRegistryGlob(cfg.PKIDir)
	return middleware.RelayAuthConfig{
		Registry:     relayauth.NewStore(registry),
		LegacySecret: os.Getenv("INTERNAL_RELAY_AUTH_SECRET"),
		// One accepted signature, one request. The window a verified signature stays replayable in
		// is the window in which a captured transfer-limit Restore keeps giving a bank its daily
		// allowance back, so the guard's memory is exactly that window.
		Replay:           relayauth.NewReplayGuard(relayauth.DefaultMaxSkew),
		RequireSignature: cfg.RelayRequireSignature,
	}
}

// validateRelayAuthForBoot decides whether this gateway may serve traffic with the relay-auth
// settings it was given, judging the registry that will ACTUALLY verify requests.
//
// The file glob alone is the wrong thing to judge on a central bank. Its peers are the banks it
// onboarded, and those are pinned from the participants table — its PKI dir holds no peer
// certificates at all. Validating the file registry therefore refused to start exactly the
// deployment the enforcement flag exists for: enforcement on, peers pinned from the database, the
// relay's leaf certificate not distributed to this host (a documented cross-VM gap). So the pins are
// folded in first, using a connection opened and closed here — reads only, and before any gRPC dial,
// worker or on-chain write, which is the property that made the check belong at boot in the first
// place.
//
// A database that cannot be read is deliberately NOT a refusal. Then nothing is known about the pins,
// and refusing over a transient outage would take the gateway down for a condition that resolves
// itself: the periodic refresher pins the peers as soon as the database answers. The state is logged
// instead, and requests still fail closed one at a time.
func validateRelayAuthForBoot(cfg config.Config) (middleware.RelayAuthConfig, error) {
	c := relayAuthConfigFor(cfg)
	if !c.RequireSignature {
		return c, nil
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// No participant source exists; the files are the whole registry and judging them is right.
		return c, c.Validate()
	}
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Printf("[app] relay auth: could not read participant pins at boot (%v) — starting anyway; "+
			"internal requests fail closed until the periodic refresh pins this CB's peers", err)
		return c, nil
	}
	if sqlDB, sqlErr := db.DB(); sqlErr == nil {
		defer func() { _ = sqlDB.Close() }()
	}
	pins, pinErr := loadParticipantPins(context.Background(), db)
	if pinErr != nil {
		log.Printf("[app] relay auth: could not read participant pins at boot (%v) — starting anyway; "+
			"internal requests fail closed until the periodic refresh pins this CB's peers", pinErr)
		return c, nil
	}
	return c, validateRelayAuthWithPins(c, cfg.PKIDir, pins)
}

// validateRelayAuthWithPins is the decision itself: assemble both sources, then judge. Split out so
// the rule can be tested with pins in hand, which is the part that was wrong — not the plumbing that
// fetches them.
func validateRelayAuthWithPins(c middleware.RelayAuthConfig, pkiDir string, pins []relayauth.ParticipantPin) error {
	files, _ := relayauth.LoadRegistryGlob(pkiDir)
	c.Registry.Set(relayauth.BuildRegistry(files, pins))
	return c.Validate()
}

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
	// Refuse a relay-auth configuration that would answer 401 to every internal request
	// (enforcement demanded, nothing pinned to verify against) BEFORE anything else happens.
	//
	// Placement matters and was chosen from a live run: validating after buildV2Dependencies also
	// refuses to start, but by then the wiring has already dialled gRPC, started background workers
	// and — through bootstrapLiquidityProviderRole — submitted an on-chain transaction. A process
	// that refuses to start must not have written to the ledger first.
	if _, err := validateRelayAuthForBoot(cfg); err != nil {
		return nil, fmt.Errorf("relay auth configuration: %w", err)
	}

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

	// Make the replay guard hold across REPLICAS, not just inside this process.
	//
	// Its in-memory half protects one gateway. A central bank that scales its gateway out gets no
	// protection from that at all — the captured request goes to a replica which has never seen the
	// signature — and the control this exists for is a compliance one: a replayed transfer-limit
	// restore credits a bank's daily allowance back and lets it transact past its configured limit.
	// Redis is where that becomes a shared decision; it already runs per entity (REDIS_ADDR, the
	// same instance the auth service keeps its login nonces in), so this adds no infrastructure.
	//
	// Without REDIS_ADDR the guard stays single-process and says so, because "one replica" then
	// becomes a property the deployment has to hold rather than one the code enforces.
	if redisAddr := os.Getenv("REDIS_ADDR"); redisAddr != "" {
		seen := relayauth.NewRedisSeenStore(redisAddr, os.Getenv("REDIS_PASSWORD"), 0)
		v2Deps.RelayAuth.Replay.WithShared(seen)
		closers = append(closers, seen)
		log.Printf("[app] relay auth: replay guard shared via Redis at %s — one signature is admitted "+
			"once across every replica of this gateway", redisAddr)
	} else {
		log.Printf("[app] relay auth: replay guard is IN-MEMORY only (REDIS_ADDR unset) — a captured " +
			"request replayed against a DIFFERENT replica of this gateway would not be caught; run a " +
			"single replica, or set REDIS_ADDR. Harmless on a gateway that serves no signed internal " +
			"routes (the hub), material on a central bank")
	}

	deps := router.Dependencies{
		AuthHandler:       authHandler,
		ComplianceHandler: complianceHandler,
		GovernanceHandler: governanceHandler,
		SupervisorHandler: supervisorHandler,
		SpokesHandler:     handlers.NewSpokesHandler(complianceGRPC),
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
				deps.PaymentProxyHandler = newSignedPaymentProxy(cfg)
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
				deps.PaymentProxyHandler = newSignedPaymentProxy(cfg)
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
	// The CB's relayer signs on the hub with its own identity (separate nonce space); it needs
	// CENTRAL_BANK_ROLE on this CB's W-token to mint and burn. This only CHECKS the grant — making
	// it is a provisioning act now that token administration no longer rests with this gateway.
	// A no-op on a bank.
	verifyRelayerIssuanceRole(context.Background())

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
	hubRPC := os.Getenv("HUB_BESU_RPC_URL")
	signerKey := os.Getenv("SIGNER_PRIVATE_KEY")
	// This gateway's own Hub address. A CB needs it to tell a delegating bank where a swap output
	// landed; a bank that delegates every Hub act has no signing key and leaves this empty.
	//
	// LOCAL_CB_HUB_SIGNER (the address) is preferred over deriving it from SIGNER_PRIVATE_KEY,
	// because it is the form that survives production custody: a KMS never exports the key, so the
	// address has to arrive as configuration. Deriving from the key stays as the fallback for
	// deployments that only set the key, and the two are cross-checked when both are present —
	// a mismatch means the environment describes two different identities, which would have the
	// gateway report an address it cannot sign from.
	hubSignerAddr := strings.TrimSpace(os.Getenv("LOCAL_CB_HUB_SIGNER"))
	if signerKey != "" {
		if privKey, keyErr := crypto.HexToECDSA(strings.TrimPrefix(signerKey, "0x")); keyErr == nil {
			derived := crypto.PubkeyToAddress(privKey.PublicKey).Hex()
			switch {
			case hubSignerAddr == "":
				hubSignerAddr = derived
			case !strings.EqualFold(hubSignerAddr, derived):
				log.Printf("warning: LOCAL_CB_HUB_SIGNER (%s) is not the address of SIGNER_PRIVATE_KEY (%s) — using the derived address, since that is the one this gateway can actually sign from", hubSignerAddr, derived)
				hubSignerAddr = derived
			}
		} else {
			log.Printf("warning: SIGNER_PRIVATE_KEY is not a valid secp256k1 key: %v — Hub signing disabled", keyErr)
		}
	}
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
			// The CB publishes no static single-AMM config (the dynamic per-pair
			// model): drop the static client so pool status, the pool gate and the
			// cross-currency quote reserve reader all fall through to the on-chain
			// PairRegistry resolver instead of a fixed sovereign AMM.
			log.Printf("warning: could not resolve hub liquidity from CB at %s: %v; using dynamic per-pair resolution", cfg.CentralBankAPIURL, err)
			cbPoolClient = nil
		} else {
			resolvedHubCfg = hubCfg
			if hubCfg.SovereignHubTokenAAddress != "" {
				hubTokenAAddr = hubCfg.SovereignHubTokenAAddress
			}
			if hubCfg.SovereignHubTokenBAddress != "" {
				hubTokenBAddr = hubCfg.SovereignHubTokenBAddress
			}
			log.Printf("hub liquidity config resolved from CB (%s)", cfg.CentralBankAPIURL)
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

	// TD-001: there is no default/bootstrap AMM. Every AMM operation resolves the
	// pool's dedicated contract per pool_pair via the on-chain PairRegistry (below).

	// Dynamic per-pair resolver: quote/swap/liquidity/mint resolve the AMM +
	// W-tokens for a pool_pair from the on-chain PairRegistry, so a corridor opened
	// at runtime (e.g. a new spoke) works with no gateway env/restart.
	var pairResolver *pairAMMResolver
	if pairRegistryAddr != "" && hubRPC != "" {
		hubChainID := int64(0)
		if chainIDStr != "" {
			bid := new(big.Int)
			if _, ok := bid.SetString(chainIDStr, 10); ok {
				hubChainID = bid.Int64()
			}
		}
		if pr, err := NewPairRegistryClient(context.Background(), PairRegistryConfig{
			RPCURL:          hubRPC,
			ContractAddress: pairRegistryAddr,
			ChainID:         hubChainID,
			PrivateKeyHex:   signerKey,
			Timeout:         15 * time.Second,
		}); err != nil {
			log.Printf("warning: pair AMM resolver init failed: %v", err)
		} else {
			pairResolver = newPairAMMResolver(pr, hubRPC, hubChainID, signerKey, 15*time.Second, currencyCodeFromSymbol(os.Getenv("NATIVE_ASSET_SYMBOL")))
			deps.PairSideResolver = pairResolver
			// Sovereign seeding driver (escrow-and-finalize, no LCR): resolves side + AMM +
			// commit id per pair and drives depositForCommit/finalizeCommit/cancelCommitDeposit.
			deps.SovereignSeed = &sovereignSeedAdapter{a: &ammAdapter{resolver: pairResolver}}
			log.Printf("dynamic per-pair AMM resolution enabled (PairRegistry %s)", pairRegistryAddr)
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
	if pairResolver != nil {
		adapter := &ammAdapter{resolver: pairResolver}
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
	if hubTokenAAddr != "" && hubTokenBAddr != "" && hubRPC != "" && signerKey != "" {
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
			tp, errTP := NewTokenPrepareAdapter(context.Background(), tA, tB, pairResolver)
			if errTP != nil {
				log.Printf("warning: Hub tokenPrepareAdapter init failed (hasRole check): %v", errTP)
			} else {
				deps.TokenPreparer = tp
			}
		}
	}

	// Sovereign dynamic model: when no static HUB_TOKEN_A/B_ADDRESS is configured
	// (the per-pair sovereign deploy), still expose a token preparer backed only by
	// the on-chain PairRegistry resolver. Every mint/approve resolves the pair's
	// W-tokens + AMM from pool_pair, so the sovereign-seed (/liquidity/deposit-side)
	// and token routes register without pinning a fixed pair via env.
	if deps.TokenPreparer == nil && pairResolver != nil && signerKey != "" {
		deps.TokenPreparer = NewDynamicTokenPrepareAdapter(pairResolver)
		log.Printf("token preparer: dynamic per-pair mode (on-chain PairRegistry; no static HUB_TOKEN_A/B)")
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
	if db != nil && pairResolver != nil {
		adapter := &ammAdapter{resolver: pairResolver}
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
	if db != nil && pairResolver != nil {
		adapter := &ammAdapter{resolver: pairResolver}
		deps.CircuitBreakerService = services.NewCircuitBreakerService(db, adapter)
	}
	// Circuit-breaker institutional attestation is signed server-side with the CB's PKI
	// key (PKI_DIR/<BANK_CODE>.key), so operators never supply a signature by hand.
	if cfg.PKIDir != "" && cfg.BankCode != "" {
		if cbSigner, sErr := relayauth.LoadSigner(cfg.PKIDir, cfg.RelayKeyID); sErr == nil {
			deps.CircuitBreakerSigner = cbSigner
		} else {
			log.Printf("[app] circuit-breaker attestation key unavailable for %q: %v (attestation left empty)", cfg.BankCode, sErr)
		}
	}
	if db != nil {
		deps.OversightService = services.NewOversightService(db)
	}

	// R1-10.1: Transfer limit enforcement — wired differently for CB vs commercial bank.
	// CB (CentralBankAPIURL == ""): local DB checker + internal pre-auth endpoint + governance CRUD.
	// Commercial bank (CentralBankAPIURL != ""): remote checker delegates to CB; fail-closed.
	var transferLimitChecker services.TransferLimitCheckerIface
	if cfg.CentralBankAPIURL == "" {
		if db != nil {
			limitRepo := newTransferLimitRepository(db)
			volumeRepo := newTransferVolumeRepository(db)
			localChecker := services.NewTransferLimitChecker(limitRepo, volumeRepo)
			transferLimitChecker = localChecker
			sovereignCurrency := cfg.FiatSymbol
			if sovereignCurrency == "" {
				sovereignCurrency = os.Getenv("NATIVE_ASSET_SYMBOL")
			}
			deps.TransferLimitHandler = handlers.NewTransferLimitHandler(limitRepo).
				WithFallbackBankCode(cfg.BankCode).
				WithSovereignCurrency(sovereignCurrency)
			deps.TransferLimitInternalHandler = handlers.NewTransferLimitInternalHandler(localChecker)
		}
	} else {
		relaySecret := os.Getenv("INTERNAL_RELAY_AUTH_SECRET")
		if relaySecret != "" {
			remoteChecker := services.NewRemoteTransferLimitChecker(cfg.CentralBankAPIURL, relaySecret, cfg.RequestTimeout)
			// Sign the delegation with this entity's own key, so the CB can attribute the daily-limit
			// call to a specific bank instead of to "whoever holds the shared secret" — which is
			// every entity, since the secret is identical across the deployment.
			if s, sErr := relayauth.LoadSigner(cfg.PKIDir, cfg.RelayKeyID); sErr == nil {
				remoteChecker = remoteChecker.WithSigner(s)
				log.Printf("[app] transfer limit pre-auth: per-entity signature enabled (key-id=%s)", cfg.RelayKeyID)
			} else if cfg.PKIDir != "" {
				log.Printf("[app] transfer limit pre-auth: signing unavailable (%v) — falling back to the shared secret", sErr)
			}
			transferLimitChecker = remoteChecker
			log.Printf("[app] transfer limit enforcement: delegating pre-auth to CB at %s", cfg.CentralBankAPIURL)
		} else {
			log.Printf("[app] WARNING: CENTRAL_BANK_API_URL set but INTERNAL_RELAY_AUTH_SECRET missing — transfer limit enforcement disabled")
		}
	}
	if transferLimitChecker != nil {
		deps.TransferLimitChecker = transferLimitChecker
	}

	// 009-commercial-cross-currency-swap: Wire orchestrator for cross-currency swaps (T014).
	if db != nil && swapSvc != nil && bridgeLockMintSvc != nil && bridgeBurnUnlockSvc != nil && pairResolver != nil {
		adapter := &ammAdapter{resolver: pairResolver}
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
			// Derive this gateway's own sovereign currency from its native token symbol
			// (e.g. tCeBM_BRL -> BRL) instead of hardcoding BRL/ARS; the source spoke follows
			// the toolkit's "spoke-<currency>" convention. Generalizes to any sovereign spoke.
			selfCurrency := currencyCodeFromSymbol(os.Getenv("NATIVE_ASSET_SYMBOL"))
			bridgeAssets = services.CrossCurrencyBridgeAssetsFromHub(
				resolvedHubCfg,
				selfCurrency, "",
				os.Getenv("TOKEN_ADDRESS"),
				"spoke-"+strings.ToLower(selfCurrency),
			)
		}
		var bridgePoller services.BridgePositionPoller
		if deps.BridgePositionReader != nil {
			if r, ok := deps.BridgePositionReader.(*services.BridgePositionReader); ok {
				bridgePoller = r
			}
		}

		// R2-CR-6: per-CB signer for outbound internal relay calls, loaded from this
		// gateway's PKI key (PKI_DIR/<BANK_CODE>.key). nil falls back to the legacy secret.
		var relaySigner *relayauth.Signer
		if cfg.PKIDir != "" && cfg.BankCode != "" {
			if s, sErr := relayauth.LoadSigner(cfg.PKIDir, cfg.RelayKeyID); sErr == nil {
				relaySigner = s
			} else {
				log.Printf("[app] relay signing key unavailable for %q: %v (internal relay calls use legacy secret)", cfg.BankCode, sErr)
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
				// R2-CR-6: sign bridge-in with this gateway's PKI key so the issuing CB can
				// authenticate it asymmetrically (per-CB), not just on the shared secret.
				if relaySigner != nil {
					bridgeInRelay = bridgeInRelay.WithSigner(relaySigner)
					log.Printf("[app] bridge-in relay: per-CB signature enabled (key-id=%s)", cfg.BankCode)
				}
				orchestrator = orchestrator.WithBridgeInRelay(bridgeInRelay)
				log.Printf("[app] CrossCurrencySwapOrchestrator: bridge-in relay wired (CB %s)", cbURL)

				// Step 2 goes to the same CB over the same channel: the Hub AMM admits only
				// verified Hub participants and this gateway is not one. Delegating the trade
				// keeps the CB's key inside the CB — the alternative is signing on the Hub with
				// a key this container should never hold.
				hubSwapRelay := services.NewCrossCurrencyHubSwapRelay(cbURL, relaySecret)
				if relaySigner != nil {
					hubSwapRelay = hubSwapRelay.WithSigner(relaySigner)
				}
				orchestrator = orchestrator.WithHubSwapRelay(hubSwapRelay)
				log.Printf("[app] CrossCurrencySwapOrchestrator: hub-swap relay wired (CB %s) — no Hub signing key needed on this gateway", cbURL)

				// Step 4 goes back to the same CB over the same channel: it bridged the
				// slippage buffer in, so it is the one that can give the remainder back.
				residueRelay := services.NewCrossCurrencyResidueRelay(cbURL, relaySecret)
				if relaySigner != nil {
					residueRelay = residueRelay.WithSigner(relaySigner)
				}
				orchestrator = orchestrator.WithResidueReturnRelay(residueRelay)
				log.Printf("[app] CrossCurrencySwapOrchestrator: residue-return relay wired (CB %s)", cbURL)
			} else {
				log.Printf("[app] WARNING: CENTRAL_BANK_API_URL set but INTERNAL_RELAY_AUTH_SECRET empty — bridge-in cannot be delegated to the CB; commercial lock-mint will fail (no CENTRAL_BANK_ROLE)")
			}
		}

		// The Hub signer address (derived above) tells CB-B where W-<target> landed after a
		// locally executed swap. When Step 2 is delegated, the executing CB reports its own
		// address on the response instead — this gateway may hold no Hub key at all.
		if hubSignerAddr != "" {
			orchestrator = orchestrator.WithHubSignerAddress(hubSignerAddr)
			log.Printf("[app] CrossCurrencySwapOrchestrator: hub signer address = %s", hubSignerAddr)
		}

		if transferLimitChecker != nil {
			orchestrator = orchestrator.WithTransferLimitChecker(transferLimitChecker)
		}
		// Local Step 4 path (this gateway is the issuing CB): the residue must be minted back
		// to the payer's own spoke wallet, resolved from the participants registry.
		if db != nil {
			orchestrator = orchestrator.WithPayerWalletResolver(services.NewParticipantResolver(db))
			// A locally executed Step 2 must record what it cost, in the same place the delegated
			// path does. Without it the Hub reconciliation reads the position as never swapped and
			// claims more is on the Hub than the balance holds.
			orchestrator = orchestrator.WithHubSwapConsumptionRecorder(newCrossCurrencyHubSwapRepository(db))
		}
		// Dynamic per-pair model: let Step 3 tell the Cacti relay which AMM to run
		// its isPaused() gate against, resolved from the on-chain PairRegistry.
		if pairResolver != nil {
			orchestrator = orchestrator.WithAMMAddressResolver(&ammAddrResolverAdapter{r: pairResolver})
		}
		deps.CrossCurrencySwapOrchestrator = orchestrator
		// Expose the swap repository for the paginated history endpoint (GET /amm/swap/cross-currency).
		deps.CrossCurrencySwapLister = swapRepo
		// Re-drive residue returns whose enqueue failed. A failed enqueue creates no bridge
		// position, so the relayer queue has nothing to retry and the payer's unspent reserve
		// would sit on the issuing CB's Hub address indefinitely.
		startResidueRetryWorker(orchestrator, swapRepo)

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
	// PAIR_REGISTRY_CONTRACT_ADDRESS enables propose/confirm; without a signing key the client is
	// still built, read-only, because the hub registry — not this entity's database — is the
	// catalogue of pairs. A commercial bank holds no hub key and would otherwise list nothing.
	if db != nil {
		pairRepo := NewPairRepository(db)
		pairMode := resolveHubRegistryMode(pairRegistryAddr, hubRPC, signerKey)
		// DB-only is the floor, not a failure: the pairs route stays served even when the hub is
		// unreachable, rather than disappearing into a 404 that reads like a missing feature.
		deps.PairService = services.NewPairService(nil, pairRepo)

		if pairMode != hubRegistryDisabled {
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
				// Enables ProposePair to deploy a dedicated per-pair AMM (empty amm_address path).
				IdentityRegistryAddress: os.Getenv("HUB_IDENTITY_REGISTRY_ADDRESS"),
			})
			if err != nil {
				log.Printf("warning: PairRegistry client init failed, pairs served from DB only: %v", err)
			} else {
				// The authority reader lets an unauthorized confirm be refused with its real reason
				// instead of a reverted transaction and a generic message.
				deps.PairService = services.NewPairService(prClient, pairRepo).
					WithTokenAuthorityReader(prClient)
				log.Printf("[app] PairRegistry client ready (%s)", pairMode)
			}
		}
	}

	// 006-hub-currency-registry: CurrencyRegistry client (read/write on-chain, no DB).
	// Listing currencies is a view call and maps token addresses to symbols for every portal, so it
	// is built without a signing key too; the adapter's write methods refuse a nil signer.
	if currencyMode := resolveHubRegistryMode(currencyRegistryAddr, hubRPC, signerKey); currencyMode != hubRegistryDisabled {
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
			log.Printf("[app] CurrencyRegistry client ready (%s)", currencyMode)

			// Sovereign wrapped-token supply (GET /api/v2/hub/token/supply): resolves this
			// CB's own W-tCeBM_<CUR> through the registry it just wired, so a currency
			// registered at runtime needs no restart.
			if ssa := newSovereignSupplyAdapter(deps.CurrencyService, hubRPC,
				os.Getenv("NATIVE_ASSET_SYMBOL"), 15*time.Second); ssa != nil {
				deps.SovereignSupplyReader = ssa
			} else {
				log.Printf("[app] sovereign token supply endpoint disabled (NATIVE_ASSET_SYMBOL or HUB_BESU_RPC_URL unset)")
			}
		}
	}

	// 007-bridge-based-cb-liquidity: Sovereign CB liquidity services.
	// Requires LIQUIDITY_COMMIT_REGISTRY_ADDRESS and INTERNAL_RELAY_AUTH_SECRET.
	deps.InternalRelayAuthSecret = os.Getenv("INTERNAL_RELAY_AUTH_SECRET")
	// R2-CR-6: per-CB asymmetric relay auth. Pin peer verifying keys from the PKI certs
	// (PKI_DIR/<entity>.crt). Internal relay routes prefer a valid signature and fall back
	// to the shared secret until RELAY_REQUIRE_SIGNATURE is set (post-cutover enforcement).
	if _, relayRegErr := relayauth.LoadRegistryGlob(cfg.PKIDir); relayRegErr != nil {
		log.Printf("[app] relay auth: could not load peer certs from PKI_DIR=%q: %v", cfg.PKIDir, relayRegErr)
	}
	deps.RelayAuth = relayAuthConfigFor(cfg)
	// The tenant scope of /internal/v1/payments listings comes from the participants table: it maps
	// the entity id this gateway verified to the address that bank's records are keyed by. Without a
	// database those listings fail closed rather than answering from every bank's rows.
	if db != nil {
		deps.RequesterScopeResolver = services.NewParticipantResolver(db)
	}

	// Fold in the peers this central bank onboarded, THEN report. Order matters: the participants
	// table is what makes a CB's registry non-empty at all (its PKI dir holds no peer certificates),
	// so refreshing after the report would describe a state that never existed.
	//
	// Two sources on purpose. The table is authoritative for onboarded peers because it carries the
	// ACTIVE status — deactivating a bank in compliance is what revokes its ability to authenticate.
	// Files cover peers that are never onboarded, the Cacti relay being the case that matters.
	refreshRelayRegistry(context.Background(), deps.RelayAuth.Registry, db, cfg.PKIDir)
	// Reload on demand when a request presents an unknown key-id, rate-limited. This is what makes a
	// bank verifiable the moment it finishes onboarding instead of at the next periodic sweep — the
	// sample deployment onboards a bank and immediately makes a deposit, which would otherwise 401.
	deps.RelayAuth.Registry.SetRefresher(func() {
		refreshRelayRegistry(context.Background(), deps.RelayAuth.Registry, db, cfg.PKIDir)
	}, relayRegistryMinRefreshInterval)
	if stop := startRelayRegistryRefresher(deps.RelayAuth.Registry, db, cfg.PKIDir); stop != nil {
		_ = stop // process-lifetime, like the other background workers wired here
	}

	relayRegistry := deps.RelayAuth.Registry.Get()
	if relayRegistry != nil && relayRegistry.Len() > 0 {
		ids := relayRegistry.IDs()
		log.Printf("[app] relay auth: %d peer key(s) pinned %v; require_signature=%v", relayRegistry.Len(), ids, cfg.RelayRequireSignature)
		// A registry holding ONLY this entity's own id is the dangerous middle state: non-empty, so
		// the middleware verifies strictly and stops falling back to the shared secret, but with no
		// peer pinned every SIGNED request is rejected with 401. Banks already sign their internal
		// calls, so on a CB this silently breaks bridge-in, the delegated hub swap and the residue
		// return. Not a refusal to start — the entity may legitimately receive no internal calls —
		// but it must not be discovered from the 401s.
		if len(ids) == 1 && ids[0] == cfg.RelayKeyID {
			log.Printf("[app] relay auth: WARNING: the only pinned key is this entity's own (%s) — no PEER identity was found, "+
				"neither an onboarded participant with an issued certificate nor a <key-id>.crt in PKI_DIR. "+
				"Any signed request from a peer will be rejected with 401 (RELAY_SIGNATURE_INVALID) instead of falling back to the shared secret.", cfg.RelayKeyID)
		}
	} else if cfg.RelayRequireSignature {
		// Do not claim the legacy fallback here: with enforcement on and nothing pinned, the shared
		// secret is NOT accepted — every internal request is rejected with 401.
		//
		// Reaching this line means the boot guard let the process through, which it does only when the
		// participant pins could not be read (a database that was not answering yet). So the state is
		// expected to repair itself on the next refresh, and saying "refusing to start" here — as this
		// line used to — described the one thing that did not happen.
		log.Printf("[app] relay auth: no peer keys pinned AND require_signature=true — every internal request " +
			"will be rejected with 401 until this CB's peers are pinned; the participants table could not be " +
			"read at boot, so the periodic refresh is what will fix this")
	} else {
		log.Printf("[app] relay auth: no peer keys pinned; internal routes use legacy shared secret")
	}
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
			if pairResolver != nil {
				sovereignAmm = &ammAdapter{resolver: pairResolver}
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
	deps.FiatSymbol = cfg.FiatSymbol
	deps.WTokenAddress = cfg.WTokenAddress
	deps.BankCode = cfg.BankCode
	deps.CommitSide = cfg.CommitSide
	// FR-013: approve-amm side auto-detection from BANK_CODE.
	deps.ApproveSide = cfg.ApproveSide
	// Fix: populate LOCAL_CB_HUB_SIGNER for balance checks (recipient of lock-mint tokens).
	deps.LocalCBHubSigner = strings.ToLower(os.Getenv("LOCAL_CB_HUB_SIGNER"))
	// This gateway's own Hub address: a CB reports it on a delegated swap so the bank knows
	// where the output landed. Empty on a bank that holds no Hub key.
	deps.HubSignerAddress = hubSignerAddr

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
		// R2-CR-6: verify the relay-claimed swap on the Hub before any burn/mint, and
		// consume each swap_tx_hash at most once. Without an AMM client the bridge-out
		// endpoint fails closed rather than minting on the relay's word.
		if pairResolver != nil {
			deps.CrossCurrencySwapVerifier = &swapVerifierAdapter{resolver: pairResolver}
		} else {
			log.Printf("[app] WARNING: pair resolver unavailable (PAIR_REGISTRY_CONTRACT_ADDRESS / HUB_BESU_RPC_URL) — cross-currency bridge-out will fail closed")
		}
		deps.CrossCurrencyDuplicateFinder = bridgeBurnUnlockSvc
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

		// Step 2 receiver: the Hub AMM admits only verified Hub participants, and only a CB
		// holds a Hub identity — so the CB executes the trade for the bank instead of handing
		// the bank its signing key. Requires a signing AMM client (pairResolver + signerKey)
		// and the replay guard; without either the endpoint is not registered and a delegating
		// bank sees a clean 404 rather than an unguarded swap.
		if pairResolver != nil && signerKey != "" && swapSvc != nil {
			deps.CrossCurrencyHubSwapExecutor = &swapServiceAdapter{svc: swapSvc}
			deps.CrossCurrencyHubSwapRecorder = newCrossCurrencyHubSwapRepository(db)
			deps.CrossCurrencyHubSwapDirection = &ammAddrResolverAdapter{r: pairResolver}
			if deps.CrossCurrencyBridgePositionReader == nil {
				deps.CrossCurrencyBridgePositionReader = services.NewBridgePositionReader(db)
			}
			if deps.CrossCurrencyBeneficiaryResolver == nil {
				deps.CrossCurrencyBeneficiaryResolver = services.NewParticipantResolver(db)
			}
			log.Printf("[app] sovereign hub swap delegation enabled (POST %s)", services.HubSwapPath)
		} else {
			log.Printf("[app] WARNING: hub swap delegation not registered (needs PAIR_REGISTRY_CONTRACT_ADDRESS + HUB_BESU_RPC_URL + SIGNER_PRIVATE_KEY) — delegating banks will fall back to signing on the Hub themselves")
		}

		// Hub reconciliation: this CB's own W-token balance against its own records. Wired on the
		// same condition as bridge-in — it reconciles the money this CB minted for its banks, and
		// only an issuing CB has that. Reports; never acts.
		if hubRPC != "" && deps.WTokenAddress != "" && hubSignerAddr != "" {
			// Process-lifetime, like the other Hub clients built here: buildV2Dependencies has no
			// closer list, and the checker needs the connection for as long as it runs.
			if reader := newHubBalanceReader(context.Background(), hubRPC, 15*time.Second); reader != nil {
				// BANK_CODE on a CB gateway is the CB's own entity id, so it identifies the
				// positions that are this CB's own money (liquidity it deployed) rather than an
				// obligation toward a bank. Empty simply disables that exclusion.
				reconciler := services.NewHubReconciliationService(
					reader, newHubReconciliationRepository(db, hubSignerAddr, cfg.BankCode),
					deps.WTokenAddress, hubSignerAddr)
				if reconciler != nil {
					deps.HubReconciler = reconciler
					startHubReconciliationChecker(reconciler)
					log.Printf("[app] hub reconciliation enabled for %s held at %s", deps.WTokenAddress, hubSignerAddr)
				}
			}
		} else {
			log.Printf("[app] hub reconciliation not enabled (needs HUB_BESU_RPC_URL, W_TOKEN_ADDRESS and a hub signer) — an unattributable Hub balance would go unnoticed")
		}

		// Step 4 receiver: the CB that bridged W-<source> in is also the only one that can
		// give the unspent slippage buffer back. Wired on the same condition as bridge-in,
		// since it is the mirror image of the same sovereign privilege.
		if bridgeBurnUnlockSvc != nil {
			deps.CrossCurrencyResidueEnqueuer = bridgeBurnUnlockSvc
			deps.CrossCurrencyResidueDuplicateFinder = bridgeBurnUnlockSvc
			deps.CrossCurrencyBridgePositionReader = services.NewBridgePositionReader(db)
			if deps.CrossCurrencyBeneficiaryResolver == nil {
				deps.CrossCurrencyBeneficiaryResolver = services.NewParticipantResolver(db)
			}
			log.Printf("[app] cross-currency residue return enabled (POST %s)", services.ResidueReturnPath)
		} else {
			log.Printf("[app] WARNING: no burn-unlock service — cross-currency residue return endpoint not registered; slippage buffers will strand on the Hub")
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

// swapVerifierAdapter adapts ammclient SwapByTxHash to the handler's VerifiedSwap type (R2-CR-6).
// In the dynamic per-pair model it resolves the pool's dedicated AMM from the
// PairRegistry and verifies the LogSwap against THAT AMM; it falls back to the
// client's configured AMM only when no pair/resolver is available.
type swapVerifierAdapter struct {
	resolver *pairAMMResolver
}

func (a *swapVerifierAdapter) VerifySwap(ctx context.Context, txHash, poolPair string) (*handlers.VerifiedSwap, error) {
	if a.resolver == nil {
		return nil, fmt.Errorf("swap verifier: no pair resolver configured")
	}
	// Resolve the pool's dedicated AMM (or the primary pool when no pool_pair is
	// carried) and verify the LogSwap against THAT AMM — there is no default AMM.
	c, err := a.resolver.ammFor(ctx, poolPair)
	if poolPair == "" {
		c, err = a.resolver.primaryClient(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("resolve AMM for pool %q: %w", poolPair, err)
	}
	vs, err := c.SwapByTxHash(ctx, txHash)
	if err != nil {
		return nil, err
	}
	return &handlers.VerifiedSwap{
		User:      vs.User,
		TokenIn:   vs.TokenIn,
		TokenOut:  vs.TokenOut,
		AmountIn:  vs.AmountIn,
		AmountOut: vs.AmountOut,
		Recipient: vs.Recipient,
	}, nil
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

func (a *bridgeBurnUnlockAdapter) EnqueueResidueReturn(ctx context.Context, ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount, correlationID string, burnFromHubAddress, beneficiarySpokeAddress, swapTxHash, parentPositionID string) (*services.BridgePositionResult, error) {
	return a.svc.EnqueueResidueReturn(ctx, ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount, correlationID,
		burnFromHubAddress, beneficiarySpokeAddress, swapTxHash, parentPositionID)
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

// OutputIsTokenA reports whether buying targetCurrency on pair outputs TOKEN_A, so the
// cross-currency quote is oriented to the requested direction (bidirectional pair).
func (a *ammQuoteAdapter) OutputIsTokenA(ctx context.Context, pair, targetCurrency string) (bool, error) {
	return a.adapter.resolver.OutputIsTokenA(ctx, pair, targetCurrency)
}

// ammAddrResolverAdapter exposes the per-pair resolver's on-chain AMM address
// lookup to the cross-currency swap orchestrator (services.AMMAddressResolver).
type ammAddrResolverAdapter struct {
	r *pairAMMResolver
}

func (a *ammAddrResolverAdapter) AMMAddressFor(ctx context.Context, poolPair string) (string, error) {
	return a.r.ammAddressFor(ctx, poolPair)
}

// OutputIsTokenA reports whether buying targetCurrency on poolPair outputs TOKEN_A,
// so the cross-currency swap can run in either direction over one sovereign pair.
func (a *ammAddrResolverAdapter) OutputIsTokenA(ctx context.Context, poolPair, targetCurrency string) (bool, error) {
	return a.r.OutputIsTokenA(ctx, poolPair, targetCurrency)
}
