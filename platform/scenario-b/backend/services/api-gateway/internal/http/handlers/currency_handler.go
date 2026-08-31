// SPDX-License-Identifier: Apache-2.0

// Package handlers provides HTTP handlers for CurrencyRegistry operations (006-hub-currency-registry).
// Exposes three endpoints:
//
//	POST   /api/v2/hub/currencies           — Central Bank registers its currency on the hub.
//	DELETE /api/v2/hub/currencies/:symbol   — Central Bank removes its currency from the hub.
//	GET    /api/v2/hub/currencies           — List all registered currencies (permissionless).
package handlers

import (
	"errors"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// CurrencyHandler handles CurrencyRegistry HTTP operations.
type CurrencyHandler struct {
	svc services.CurrencyServiceIface
}

// NewCurrencyHandler creates a CurrencyHandler backed by the given service.
func NewCurrencyHandler(svc services.CurrencyServiceIface) *CurrencyHandler {
	return &CurrencyHandler{svc: svc}
}

// RegisterCurrency handles POST /api/v2/hub/currencies (FR-003 / 006-hub-currency-registry).
func (h *CurrencyHandler) RegisterCurrency(c *fiber.Ctx) error {
	var req services.CurrencyRegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Symbol == "" || req.CountryName == "" || req.TokenAddress == "" || req.ProposerCB == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "symbol, country_name, token_address, proposer_cb are required",
		})
	}

	result, err := h.svc.RegisterCurrency(c.Context(), req)
	if err != nil {
		return currencyErrorResponse(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

// RemoveCurrency handles DELETE /api/v2/hub/currencies/:symbol (FR-004 / 006-hub-currency-registry).
func (h *CurrencyHandler) RemoveCurrency(c *fiber.Ctx) error {
	symbol := c.Params("symbol")
	if symbol == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "symbol path parameter is required"})
	}

	result, err := h.svc.RemoveCurrency(c.Context(), services.CurrencyRemoveRequest{Symbol: symbol})
	if err != nil {
		return currencyErrorResponse(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(result)
}

// ListCurrencies handles GET /api/v2/hub/currencies (FR-005 / 006-hub-currency-registry).
func (h *CurrencyHandler) ListCurrencies(c *fiber.Ctx) error {
	entries, err := h.svc.ListCurrencies(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to list currencies",
			"code":  "ONCHAIN_ERROR",
		})
	}

	resp := make([]fiber.Map, len(entries))
	for i, e := range entries {
		resp[i] = fiber.Map{
			"symbol":        e.Symbol,
			"country_name":  e.CountryName,
			"token_address": e.TokenAddress,
			"proposer_cb":   e.ProposerCB,
		}
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"currencies": resp})
}

// currencyErrorResponse maps domain errors to HTTP status codes.
func currencyErrorResponse(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, services.ErrCurrencyAlreadyExists):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": err.Error(), "code": "CURRENCY_ALREADY_EXISTS",
		})
	case errors.Is(err, services.ErrTokenAlreadyRegistered):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": err.Error(), "code": "TOKEN_ALREADY_REGISTERED",
		})
	case errors.Is(err, services.ErrCurrencyNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(), "code": "CURRENCY_NOT_FOUND",
		})
	case errors.Is(err, services.ErrCurrencyUnauthorized):
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": err.Error(), "code": "UNAUTHORIZED",
		})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(), "code": "ONCHAIN_ERROR",
		})
	}
}
