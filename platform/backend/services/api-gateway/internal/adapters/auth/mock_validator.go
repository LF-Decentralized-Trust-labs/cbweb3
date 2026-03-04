// This file implements JWT validation for tokens issued by the mock provider.
package auth

import (
	"context"
	"crypto/rsa"
	"errors"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

// MockTokenValidator validates JWTs produced by the mock auth provider.
type MockTokenValidator struct {
	publicKey *rsa.PublicKey
	issuer    string
	audience  string
}

// NewMockTokenValidator creates a validator for issuer/audience constrained tokens.
func NewMockTokenValidator(publicKey *rsa.PublicKey, issuer, audience string) *MockTokenValidator {
	return &MockTokenValidator{
		publicKey: publicKey,
		issuer:    issuer,
		audience:  audience,
	}
}

// Validate verifies token signature and required claims.
func (m *MockTokenValidator) Validate(_ context.Context, token string) (domain.TokenClaims, error) {
	claims := &mockJWTClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, errors.New("unexpected signing algorithm")
		}
		return m.publicKey, nil
	})
	if err != nil || !parsed.Valid {
		return domain.TokenClaims{}, domain.ErrInvalidToken
	}
	if claims.Issuer != m.issuer {
		return domain.TokenClaims{}, domain.ErrInvalidToken
	}
	validAudience := false
	for _, aud := range claims.RegisteredClaims.Audience {
		if aud == m.audience {
			validAudience = true
			break
		}
	}
	if !validAudience {
		return domain.TokenClaims{}, domain.ErrInvalidToken
	}
	return domain.TokenClaims{
		Subject: claims.Subject,
		Issuer:  claims.Issuer,
		Scope:   claims.Scope,
		Roles:   claims.Roles,
	}, nil
}

