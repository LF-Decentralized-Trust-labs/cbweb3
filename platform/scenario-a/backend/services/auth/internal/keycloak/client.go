// SPDX-License-Identifier: Apache-2.0

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

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/domain"
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
	// UpdateUsername renames a Keycloak user's login username via the Admin REST
	// API. NOTE: some Keycloak realm configurations mark the username field as
	// read-only; callers should treat errors as non-fatal.
	UpdateUsername(ctx context.Context, adminToken, userID, newUsername string) error
	// ResetPassword sets (or clears) the password for a Keycloak user via the
	// Admin REST API. Pass an empty string to create a credential-less account
	// that accepts grant_type=password with an empty password field.
	ResetPassword(ctx context.Context, adminToken, userID, password string) error
	// GetUserUsername returns the Keycloak login username for the user identified
	// by userID (UUID). Used by VerifyPKILogin to perform grant_type=password with
	// the user's actual username, which may differ from the UUID when UpdateUsername
	// is blocked by a read-only username policy.
	GetUserUsername(ctx context.Context, adminToken, userID string) (string, error)
	// GetUserEmail returns the email address for the user identified by userID (UUID).
	GetUserEmail(ctx context.Context, adminToken, userID string) (string, error)
	// AssignRealmRole assigns a realm-level role to the user identified by userID.
	// It first looks up the role by name to obtain its UUID, then calls the
	// role-mappings API. Non-fatal: callers should log and continue on error.
	AssignRealmRole(ctx context.Context, adminToken, userID, roleName string) error
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

