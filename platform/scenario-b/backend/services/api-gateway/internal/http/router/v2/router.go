// SPDX-License-Identifier: Apache-2.0

// Package v2 registers API Gateway routes for Scenario B (API v2).
package v2

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// Dependencies groups all handlers and services required by the v2 route registration.
type Dependencies struct {
	AuthProvider      interfaces.IAuthProvider
	QuoteService      quoteServiceIface
	SwapService       swapServiceIface
	PoolStatusService poolStatusServiceIface
	// 009-commercial-cross-currency-swap — orchestrated cross-currency swap + quote generator
	CrossCurrencySwapOrchestrator crossCurrencySwapOrchestratorIface
	SwapQuoteGenerator            swapQuoteGeneratorIface
	// US2
	BridgeLockMintService   bridgeLockMintServiceIface
	BridgeBurnUnlockService bridgeBurnUnlockServiceIface
	BridgePositionReader    bridgePositionReaderIface
	LiquidityService        liquidityServiceIface
	TokenPreparer           handlers.AMMTokenPreparer
	// US3
	CircuitBreakerService handlers.CircuitBreakerServiceIface
	OversightService      handlers.OversightServiceIface
	// Phase 8 — PairRegistry multi-pair (005-cooperative-liquidity / FR-017)
	PairService services.PairServiceIface
	// 006-hub-currency-registry — hub currency discovery
	CurrencyService services.CurrencyServiceIface
	// 007-bridge-based-cb-liquidity — sovereign CB liquidity
	SovereignLiquidityService handlers.SovereignLiquidityServiceIface
	// LCRRegistrar calls registerCommit on the Hub LiquidityCommitRegistry (T010).
	LCRRegistrar handlers.OnChainCommitRegistrarIface
	// SovereignBridgeChecker gates CommitLiquidity on bridge_state for sovereign pairs (T008).
	SovereignBridgeChecker handlers.SovereignBridgeCheckerIface
	// CBChecker enables the anti-G5-cross guard in mint-and-approve (FR-004 / T016).
	// When nil, the guard is disabled and mint-and-approve behaves as before.
	CBChecker handlers.CentralBankChecker
	// TransferLimitHandler manages configurable CB daily transfer limits (R1-10.1).
	// When nil, transfer limit endpoints are not registered.
	TransferLimitHandler *handlers.TransferLimitHandler
	// TransferLimitInternalHandler serves CB-internal pre-auth endpoints for commercial banks (R1-10.1 Option A).
	// Registered only on CB gateways (CentralBankAPIURL == ""); nil on commercial banks.
	TransferLimitInternalHandler *handlers.TransferLimitInternalHandler
	// TransferLimitChecker enforces limits on bridge lock-mint and cross-currency swap (R1-10.1).
	// When nil, enforcement is skipped (no limits configured).
	TransferLimitChecker handlers.BridgeLimitCheckerIface
	// InternalRelayAuthSecret is the shared secret for X-Relay-Auth on internal routes.
	InternalRelayAuthSecret string
	// RelayAuth configures per-CB asymmetric signature verification on internal routes,
	// with the shared secret as a migration fallback (R2-CR-6).
	RelayAuth middleware.RelayAuthConfig
	// FiatTokenAddress is the tCeBM contract address on this CB's spoke (TOKEN_ADDRESS).
	// Used by the cross-currency bridge-out handler so CB-B can enqueue burn without trusting
	// the relay payload's token address.
	FiatTokenAddress string
	// CrossCurrencyBurnEnqueuer enables the POST /internal/amm/cross-currency-bridge-out route.
	// Set only on CB gateways that act as bridge-out receivers (e.g. CB-B in BRL→ARS flow).
	CrossCurrencyBurnEnqueuer handlers.CrossCurrencyBurnEnqueuerIface
	// CrossCurrencyBeneficiaryResolver resolves a bank_id to its on-chain wallet address.
	// CB-B uses this to determine where to mint tCeBM without the frontend knowing peer addresses.
	CrossCurrencyBeneficiaryResolver handlers.BeneficiaryResolverIface
	// CrossCurrencySwapVerifier verifies the relay-claimed swap on the Hub before any
	// burn/mint (R2-CR-6). The bridge-out endpoint fails closed when nil.
	CrossCurrencySwapVerifier handlers.SwapVerifierIface
	// CrossCurrencyDuplicateFinder is the swap_tx_hash idempotency lookup for bridge-out
	// replay protection (R2-CR-6).
	CrossCurrencyDuplicateFinder handlers.BridgeOutDuplicateFinderIface
	// CrossCurrencyLockMintEnqueuer enables the POST /internal/amm/cross-currency-bridge-in route.
	// Set only on CB gateways that act as bridge-in issuers (e.g. CB-A in BRL→ARS flow).
	CrossCurrencyLockMintEnqueuer handlers.CrossCurrencyLockMintEnqueuerIface
	// CrossCurrencyBridgeStateReader reads bridge position state for the synchronous
	// bridge-in ACTIVE wait. Set alongside CrossCurrencyLockMintEnqueuer on CB gateways.
	CrossCurrencyBridgeStateReader handlers.BridgeStateReaderIface
	// CrossCurrencyPayerBalanceChecker checks the payer bank's tCeBM balance before bridge-in.
	// Enforces Reserve Tokenisation: the CB MUST NOT mint new tCeBM if the bank hasn't tokenized.
	CrossCurrencyPayerBalanceChecker handlers.PayerBalanceCheckerIface
	// CrossCurrencyPayerWalletResolver resolves payer bank_code → spoke wallet for the balance check.
	CrossCurrencyPayerWalletResolver handlers.PayerWalletResolverIface
	// LPPositionRepo enables GET /api/v2/amm/liquidity/positions (008-fix-cb-liquidity).
	LPPositionRepo handlers.LPPositionReaderIface
	// LPBalanceReader enables GET /api/v2/amm/lp-balance — the CB's live on-chain CBW3-LP position (013).
	LPBalanceReader handlers.LPBalanceReaderIface
	// Simplified API config (008-fix-cb-liquidity)
	SpokeNetwork      string // spoke-a, spoke-b (for bridge lock-mint derivation)
	NativeAssetSymbol string // tCeBM_BRL, tCeBM_ARS (for bridge lock-mint derivation)
	FiatSymbol        string // human-readable currency symbol for transfer-limit matching (e.g. "BRL", "ARS")
	WTokenAddress     string // Hub W-tCeBM token address (for bridge + commit derivation)
	BankCode          string // BANK_CODE fallback when JWT claims do not include BankID (bridge lock-mint)
	CommitSide        string // A or B (derived from BANK_CODE for commit derivation)
	ApproveSide       string // A or B (derived from BANK_CODE for approve-amm auto-detect; FR-013)
	LocalCBHubSigner  string // LOCAL_CB_HUB_SIGNER — address that receives lock-mint tokens (for balance checks)
}

