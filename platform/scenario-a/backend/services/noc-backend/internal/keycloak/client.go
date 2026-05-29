// Package keycloak provides a minimal Keycloak JWT validator for the NOC backend.
// It only needs to validate RS256 tokens and extract subject + realm roles.
package keycloak

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenClaims is the normalized token payload used by NOC backend.
type TokenClaims struct {
	Subject string
	Roles   []string
}

// Client validates Keycloak JWTs.
type Client interface {
	ValidateToken(ctx context.Context, accessToken string) (TokenClaims, error)
}

// Config holds Keycloak connection parameters.
type Config struct {
	BaseURL        string
	Realm          string
	JWKSCacheTTL   time.Duration
	RequestTimeout time.Duration
}

type jwksKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksResponse struct {
	Keys []jwksKey `json:"keys"`
}

type jwksCache struct {
	keys      map[string]jwksKey
	fetchedAt time.Time
}

type client struct {
	cfg        Config
	httpClient *http.Client
	mu         sync.RWMutex
	cache      *jwksCache
}

// New creates a Keycloak JWT validation client.
func New(cfg Config) (Client, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, errors.New("keycloak: BaseURL is required")
	}
	if strings.TrimSpace(cfg.Realm) == "" {
		return nil, errors.New("keycloak: Realm is required")
	}
	if cfg.JWKSCacheTTL <= 0 {
		cfg.JWKSCacheTTL = 5 * time.Minute
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 10 * time.Second
	}
	return &client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.RequestTimeout},
	}, nil
}

func (c *client) certsURL() string {
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", c.cfg.BaseURL, c.cfg.Realm)
}

// ValidateToken parses and validates an RS256 JWT, returning subject and realm roles.
func (c *client) ValidateToken(ctx context.Context, accessToken string) (TokenClaims, error) {
	keyFunc := func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("keycloak: unexpected signing method: %v", token.Header["alg"])
		}
		kid, _ := token.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("keycloak: missing kid in JWT header")
		}
		return c.rsaKeyForKID(ctx, kid)
	}

	token, err := jwt.Parse(accessToken, keyFunc, jwt.WithValidMethods([]string{"RS256"}))
	if err != nil {
		return TokenClaims{}, fmt.Errorf("keycloak: invalid token: %w", err)
	}
	if !token.Valid {
		return TokenClaims{}, errors.New("keycloak: token is not valid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return TokenClaims{}, errors.New("keycloak: cannot read token claims")
	}

	subject, _ := claims["sub"].(string)

	return TokenClaims{
		Subject: subject,
		Roles:   extractRealmRoles(claims),
	}, nil
}

func (c *client) rsaKeyForKID(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	keys, err := c.getJWKS(ctx)
	if err != nil {
		return nil, err
	}
	k, ok := keys[kid]
	if !ok {
		return nil, fmt.Errorf("keycloak: no JWKS key found for kid %q", kid)
	}
	return jwksKeyToRSA(k)
}

func (c *client) getJWKS(ctx context.Context) (map[string]jwksKey, error) {
	c.mu.RLock()
	if c.cache != nil && time.Since(c.cache.fetchedAt) < c.cfg.JWKSCacheTTL {
		keys := c.cache.keys
		c.mu.RUnlock()
		return keys, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cache != nil && time.Since(c.cache.fetchedAt) < c.cfg.JWKSCacheTTL {
		return c.cache.keys, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.certsURL(), nil)
	if err != nil {
		return nil, fmt.Errorf("keycloak: building JWKS request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("keycloak: JWKS fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("keycloak: JWKS endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("keycloak: decoding JWKS response: %w", err)
	}

	keys := make(map[string]jwksKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		keys[k.Kid] = k
	}
	c.cache = &jwksCache{keys: keys, fetchedAt: time.Now()}
	return keys, nil
}

func jwksKeyToRSA(k jwksKey) (*rsa.PublicKey, error) {
	if k.Kty != "RSA" {
		return nil, fmt.Errorf("keycloak: unsupported key type %q", k.Kty)
	}
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("keycloak: decoding RSA modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("keycloak: decoding RSA exponent: %w", err)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}

func extractRealmRoles(claims jwt.MapClaims) []string {
	realmAccess, ok := claims["realm_access"].(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := realmAccess["roles"]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	roles := make([]string, 0, len(items))
	for _, item := range items {
		if role, ok := item.(string); ok && strings.TrimSpace(role) != "" {
			roles = append(roles, role)
		}
	}
	return roles
}

// noOpClient is a no-op Keycloak client for local development (NOC_SKIP_AUTH=true).
// It accepts any token string and returns a super-admin claim set.
type noOpClient struct{}

// NewNoOp returns a Client that always approves any token with all NOC roles.
// ONLY for local development — never use in production.
func NewNoOp() Client {
	return &noOpClient{}
}

func (n *noOpClient) ValidateToken(_ context.Context, _ string) (TokenClaims, error) {
	return TokenClaims{
		Subject: "dev-user",
		Roles:   []string{"noc-admin", "noc-operator", "noc-viewer"},
	}, nil
}
