// SPDX-License-Identifier: Apache-2.0

// Package handlers provides the cross-currency residue-return handler for the issuing CB.
//
// POST /internal/amm/cross-currency-residue-return
//
// Step 1 of a cross-currency swap must bridge MaxAmountIn — the desired amount plus the
// slippage buffer — because the true cost is unknown until the AMM swap runs. Step 2 consumes
// only the realized amount_in. This endpoint returns the difference to the payer: burn the
// unspent W-<source> on the Hub, deliver tCeBM-<source> on the source spoke. Both require
// CENTRAL_BANK_ROLE on that currency, which only the issuing CB holds — so the initiating
// gateway delegates here, exactly as it delegates the bridge-in.
//
// Security model (inherited from R2-CR-6): the request is only a trigger. The amount is never
// taken from the body — the body carries no amount at all. It is derived from two sources the
// caller cannot forge:
//   - the amount bridged in, read from the bridge-in position *this CB itself created*;
//   - the realized amount_in, decoded from the LogSwap of the trusted AMM on the Hub.
//
// residue = position.mirrored_amount − LogSwap.amountIn
//
// Further gates: the swap's input token must be this CB's W-token; the swap sender must match
// the address this CB minted to; the payer's spoke wallet is resolved from this CB's own
// participants registry; and each swap is refundable at most once (replay returns the
// existing position without enqueuing a second burn).
//
// Protected by the same X-Relay-Auth / per-CB signature middleware as bridge-in.
package handlers

