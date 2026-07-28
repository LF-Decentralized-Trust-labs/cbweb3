// SPDX-License-Identifier: Apache-2.0

// Package handlers provides the cross-currency bridge-out handler for sovereign CB-B (009).
//
// POST /internal/amm/cross-currency-bridge-out
//
// Called by the Cacti CrossCurrencySwapRelay after CB-A's Hub AMM swap succeeds.
// CB-B receives the notification and enqueues its own BURN_UNLOCK operation:
//  1. burn W-ARS on the Hub  (payment-orchestrator CB-B)
//  2. release / mint tCeBM-ARS to the beneficiary bank on Spoke-B
//
// Security model (R2-CR-6): the relay message is only a trigger — every fact that
// authorizes the burn/mint is established independently of the request body:
//   - the swap is verified on the Hub via its receipt (LogSwap from the trusted AMM);
//   - the amount and the burn-from address come from the decoded on-chain events,
//     never from the JSON;
//   - the beneficiary's spoke address is resolved from CB-B's own participants registry;
//   - each swap_tx_hash is consumed at most once (idempotent replay returns the
//     existing position without enqueuing a second burn/mint).
//
// Protected by X-Relay-Auth middleware (same secret as execute-matched-commit).
package handlers

import (
	"context"
	"log"
	"math/big"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// sanitizeLogField strips CR/LF so attacker-controlled fields (swap_tx_hash,
// correlation_id) cannot inject forged log lines (R2-CR-6 review, Low).
func sanitizeLogField(s string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}

// CrossCurrencyBurnEnqueuerIface is the subset of BridgeBurnUnlockService used here.
type CrossCurrencyBurnEnqueuerIface interface {
	EnqueueBurnAfterSwap(
		ctx context.Context,
		ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount, correlationID string,
		extras ...string,
	) (*services.BridgePositionResult, error)
}

// BeneficiaryResolverIface resolves a bank_code to its on-chain wallet address.
type BeneficiaryResolverIface interface {
	ResolveWalletAddress(ctx context.Context, bankCode string) (string, error)
}

// VerifiedSwap carries the on-chain facts of an executed AMM swap, decoded from the
// transaction receipt on the Hub (mirrors ammclient.VerifiedSwap).
type VerifiedSwap struct {
	TokenOut  string // ERC-20 the pool paid out — must be this CB's W-token
	AmountIn  string // gross input amount per LogSwap
	AmountOut string // output amount per LogSwap
	Recipient string // address that actually received TokenOut — the only valid burn-from
}

// SwapVerifierIface verifies a swap transaction on the Hub and returns its on-chain facts.
// poolPair identifies the pair whose dedicated AMM must have emitted the LogSwap
// (dynamic per-pair model); the verifier resolves that AMM from the PairRegistry.
type SwapVerifierIface interface {
	VerifySwap(ctx context.Context, txHash, poolPair string) (*VerifiedSwap, error)
}

// BridgeOutDuplicateFinderIface looks up an existing bridge-out position by swap_tx_hash.
// Returns (nil, nil) when the swap has not been processed before.
type BridgeOutDuplicateFinderIface interface {
	FindBySwapTxHash(ctx context.Context, txHash string) (*services.BridgePositionResult, error)
}

// CrossCurrencyBridgeOutHandler handles POST /internal/amm/cross-currency-bridge-out.
type CrossCurrencyBridgeOutHandler struct {
	burnEnqueuer        CrossCurrencyBurnEnqueuerIface
	beneficiaryResolver BeneficiaryResolverIface
	swapVerifier        SwapVerifierIface
	duplicateFinder     BridgeOutDuplicateFinderIface
	wTokenAddress       string // W-ARS on Hub  (W_TOKEN_ADDRESS for CB-B)
	fiatTokenAddress    string // tCeBM-ARS on Spoke-B  (TOKEN_ADDRESS for CB-B)
	spokeNetwork        string // "spoke-b"
}

// NewCrossCurrencyBridgeOutHandler constructs the handler.
func NewCrossCurrencyBridgeOutHandler(
	burnEnqueuer CrossCurrencyBurnEnqueuerIface,
	beneficiaryResolver BeneficiaryResolverIface,
	wTokenAddress, fiatTokenAddress, spokeNetwork string,
) *CrossCurrencyBridgeOutHandler {
	return &CrossCurrencyBridgeOutHandler{
		burnEnqueuer:        burnEnqueuer,
		beneficiaryResolver: beneficiaryResolver,
		wTokenAddress:       wTokenAddress,
		fiatTokenAddress:    fiatTokenAddress,
		spokeNetwork:        spokeNetwork,
	}
}

// WithSwapVerification attaches the on-chain swap verifier and the idempotency lookup.
// The endpoint fails closed when no verifier is configured.
func (h *CrossCurrencyBridgeOutHandler) WithSwapVerification(
	verifier SwapVerifierIface,
	finder BridgeOutDuplicateFinderIface,
) *CrossCurrencyBridgeOutHandler {
	h.swapVerifier = verifier
	h.duplicateFinder = finder
	return h
}

// HandleBridgeOut processes the inbound Cacti relay notification.
//
// Expected JSON body:
//
//	{
//	  "correlation_id":       "<uuid>",
//	  "swap_tx_hash":         "0x...",
//	  "pool_pair":            "W-BRL-ARS",
//	  "amount_out":           "60",
//	  "beneficiary_bank_id":  "bank-b",
//	  "spoke_out":            "spoke-b",
//	  "wrapped_target_token": "0x82d50ad3..."
//	}
//
// amount_out is cross-checked against the on-chain LogSwap amount; swap_sender_address
// (legacy field) is ignored — the burn-from address is decoded from the swap receipt.
func (h *CrossCurrencyBridgeOutHandler) HandleBridgeOut(c *fiber.Ctx) error {
	var req struct {
		CorrelationID     string `json:"correlation_id"`
		SwapTxHash        string `json:"swap_tx_hash"`
		PoolPair          string `json:"pool_pair"`
		AmountOut         string `json:"amount_out"`
		BeneficiaryBankID string `json:"beneficiary_bank_id"`
		SpokeOut          string `json:"spoke_out"`
		// SwapSenderAddress is accepted for relay compatibility but never trusted:
		// the burn-from address is the verified on-chain recipient of the swap output.
		SwapSenderAddress string `json:"swap_sender_address"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.CorrelationID == "" || req.AmountOut == "" || req.BeneficiaryBankID == "" || req.SwapTxHash == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "correlation_id, swap_tx_hash, amount_out, beneficiary_bank_id are required",
		})
	}
	// Reject non-positive amounts up front: a production AMM would have reverted via
	// minAmountOut, but a misconfigured contract or test relay could otherwise mint a
	// phantom zero-amount bridge-out position (R2-CR-6 review, Medium).
	if amt, ok := new(big.Int).SetString(strings.TrimSpace(req.AmountOut), 10); !ok || amt.Sign() <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "amount_out must be a positive integer",
			"code":  "INVALID_AMOUNT",
		})
	}

	if h.wTokenAddress == "" {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "CB-B W-token address not configured (W_TOKEN_ADDRESS)",
			"code":  "CB_NOT_CONFIGURED",
		})
	}
	if h.swapVerifier == nil {
		// Fail closed: minting tCeBM on the relay's word alone is exactly the
		// vulnerability this endpoint must not have (R2-CR-6).
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "swap verification not configured on this gateway (HUB_BESU_RPC_URL / AMM_CONTRACT_ADDRESS)",
			"code":  "VERIFIER_NOT_CONFIGURED",
		})
	}

	// Replay protection: each Hub swap can be consumed at most once.
	if h.duplicateFinder != nil {
		existing, err := h.duplicateFinder.FindBySwapTxHash(c.Context(), req.SwapTxHash)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "idempotency lookup failed: " + err.Error(),
			})
		}
		if existing != nil {
			log.Printf("[correlation_id=%s] bridge-out replay detected: swap_tx_hash=%s already consumed by position %s",
				sanitizeLogField(req.CorrelationID), sanitizeLogField(req.SwapTxHash), existing.PositionID)
			return c.Status(fiber.StatusOK).JSON(fiber.Map{
				"status":         "duplicate",
				"position_id":    existing.PositionID,
				"bridge_state":   existing.BridgeState,
				"correlation_id": req.CorrelationID,
			})
		}
	}

	// Verify the swap on the Hub: receipt exists, succeeded, and contains a LogSwap
	// emitted by the trusted AMM contract.
	verified, err := h.swapVerifier.VerifySwap(c.Context(), req.SwapTxHash, req.PoolPair)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "swap not verified on Hub: " + err.Error(),
			"code":  "SWAP_NOT_VERIFIED",
		})
	}
	if !strings.EqualFold(verified.TokenOut, h.wTokenAddress) {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "swap output token " + verified.TokenOut + " is not this CB's W-token",
			"code":  "SWAP_TOKEN_MISMATCH",
		})
	}
	if verified.AmountOut != req.AmountOut {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "relay amount_out " + req.AmountOut + " does not match on-chain amount " + verified.AmountOut,
			"code":  "AMOUNT_MISMATCH",
		})
	}
	if verified.Recipient == "" {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "could not establish on-chain recipient of the swap output",
			"code":  "SWAP_NOT_VERIFIED",
		})
	}
	if req.SwapSenderAddress != "" && !strings.EqualFold(req.SwapSenderAddress, verified.Recipient) {
		log.Printf("[correlation_id=%s] bridge-out: relay-claimed sender %s differs from verified recipient %s — using on-chain value",
			sanitizeLogField(req.CorrelationID), sanitizeLogField(req.SwapSenderAddress), verified.Recipient)
	}

	// Resolve the beneficiary on-chain address from CB-B's own participants registry.
	// CB-B is the sovereign authority for its member banks and knows their wallet addresses.
	// The frontend / CB-A never need to send on-chain addresses.
	var beneficiaryAddr string
	if h.beneficiaryResolver != nil {
		var resolveErr error
		beneficiaryAddr, resolveErr = h.beneficiaryResolver.ResolveWalletAddress(c.Context(), req.BeneficiaryBankID)
		if resolveErr != nil {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error": "beneficiary bank not found or not active: " + resolveErr.Error(),
				"code":  "BENEFICIARY_NOT_FOUND",
			})
		}
	}

	result, err := h.burnEnqueuer.EnqueueBurnAfterSwap(
		c.Context(),
		req.BeneficiaryBankID,
		h.spokeNetwork,
		h.fiatTokenAddress, // tCeBM-ARS on Spoke-B (native asset)
		h.wTokenAddress,    // W-ARS on Hub (mirrored asset to burn)
		verified.AmountOut, // on-chain amount, not the relay's
		req.CorrelationID,
		verified.Recipient, // burnFromHubAddress — where the W-ARS verifiably sits
		beneficiaryAddr,    // beneficiarySpokeAddress — resolved from participants registry
		req.SwapTxHash,     // persisted for idempotency (unique per position)
	)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "bridge-out enqueue failed: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"status":         "accepted",
		"position_id":    result.PositionID,
		"correlation_id": req.CorrelationID,
		"amount_out":     verified.AmountOut,
		"beneficiary":    req.BeneficiaryBankID,
		"spoke":          h.spokeNetwork,
	})
}
