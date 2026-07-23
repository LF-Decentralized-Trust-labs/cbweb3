// SPDX-License-Identifier: Apache-2.0

package keycloak

import "time"

// TokenResponse holds the token payload returned by Keycloak.
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	TokenType        string `json:"token_type"`
}

// CreateUserRequest carries the fields sent to Keycloak's Admin REST API
// POST /admin/realms/{realm}/users endpoint.
type CreateUserRequest struct {
	Username      string   `json:"username"`
	Email         string   `json:"email"`
	FirstName     string   `json:"firstName"`
	LastName      string   `json:"lastName"`
	EmailVerified bool     `json:"emailVerified"`
	Enabled       bool     `json:"enabled"`
	RealmRoles    []string `json:"-"` // assigned separately via role-mapping API
}

type jwksKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksResponse struct {
	Keys []jwksKey `json:"keys"`
}

// Config holds connection parameters for the Keycloak client.
type Config struct {
	BaseURL      string
	Realm        string
	ClientID     string
	ClientSecret string
	// Audience is the expected value of the "aud" claim on access tokens
	// validated by ValidateToken. When empty, audience validation is skipped;
	// callers should set it (typically to the OIDC client ID) so tokens minted
	// for a different client are rejected. The expected issuer is derived from
	// BaseURL + Realm and is always enforced.
	Audience       string
	JWKSCacheTTL   time.Duration
	RequestTimeout time.Duration
}
