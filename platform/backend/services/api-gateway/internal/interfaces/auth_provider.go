// This file declares the authentication provider contract used by handlers.
package interfaces

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// IAuthProvider defines client authentication and token lifecycle behavior.
type IAuthProvider interface {
	Authenticate(ctx context.Context, clientID, clientSecret string) (domain.AuthToken, error)
	RefreshToken(ctx context.Context, refreshToken string) (domain.AuthToken, error)
	Logout(ctx context.Context, accessToken string) error
}
