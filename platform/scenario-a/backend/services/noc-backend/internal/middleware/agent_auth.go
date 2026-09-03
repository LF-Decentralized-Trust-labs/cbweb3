// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/repository"
)

const agentKey = "noc_agent"

// AgentAuth validates the X-Agent-Key header against provisioned key hashes.
// On first push the agent record does not yet exist in noc_agents; in that case
// the middleware looks up noc_provisioned_keys and auto-creates the NocAgent.
func AgentAuth(db *gorm.DB) fiber.Handler {
	agents := repository.NewAgentsRepository(db)

	return func(c *fiber.Ctx) error {
		rawKey := c.Get("X-Agent-Key")
		if rawKey == "" {
			rawKey = strings.TrimPrefix(c.Get("Authorization"), "Bearer ")
		}
		if rawKey == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing agent API key"})
		}

		hash := hashKey(rawKey)

		// Fast path: agent already registered
		var agent domain.NocAgent
		err := db.Where("api_key_hash = ?", hash).First(&agent).Error
		if err == nil {
			c.Locals(agentKey, agent)
			return c.Next()
		}
		if err != gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal error"})
		}

		// First-push path: look up provisioned key and auto-create the agent
		pk, err := agents.FindProvisionedKey(rawKey)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid agent key"})
		}

		hint := pk.Hint
		if hint == "" {
			hint = pk.ID.String()
		}
		newAgent, err := agents.FindOrCreateAgent(rawKey, pk.SpokeID, "noc-agent-"+hint, 15)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "agent auto-registration failed"})
		}
		_ = agents.MarkKeyUsed(pk.ID)

		c.Locals(agentKey, *newAgent)
		return c.Next()
	}
}

// GetAgent retrieves the NocAgent stored by AgentAuth.
func GetAgent(c *fiber.Ctx) (domain.NocAgent, bool) {
	agent, ok := c.Locals(agentKey).(domain.NocAgent)
	return agent, ok
}

// hashKey returns the hex-encoded SHA-256 of a raw key string.
func hashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}
