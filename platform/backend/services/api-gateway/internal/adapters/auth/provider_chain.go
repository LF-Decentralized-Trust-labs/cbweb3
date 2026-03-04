// This file composes auth providers and validators with fallback chaining behavior.
package auth

import (
	"context"
	"errors"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
)

// ProviderChain tries multiple auth providers until one succeeds.
type ProviderChain struct {
	providers []interfaces.IAuthProvider
}

// NewProviderChain creates a provider fallback chain in priority order.
func NewProviderChain(providers ...interfaces.IAuthProvider) *ProviderChain {
	return &ProviderChain{providers: providers}
}

// Authenticate delegates auth to providers until one returns a token.
func (p *ProviderChain) Authenticate(ctx context.Context, clientID, clientSecret string) (domain.AuthToken, error) {
	var lastErr error
	for _, provider := range p.providers {
		token, err := provider.Authenticate(ctx, clientID, clientSecret)
		if err == nil {
			return token, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = domain.ErrInvalidCredentials
	}
	return domain.AuthToken{}, lastErr
}

// ValidatorChain tries multiple token validators until one succeeds.
type ValidatorChain struct {
	validators []interfaces.TokenValidator
}

// NewValidatorChain creates a validator fallback chain in priority order.
func NewValidatorChain(validators ...interfaces.TokenValidator) *ValidatorChain {
	return &ValidatorChain{validators: validators}
}

// Validate delegates token validation to validators until one succeeds.
func (v *ValidatorChain) Validate(ctx context.Context, token string) (domain.TokenClaims, error) {
	var lastErr error
	for _, validator := range v.validators {
		claims, err := validator.Validate(ctx, token)
		if err == nil {
			return claims, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("no validator configured")
	}
	return domain.TokenClaims{}, lastErr
}

