// SPDX-License-Identifier: Apache-2.0

// Package services provides CrossCurrencySwapOrchestrator for coordinating the 3-step
// cross-currency swap flow (bridge-in → swap Hub → bridge-out).
//
// Feature: 009-commercial-cross-currency-swap
// Spec: specs/009-commercial-cross-currency-swap/research.md (Q1: Simple Transaction Coordinator)
package services

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// CrossCurrencySwapRepository persists swap operation tracking.
type CrossCurrencySwapRepository interface {
	Create(ctx context.Context, op *domain.CrossCurrencySwapOperation) error
	GetByID(ctx context.Context, swapID string) (*domain.CrossCurrencySwapOperation, error)
	UpdateStatus(ctx context.Context, swapID string, status domain.SwapOperationStatus) error
	UpdateBridgeInPositionID(ctx context.Context, swapID string, positionID string) error
	// UpdateSwapResult atomically persists both the swap tx hash and the realized amount_in.
	// These two fields must be written together: amount_in now holds the real cost decoded
	// from LogSwap (not the MaxAmountIn cap), so a partial write would leave the record with a
	// tx hash but a stale/empty amount_in.
	UpdateSwapResult(ctx context.Context, swapID string, txHash string, amountIn string) error
	UpdateBridgeOutPositionID(ctx context.Context, swapID string, positionID string) error
	UpdateFailureReason(ctx context.Context, swapID string, reason string) error
}

// BridgeLockMintServiceIface handles bridge Spoke-A → Hub (lock native, mint wrapped).
// mintToHubAddress optionally overrides the Hub mint recipient (used by the CB self-service
// local path; commercial bridge-in is delegated via the bridge-in relay instead).
type BridgeLockMintServiceIface interface {
	LockAndEnqueue(ctx context.Context, ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount, correlationID string, mintToHubAddress ...string) (*BridgePositionResult, error)
}

// BridgeBurnUnlockServiceIface handles bridge Hub → Spoke-B (burn wrapped, unlock native).
type BridgeBurnUnlockServiceIface interface {
	BurnAndEnqueue(ctx context.Context, positionID, correlationID string) (*BridgePositionResult, error)
	EnqueueBurnAfterSwap(ctx context.Context, ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount, correlationID string, extras ...string) (*BridgePositionResult, error)
}

// CactiCrossRelayIface is an optional bridge-out relay that routes through Cacti so CB-B
// executes its own burn+release (sovereign model). When set, Step 3 uses it instead of
// calling EnqueueBurnAfterSwap locally (which would be CB-A trying to burn CB-B's tokens).
type CactiCrossRelayIface interface {
	NotifyBridgeOut(ctx context.Context, req CactiCrossCurrencyBridgeOutRequest) (string, error)
}

// BridgeInRelayIface is an optional bridge-in relay that delegates the Step 1 lock-mint to
// the issuing CB of the initiating bank's spoke (sovereign model). When set, Step 1 uses it
// instead of LockAndEnqueue locally (which would be a commercial bank trying to mint
// W-<source> on the Hub — only the issuing CB holds CENTRAL_BANK_ROLE). The CB performs the
// lock-mint and waits for ACTIVE, so no local poll is needed on this gateway.
type BridgeInRelayIface interface {
	NotifyBridgeIn(ctx context.Context, req CrossCurrencyBridgeInRequest) (string, error)
}

// SwapServiceIface executes swap on Hub AMM.
type SwapServiceIface interface {
	Execute(ctx context.Context, req SwapRequest) (*SwapResult, error)
}

// PoolStatusChecker validates pool is ACTIVE before swap.
type PoolStatusChecker interface {
	IsActive(ctx context.Context, poolPair string) (bool, error)
}

// CircuitBreakerChecker validates pool circuit breaker is not halted.
type CircuitBreakerChecker interface {
	IsHalted(ctx context.Context, poolPair string) (bool, error)
}

// BridgePositionPoller reads bridge position lifecycle state from persistence.
type BridgePositionPoller interface {
	GetBridgeState(ctx context.Context, positionID string) (domain.BridgeState, error)
}