type quoteServiceIface interface {
	GetExactOutputQuote(ctx context.Context, pair, amountOut string) (*services.QuoteResponse, error)
}

type swapServiceIface interface {
	Execute(ctx context.Context, req services.SwapRequest) (*services.SwapResult, error)
}

type poolStatusServiceIface interface {
	GetPoolStatus(ctx context.Context, pair string) (*services.PoolStatusResponse, error)
}

type crossCurrencySwapOrchestratorIface interface {
	Execute(ctx context.Context, req services.CrossCurrencySwapRequest) (*services.CrossCurrencySwapResult, error)
	GetStatus(ctx context.Context, swapID string) (*services.CrossCurrencySwapResult, error)
}

type swapQuoteGeneratorIface interface {
	GenerateQuote(ctx context.Context, req services.QuoteRequest) (*services.QuoteResult, error)
}

type bridgeLockMintServiceIface interface {
	LockAndEnqueue(ctx context.Context, ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount string) (*services.BridgePositionResult, error)
}

type bridgeBurnUnlockServiceIface interface {
	BurnAndEnqueue(ctx context.Context, positionID string) (*services.BridgePositionResult, error)
}

type bridgePositionReaderIface interface {
	ListPositions(ctx context.Context, stateFilter string) ([]services.BridgePositionResult, error)
}