import (
	"context"
	"log"
	"math/big"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// CrossCurrencyResidueEnqueuerIface enqueues the RESIDUE leg (burn on Hub, return on spoke).
type CrossCurrencyResidueEnqueuerIface interface {
	EnqueueResidueReturn(
		ctx context.Context,
		ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount, correlationID string,
		burnFromHubAddress, beneficiarySpokeAddress, swapTxHash, parentPositionID string,
	) (*services.BridgePositionResult, error)
}

// BridgePositionDetailReaderIface reads the internal detail of a bridge position. The
// residue amount is derived from the bridge-in position this CB created, never from the body.
type BridgePositionDetailReaderIface interface {
	GetPosition(ctx context.Context, positionID string) (*services.BridgePositionDetail, error)
}

// ResidueDuplicateFinderIface is the residue-leg idempotency lookup. Returns (nil, nil) when
// the swap's residue has not been returned yet.
type ResidueDuplicateFinderIface interface {
	FindResidueBySwapTxHash(ctx context.Context, txHash string) (*services.BridgePositionResult, error)
}

// CrossCurrencyResidueHandler handles POST /internal/amm/cross-currency-residue-return.
type CrossCurrencyResidueHandler struct {
	residueEnqueuer  CrossCurrencyResidueEnqueuerIface
	positionReader   BridgePositionDetailReaderIface
	swapVerifier     SwapVerifierIface
	duplicateFinder  ResidueDuplicateFinderIface
	payerResolver    BeneficiaryResolverIface
	wTokenAddress    string // W-<source> on Hub (this CB's sovereign W token)
	fiatTokenAddress string // tCeBM-<source> on this CB's spoke
	spokeNetwork     string // "spoke-a"
}

// NewCrossCurrencyResidueHandler constructs the handler.
func NewCrossCurrencyResidueHandler(
	residueEnqueuer CrossCurrencyResidueEnqueuerIface,
	positionReader BridgePositionDetailReaderIface,
	payerResolver BeneficiaryResolverIface,
	wTokenAddress, fiatTokenAddress, spokeNetwork string,
) *CrossCurrencyResidueHandler {
	return &CrossCurrencyResidueHandler{
		residueEnqueuer:  residueEnqueuer,
		positionReader:   positionReader,
		payerResolver:    payerResolver,
		wTokenAddress:    wTokenAddress,
		fiatTokenAddress: fiatTokenAddress,
		spokeNetwork:     spokeNetwork,
	}
}

// WithSwapVerification attaches the on-chain swap verifier and the residue idempotency
// lookup. The endpoint fails closed when no verifier is configured.
func (h *CrossCurrencyResidueHandler) WithSwapVerification(
	verifier SwapVerifierIface,
	finder ResidueDuplicateFinderIface,
) *CrossCurrencyResidueHandler {
	h.swapVerifier = verifier
	h.duplicateFinder = finder
	return h
}

// HandleResidueReturn processes the inbound residue-return delegation.
//
// Expected JSON body:
//
//	{
//	  "correlation_id":        "<uuid>",
//	  "swap_tx_hash":          "0x...",
//	  "pool_pair":             "W-BRL-ARS",
//	  "bridge_in_position_id": "<uuid>",
//	  "payer_bank_id":         "bank-a",
//	  "spoke_in":              "spoke-brl"
//	}
func (h *CrossCurrencyResidueHandler) HandleResidueReturn(c *fiber.Ctx) error {
	var req struct {
		CorrelationID      string `json:"correlation_id"`
		SwapTxHash         string `json:"swap_tx_hash"`
		PoolPair           string `json:"pool_pair"`
		BridgeInPositionID string `json:"bridge_in_position_id"`
		PayerBankID        string `json:"payer_bank_id"`
		SpokeIn            string `json:"spoke_in"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.CorrelationID == "" || req.SwapTxHash == "" || req.BridgeInPositionID == "" || req.PayerBankID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "correlation_id, swap_tx_hash, bridge_in_position_id, payer_bank_id are required",
		})
	}

	// The verified caller may only ask for its own residue back. The position's ownership is
	// checked further down, but against the request's payer_bank_id — which is only a boundary
	// once that id is known to be the caller's own.
	if ok, refusal := authorizeRelayCallerFor(c, req.PayerBankID); !ok {
		return refusal
	}

	if h.wTokenAddress == "" || h.fiatTokenAddress == "" {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "CB sovereign token addresses not configured (W_TOKEN_ADDRESS / TOKEN_ADDRESS)",
			"code":  "CB_NOT_CONFIGURED",
		})
	}
	if h.swapVerifier == nil {
		// Fail closed: returning value on the relay's word alone is the same vulnerability
		// the bridge-out endpoint must not have (R2-CR-6).
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "swap verification not configured on this gateway (HUB_BESU_RPC_URL / AMM_CONTRACT_ADDRESS)",
			"code":  "VERIFIER_NOT_CONFIGURED",
		})
	}
	if h.positionReader == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "bridge position reader not configured on this gateway",
			"code":  "POSITION_READER_NOT_CONFIGURED",
		})
	}

	// The bridge-in position is this CB's own record of what it moved to the Hub. It is the
	// only admissible source for the bridged amount.
	pos, err := h.positionReader.GetPosition(c.Context(), req.BridgeInPositionID)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "bridge-in position not found: " + err.Error(),
			"code":  "POSITION_NOT_FOUND",
		})
	}
	// Ownership is checked BEFORE the replay lookup: answering "duplicate" to a request that
	// names another bank's swap would disclose that position's id and state to a caller with no
	// claim to it. The reordering costs nothing — both are local reads.
	if !strings.EqualFold(strings.TrimSpace(pos.OwnerBankID), strings.TrimSpace(req.PayerBankID)) {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "bridge-in position belongs to another bank",
			"code":  "POSITION_OWNER_MISMATCH",
		})
	}

	// Replay protection: each swap's residue is returned at most once.
	if h.duplicateFinder != nil {
		existing, err := h.duplicateFinder.FindResidueBySwapTxHash(c.Context(), req.SwapTxHash)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "idempotency lookup failed: " + err.Error(),
			})
		}
		if existing != nil {
			log.Printf("[correlation_id=%s] residue-return replay detected: swap_tx_hash=%s already refunded by position %s",
				sanitizeLogField(req.CorrelationID), sanitizeLogField(req.SwapTxHash), existing.PositionID)
			return c.Status(fiber.StatusOK).JSON(fiber.Map{
				"status":         "duplicate",
				"position_id":    existing.PositionID,
				"bridge_state":   existing.BridgeState,
				"correlation_id": req.CorrelationID,
			})
		}
	}

	// Verify the swap on the Hub and confirm its *input* token is this CB's W-token. The
	// bridge-out endpoint checks TokenOut; a residue return is the mirror image — the money
	// coming back is what went in.
	verified, err := h.swapVerifier.VerifySwap(c.Context(), req.SwapTxHash, req.PoolPair)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "swap not verified on Hub: " + err.Error(),
			"code":  "SWAP_NOT_VERIFIED",
		})
	}
	if !strings.EqualFold(verified.TokenIn, h.wTokenAddress) {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "swap input token " + verified.TokenIn + " is not this CB's W-token",
			"code":  "RESIDUE_TOKEN_MISMATCH",
		})
	}

	if !strings.EqualFold(pos.MirroredAsset, h.wTokenAddress) {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "bridge-in position is not denominated in this CB's W-token",
			"code":  "POSITION_TOKEN_MISMATCH",
		})
	}
	if pos.Leg != "" && pos.Leg != string(domain.BridgeLegSettlement) {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "referenced position is not a settlement bridge-in",
			"code":  "POSITION_LEG_MISMATCH",
		})
	}
	// Only a completed lock-mint can have a residue: if the bridge-in never reached ACTIVE
	// there is nothing on the Hub to give back.
	if !strings.EqualFold(pos.BridgeState, "ACTIVE") {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "bridge-in position is not ACTIVE (state " + pos.BridgeState + ")",
			"code":  "POSITION_NOT_ACTIVE",
		})
	}
	// The residue can only be reclaimed from the address this CB minted to, and that address
	// must be the one that actually executed the swap.
	if pos.MintToHubAddress != "" && !strings.EqualFold(pos.MintToHubAddress, verified.User) {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "swap sender " + verified.User + " is not the address this CB minted to",
			"code":  "SWAP_SENDER_MISMATCH",
		})
	}

	bridged, ok := new(big.Int).SetString(strings.TrimSpace(pos.MirroredAmount), 10)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "bridge-in position has an unparseable mirrored_amount",
		})
	}
	consumed, ok := new(big.Int).SetString(strings.TrimSpace(verified.AmountIn), 10)
	if !ok {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "on-chain amount_in is unparseable",
			"code":  "SWAP_NOT_VERIFIED",
		})
	}
	if consumed.Cmp(bridged) > 0 {
		// The swap spent more than this CB bridged in — it drew on someone else's balance
		// on the shared Hub signer. Refuse and surface it: this is a reconciliation event,
		// not a refund.
		log.Printf("[correlation_id=%s] residue-return refused: swap consumed %s but only %s was bridged in (position %s)",
			sanitizeLogField(req.CorrelationID), consumed.String(), bridged.String(), pos.PositionID)
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "swap consumed more than the bridged amount — reconciliation required",
			"code":  "RESIDUE_INCONSISTENT",
		})
	}

	residue := new(big.Int).Sub(bridged, consumed)
	if residue.Sign() == 0 {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":         "no_residue",
			"correlation_id": req.CorrelationID,
		})
	}

	// Resolve the payer's spoke wallet from this CB's own participants registry — the
	// caller never gets to name the address the money lands on.
	payerWallet := ""
	if h.payerResolver != nil {
		w, resolveErr := h.payerResolver.ResolveWalletAddress(c.Context(), req.PayerBankID)
		if resolveErr != nil {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error": "payer bank not found or not active: " + resolveErr.Error(),
				"code":  "PAYER_NOT_FOUND",
			})
		}
		payerWallet = w
	}

	spokeIn := strings.TrimSpace(req.SpokeIn)
	if spokeIn == "" {
		spokeIn = h.spokeNetwork
	}

	result, err := h.residueEnqueuer.EnqueueResidueReturn(
		c.Context(),
		req.PayerBankID,
		spokeIn,
		h.fiatTokenAddress, // tCeBM-<source> on this CB's spoke
		h.wTokenAddress,    // W-<source> on Hub, to burn
		residue.String(),   // derived, never from the request
		req.CorrelationID,
		verified.User,          // burnFromHubAddress — verified on-chain swap sender
		payerWallet,            // beneficiarySpokeAddress — resolved locally
		req.SwapTxHash,         // idempotency, scoped to the RESIDUE leg
		req.BridgeInPositionID, // parent position this return corrects
	)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "residue return enqueue failed: " + err.Error(),
		})
	}

	log.Printf("[correlation_id=%s] residue return enqueued: position_id=%s amount=%s (bridged %s, consumed %s)",
		sanitizeLogField(req.CorrelationID), result.PositionID, residue.String(), bridged.String(), consumed.String())

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"status":         "accepted",
		"position_id":    result.PositionID,
		"amount":         residue.String(),
		"correlation_id": req.CorrelationID,
	})
}
