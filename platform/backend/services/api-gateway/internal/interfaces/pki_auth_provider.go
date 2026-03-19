package interfaces

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// IPKIAuthProvider extends authentication with PKI 2FA methods.
// Implemented by IdentityGRPCAuthProvider.
type IPKIAuthProvider interface {
	// IssueLoginNonce generates a short-lived nonce for the given user (PKI step 1).
	IssueLoginNonce(ctx context.Context, userID string) (string, error)
	// VerifyPKILogin validates the signed nonce + X.509 cert and returns a JWT (PKI step 2).
	VerifyPKILogin(ctx context.Context, userID, nonceSignatureHex, certPEM string) (domain.AuthToken, error)
}
