// This file validates bearer tokens using Keycloak token introspection.
package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// KeycloakTokenValidator validates access tokens via Keycloak introspection.
type KeycloakTokenValidator struct {
	introspectionURL string
	clientID         string
	clientSecret     string
	httpClient       *http.Client
}

type keycloakIntrospectionResponse struct {
	Active bool        `json:"active"`
	Sub    string      `json:"sub"`
	Iss    string      `json:"iss"`
	Scope  string      `json:"scope"`
	Aud    interface{} `json:"aud"`
}

// NewKeycloakTokenValidator creates a validator with client credentials and timeout.
func NewKeycloakTokenValidator(introspectionURL, clientID, clientSecret string, timeout time.Duration) *KeycloakTokenValidator {
	return &KeycloakTokenValidator{
		introspectionURL: introspectionURL,
		clientID:         clientID,
		clientSecret:     clientSecret,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Validate introspects the token and maps active claims to domain claims.
func (k *KeycloakTokenValidator) Validate(ctx context.Context, token string) (domain.TokenClaims, error) {
	if strings.TrimSpace(k.introspectionURL) == "" {
		return domain.TokenClaims{}, domain.ErrInvalidToken
	}
	form := url.Values{}
	form.Set("token", token)
	form.Set("client_id", k.clientID)
	form.Set("client_secret", k.clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, k.introspectionURL, strings.NewReader(form.Encode()))
	if err != nil {
		return domain.TokenClaims{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return domain.TokenClaims{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return domain.TokenClaims{}, domain.ErrInvalidToken
	}

	var payload keycloakIntrospectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return domain.TokenClaims{}, err
	}
	if !payload.Active || strings.TrimSpace(payload.Sub) == "" {
		return domain.TokenClaims{}, domain.ErrInvalidToken
	}

	return domain.TokenClaims{
		Subject: payload.Sub,
		Issuer:  payload.Iss,
		Scope:   payload.Scope,
		Roles:   audiencesToRoles(payload.Aud),
	}, nil
}

// audiencesToRoles normalizes token audience values into a roles slice.
func audiencesToRoles(aud interface{}) []string {
	switch raw := aud.(type) {
	case string:
		if strings.TrimSpace(raw) == "" {
			return nil
		}
		return []string{raw}
	case []interface{}:
		roles := make([]string, 0, len(raw))
		for _, item := range raw {
			roles = append(roles, stringify(item))
		}
		return roles
	default:
		return nil
	}
}

// stringify converts supported audience value types to strings.
func stringify(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return ""
	}
}

