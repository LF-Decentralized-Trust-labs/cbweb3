// This file declares the token validation contract used by auth middleware.
package interfaces

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// TokenValidator defines bearer token validation behavior.
type TokenValidator interface {
	Validate(ctx context.Context, token string) (domain.TokenClaims, error)
}

