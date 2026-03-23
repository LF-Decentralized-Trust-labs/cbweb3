// Package noncestore provides short-lived nonce storage for PKI 2FA login.
// InMemoryStore is suitable for single-instance dev; RedisStore for production.
package noncestore

import (
	"context"
	"time"
)

// NonceStore manages short-lived PKI login nonces.
type NonceStore interface {
	// Set stores a nonce for the given userID with the specified TTL.
	Set(ctx context.Context, userID, nonce string, ttl time.Duration) error

	// GetAndDelete atomically retrieves and deletes the nonce for userID.
	// Returns ("", false, nil) when the nonce is not found or has expired.
	GetAndDelete(ctx context.Context, userID string) (nonce string, found bool, err error)
}
