// This file declares the authentication provider contract used by handlers.
package interfaces

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// IAuthProvider defines client authentication behavior for token issuance.
type IAuthProvider interface {
	Authenticate(ctx context.Context, clientID, clientSecret string) (domain.AuthToken, error)
}