type liquidityServiceIface interface {
	AddLiquidity(ctx context.Context, req services.LiquidityProvisionRequest) (*services.LPResult, error)
	RemoveLiquidity(ctx context.Context, req services.LiquidityRemoveRequest) (*services.LPResult, error)
	RegisterCommit(ctx context.Context, req services.CommitRequest) (*services.CommitResult, error)
	ListCommits(ctx context.Context, poolPair, status string) ([]domain.PoolCommit, error)
	GetCommit(ctx context.Context, commitID string) (*domain.PoolCommit, error)
	CancelCommit(ctx context.Context, commitID, providerID string) error
}

// Register attaches all Scenario B v2 routes to the Fiber app.
func Register(app *fiber.App, deps Dependencies) {
	registerUS1Routes(app, deps)
	registerUS2Routes(app, deps)
	registerUS3Routes(app, deps)
	registerPairRegistryRoutes(app, deps)
	registerCurrencyRegistryRoutes(app, deps)
	registerSovereignRoutes(app, deps)
	registerTransferLimitInternalRoutes(app, deps)
}

// registerTransferLimitInternalRoutes registers the CB-internal pre-auth endpoints (R1-10.1 Option A).
// Only registered on CB gateways (TransferLimitInternalHandler != nil); no-op on commercial banks.
func registerTransferLimitInternalRoutes(app *fiber.App, deps Dependencies) {
	if deps.TransferLimitInternalHandler == nil {
		return
	}
	internal := app.Group("/internal/v2/transfer-limits",
		middleware.RequireRelayAuth(deps.InternalRelayAuthSecret),
	)
	internal.Post("/check-and-deduct", deps.TransferLimitInternalHandler.HandleCheckAndDeduct)
	internal.Post("/restore", deps.TransferLimitInternalHandler.HandleRestore)
}

// registerUS1Routes registers AMM quote, swap, and pool status routes (T045 / FR-027 / FR-028).
func registerUS1Routes(app *fiber.App, deps Dependencies) {
	if deps.QuoteService == nil && deps.SwapService == nil && deps.PoolStatusService == nil && deps.CrossCurrencySwapOrchestrator == nil {
		return
	}

	amm := app.Group("/api/v2/amm")

	if deps.QuoteService != nil {
		quoteHandler := handlers.NewQuoteHandler(deps.QuoteService, deps.SwapQuoteGenerator)
		amm.Get("/quote/exact-output", quoteHandler.GetExactOutputQuote)
		// 009-commercial-cross-currency-swap: quote with 15s TTL (T032)
		if deps.SwapQuoteGenerator != nil {
			amm.Get("/quote/cross-currency", quoteHandler.GetCrossCurrencyQuote)
		}
	}

	if deps.SwapService != nil && deps.AuthProvider != nil {
		swapHandler := handlers.NewSwapHandler(deps.SwapService)
		amm.Post("/swap/exact-output",
			middleware.RequireCookieAuth(deps.AuthProvider),
			middleware.RequireCommercialBankRole(),
			swapHandler.SwapExactOutput,
		)
	}

	// 009-commercial-cross-currency-swap: orchestrated cross-currency swap for commercial banks (T025 / FR-001).
	// POST initiates the swap; GET :id polls status for async flows.
	if deps.CrossCurrencySwapOrchestrator != nil && deps.AuthProvider != nil {
		crossCurrencySwapHandler := handlers.NewCrossCurrencySwapHandler(deps.CrossCurrencySwapOrchestrator, deps.BankCode)
		amm.Post("/swap/cross-currency",
			middleware.RequireCookieAuth(deps.AuthProvider),
			middleware.RequireCommercialBankRole(),
			crossCurrencySwapHandler.SwapCrossCurrency,
		)
		amm.Get("/swap/cross-currency/:id",
			middleware.RequireCookieAuth(deps.AuthProvider),
			middleware.RequireCommercialBankRole(),
			crossCurrencySwapHandler.GetSwapStatus,
		)
	}

	if deps.PoolStatusService != nil {
		poolHandler := handlers.NewPoolHandler(deps.PoolStatusService)
		amm.Get("/pool/:pair/status", poolHandler.GetPoolStatus)
	}
	// T054: CB gateways expose sovereign Hub AMM config for commercial bank swap clients.
	amm.Get("/hub-liquidity-config", handlers.GetHubLiquidityConfig)
}

