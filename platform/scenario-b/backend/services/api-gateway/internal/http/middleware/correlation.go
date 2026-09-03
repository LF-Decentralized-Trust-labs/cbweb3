// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/metadata"
)

const (
	// CorrelationIDHeader is the canonical header name (NFR-OPS-001, D6).
	CorrelationIDHeader = "X-Correlation-Id"
	// CorrelationIDLocal is the fiber Locals key used to propagate the ID.
	CorrelationIDLocal = "correlation_id"
)

// CorrelationID ensures every request carries a server-generated X-Correlation-Id header.
// Any value provided by the client is ignored/overridden.
// The value is:
//   - Accessible via c.Locals(CorrelationIDLocal) for downstream gRPC calls.
//   - Embedded in c.UserContext() as gRPC outgoing metadata so adapters propagate it automatically.
//   - Echoed back in the response header for end-to-end tracing.
func CorrelationID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		corrID := newCorrelationID()
		c.Locals(CorrelationIDLocal, corrID)
		c.Set(CorrelationIDHeader, corrID)
		// Embed into Go context so handlers can pass correlation ID to gRPC adapters via c.UserContext().
		c.SetUserContext(WithCorrelationID(c.UserContext(), corrID))
		c.SetUserContext(WithClientIP(c.UserContext(), c.IP()))
		return c.Next()
	}
}

// corrIDKey is an unexported type to avoid context key collisions.
type corrIDKey struct{}

// WithCorrelationID embeds the correlation ID into a Go context as a value
// AND as gRPC outgoing metadata, so downstream Invoke calls propagate it automatically.
func WithCorrelationID(parent context.Context, id string) context.Context {
	if id == "" {
		return parent
	}
	ctx := context.WithValue(parent, corrIDKey{}, id)
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}
	md = md.Copy()
	md.Set("x-correlation-id", id)
	return metadata.NewOutgoingContext(ctx, md)
}

// CorrelationIDFromContext extracts the correlation ID previously set by WithCorrelationID.
func CorrelationIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(corrIDKey{}).(string)
	return id
}

// WithClientIP embeds the client IP into the gRPC outgoing metadata
// so the identity service can record it in audit entries.
func WithClientIP(parent context.Context, ip string) context.Context {
	if ip == "" {
		return parent
	}
	md, ok := metadata.FromOutgoingContext(parent)
	if !ok {
		md = metadata.New(nil)
	}
	md = md.Copy()
	md.Set("x-forwarded-for", ip)
	return metadata.NewOutgoingContext(parent, md)
}

// newCorrelationID generates a 16-byte random hex string (UUIDv4 equivalent without dashes).
func newCorrelationID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}