// CrossCurrencySwapRequest carries the parameters for orchestrated swap.
type CrossCurrencySwapRequest struct {
	SwapID            string  // Generated UUID for tracking
	CorrelationID     string  // Generated UUID for linking 3 sub-operations
	SourceCurrency    string  // e.g., "BRL"
	TargetCurrency    string  // e.g., "ARS"
	PoolPair          string  // e.g., "W-BRL-ARS"
	AmountOut         string  // Desired amount in target currency (wei)
	MaxAmountIn       string  // Slippage protection (wei)
	PayerBankID       string  // Commercial bank initiating swap
	BeneficiaryBankID string  // Receiver bank
	QuoteID           *string // Optional quote ID for validation
}

// CrossCurrencySwapResult carries the outcome of orchestrated swap.
type CrossCurrencySwapResult struct {
	SwapID              string
	CorrelationID       string
	Status              domain.SwapOperationStatus
	AmountIn            string
	AmountOut           string
	EffectiveRate       float64
	BridgeInPositionID  string
	SwapTxHash          string
	BridgeOutPositionID string
	FailureReason       string
	CreatedAt           time.Time
	CompletedAt         *time.Time
}

// CrossCurrencySwapOrchestrator coordinates the 3-step swap flow with pre-validation and rollback.
type CrossCurrencySwapOrchestrator struct {
	swapRepo            CrossCurrencySwapRepository
	quoteRepo           SwapQuoteRepository
	bridgeLockMint      BridgeLockMintServiceIface
	bridgeBurnUnlock    BridgeBurnUnlockServiceIface
	swapService         SwapServiceIface
	poolStatusChecker   PoolStatusChecker
	circuitBreakerCheck CircuitBreakerChecker
	rollbackCoordinator *SwapRollbackCoordinator
	bridgeAssets        *CrossCurrencyBridgeAssets
	bridgePoller        BridgePositionPoller
	// cactiRelay is optional: when set, Step 3 delegates bridge-out to CB-B via Cacti
	// (sovereign model) instead of enqueuing locally on CB-A's relayer.
	cactiRelay CactiCrossRelayIface
	// bridgeInRelay is optional: when set, Step 1 delegates bridge-in lock-mint to the
	// issuing CB of this bank's spoke (sovereign model) instead of enqueuing locally.
	bridgeInRelay BridgeInRelayIface
	// hubSignerAddress is the Hub address used by this gateway's signer (SIGNER_PRIVATE_KEY).
	// After the AMM swap, W-ARS lands on this address; CB-B uses it as burnFrom.
	hubSignerAddress string
	// transferLimitChecker enforces configurable CB daily transfer limits (R1-10.1).
	transferLimitChecker TransferLimitCheckerIface
	// ammAddrResolver resolves a pool_pair to its on-chain AMM address (dynamic
	// per-pair model). Step 3 sends it to the Cacti relay so the relay can read
	// isPaused() on the correct AMM for its circuit-breaker gate.
	ammAddrResolver AMMAddressResolver
}

// AMMAddressResolver resolves a pool_pair (e.g. "W-BRL-W-ARS") to the on-chain
// address of its dedicated AMM, via the PairRegistry. Implemented in the app layer
// over the shared per-pair resolver.
type AMMAddressResolver interface {
	AMMAddressFor(ctx context.Context, poolPair string) (string, error)
	// OutputIsTokenA reports whether buying targetCurrency on poolPair outputs the
	// pair's TOKEN_A, so Step 2 can swap in either direction over one sovereign pair.
	OutputIsTokenA(ctx context.Context, poolPair, targetCurrency string) (bool, error)
}

// NewCrossCurrencySwapOrchestrator creates an orchestrator.
func NewCrossCurrencySwapOrchestrator(
	swapRepo CrossCurrencySwapRepository,
	quoteRepo SwapQuoteRepository,
	bridgeLockMint BridgeLockMintServiceIface,
	bridgeBurnUnlock BridgeBurnUnlockServiceIface,
	swapService SwapServiceIface,
	poolStatusChecker PoolStatusChecker,
	circuitBreakerCheck CircuitBreakerChecker,
	rollbackCoordinator *SwapRollbackCoordinator,
	bridgeAssets *CrossCurrencyBridgeAssets,
	bridgePoller BridgePositionPoller,
) *CrossCurrencySwapOrchestrator {
	return &CrossCurrencySwapOrchestrator{
		swapRepo:            swapRepo,
		quoteRepo:           quoteRepo,
		bridgeLockMint:      bridgeLockMint,
		bridgeBurnUnlock:    bridgeBurnUnlock,
		swapService:         swapService,
		poolStatusChecker:   poolStatusChecker,
		circuitBreakerCheck: circuitBreakerCheck,
		rollbackCoordinator: rollbackCoordinator,
		bridgeAssets:        bridgeAssets,
		bridgePoller:        bridgePoller,
	}
}