// registerUS2Routes registers bridge and liquidity provision routes (T070 / FR-029).
func registerUS2Routes(app *fiber.App, deps Dependencies) {
	bridge := app.Group("/api/v2/bridge")
	if deps.BridgeLockMintService != nil && deps.BridgeBurnUnlockService != nil && deps.BridgePositionReader != nil {
		// 008-fix-cb-liquidity: use simplified handler when config is available
		var bh *handlers.BridgeHandler
		if deps.SpokeNetwork != "" && deps.NativeAssetSymbol != "" {
			bh = handlers.NewBridgeHandlerSimplified(
				deps.BridgeLockMintService,
				deps.BridgeBurnUnlockService,
				deps.BridgePositionReader,
				deps.SpokeNetwork,
				deps.NativeAssetSymbol,
				deps.WTokenAddress,
				deps.BankCode,
			)
		} else {
			// Fallback to legacy handler (requires full payload)
			bh = handlers.NewBridgeHandler(
				deps.BridgeLockMintService,
				deps.BridgeBurnUnlockService,
				deps.BridgePositionReader,
			).SetFallbackBankCode(deps.BankCode)
		}
		// R1-10.1: attach transfer limit checker and fiat symbol to bridge handler when configured.
		if deps.TransferLimitChecker != nil {
			bh = bh.WithLimitChecker(deps.TransferLimitChecker)
		}
		if deps.FiatSymbol != "" {
			bh = bh.WithFiatSymbol(deps.FiatSymbol)
		}
		if deps.AuthProvider != nil {
			// spec-007 FR-001: CBs use the same lock-mint/burn-unlock/positions endpoints as
			// commercial banks. RequireAnyAuth accepts cookie (browser) or Bearer header (M2M/CB
			// sovereign flows). RequireRole permits both commercial_bank and central_bank callers.
			bridge.Post("/lock-mint",
				middleware.RequireAnyAuth(deps.AuthProvider),
				middleware.RequireRole(domain.RoleCommercialBankScenarioB, domain.RoleCentralBankScenarioB),
				bh.LockMint,
			)
			bridge.Post("/burn-unlock",
				middleware.RequireAnyAuth(deps.AuthProvider),
				middleware.RequireRole(domain.RoleCommercialBankScenarioB, domain.RoleCentralBankScenarioB),
				bh.BurnUnlock,
			)
			bridge.Get("/positions",
				middleware.RequireAnyAuth(deps.AuthProvider),
				middleware.RequireRole(domain.RoleCommercialBankScenarioB, domain.RoleCentralBankScenarioB),
				bh.ListPositions,
			)
		}
	}

	amm := app.Group("/api/v2/amm")
	if deps.LiquidityService != nil && deps.AuthProvider != nil {
		// 008-fix-cb-liquidity: use sovereign handler (with LCR on-chain registration) when
		// deps.LCRRegistrar is wired (LIQUIDITY_COMMIT_REGISTRY_ADDRESS is set).
		// Falls back to cooperative handler when sovereign deps are absent.
		var lh *handlers.LiquidityHandler
		if deps.LCRRegistrar != nil {
			lh = handlers.NewLiquidityHandlerSovereign(
				deps.LiquidityService,
				deps.SovereignBridgeChecker,
				deps.LCRRegistrar,
				deps.SovereignLiquidityService,
				deps.LPPositionRepo,   // NEW: for GET /liquidity/positions
				deps.CommitSide,       // NEW: derived from BANK_CODE
				deps.WTokenAddress,    // NEW: from config
				deps.LocalCBHubSigner, // NEW: LOCAL_CB_HUB_SIGNER for balance checks
			)
		} else {
			lh = handlers.NewLiquidityHandler(deps.LiquidityService)
		}
		// Legacy dual-sided liquidity provisioning (used by tryouts and frontend governance).
		// For sovereign CB flow, use commit-reveal + bridge-based approach instead.
		amm.Post("/liquidity/add",
			middleware.RequireCookieAuth(deps.AuthProvider),
			middleware.RequireLiquidityProviderRole(),
			lh.AddLiquidity,
		)
		// D7/013: withdrawal is a sovereign CB operation — RequireAnyAuth so CB M2M
		// (bearer) clients can withdraw, mirroring /liquidity/commit.
		amm.Post("/liquidity/remove",
			middleware.RequireAnyAuth(deps.AuthProvider),
			middleware.RequireLiquidityProviderRole(),
			lh.RemoveLiquidity,
		)
		// Commit-reveal routes (005-cooperative-liquidity / T013 / T014 / T015).
		// spec-007: CBs call commit via Bearer token (M2M); RequireAnyAuth accepts both.
		amm.Post("/liquidity/commit",
			middleware.RequireAnyAuth(deps.AuthProvider),
			middleware.RequireLiquidityProviderRole(),
			lh.CommitLiquidity,
		)
		amm.Get("/liquidity/commits",
			middleware.RequireAnyAuth(deps.AuthProvider),
			middleware.RequireLiquidityProviderRole(),
			lh.ListCommits,
		)
		// Single-commit status — front-end polls this for async commit progress.
		amm.Get("/liquidity/commits/:commit_id",
			middleware.RequireAnyAuth(deps.AuthProvider),
			middleware.RequireLiquidityProviderRole(),
			lh.GetCommit,
		)
		amm.Delete("/liquidity/commits/:commit_id",
			middleware.RequireAnyAuth(deps.AuthProvider),
			middleware.RequireLiquidityProviderRole(),
			lh.CancelCommit,
		)
		// LP positions listing (008-fix-cb-liquidity / FR-028).
		// Public endpoint — no auth required (each CB sees only its own positions in its DB).
		amm.Get("/liquidity/positions", lh.ListPositions)
		// On-chain LP-share position (013-amm-lp-shares). Public like /liquidity/positions —
		// each CB gateway reports only its own signer's balance.
		if deps.LPBalanceReader != nil {
			lh.WithLPBalanceReader(deps.LPBalanceReader)
			amm.Get("/lp-balance", lh.GetLPBalance)
		}
	}

	if deps.TokenPreparer != nil && deps.AuthProvider != nil {
		// FR-013: always use NewTokenHandlerWithConfig to wire approveSide (derived from BANK_CODE).
		// approveSide="" when BANK_CODE is not set (CB G5-cross explicit side still works).
		th := handlers.NewTokenHandlerWithConfig(deps.TokenPreparer, deps.CBChecker, deps.ApproveSide)
		// spec-007 SC-002: anti-G5-cross guard is enforced in MintAndApprove handler.
		// RequireAnyAuth allows CB scripts (bearer) to hit the endpoint so the guard can fire HTTP 403.
		amm.Post("/token/mint-and-approve",
			middleware.RequireAnyAuth(deps.AuthProvider),
			middleware.RequireCentralBankRole(),
			th.MintAndApprove,
		)
		amm.Post("/token/approve-amm",
			middleware.RequireAnyAuth(deps.AuthProvider),
			middleware.RequireRole(domain.RoleCentralBankScenarioB, domain.RoleCommercialBankScenarioB),
			th.ApproveAMM,
		)
	}
}

