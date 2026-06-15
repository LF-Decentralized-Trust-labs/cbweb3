// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// httpClient is a thin wrapper around http.Client that sends JSON requests to a
// fixed base URL with an optional Bearer token.
type httpClient struct {
	base   string
	token  string
	client *http.Client
}

func newHTTPClient(base, token string) *httpClient {
	return &httpClient{
		base:  strings.TrimRight(base, "/"),
		token: token,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// withToken returns a shallow copy of the client with a different token.
func (c *httpClient) withToken(token string) *httpClient {
	cp := *c
	cp.token = token
	return &cp
}

// setAuth injects the token as both a Bearer header and an access_token cookie so
// that routes using RequireBearerAuth, RequireCookieAuth, or RequireAnyAuth all work.
func (c *httpClient) setAuth(req *http.Request) {
	if c.token == "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: c.token})
}

// get performs GET {base}{path} and JSON-decodes the response into out (if non-nil).
// Returns an error instead of calling t.Fatal so it can be used inside pollUntil.
func (c *httpClient) get(t *testing.T, path string, out interface{}) error {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	c.setAuth(req)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s: HTTP %d: %s", path, resp.StatusCode, string(body))
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("GET %s: decode response: %w (body: %s)", path, err, string(body))
		}
	}
	return nil
}

// mustGet is like get but calls t.Fatal on any error.
func (c *httpClient) mustGet(t *testing.T, path string, out interface{}) {
	t.Helper()
	require.NoError(t, c.get(t, path, out), "GET %s", path)
}

// post performs POST {base}{path} with in as JSON body and decodes the response into out.
// Returns an error instead of calling t.Fatal.
func (c *httpClient) post(t *testing.T, path string, in, out interface{}) error {
	t.Helper()
	payload, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("marshal request body: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, c.base+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	c.setAuth(req)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("POST %s: HTTP %d: %s", path, resp.StatusCode, string(body))
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("POST %s: decode response: %w (body: %s)", path, err, string(body))
		}
	}
	return nil
}

// mustPost is like post but calls t.Fatal on any error.
func (c *httpClient) mustPost(t *testing.T, path string, in, out interface{}) {
	t.Helper()
	require.NoError(t, c.post(t, path, in, out), "POST %s", path)
}

// gatewayLogin obtains a Bearer token by calling the entity's API gateway
// POST /api/v1/auth/login, which routes to Keycloak internally. This avoids
// the HTTPS requirement on Keycloak's direct token endpoint.
//
// Retries on HTTP 503 for up to 60 s: the API gateway health check passes as
// soon as its HTTP server starts, but the auth service (separate container)
// needs a few more seconds to establish its Keycloak connection. Until it does,
// the gateway returns 503. Any other non-200 response fails immediately.
func gatewayLogin(t *testing.T, gatewayURL, clientID, clientSecret string) string {
	t.Helper()
	loginURL := strings.TrimRight(gatewayURL, "/") + "/api/v1/auth/login"

	payload, _ := json.Marshal(map[string]string{
		"clientId":     clientID,
		"clientSecret": clientSecret,
	})

	deadline := time.Now().Add(60 * time.Second)
	for {
		resp, err := http.Post(loginURL, "application/json", bytes.NewReader(payload)) //nolint:noctx
		require.NoError(t, err, "gateway login request failed for %s", gatewayURL)
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.NoError(t, err)

		if resp.StatusCode == http.StatusOK {
			var result struct {
				AccessToken string `json:"accessToken"`
			}
			require.NoError(t, json.Unmarshal(body, &result), "decode gateway login response")
			require.NotEmpty(t, result.AccessToken, "empty accessToken from %s", gatewayURL)
			return result.AccessToken
		}

		// 503 = auth/compliance service still warming up — retry until deadline.
		// Any other non-200 (including 401) is a hard credential/config failure.
		require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode,
			"gateway login returned %d for %s: %s", resp.StatusCode, gatewayURL, string(body))
		require.True(t, time.Now().Before(deadline),
			"gateway login still returning 503 for %s after 60s — auth service not ready", gatewayURL)
		t.Logf("gateway login: auth service not ready at %s (503), retrying...", gatewayURL)
		time.Sleep(3 * time.Second)
	}
}
