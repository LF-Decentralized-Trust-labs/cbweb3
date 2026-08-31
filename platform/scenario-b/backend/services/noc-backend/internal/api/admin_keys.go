// SPDX-License-Identifier: Apache-2.0

package api

import (
	"crypto/sha256"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/backend/services/noc-backend/internal/repository"
)

// KeysHandler manages provisioned agent API keys.
type KeysHandler struct {
	agents *repository.AgentsRepository
	spokes *repository.SpokesRepository
}

// NewKeysHandler creates a new KeysHandler.
func NewKeysHandler(agents *repository.AgentsRepository, spokes *repository.SpokesRepository) *KeysHandler {
	return &KeysHandler{agents: agents, spokes: spokes}
}

// Register mounts routes under the given router group.
func (h *KeysHandler) Register(g fiber.Router) {
	g.Post("/agents/provision-key", h.provisionKey)
}

type provisionKeyRequest struct {
	RawKey  string `json:"raw_key"`
	SpokeID string `json:"spoke_id"`
	Hint    string `json:"hint"`
}

// provisionKey stores a new pre-shared key hash bound to a spoke.
func (h *KeysHandler) provisionKey(c *fiber.Ctx) error {
	var req provisionKeyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.RawKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "raw_key is required"})
	}
	if req.SpokeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "spoke_id is required"})
	}

	spokeID, err := uuid.Parse(req.SpokeID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid spoke_id"})
	}

	// Ensure spoke exists
	if _, err := h.spokes.GetByID(spokeID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "spoke not found"})
	}

	pk, err := h.agents.ProvisionKey(req.RawKey, spokeID, req.Hint)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to provision key"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":         pk.ID,
		"key_hash":   pk.KeyHash,
		"key_prefix": keyPrefix(req.RawKey),
		"spoke_id":   pk.SpokeID,
		"hint":       pk.Hint,
		"created_at": pk.CreatedAt,
	})
}

// keyPrefix returns the first 8 chars of the SHA-256 hex for display.
func keyPrefix(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)[:8]
}
