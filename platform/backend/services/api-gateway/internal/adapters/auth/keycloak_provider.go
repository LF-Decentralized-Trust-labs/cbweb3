// This file implements client authentication against the Keycloak token endpoint.
package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// KeycloakAuthProvider authenticates clients using Keycloak's token endpoint.
type KeycloakAuthProvider struct {
	tokenURL   string
	httpClient *http.Client
}

type keycloakTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// NewKeycloakAuthProvider creates a Keycloak provider with an HTTP timeout.
func NewKeycloakAuthProvider(tokenURL string, timeout time.Duration) *KeycloakAuthProvider {
	return &KeycloakAuthProvider{
		tokenURL: tokenURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Authenticate requests a client-credentials token from Keycloak.
func (k *KeycloakAuthProvider) Authenticate(ctx context.Context, clientID, clientSecret string) (domain.AuthToken, error) {
	if strings.TrimSpace(k.tokenURL) == "" {
		return domain.AuthToken{}, domain.ErrInvalidCredentials
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, k.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return domain.AuthToken{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return domain.AuthToken{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return domain.AuthToken{}, domain.ErrInvalidCredentials
	}

	var payload keycloakTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return domain.AuthToken{}, err
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return domain.AuthToken{}, domain.ErrInvalidCredentials
	}

	tokenType := payload.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	return domain.AuthToken{
		AccessToken: payload.AccessToken,
		ExpiresIn:   payload.ExpiresIn,
		TokenType:   tokenType,
	}, nil
}

