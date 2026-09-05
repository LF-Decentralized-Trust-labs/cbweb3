// SPDX-License-Identifier: Apache-2.0

package keycloak

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Tokens is what a grant returns: the session, and how long each half of it lives.
//
// The lifetimes are not decoration — they become the MaxAge of the cookies the browser
// stores, and the refresh cookie has to outlive the access one or silent refresh is
// impossible.
type Tokens struct {
	AccessToken      string
	RefreshToken     string
	ExpiresIn        int
	RefreshExpiresIn int
}

// ErrInvalidCredentials is a rejection by the identity provider: wrong password, or a
// refresh token that has expired.
//
// Kept apart from every other failure on purpose. Telling an operator their credentials
// are wrong when Keycloak is unreachable sends them to rotate a secret that was fine —
// the finding the api-gateways already fixed, and the reason CodeAuthServiceUnavailable
// exists there.
var ErrInvalidCredentials = errors.New("keycloak: invalid credentials")

// IsInvalidCredentials reports whether err is the identity provider rejecting the
// credentials, as opposed to the identity provider being unavailable.
func IsInvalidCredentials(err error) bool {
	return errors.Is(err, ErrInvalidCredentials)
}

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
}

func (c *client) tokenURL() string {
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.cfg.BaseURL, c.cfg.Realm)
}

// PasswordGrant exchanges an operator's credentials for a session.
//
// This exists because the login had to move to the server. The portal used to call
// Keycloak from the browser and keep the tokens in localStorage, where any script on the
// page can read them; a cookie the browser cannot read can only be set by a server, so
// the server is now the one that talks to the realm.
func (c *client) PasswordGrant(ctx context.Context, username, password string) (Tokens, error) {
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("username", username)
	form.Set("password", password)
	return c.grant(ctx, form)
}

// RefreshGrant exchanges a refresh token for a new session.
func (c *client) RefreshGrant(ctx context.Context, refreshToken string) (Tokens, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	return c.grant(ctx, form)
}

func (c *client) grant(ctx context.Context, form url.Values) (Tokens, error) {
	form.Set("client_id", c.cfg.ClientID)
	// Omitted rather than sent empty. noc-portal is a public client today, and
	// client_secret="" is not the same as no client_secret — some realm configurations
	// reject the empty value, which would present as a login that cannot succeed.
	if secret := strings.TrimSpace(c.cfg.ClientSecret); secret != "" {
		form.Set("client_secret", secret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return Tokens{}, fmt.Errorf("keycloak: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Tokens{}, fmt.Errorf("keycloak: token endpoint unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusBadRequest {
		// Keycloak answers invalid_grant with 400 for a stale refresh token and 401 for a
		// wrong password. Both are the provider rejecting what it was given, which is the
		// distinction the caller needs — not the status code.
		return Tokens{}, ErrInvalidCredentials
	}
	if resp.StatusCode != http.StatusOK {
		return Tokens{}, fmt.Errorf("keycloak: token endpoint returned %d: %s",
			resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if readErr != nil {
		return Tokens{}, fmt.Errorf("keycloak: read token response: %w", readErr)
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return Tokens{}, fmt.Errorf("keycloak: decode token response: %w", err)
	}
	if strings.TrimSpace(tr.AccessToken) == "" {
		// A 200 with no token is not a session. Passing it through would set an empty
		// cookie and present as a login that "succeeded" and then failed on every request.
		return Tokens{}, errors.New("keycloak: token response carried no access_token")
	}
	return Tokens{
		AccessToken:      tr.AccessToken,
		RefreshToken:     tr.RefreshToken,
		ExpiresIn:        tr.ExpiresIn,
		RefreshExpiresIn: tr.RefreshExpiresIn,
	}, nil
}
