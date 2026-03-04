// This file implements a mock auth provider that issues local JWT access tokens.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

// MockAuthProvider issues signed JWTs for predefined client credentials.
type MockAuthProvider struct {
	clients    map[string]string
	privateKey *rsa.PrivateKey
	issuer     string
	audience   string
	ttl        time.Duration
}

type mockJWTClaims struct {
	Roles []string `json:"roles,omitempty"`
	Scope string   `json:"scope,omitempty"`
	jwt.RegisteredClaims
}

// NewMockAuthProvider builds a mock provider and generates an RSA keypair.
func NewMockAuthProvider(clients map[string]string, issuer, audience string, ttl time.Duration) (*MockAuthProvider, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return &MockAuthProvider{
		clients:    clients,
		privateKey: key,
		issuer:     issuer,
		audience:   audience,
		ttl:        ttl,
	}, nil
}

// Authenticate validates mock credentials and returns a local JWT access token.
func (m *MockAuthProvider) Authenticate(_ context.Context, clientID, clientSecret string) (domain.AuthToken, error) {
	secret, ok := m.clients[clientID]
	if !ok || secret != clientSecret {
		return domain.AuthToken{}, domain.ErrInvalidCredentials
	}

	now := time.Now()
	claims := mockJWTClaims{
		Roles: []string{"bank"},
		Scope: "gateway:write gateway:read",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   clientID,
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{m.audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(m.privateKey)
	if err != nil {
		return domain.AuthToken{}, err
	}

	return domain.AuthToken{
		AccessToken: signed,
		ExpiresIn:   int(m.ttl.Seconds()),
		TokenType:   "Bearer",
	}, nil
}

// PublicKey exposes the public key used to verify issued JWTs.
func (m *MockAuthProvider) PublicKey() *rsa.PublicKey {
	return &m.privateKey.PublicKey
}

// SignWalletBindMessage returns the canonical message expected for wallet binding.
func (m *MockAuthProvider) SignWalletBindMessage(subject, walletAddress string) (string, error) {
	if subject == "" || walletAddress == "" {
		return "", errors.New("subject and wallet address are required")
	}
	return walletBindMessage(subject, walletAddress), nil
}

