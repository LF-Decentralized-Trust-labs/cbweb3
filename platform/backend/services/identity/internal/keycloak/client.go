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
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

// Client defines the Keycloak operations used by the identity service.
type Client interface {
	Login(ctx context.Context, username, password string) (TokenResponse, error)
	Refresh(ctx context.Context, refreshToken string) (TokenResponse, error)
	Logout(ctx context.Context, refreshToken string) error
	ValidateToken(ctx context.Context, accessToken string) (domain.TokenClaims, error)
	// CreateUser provisions a new user in Keycloak via the Admin REST API and
	// returns the newly created user's UUID. The caller must supply a valid
	// admin access token obtained from the service-account credentials.
	CreateUser(ctx context.Context, adminToken string, user CreateUserRequest) (userID string, err error)
	// GetAdminToken exchanges client_credentials for an admin-capable access
	// token using the service account associated with ClientID/ClientSecret.
	GetAdminToken(ctx context.Context) (string, error)
}

type jwksCache struct {
	keys      map[string]jwksKey
	fetchedAt time.Time
}

type keycloakClient struct {
	cfg        Config
	httpClient *http.Client
	mu         sync.RWMutex
	cache      *jwksCache
}

// New creates a Keycloak client. Returns an error if BaseURL or Realm is empty.
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
	return &keycloakClient{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.RequestTimeout},
	}, nil
}

func (c *keycloakClient) tokenURL() string {
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.cfg.BaseURL, c.cfg.Realm)
}

func (c *keycloakClient) logoutURL() string {
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/logout", c.cfg.BaseURL, c.cfg.Realm)
}

func (c *keycloakClient) certsURL() string {
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", c.cfg.BaseURL, c.cfg.Realm)
}

// Login authenticates with username/password and returns tokens.
func (c *keycloakClient) Login(ctx context.Context, username, password string) (TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	form.Set("username", username)
	form.Set("password", password)
	return c.postForm(ctx, c.tokenURL(), form)
}

// Refresh exchanges a refresh token for a new token pair.
func (c *keycloakClient) Refresh(ctx context.Context, refreshToken string) (TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	form.Set("refresh_token", refreshToken)
	return c.postForm(ctx, c.tokenURL(), form)
}

// Logout revokes the refresh token in Keycloak.
func (c *keycloakClient) Logout(ctx context.Context, refreshToken string) error {
	form := url.Values{}
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	form.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.logoutURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("keycloak: building logout request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("keycloak: logout request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("keycloak: logout returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// ValidateToken parses and validates an RS256 JWT issued by Keycloak, returning
// enriched TokenClaims including realm roles and custom attributes.
func (c *keycloakClient) ValidateToken(ctx context.Context, accessToken string) (domain.TokenClaims, error) {
	// Parse JWT header to extract kid, then do full validated parse.
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

	token, err := jwt.Parse(accessToken, keyFunc,
		jwt.WithValidMethods([]string{"RS256"}),
	)
	if err != nil {
		return domain.TokenClaims{}, fmt.Errorf("keycloak: invalid token: %w", err)
	}
	if !token.Valid {
		return domain.TokenClaims{}, errors.New("keycloak: token is not valid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return domain.TokenClaims{}, errors.New("keycloak: cannot read token claims")
	}

	subject, _ := claims["sub"].(string)
	issuer, _ := claims["iss"].(string)

	roles := extractRealmRoles(claims)

	did, _ := claims["did"].(string)
	wallet, _ := claims["wallet"].(string)
	country, _ := claims["country"].(string)
	bankID, _ := claims["bank_id"].(string)
	privacyGroup, _ := claims["privacy_group"].(string)

	return domain.TokenClaims{
		Subject:      subject,
		Issuer:       issuer,
		Roles:        roles,
		DID:          did,
		Wallet:       wallet,
		Country:      country,
		BankID:       bankID,
		PrivacyGroup: privacyGroup,
	}, nil
}

// rsaKeyForKID returns the RSA public key from the JWKS that matches kid.
func (c *keycloakClient) rsaKeyForKID(ctx context.Context, kid string) (*rsa.PublicKey, error) {
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

// getJWKS returns the cached JWKS or fetches a fresh copy from Keycloak.
func (c *keycloakClient) getJWKS(ctx context.Context) (map[string]jwksKey, error) {
	c.mu.RLock()
	if c.cache != nil && time.Since(c.cache.fetchedAt) < c.cfg.JWKSCacheTTL {
		keys := c.cache.keys
		c.mu.RUnlock()
		return keys, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Re-check after acquiring write lock.
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

// jwksKeyToRSA constructs an *rsa.PublicKey from a JWKS key entry.
func jwksKeyToRSA(k jwksKey) (*rsa.PublicKey, error) {
	if k.Kty != "RSA" {
		return nil, fmt.Errorf("keycloak: unsupported key type %q (expected RSA)", k.Kty)
	}

	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("keycloak: decoding RSA modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("keycloak: decoding RSA exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}

// postForm sends a POST with URL-encoded form data and decodes the TokenResponse.
func (c *keycloakClient) postForm(ctx context.Context, endpoint string, form url.Values) (TokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenResponse{}, fmt.Errorf("keycloak: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("keycloak: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return TokenResponse{}, fmt.Errorf("keycloak: server returned %d: %s", resp.StatusCode, string(body))
	}

	var tr TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return TokenResponse{}, fmt.Errorf("keycloak: decoding token response: %w", err)
	}
	return tr, nil
}

// adminUsersURL returns the Keycloak Admin REST API URL for user management.
func (c *keycloakClient) adminUsersURL() string {
	return fmt.Sprintf("%s/admin/realms/%s/users", c.cfg.BaseURL, c.cfg.Realm)
}

// GetAdminToken exchanges client_credentials for an admin-scoped access token
// using the service account attached to ClientID/ClientSecret.
func (c *keycloakClient) GetAdminToken(ctx context.Context) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	tr, err := c.postForm(ctx, c.tokenURL(), form)
	if err != nil {
		return "", fmt.Errorf("keycloak: obtaining admin token: %w", err)
	}
	return tr.AccessToken, nil
}

// CreateUser creates a user in Keycloak via the Admin REST API and returns the
// new user's UUID extracted from the Location header of the 201 response.
// adminToken must be a valid admin-capable Bearer token.
func (c *keycloakClient) CreateUser(ctx context.Context, adminToken string, user CreateUserRequest) (string, error) {
	body, err := json.Marshal(user)
	if err != nil {
		return "", fmt.Errorf("keycloak: marshaling create-user request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.adminUsersURL(), strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("keycloak: building create-user request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("keycloak: create-user request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return "", fmt.Errorf("keycloak: user %q already exists", user.Username)
	}
	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("keycloak: create-user returned %d: %s", resp.StatusCode, string(respBody))
	}

	// Keycloak returns the new user UUID in the Location header:
	// Location: {baseURL}/admin/realms/{realm}/users/{uuid}
	location := resp.Header.Get("Location")
	if location == "" {
		return "", errors.New("keycloak: create-user response missing Location header")
	}
	parts := strings.Split(location, "/")
	if len(parts) == 0 {
		return "", errors.New("keycloak: cannot parse user UUID from Location header")
	}
	return parts[len(parts)-1], nil
}

// extractRealmRoles extracts the roles array from the Keycloak claim
// "realm_access": {"roles": ["role1", "role2"]}.
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
