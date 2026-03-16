package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gofiber/fiber/v2"
)

const (
	// CorrelationIDHeader is the canonical header name (NFR-OPS-001, D6).
	CorrelationIDHeader = "X-Correlation-Id"
	// CorrelationIDLocal is the fiber Locals key used to propagate the ID.
	CorrelationIDLocal = "correlation_id"
)

// CorrelationID ensures every request carries an X-Correlation-Id header.
// If the client provides one, it is used; otherwise a new UUIDv4-like hex is generated.
// The value is:
//   - Accessible via c.Locals(CorrelationIDLocal) for downstream gRPC calls.
//   - Echoed back in the response header for end-to-end tracing.
func CorrelationID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		corrID := c.Get(CorrelationIDHeader)
		if corrID == "" {
			corrID = newCorrelationID()
		}
		c.Locals(CorrelationIDLocal, corrID)
		c.Set(CorrelationIDHeader, corrID)
		return c.Next()
	}
}

// newCorrelationID generates a 16-byte random hex string (UUIDv4 equivalent without dashes).
func newCorrelationID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}