// WithCactiRelay attaches the Cacti relay for sovereign bridge-out (Step 3).
// When set, Step 3 delegates to CB-B via Cacti instead of running locally.
func (o *CrossCurrencySwapOrchestrator) WithCactiRelay(relay CactiCrossRelayIface) *CrossCurrencySwapOrchestrator {
	o.cactiRelay = relay
	return o
}

// WithBridgeInRelay attaches the bridge-in relay for sovereign lock-mint (Step 1).
// When set, Step 1 delegates the W-<source> mint to the issuing CB instead of enqueuing
// it on this (commercial) gateway's own relayer, which lacks CENTRAL_BANK_ROLE.
func (o *CrossCurrencySwapOrchestrator) WithBridgeInRelay(relay BridgeInRelayIface) *CrossCurrencySwapOrchestrator {
	o.bridgeInRelay = relay
	return o
}

// WithHubSignerAddress sets the Hub address of this gateway's signer.
// The AMM swap output (W-ARS) lands on this address; passed to CB-B so it knows
// where to burn from (CB-B has CENTRAL_BANK_ROLE = can burn from any address).
func (o *CrossCurrencySwapOrchestrator) WithHubSignerAddress(addr string) *CrossCurrencySwapOrchestrator {
	o.hubSignerAddress = addr
	return o
}

// WithAMMAddressResolver attaches the per-pair AMM address resolver so Step 3 can
// tell the Cacti relay which AMM to run its isPaused() circuit-breaker gate against.
func (o *CrossCurrencySwapOrchestrator) WithAMMAddressResolver(r AMMAddressResolver) *CrossCurrencySwapOrchestrator {
	o.ammAddrResolver = r
	return o
}

// WithTransferLimitChecker attaches the CB transfer limit enforcer (R1-10.1).
// When set, CheckAndDeduct is called before Step 1 (bridge-in) and Restore is called on failure.
func (o *CrossCurrencySwapOrchestrator) WithTransferLimitChecker(checker TransferLimitCheckerIface) *CrossCurrencySwapOrchestrator {
	o.transferLimitChecker = checker
	return o
}