// registerUS3Routes registers governance circuit-breaker and oversight routes (T091 / FR-030 / FR-034).
func registerUS3Routes(app *fiber.App, deps Dependencies) {
	gov := app.Group("/api/v2/governance")

	if deps.CircuitBreakerService != nil {
		gh := handlers.NewGovernanceScenarioBHandler(deps.CircuitBreakerService)
		gov.Post("/circuit-breaker/pause",
			middleware.RequireCookieAuth(deps.AuthProvider),
			middleware.RequireCentralBankRole(),
			gh.PauseCircuitBreaker,
		)
		gov.Post("/circuit-breaker/resume-request",
			middleware.RequireCookieAuth(deps.AuthProvider),
			middleware.RequireCentralBankRole(),
			gh.ProposeResume,
		)
		gov.Post("/circuit-breaker/resume-sign",
			middleware.RequireCookieAuth(deps.AuthProvider),
			middleware.RequireCentralBankRole(),
			gh.SignResume,
		)
		gov.Get("/circuit-breaker/status", gh.GetCircuitBreakerStatus)
	}

	if deps.TransferLimitHandler != nil {
		gov.Post("/transfer-limits",
			middleware.RequireCookieAuth(deps.AuthProvider),
			middleware.RequireCentralBankRole(),
			deps.TransferLimitHandler.CreateTransferLimit,
		)
		gov.Get("/transfer-limits",
			middleware.RequireCookieAuth(deps.AuthProvider),
			middleware.RequireCentralBankRole(),
			deps.TransferLimitHandler.ListTransferLimits,
		)
		gov.Delete("/transfer-limits/:id",
			middleware.RequireCookieAuth(deps.AuthProvider),
			middleware.RequireCentralBankRole(),
			deps.TransferLimitHandler.DeleteTransferLimit,
		)
	}

	oversight := app.Group("/api/v2/oversight")
	if deps.OversightService != nil {
		oh := handlers.NewOversightHandler(deps.OversightService)
		oversight.Post("/disclosure-request",
			middleware.RequireCookieAuth(deps.AuthProvider),
			middleware.RequireCentralBankRole(),
			oh.OpenDisclosure,
		)
		oversight.Post("/disclosure-sign",
			middleware.RequireCookieAuth(deps.AuthProvider),
			middleware.RequireCentralBankRole(),
			oh.SignDisclosure,
		)
		oversight.Get("/disclosure-status/:requestID", oh.GetDisclosureStatus)
	}
}

