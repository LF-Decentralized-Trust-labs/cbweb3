package tokenissuer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type KeycloakIssuer struct {
	tokenURL     string
	clientID     string
	clientSecret string
	httpClient   *http.Client
}

func NewKeycloakIssuer(tokenURL, clientID, clientSecret string, timeout time.Duration) Issuer {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &KeycloakIssuer{
		tokenURL:     strings.TrimSpace(tokenURL),
		clientID:     strings.TrimSpace(clientID),
		clientSecret: strings.TrimSpace(clientSecret),
		httpClient:   &http.Client{Timeout: timeout},
	}
}

func (i *KeycloakIssuer) Name() string { return "keycloak" }

func (i *KeycloakIssuer) Issue(ctx context.Context, _ IssueRequest) (IssueResponse, error) {
	if i.tokenURL == "" || i.clientID == "" || i.clientSecret == "" {
		return IssueResponse{}, errors.New("keycloak issuer is not configured")
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", i.clientID)
	form.Set("client_secret", i.clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, i.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return IssueResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := i.httpClient.Do(req)
	if err != nil {
		return IssueResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return IssueResponse{}, errors.New("keycloak token endpoint returned non-200: " + string(body))
	}

	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return IssueResponse{}, err
	}
	if payload.AccessToken == "" {
		return IssueResponse{}, errors.New("keycloak returned empty token")
	}
	if payload.TokenType == "" {
		payload.TokenType = "Bearer"
	}
	return IssueResponse{
		AccessToken: payload.AccessToken,
		TokenType:   payload.TokenType,
		ExpiresIn:   payload.ExpiresIn,
	}, nil
}

func (i *KeycloakIssuer) Validate(_ context.Context, accessToken string) (IssueRequest, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return IssueRequest{}, errors.New("access token is required")
	}
	claims := jwt.MapClaims{}
	parser := jwt.NewParser()
	if _, _, err := parser.ParseUnverified(accessToken, claims); err != nil {
		return IssueRequest{}, err
	}
	subject, _ := claims["sub"].(string)
	if strings.TrimSpace(subject) == "" {
		return IssueRequest{}, errors.New("missing subject claim")
	}
	var roles []string
	if raw, ok := claims["roles"].([]any); ok {
		for _, item := range raw {
			if role, ok := item.(string); ok && strings.TrimSpace(role) != "" {
				roles = append(roles, role)
			}
		}
	}
	return IssueRequest{Subject: subject, Roles: roles}, nil
}