// Execute orchestrates the full 3-step swap flow with pre-validation.
// Returns error on any step failure; rollback is automatic if swap fails after bridge-in.
func (o *CrossCurrencySwapOrchestrator) Execute(ctx context.Context, req CrossCurrencySwapRequest) (*CrossCurrencySwapResult, error) {
	log.Printf("[correlation_id=%s] starting cross-currency swap: %s → %s (pool=%s, amount_out=%s)",
		req.CorrelationID, req.SourceCurrency, req.TargetCurrency, req.PoolPair, req.AmountOut)

	// Create initial swap operation record
	swapOp := &domain.CrossCurrencySwapOperation{
		SwapID:            req.SwapID,
		CorrelationID:     req.CorrelationID,
		PayerBankID:       req.PayerBankID,
		BeneficiaryBankID: req.BeneficiaryBankID,
		SourceCurrency:    req.SourceCurrency,
		TargetCurrency:    req.TargetCurrency,
		PoolPair:          req.PoolPair,
		AmountOut:         req.AmountOut,
		MaxAmountIn:       req.MaxAmountIn,
		Status:            domain.SwapStatusQuoting,
		CreatedAt:         time.Now(),
	}
	if req.QuoteID != nil {
		swapOp.QuoteID = req.QuoteID
	}

	if err := o.swapRepo.Create(ctx, swapOp); err != nil {
		return nil, fmt.Errorf("failed to create swap operation: %w", err)
	}

	// Quote expiry validation (T034): If quote_id provided, verify it hasn't expired (15s TTL)
	if req.QuoteID != nil && *req.QuoteID != "" && o.quoteRepo != nil {
		quote, err := o.quoteRepo.FindByID(ctx, *req.QuoteID)
		if err != nil {
			_ = o.failSwap(ctx, req.SwapID, fmt.Sprintf("quote not found: %v", err))
			return nil, fmt.Errorf("quote %s not found: %w", *req.QuoteID, err)
		}

		// Check if quote has expired (NOW() > valid_until)
		if quote.IsExpired() {
			_ = o.failSwap(ctx, req.SwapID, fmt.Sprintf("quote expired (valid_until: %s)", quote.ValidUntil.Format(time.RFC3339)))
			return nil, fmt.Errorf("quote %s expired (valid until %s, now %s)",
				*req.QuoteID, quote.ValidUntil.Format(time.RFC3339), time.Now().Format(time.RFC3339))
		}

		log.Printf("[correlation_id=%s] quote %s validated (time remaining: %.0f seconds)",
			req.CorrelationID, *req.QuoteID, quote.TimeRemainingSeconds())
	}

	// Pre-condition 1: Pool must be ACTIVE
	if o.poolStatusChecker != nil {
		active, err := o.poolStatusChecker.IsActive(ctx, req.PoolPair)
		if err != nil {
			_ = o.failSwap(ctx, req.SwapID, fmt.Sprintf("pool status check failed: %v", err))
			return nil, fmt.Errorf("pool status check failed: %w", err)
		}
		if !active {
			_ = o.failSwap(ctx, req.SwapID, "pool is not ACTIVE")
			return nil, fmt.Errorf("pool %s is not ACTIVE", req.PoolPair)
		}
	}

	// Pre-condition 2: Circuit breaker must not be halted
	if o.circuitBreakerCheck != nil {
		halted, err := o.circuitBreakerCheck.IsHalted(ctx, req.PoolPair)
		if err != nil {
			_ = o.failSwap(ctx, req.SwapID, fmt.Sprintf("circuit breaker check failed: %v", err))
			return nil, fmt.Errorf("circuit breaker check failed: %w", err)
		}
		if halted {
			_ = o.failSwap(ctx, req.SwapID, "circuit breaker is HALTED")
			return nil, fmt.Errorf("pool %s circuit breaker is HALTED", req.PoolPair)
		}
	}

	// Pre-condition 3: Daily transfer limit check (R1-10.1).
	// MaxAmountIn is the worst-case amount the payer will spend; use it for limit accounting.
	if o.transferLimitChecker != nil {
		if err := o.transferLimitChecker.CheckAndDeduct(ctx, req.PayerBankID, req.SourceCurrency, req.MaxAmountIn); err != nil {
			_ = o.failSwap(ctx, req.SwapID, fmt.Sprintf("transfer limit check failed: %v", err))
			return nil, err
		}
		// Restore quota on any subsequent failure in Steps 1–3.
		defer func() {
			// Only restore if the swap ultimately failed (checked via DB status).
			if recovered := o.swapRepo; recovered != nil {
				op, fetchErr := recovered.GetByID(ctx, req.SwapID)
				if fetchErr == nil && op != nil && op.Status == domain.SwapStatusFailed {
					o.transferLimitChecker.Restore(ctx, req.PayerBankID, req.SourceCurrency, req.MaxAmountIn)
				}
			}
		}()
	}

	// Step 1: Bridge-In (Spoke-A → Hub)
	log.Printf("[correlation_id=%s] Step 1: Bridge-In (lock %s on Spoke-A, mint W-%s on Hub)",
		req.CorrelationID, req.SourceCurrency, req.SourceCurrency)
	_ = o.swapRepo.UpdateStatus(ctx, req.SwapID, domain.SwapStatusBridgeInProgress)

	// Source spoke is derived from the source currency (symmetric to spokeOut below):
	// "spoke-<currency>" is the convention the toolkit registers per spoke, so this
	// generalizes to any sovereign spoke (N currencies) with no BRL/ARS hardcode.
	spokeIn := "spoke-" + strings.ToLower(req.SourceCurrency)
	nativeAsset := req.SourceCurrency
	mirroredAsset := "W-" + req.SourceCurrency
	if o.bridgeAssets != nil {
		// Local (non-sovereign) dev path may pin the native/wrapped source token
		// addresses. The sovereign path ignores these (the issuing CB resolves its own
		// tokens from its per-CB config) and only needs spokeIn, derived above.
		if o.bridgeAssets.NativeSourceToken != "" {
			nativeAsset = o.bridgeAssets.NativeSourceToken
		}
		if o.bridgeAssets.WrappedSourceToken != "" {
			mirroredAsset = o.bridgeAssets.WrappedSourceToken
		}
	}

	var bridgeInPositionID string
	if o.bridgeInRelay != nil {
		// ── Sovereign bridge-in: delegate the W-<source> mint to the issuing CB ──
		// Only the issuing CB holds CENTRAL_BANK_ROLE on W-<source>. The CB performs the
		// lock-mint on its own relayer and blocks until the position is ACTIVE, so the
		// returned position is already settled — no local poll on this gateway.
		posID, relayErr := o.bridgeInRelay.NotifyBridgeIn(ctx, CrossCurrencyBridgeInRequest{
			CorrelationID:  req.CorrelationID,
			PayerBankID:    req.PayerBankID,
			SourceCurrency: req.SourceCurrency,
			// Bridge in the full cap: the worst-case amount_in must be reserved on the Hub
			// before the swap runs, since the realized cost is only known afterwards.
			// TODO(stranded-buffer): the residue (MaxAmountIn − realized amount_in) is left on
			// the Hub signer with no automatic bridge-back. Tracked as a separate R2 follow-up.
			Amount:  req.MaxAmountIn,
			SpokeIn: spokeIn,
			// Mint W-<source> to this gateway's swap signer so Step 2 can spend it (and
			// Step 3 burns from the same address). Mirrors bridge-out's SwapSenderAddress.
			SwapSenderAddress: o.hubSignerAddress,
		})
		if relayErr != nil {
			_ = o.failSwap(ctx, req.SwapID, fmt.Sprintf("bridge-in relay failed: %v", relayErr))
			return nil, fmt.Errorf("bridge-in relay failed: %w", relayErr)
		}
		bridgeInPositionID = posID
	} else {
		// ── Local bridge-in (CB self-service): enqueue lock-mint on this relayer ──
		// Valid only when this gateway's signer holds CENTRAL_BANK_ROLE on W-<source>
		// (i.e. this gateway is the issuing CB itself).
		bridgeInResult, err := o.bridgeLockMint.LockAndEnqueue(ctx,
			req.PayerBankID,
			spokeIn,
			nativeAsset,
			mirroredAsset,
			// Bridge in the full cap (worst-case amount_in reserved before the swap runs).
			// TODO(stranded-buffer): the residue (MaxAmountIn − realized amount_in) is left on
			// the Hub signer with no automatic bridge-back. Tracked as a separate R2 follow-up.
			req.MaxAmountIn,
			req.CorrelationID)
		if err != nil {
			_ = o.failSwap(ctx, req.SwapID, fmt.Sprintf("bridge-in failed: %v", err))
			return nil, fmt.Errorf("bridge-in failed: %w", err)
		}

		// Wait for bridge-in to become ACTIVE (polling with 120s timeout)
		bridgeInPositionID = bridgeInResult.PositionID
		if err := o.waitForBridgeActive(ctx, bridgeInPositionID, 120*time.Second); err != nil {
			_ = o.failSwap(ctx, req.SwapID, fmt.Sprintf("bridge-in timeout: %v", err))
			return nil, fmt.Errorf("bridge-in timeout: %w", err)
		}
	}

	_ = o.swapRepo.UpdateBridgeInPositionID(ctx, req.SwapID, bridgeInPositionID)
	log.Printf("[correlation_id=%s] bridge-in ACTIVE (position_id=%s)", req.CorrelationID, bridgeInPositionID)

	// Step 2: Swap Hub (W-BRL → W-ARS)
	log.Printf("[correlation_id=%s] Step 2: Swap Hub (W-%s → W-%s via AMM)",
		req.CorrelationID, req.SourceCurrency, req.TargetCurrency)
	_ = o.swapRepo.UpdateStatus(ctx, req.SwapID, domain.SwapStatusSwapInProgress)

	// Resolve the swap direction from the requested target currency vs the pair's
	// token orientation, so a single sovereign pair serves both directions (e.g. the
	// BRL↔COP pair handles both BRL→COP and COP→BRL). Defaults to A→B when unresolved.
	outputIsTokenA := false
	if o.ammAddrResolver != nil {
		if isA, dErr := o.ammAddrResolver.OutputIsTokenA(ctx, req.PoolPair, req.TargetCurrency); dErr == nil {
			outputIsTokenA = isA
		} else {
			log.Printf("[correlation_id=%s] WARNING: could not resolve swap direction for pool %s target %s: %v (defaulting A→B)",
				req.CorrelationID, req.PoolPair, req.TargetCurrency, dErr)
		}
	}

	swapReq := SwapRequest{
		Pair:           req.PoolPair,
		AmountOut:      req.AmountOut,
		MaxAmountIn:    req.MaxAmountIn,
		PayerID:        req.PayerBankID,
		BeneficiaryID:  req.BeneficiaryBankID,
		OutputIsTokenA: outputIsTokenA,
	}
	swapResult, err := o.swapService.Execute(ctx, swapReq)
	if err != nil {
		// Swap failed after bridge-in — trigger automatic rollback
		log.Printf("[correlation_id=%s] swap failed, triggering rollback: %v", req.CorrelationID, err)
		_ = o.failSwap(ctx, req.SwapID, fmt.Sprintf("swap failed: %v", err))

		if o.rollbackCoordinator != nil {
			if rollbackErr := o.rollbackCoordinator.ReverseBridge(ctx, req.SwapID, bridgeInPositionID); rollbackErr != nil {
				log.Printf("[correlation_id=%s] rollback failed: %v", req.CorrelationID, rollbackErr)
			}
		}
		return nil, fmt.Errorf("swap failed: %w", err)
	}

	_ = o.swapRepo.UpdateSwapResult(ctx, req.SwapID, swapResult.TxHash, swapResult.AmountIn)
	log.Printf("[correlation_id=%s] swap completed (tx_hash=%s, amount_in=%s)",
		req.CorrelationID, swapResult.TxHash, swapResult.AmountIn)

	// Slippage validation (T035): Verify actual amount_in <= max_amount_in
	// Parse amounts as big.Int for precise comparison
	actualAmountIn, ok1 := new(big.Int).SetString(swapResult.AmountIn, 10)
	maxAmountIn, ok2 := new(big.Int).SetString(req.MaxAmountIn, 10)
	if !ok1 || !ok2 {
		_ = o.failSwap(ctx, req.SwapID, "invalid amount format for slippage check")
		return nil, fmt.Errorf("invalid amount format: actual=%s, max=%s", swapResult.AmountIn, req.MaxAmountIn)
	}

	if actualAmountIn.Cmp(maxAmountIn) > 0 {
		// Slippage exceeded — trigger rollback
		slippageError := fmt.Sprintf("slippage limit exceeded: actual_amount_in=%s > max_amount_in=%s",
			swapResult.AmountIn, req.MaxAmountIn)
		log.Printf("[correlation_id=%s] %s, triggering rollback", req.CorrelationID, slippageError)
		_ = o.failSwap(ctx, req.SwapID, slippageError)

		if o.rollbackCoordinator != nil {
			if rollbackErr := o.rollbackCoordinator.ReverseBridge(ctx, req.SwapID, bridgeInPositionID); rollbackErr != nil {
				log.Printf("[correlation_id=%s] rollback failed: %v", req.CorrelationID, rollbackErr)
			}
		}
		return nil, fmt.Errorf("slippage limit exceeded: actual %s > max %s", swapResult.AmountIn, req.MaxAmountIn)
	}

	log.Printf("[correlation_id=%s] slippage check passed: %s <= %s",
		req.CorrelationID, swapResult.AmountIn, req.MaxAmountIn)

	// Step 3: Bridge-Out (Hub → Spoke-B)
	// Two paths depending on whether a sovereign Cacti relay is configured:
	//
	//   A) cactiRelay != nil  — sovereign model:
	//      CB-A notifies Cacti → Cacti forwards to CB-B → CB-B burns W-ARS + releases ARS.
	//      The bridge-out runs on CB-B's own payment-orchestrator with the correct signer.
	//      We do NOT wait for the spoke release here; the swap is considered settled once
	//      Cacti acknowledges (fire-and-forward). Bridge-out position tracking is CB-B's concern.
	//
	//   B) cactiRelay == nil  — legacy local mode (BRIDGE_SKIP_SPOKE_LOCK dev fallback):
	//      CB-A's relayer enqueues the burn directly. Only works if CB-A has the W-ARS
	//      contract address configured (bridgeAssets.WrappedTargetToken != "").
	log.Printf("[correlation_id=%s] Step 3: Bridge-Out (burn W-%s on Hub, unlock %s on Spoke-B)",
		req.CorrelationID, req.TargetCurrency, req.TargetCurrency)
	_ = o.swapRepo.UpdateStatus(ctx, req.SwapID, domain.SwapStatusBridgeOutProgress)

	// Dynamic per-pair model: the beneficiary spoke id follows the "spoke-<currency>"
	// convention (spoke-brl, spoke-ars, spoke-cop) the toolkit registers in the relay.
	spokeOut := "spoke-" + strings.ToLower(req.TargetCurrency)
	var bridgeOutPositionID string

	if o.cactiRelay != nil {
		// ── Path A: sovereign model via Cacti relay ──────────────────────────
		// Resolve the pair's on-chain AMM (dynamic per-pair model) so the relay can
		// run its isPaused() circuit-breaker gate against the correct AMM. Best-effort:
		// on failure the field is empty and the relay fails safe (refuses the burn).
		ammAddr := ""
		if o.ammAddrResolver != nil {
			if a, aErr := o.ammAddrResolver.AMMAddressFor(ctx, req.PoolPair); aErr == nil {
				ammAddr = a
			} else {
				log.Printf("[correlation_id=%s] WARNING: could not resolve AMM address for pool %s: %v", req.CorrelationID, req.PoolPair, aErr)
			}
		}
		relayReq := CactiCrossCurrencyBridgeOutRequest{
			CorrelationID:     req.CorrelationID,
			SwapTxHash:        swapResult.TxHash,
			PoolPair:          req.PoolPair,
			AmountOut:         req.AmountOut,
			BeneficiaryBankID: req.BeneficiaryBankID,
			SpokeOut:          spokeOut,
			AmmAddress:        ammAddr,
			// WrappedTargetToken is informational; CB-B uses its own configured address.
			WrappedTargetToken: func() string {
				if o.bridgeAssets != nil {
					return o.bridgeAssets.WrappedTargetToken
				}
				return ""
			}(),
			// SwapSenderAddress is where the W-ARS landed after the AMM swap.
			// CB-B burns from this address (CENTRAL_BANK_ROLE allows burn from any address).
			SwapSenderAddress: o.hubSignerAddress,
			// BeneficiarySpokeAddress is intentionally NOT sent — CB-B resolves
			// the beneficiary address internally from its participants registry.
		}
		correlationBack, relayErr := o.cactiRelay.NotifyBridgeOut(ctx, relayReq)
		if relayErr != nil {
			_ = o.failSwap(ctx, req.SwapID, fmt.Sprintf("cacti bridge-out relay failed (partial success): %v", relayErr))
			log.Printf("[correlation_id=%s] CRITICAL: swap succeeded but Cacti relay failed (manual intervention required)", req.CorrelationID)
			return nil, fmt.Errorf("cacti bridge-out relay failed (swap succeeded, manual intervention required): %w", relayErr)
		}
		bridgeOutPositionID = correlationBack // correlation_id echoed back by Cacti/CB-B
		log.Printf("[correlation_id=%s] Cacti relay accepted bridge-out → CB-B (echo=%s)", req.CorrelationID, bridgeOutPositionID)
	} else {
		// ── Path B: legacy local mode ────────────────────────────────────────
		nativeOut := req.TargetCurrency
		mirroredOut := "W-" + req.TargetCurrency
		if o.bridgeAssets != nil && o.bridgeAssets.WrappedTargetToken != "" {
			mirroredOut = o.bridgeAssets.WrappedTargetToken
		}
		bridgeOutResult, bridgeOutErr := o.bridgeBurnUnlock.EnqueueBurnAfterSwap(
			ctx,
			req.BeneficiaryBankID,
			spokeOut,
			nativeOut,
			mirroredOut,
			req.AmountOut,
			req.CorrelationID,
			// extras: no burn-from / beneficiary override (legacy executor fallbacks),
			// but bind the position to the swap tx so replay protection applies (R2-CR-6).
			"", "", swapResult.TxHash,
		)
		if bridgeOutErr != nil {
			_ = o.failSwap(ctx, req.SwapID, fmt.Sprintf("bridge-out failed (partial success): %v", bridgeOutErr))
			log.Printf("[correlation_id=%s] CRITICAL: swap succeeded but bridge-out failed (manual intervention required)", req.CorrelationID)
			return nil, fmt.Errorf("bridge-out failed (swap succeeded, manual intervention required): %w", bridgeOutErr)
		}
		bridgeOutPositionID = bridgeOutResult.PositionID
		_ = o.swapRepo.UpdateBridgeOutPositionID(ctx, req.SwapID, bridgeOutPositionID)

		// Wait for bridge-out to become UNLOCKED (polling with 120s timeout)
		if pollErr := o.waitForBridgeUnlocked(ctx, bridgeOutPositionID, 120*time.Second); pollErr != nil {
			_ = o.failSwap(ctx, req.SwapID, fmt.Sprintf("bridge-out timeout: %v", pollErr))
			return nil, fmt.Errorf("bridge-out timeout: %w", pollErr)
		}
	}

	_ = o.swapRepo.UpdateBridgeOutPositionID(ctx, req.SwapID, bridgeOutPositionID)
	log.Printf("[correlation_id=%s] bridge-out dispatched (position_or_correlation=%s)", req.CorrelationID, bridgeOutPositionID)

	// Mark swap as COMPLETED
	now := time.Now()
	_ = o.swapRepo.UpdateStatus(ctx, req.SwapID, domain.SwapStatusCompleted)
	log.Printf("[correlation_id=%s] cross-currency swap COMPLETED (total_duration=%v)",
		req.CorrelationID, now.Sub(swapOp.CreatedAt))

	// Calculate effective rate
	effectiveRate := 0.0
	if swapResult.AmountIn != "0" {
		// TODO: Proper big.Int arithmetic for rate calculation
		effectiveRate = 1.0 // Placeholder
	}

	return &CrossCurrencySwapResult{
		SwapID:              req.SwapID,
		CorrelationID:       req.CorrelationID,
		Status:              domain.SwapStatusCompleted,
		AmountIn:            swapResult.AmountIn,
		AmountOut:           req.AmountOut,
		EffectiveRate:       effectiveRate,
		BridgeInPositionID:  bridgeInPositionID,
		SwapTxHash:          swapResult.TxHash,
		BridgeOutPositionID: bridgeOutPositionID,
		CreatedAt:           swapOp.CreatedAt,
		CompletedAt:         &now,
	}, nil
}