// registerCurrencyRegistryRoutes registers hub currency discovery endpoints (FR-003/FR-004/FR-005 — 006-hub-currency-registry).
// POST   /api/v2/hub/currencies         — Central Bank registers its currency (requires CB role + auth).
// DELETE /api/v2/hub/currencies/:symbol — Central Bank removes its currency (requires CB role + auth).
// GET    /api/v2/hub/currencies         — List all registered currencies (public).
func registerCurrencyRegistryRoutes(app *fiber.App, deps Dependencies) {
	if deps.CurrencyService == nil {
		return
	}
	ch := handlers.NewCurrencyHandler(deps.CurrencyService)
	hub := app.Group("/api/v2/hub")
	hub.Post("/currencies",
		middleware.RequireCookieAuth(deps.AuthProvider),
		middleware.RequireCentralBankRole(),
		ch.RegisterCurrency,
	)
	hub.Delete("/currencies/:symbol",
		middleware.RequireCookieAuth(deps.AuthProvider),
		middleware.RequireCentralBankRole(),
		ch.RemoveCurrency,
	)
	hub.Get("/currencies", ch.ListCurrencies)
}

// POST /api/v2/amm/pairs/propose — Central Bank of tokenA proposes a new pair (requires CB role + auth).
// POST /api/v2/amm/pairs/confirm — Central Bank of tokenB confirms a proposed pair (requires CB role + auth).
// GET  /api/v2/amm/pairs         — List all active pairs (public, served from DB).
func registerPairRegistryRoutes(app *fiber.App, deps Dependencies) {
	if deps.PairService == nil {
		return
	}
	ph := handlers.NewPairHandler(deps.PairService)
	amm := app.Group("/api/v2/amm")
	amm.Post("/pairs/propose",
		middleware.RequireCookieAuth(deps.AuthProvider),
		middleware.RequireCentralBankRole(),
		ph.ProposePair,
	)
	amm.Post("/pairs/confirm",
		middleware.RequireCookieAuth(deps.AuthProvider),
		middleware.RequireCentralBankRole(),
		ph.ConfirmPair,
	)
	amm.Get("/pairs", ph.ListPairs)
}

