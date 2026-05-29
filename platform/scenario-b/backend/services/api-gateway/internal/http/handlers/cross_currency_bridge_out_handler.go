// Package handlers provides the cross-currency bridge-out handler for sovereign CB-B (009).
//
// POST /internal/amm/cross-currency-bridge-out
//
// Called by the Cacti CrossCurrencySwapRelay after CB-A's Hub AMM swap succeeds.
// CB-B receives the notification and enqueues its own BURN_UNLOCK operation:
//  1. burn W-ARS on the Hub  (payment-orchestrator CB-B)
//  2. release / mint tCeBM-ARS to the beneficiary bank on Spoke-B
//
// Protected by X-Relay-Auth middleware (same secret as execute-matched-commit).
package handlers

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

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

// CrossCurrencyBridgeOutHandler handles POST /internal/amm/cross-currency-bridge-out.
type CrossCurrencyBridgeOutHandler struct {
	burnEnqueuer        CrossCurrencyBurnEnqueuerIface
	beneficiaryResolver BeneficiaryResolverIface
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
func (h *CrossCurrencyBridgeOutHandler) HandleBridgeOut(c *fiber.Ctx) error {
	var req struct {
		CorrelationID     string `json:"correlation_id"`
		SwapTxHash        string `json:"swap_tx_hash"`
		PoolPair          string `json:"pool_pair"`
		AmountOut         string `json:"amount_out"`
		BeneficiaryBankID string `json:"beneficiary_bank_id"`
		SpokeOut          string `json:"spoke_out"`
		// SwapSenderAddress is the Hub address that received W-ARS from the AMM swap.
		// CB-B's executor burns from this address (CENTRAL_BANK_ROLE allows any-address burn).
		SwapSenderAddress string `json:"swap_sender_address"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.CorrelationID == "" || req.AmountOut == "" || req.BeneficiaryBankID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "correlation_id, amount_out, beneficiary_bank_id are required",
		})
	}

	if h.wTokenAddress == "" {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "CB-B W-token address not configured (W_TOKEN_ADDRESS)",
			"code":  "CB_NOT_CONFIGURED",
		})
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
		h.fiatTokenAddress,   // tCeBM-ARS on Spoke-B (native asset)
		h.wTokenAddress,      // W-ARS on Hub (mirrored asset to burn)
		req.AmountOut,
		req.CorrelationID,
		req.SwapSenderAddress, // burnFromHubAddress — where the W-ARS actually sits
		beneficiaryAddr,       // beneficiarySpokeAddress — resolved from participants registry
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
		"amount_out":     req.AmountOut,
		"beneficiary":    req.BeneficiaryBankID,
		"spoke":          h.spokeNetwork,
	})
}