// GetStatus retrieves the current state of a cross-currency swap operation by swap_id.
// Used by GET /api/v2/amm/swap/cross-currency/:id for async status polling.
func (o *CrossCurrencySwapOrchestrator) GetStatus(ctx context.Context, swapID string) (*CrossCurrencySwapResult, error) {
	op, err := o.swapRepo.GetByID(ctx, swapID)
	if err != nil {
		return nil, fmt.Errorf("swap %s not found: %w", swapID, err)
	}

	result := &CrossCurrencySwapResult{
		SwapID:        op.SwapID,
		CorrelationID: op.CorrelationID,
		Status:        op.Status,
		AmountIn:      op.AmountIn,
		AmountOut:     op.AmountOut,
		EffectiveRate: op.EffectiveRate,
		CreatedAt:     op.CreatedAt,
		CompletedAt:   op.CompletedAt,
	}
	if op.BridgeInPositionID != nil {
		result.BridgeInPositionID = *op.BridgeInPositionID
	}
	if op.SwapTxHash != nil {
		result.SwapTxHash = *op.SwapTxHash
	}
	if op.BridgeOutPositionID != nil {
		result.BridgeOutPositionID = *op.BridgeOutPositionID
	}
	if op.FailureReason != nil {
		result.FailureReason = *op.FailureReason
	}
	return result, nil
}

