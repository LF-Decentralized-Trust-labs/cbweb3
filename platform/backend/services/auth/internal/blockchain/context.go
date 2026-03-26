package blockchain

import (
	"context"
	"errors"
	"strings"
)

type ctxKey struct{}

// ErrNoSignerUserID is returned when a signing operation is attempted without
// a signer user ID in the context. Callers must use WithSignerUserID to set it.
var ErrNoSignerUserID = errors.New("kms signer: no signer user ID in context")

// WithSignerUserID returns a context carrying the user ID that the KMSSigner
// should use for signing operations. This allows per-request signing where
// each commercial bank uses its own KMS-managed key.
//
// Panics if userID is empty — this is always a programming error.
func WithSignerUserID(ctx context.Context, userID string) context.Context {
	if strings.TrimSpace(userID) == "" {
		panic("blockchain.WithSignerUserID called with empty userID")
	}
	return context.WithValue(ctx, ctxKey{}, userID)
}

// SignerUserIDFromContext extracts the signer user ID previously set via
// WithSignerUserID. Returns the user ID and true, or "" and false if not set.
func SignerUserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxKey{}).(string)
	return v, ok && v != ""
}