// registerSovereignRoutes registers internal sovereign CB liquidity routes
// (007-bridge-based-cb-liquidity / FR-003 / T011 / T021).
//
// POST /internal/amm/execute-matched-commit
//
//	Called by the Cacti LiquidityCommitWatcher when CommitMatched fires.
//	Protected by X-Relay-Auth secret (RequireRelayAuth middleware).
//
// POST /api/v2/amm/liquidity/sovereign-add
//
//	Direct sovereign deposit (top-up for active pool, no bridge gate, no commit-reveal).
//	Protected by CB role + cookie auth.
func registerSovereignRoutes(app *fiber.App, deps Dependencies) {
	if deps.SovereignLiquidityService == nil {
		return
	}
	lh := handlers.NewLiquidityHandlerSovereign(
		deps.LiquidityService,
		deps.SovereignBridgeChecker,
		deps.LCRRegistrar,
		deps.SovereignLiquidityService,
		deps.LPPositionRepo,   // NEW: for GET /liquidity/positions
		deps.CommitSide,       // NEW: derived from BANK_CODE
		deps.WTokenAddress,    // NEW: from config
		deps.LocalCBHubSigner, // NEW: LOCAL_CB_HUB_SIGNER for balance checks
	)
	app.Post("/internal/amm/execute-matched-commit",
		middleware.RequireRelayAuthMigrating(deps.RelayAuth),
		lh.ExecuteMatchedCommit,
	)

	// 009-commercial-cross-currency-swap: bridge-out receiver for CB-B.
	// Called by Cacti CrossCurrencySwapRelay after CB-A's Hub AMM swap succeeds.
	if deps.CrossCurrencyBurnEnqueuer != nil {
		ccboh := handlers.NewCrossCurrencyBridgeOutHandler(
			deps.CrossCurrencyBurnEnqueuer,
			deps.CrossCurrencyBeneficiaryResolver,
			deps.WTokenAddress,
			deps.FiatTokenAddress,
			deps.SpokeNetwork,
		).WithSwapVerification(deps.CrossCurrencySwapVerifier, deps.CrossCurrencyDuplicateFinder)
		app.Post("/internal/amm/cross-currency-bridge-out",
			middleware.RequireRelayAuthMigrating(deps.RelayAuth),
			ccboh.HandleBridgeOut,
		)
	}

	// 009-commercial-cross-currency-swap: bridge-in receiver for the issuing CB (CB-A side).
	// Called by a commercial bank's orchestrator (Step 1) to perform the sovereign W-<source>
	// lock-mint that the commercial bank may not do itself.
	if deps.CrossCurrencyLockMintEnqueuer != nil && deps.CrossCurrencyBridgeStateReader != nil {
		ccbih := handlers.NewCrossCurrencyBridgeInHandler(
			deps.CrossCurrencyLockMintEnqueuer,
			deps.CrossCurrencyBridgeStateReader,
			deps.WTokenAddress,
			deps.FiatTokenAddress,
			deps.SpokeNetwork,
		)
		if deps.CrossCurrencyPayerBalanceChecker != nil && deps.CrossCurrencyPayerWalletResolver != nil {
			ccbih = ccbih.WithReserveTokenisationEnforcement(
				deps.CrossCurrencyPayerBalanceChecker,
				deps.CrossCurrencyPayerWalletResolver,
			)
		}
		app.Post("/internal/amm/cross-currency-bridge-in",
			middleware.RequireRelayAuthMigrating(deps.RelayAuth),
			ccbih.HandleBridgeIn,
		)
	}

	if deps.AuthProvider != nil {
		amm := app.Group("/api/v2/amm")
		// spec-007: sovereign-add is called by CB M2M scripts via Bearer token.
		amm.Post("/liquidity/sovereign-add",
			middleware.RequireAnyAuth(deps.AuthProvider),
			middleware.RequireCentralBankRole(),
			lh.SovereignAddLiquidity,
		)
	}
}