// failSwap marks the swap as FAILED with a reason.
func (o *CrossCurrencySwapOrchestrator) failSwap(ctx context.Context, swapID, reason string) error {
	return o.swapRepo.UpdateFailureReason(ctx, swapID, reason)
}

// waitForBridgeActive polls bridge position until state=ACTIVE or timeout.
func (o *CrossCurrencySwapOrchestrator) waitForBridgeActive(ctx context.Context, positionID string, timeout time.Duration) error {
	return o.pollBridgeState(ctx, positionID, domain.BridgeStateActive, timeout, 2*time.Second)
}

// waitForBridgeUnlocked polls bridge position until state=RELEASED or timeout.
func (o *CrossCurrencySwapOrchestrator) waitForBridgeUnlocked(ctx context.Context, positionID string, timeout time.Duration) error {
	return o.pollBridgeState(ctx, positionID, domain.BridgeStateReleased, timeout, 2*time.Second)
}

func (o *CrossCurrencySwapOrchestrator) pollBridgeState(
	ctx context.Context,
	positionID string,
	target domain.BridgeState,
	timeout time.Duration,
	interval time.Duration,
) error {
	if o.bridgePoller == nil {
		return fmt.Errorf("bridge position poller not configured")
	}
	deadline := time.Now().Add(timeout)
	for {
		state, err := o.bridgePoller.GetBridgeState(ctx, positionID)
		if err != nil {
			return err
		}
		if state == target {
			return nil
		}
		if state == domain.BridgeStateReconciliationRequired {
			return fmt.Errorf("bridge position %s requires reconciliation", positionID)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for %s on position %s (last state: %s)", target, positionID, state)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}
