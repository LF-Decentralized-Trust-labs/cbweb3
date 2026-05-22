package middleware

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/domain"
)

const agentKey = "noc_agent"

// AgentAuth validates the X-Agent-Key header against provisioned key hashes.
// On success it stores the NocAgent record in Locals.
func AgentAuth(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rawKey := c.Get("X-Agent-Key")
		if rawKey == "" {
			rawKey = strings.TrimPrefix(c.Get("Authorization"), "Bearer ")
		}
		if rawKey == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing agent API key"})
		}

		hash := hashKey(rawKey)

		var agent domain.NocAgent
		if err := db.Where("api_key_hash = ?", hash).First(&agent).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid agent key"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal error"})
		}

		c.Locals(agentKey, agent)
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