// Login authenticates and returns tokens.
//
// Login authenticates a human operator via the OIDC Resource Owner Password
// Credentials (password) grant only. Portal/operator login MUST be a real
// Keycloak USER (the per-role admin users provisioned from spec.adminUsers), not
// a confidential client: client_credentials is no longer accepted here so that an
// operator cannot log in with the realm client id/secret. The gateway's
// configured client is used solely as the OIDC client for the password grant.
//
// Service-to-service access that legitimately needs client_credentials uses
// GetAdminToken (admin API), which is unaffected by this.
func (c *keycloakClient) Login(ctx context.Context, username, password string) (TokenResponse, error) {
	if c.cfg.ClientID == "" {
		return TokenResponse{}, errors.New("keycloak: no client configured for the password grant")
	}

	pwForm := url.Values{}
	pwForm.Set("grant_type", "password")
	pwForm.Set("client_id", c.cfg.ClientID)
	pwForm.Set("client_secret", c.cfg.ClientSecret)
	pwForm.Set("username", username)
	pwForm.Set("password", password)
	return c.postForm(ctx, c.tokenURL(), pwForm)
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

	wallet, _ := claims["wallet"].(string)
	country, _ := claims["country"].(string)
	bankID, _ := claims["bank_id"].(string)
	privacyGroup, _ := claims["privacy_group"].(string)

	return domain.TokenClaims{
		Subject:      subject,
		Issuer:       issuer,
		Roles:        roles,
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

// UpdateUsername renames a Keycloak user's login name via the Admin REST API.
// This is called during PKI-user onboarding so that the user's Keycloak username
// matches their UUID, enabling keycloak.Login(ctx, userID, "") in VerifyPKILogin.
func (c *keycloakClient) UpdateUsername(ctx context.Context, adminToken, userID, newUsername string) error {
	body, err := json.Marshal(map[string]any{"username": newUsername})
	if err != nil {
		return fmt.Errorf("keycloak: marshaling update-username request: %w", err)
	}

	url := fmt.Sprintf("%s/%s", c.adminUsersURL(), userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("keycloak: building update-username request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("keycloak: update-username request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("keycloak: update-username returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// ResetPassword sets the password for a Keycloak user via the Admin REST API.
// Passing an empty string creates a credential that allows grant_type=password
// with password="" (used by VerifyPKILogin to issue a Keycloak token after
// successful PKI signature verification).
func (c *keycloakClient) ResetPassword(ctx context.Context, adminToken, userID, password string) error {
	body, err := json.Marshal(map[string]any{
		"type":      "password",
		"value":     password,
		"temporary": false,
	})
	if err != nil {
		return fmt.Errorf("keycloak: marshaling reset-password request: %w", err)
	}

	url := fmt.Sprintf("%s/%s/reset-password", c.adminUsersURL(), userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("keycloak: building reset-password request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("keycloak: reset-password request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("keycloak: reset-password returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// GetUserUsername fetches the Keycloak username for the user identified by
// userID (UUID) via the Admin REST API. This is used by VerifyPKILogin to
// obtain the user's actual login username when the username field is read-only
// and could not be changed to the UUID during onboarding.
func (c *keycloakClient) GetUserUsername(ctx context.Context, adminToken, userID string) (string, error) {
	url := fmt.Sprintf("%s/%s", c.adminUsersURL(), userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("keycloak: building get-user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("keycloak: get-user request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("keycloak: get-user returned %d: %s", resp.StatusCode, string(body))
	}

	var user struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", fmt.Errorf("keycloak: decoding get-user response: %w", err)
	}
	if user.Username == "" {
		return "", fmt.Errorf("keycloak: user %s has no username", userID)
	}
	return user.Username, nil
}

// GetUserEmail fetches the email address for the user identified by userID (UUID)
// via the Keycloak Admin REST API.
func (c *keycloakClient) GetUserEmail(ctx context.Context, adminToken, userID string) (string, error) {
	userURL := fmt.Sprintf("%s/%s", c.adminUsersURL(), userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userURL, nil)
	if err != nil {
		return "", fmt.Errorf("keycloak: building get-user-email request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("keycloak: get-user-email request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("keycloak: get-user-email returned %d: %s", resp.StatusCode, string(body))
	}

	var user struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", fmt.Errorf("keycloak: decoding get-user-email response: %w", err)
	}
	return user.Email, nil
}

// AssignRealmRole assigns a Keycloak realm role to a user via the Admin REST API.
// It first resolves the role ID by name, then posts the role mapping.
func (c *keycloakClient) AssignRealmRole(ctx context.Context, adminToken, userID, roleName string) error {
	// 1. Resolve role ID.
	roleURL := fmt.Sprintf("%s/admin/realms/%s/roles/%s", c.cfg.BaseURL, c.cfg.Realm, roleName)
	roleReq, err := http.NewRequestWithContext(ctx, http.MethodGet, roleURL, nil)
	if err != nil {
		return fmt.Errorf("keycloak: building get-role request: %w", err)
	}
	roleReq.Header.Set("Authorization", "Bearer "+adminToken)

	roleResp, err := c.httpClient.Do(roleReq)
	if err != nil {
		return fmt.Errorf("keycloak: get-role request failed: %w", err)
	}
	defer roleResp.Body.Close()

	if roleResp.StatusCode >= 400 {
		body, _ := io.ReadAll(roleResp.Body)
		return fmt.Errorf("keycloak: get-role %q returned %d: %s", roleName, roleResp.StatusCode, string(body))
	}

	var role struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(roleResp.Body).Decode(&role); err != nil {
		return fmt.Errorf("keycloak: decoding get-role response: %w", err)
	}
	if role.ID == "" {
		return fmt.Errorf("keycloak: role %q returned empty id", roleName)
	}

	// 2. Assign role to user.
	mappingURL := fmt.Sprintf("%s/admin/realms/%s/users/%s/role-mappings/realm", c.cfg.BaseURL, c.cfg.Realm, userID)
	payload, err := json.Marshal([]map[string]string{{"id": role.ID, "name": role.Name}})
	if err != nil {
		return fmt.Errorf("keycloak: marshaling role-mapping request: %w", err)
	}

	mapReq, err := http.NewRequestWithContext(ctx, http.MethodPost, mappingURL, strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("keycloak: building role-mapping request: %w", err)
	}
	mapReq.Header.Set("Content-Type", "application/json")
	mapReq.Header.Set("Authorization", "Bearer "+adminToken)

	mapResp, err := c.httpClient.Do(mapReq)
	if err != nil {
		return fmt.Errorf("keycloak: role-mapping request failed: %w", err)
	}
	defer mapResp.Body.Close()

	if mapResp.StatusCode >= 400 {
		body, _ := io.ReadAll(mapResp.Body)
		return fmt.Errorf("keycloak: role-mapping returned %d: %s", mapResp.StatusCode, string(body))
	}
	return nil
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
