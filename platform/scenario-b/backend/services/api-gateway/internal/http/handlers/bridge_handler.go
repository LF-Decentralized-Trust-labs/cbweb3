// Package handlers provides bridge HTTP handlers for Scenario B (FR-029 / SC-015).
package handlers

import (
	"context"
	"log"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// BridgeLockMintServiceIface is the interface consumed by BridgeHandler for lock-mint.
type BridgeLockMintServiceIface interface {
	LockAndEnqueue(ctx context.Context, ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount string) (*services.BridgePositionResult, error)
}

// BridgeBurnUnlockServiceIface is the interface consumed by BridgeHandler for burn-unlock.
type BridgeBurnUnlockServiceIface interface {
	BurnAndEnqueue(ctx context.Context, positionID string) (*services.BridgePositionResult, error)
}

// BridgePositionReaderIface reads bridge positions from the DB.
type BridgePositionReaderIface interface {
	ListPositions(ctx context.Context, stateFilter string) ([]services.BridgePositionResult, error)
}

// BridgeLimitCheckerIface is the transfer limit interface consumed by BridgeHandler (R1-10.1).
type BridgeLimitCheckerIface interface {
	CheckAndDeduct(ctx context.Context, payerBankID, currency, amountHuman string) error
	Restore(ctx context.Context, payerBankID, currency, amountHuman string)
}

// BridgeHandler handles Lock&Mint / Burn&Unlock bridge operations.
type BridgeHandler struct {
	lockMintSvc   BridgeLockMintServiceIface
	burnUnlockSvc BridgeBurnUnlockServiceIface
	posReader     BridgePositionReaderIface
	// Simplified API config (008-fix-cb-liquidity)
	spokeNetwork      string
	nativeAssetSymbol string
	wTokenAddress     string
	fallbackBankCode  string
	// limitChecker enforces configurable CB daily transfer limits (R1-10.1). Optional.
	limitChecker BridgeLimitCheckerIface
	// fiatSymbol is the canonical currency symbol used for limit matching (e.g. "BRL").
	// When set, overrides nativeAsset for CheckAndDeduct so limits can be expressed in
	// human-readable symbols regardless of what NATIVE_ASSET_SYMBOL holds.
	fiatSymbol string
}

// NewBridgeHandler creates a BridgeHandler (legacy mode - requires full payload).
func NewBridgeHandler(lockMint BridgeLockMintServiceIface, burnUnlock BridgeBurnUnlockServiceIface, posReader BridgePositionReaderIface) *BridgeHandler {
	return &BridgeHandler{lockMintSvc: lockMint, burnUnlockSvc: burnUnlock, posReader: posReader}
}

// NewBridgeHandlerSimplified creates a BridgeHandler with server-side field derivation (008-fix-cb-liquidity).
// When config fields are set, the handler will derive owner_bank_id from JWT and
// spoke_network/native_asset/mirrored_asset from config, allowing simplified payloads.
func NewBridgeHandlerSimplified(
	lockMint BridgeLockMintServiceIface,
	burnUnlock BridgeBurnUnlockServiceIface,
	posReader BridgePositionReaderIface,
	spokeNetwork, nativeAssetSymbol, wTokenAddress, fallbackBankCode string,
) *BridgeHandler {
	return &BridgeHandler{
		lockMintSvc:       lockMint,
		burnUnlockSvc:     burnUnlock,
		posReader:         posReader,
		spokeNetwork:      spokeNetwork,
		nativeAssetSymbol: nativeAssetSymbol,
		wTokenAddress:     wTokenAddress,
		fallbackBankCode:  strings.TrimSpace(fallbackBankCode),
	}
}

// SetFallbackBankCode configures BANK_CODE fallback for owner resolution when JWT claims lack BankID.
func (h *BridgeHandler) SetFallbackBankCode(bankCode string) *BridgeHandler {
	h.fallbackBankCode = strings.TrimSpace(bankCode)
	return h
}

// WithLimitChecker attaches a transfer limit checker to the bridge handler (R1-10.1).
func (h *BridgeHandler) WithLimitChecker(checker BridgeLimitCheckerIface) *BridgeHandler {
	h.limitChecker = checker
	return h
}

// WithFiatSymbol sets the canonical currency symbol used when enforcing transfer limits on bridge
// operations (R1-10.1). When set, this overrides the raw nativeAsset value (which may be a contract
// address) so that limits stored as e.g. "BRL" are correctly matched.
func (h *BridgeHandler) WithFiatSymbol(symbol string) *BridgeHandler {
	h.fiatSymbol = strings.TrimSpace(symbol)
	return h
}

// LockMint handles POST /api/v2/bridge/lock-mint (T066 / FR-029 / SC-015).
// Simplified API (008-fix-cb-liquidity): accepts {"amount": "..."} and derives server-side fields.
// Backward compatible: accepts legacy full payload with deprecation warnings.
func (h *BridgeHandler) LockMint(c *fiber.Ctx) error {
	var req struct {
		Amount string `json:"amount"`
		// Deprecated fields (backward compatibility - will be removed in future version)
		OwnerBankID   string `json:"owner_bank_id,omitempty"`
		SpokeNetwork  string `json:"spoke_network,omitempty"`
		NativeAsset   string `json:"native_asset,omitempty"`
		MirroredAsset string `json:"mirrored_asset,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Amount == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "amount required"})
	}

	// Derive owner_bank_id from JWT (simplified API)
	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authenticated claims"})
	}
	ownerBankID := strings.TrimSpace(claims.BankID)
	if ownerBankID == "" {
		ownerBankID = h.fallbackBankCode
		if ownerBankID != "" {
			log.Printf("[bridge] BankID missing in claims; using configured BANK_CODE fallback: %s", ownerBankID)
		}
	}
	if ownerBankID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authenticated claims"})
	}

	// Use config values if available (simplified API), otherwise require client-provided values
	spokeNetwork := h.spokeNetwork
	nativeAsset := h.nativeAssetSymbol
	mirroredAsset := h.wTokenAddress

	// Backward compatibility: accept client-provided values if config not set
	if spokeNetwork == "" {
		spokeNetwork = req.SpokeNetwork
	}
	if nativeAsset == "" {
		nativeAsset = req.NativeAsset
	}
	if mirroredAsset == "" {
		mirroredAsset = req.MirroredAsset
	}

	// Validate required fields after derivation
	if spokeNetwork == "" || nativeAsset == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "spoke_network and native_asset required (either in config or payload)",
		})
	}

	// Log deprecation warnings if client sends redundant fields
	if req.OwnerBankID != "" && req.OwnerBankID != ownerBankID {
		log.Printf("[DEPRECATED] owner_bank_id in payload (%s) differs from JWT (%s) - using JWT value", req.OwnerBankID, ownerBankID)
	}
	if req.SpokeNetwork != "" && h.spokeNetwork != "" {
		log.Printf("[DEPRECATED] spoke_network in payload - should be derived from server config")
	}
	if req.NativeAsset != "" && h.nativeAssetSymbol != "" {
		log.Printf("[DEPRECATED] native_asset in payload - should be derived from server config")
	}
	if req.MirroredAsset != "" && h.wTokenAddress != "" {
		log.Printf("[DEPRECATED] mirrored_asset in payload - should be derived from server config")
	}

	// R1-10.1: enforce daily transfer limit before submitting the lock.
	// Use fiatSymbol (e.g. "BRL") as the canonical currency key when available so that limits
	// expressed in human-readable symbols match correctly even if NATIVE_ASSET_SYMBOL is an address.
	limitCurrency := nativeAsset
	if h.fiatSymbol != "" {
		limitCurrency = h.fiatSymbol
	}
	if h.limitChecker != nil {
		if err := h.limitChecker.CheckAndDeduct(c.Context(), ownerBankID, limitCurrency, req.Amount); err != nil {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error":      err.Error(),
				"error_code": "TRANSFER_LIMIT_EXCEEDED",
			})
		}
	}

	pos, err := h.lockMintSvc.LockAndEnqueue(c.Context(), ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, req.Amount)
	if err != nil {
		if h.limitChecker != nil {
			h.limitChecker.Restore(c.Context(), ownerBankID, limitCurrency, req.Amount)
		}
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(pos)
}

// BurnUnlock handles POST /api/v2/bridge/burn-unlock (T067 / FR-029 / SC-015).
func (h *BridgeHandler) BurnUnlock(c *fiber.Ctx) error {
	var req struct {
		PositionID string `json:"position_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.PositionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "position_id required"})
	}

	pos, err := h.burnUnlockSvc.BurnAndEnqueue(c.Context(), req.PositionID)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(pos)
}

// ListPositions handles GET /api/v2/bridge/positions (T068 / FR-033).
func (h *BridgeHandler) ListPositions(c *fiber.Ctx) error {
	stateFilter := c.Query("state")
	positions, err := h.posReader.ListPositions(c.Context(), stateFilter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"positions": positions})
}
