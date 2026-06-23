// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

// Package integration_test holds the LIVE-STACK happy-path suite for Scenario A
// (Enhanced Correspondent Banking, dual-layer HTLC). It drives the full
// FX-Agreement + cross-spoke HTLC settlement flow over the real REST API of a
// running `make spoke-all` stack.
//
// This is the slow, infra-bound counterpart to the hermetic `integration_lite`
// suite that lives in the same directory under `package integration`. The two
// never compile together: this file is gated behind the `integration` build tag,
// the hermetic suite behind `integration_lite`.
//
// Run against a live stack (default — does NOT touch the chain):
//
//	SKIP_UP=1 go test -v -count=1 -tags integration -timeout 30m -run TestFullHappyPath ./...
//
// Run including stack bring-up (slow; regenerates genesis, WIPES the chain):
//
//	SKIP_UP=0 go test -v -count=1 -tags integration -timeout 45m -run TestFullHappyPath ./...
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
)

// httpClient is a thin wrapper around http.Client that sends JSON requests to a
// fixed base URL with an optional auth token.
type httpClient struct {
	base   string
	token  string
	corr   string // X-Correlation-Id sent with each request when set
	client *http.Client
}

func newHTTPClient(base, token string) *httpClient {
	return &httpClient{
		base:  strings.TrimRight(base, "/"),
		token: token,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// withToken returns a shallow copy of the client carrying a different token.
func (c *httpClient) withToken(token string) *httpClient {
	cp := *c
	cp.token = token
	return &cp
}

// withCorr returns a shallow copy of the client that sends the given
// X-Correlation-Id with each request, so the request can be tied to its
// on-chain tx in the evidence bundle's aggregated_traces.log.
func (c *httpClient) withCorr(corr string) *httpClient {
	cp := *c
	cp.corr = corr
	return &cp
}

// setAuth injects the token as both an access_token cookie and a Bearer header.
// Scenario A's payment/fx/htlc routes use RequireCookieAuth; sending both keeps
// the client compatible with any bearer-guarded routes too.
func (c *httpClient) setAuth(req *http.Request) {
	if c.token == "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: c.token})
}

// setCorr attaches the X-Correlation-Id header when one is set on the client.
func (c *httpClient) setCorr(req *http.Request) {
	if c.corr != "" {
		req.Header.Set("X-Correlation-Id", c.corr)
	}
}

// get performs GET {base}{path} and JSON-decodes the response into out (if non-nil).
// It returns an error (rather than failing the test) so it can be used inside pollUntil.
func (c *httpClient) get(path string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	c.setAuth(req)
	c.setCorr(req)
	return c.do(req, http.MethodGet, path, out)
}

// post performs POST {base}{path} with in as a JSON body and decodes into out.
func (c *httpClient) post(path string, in, out interface{}) error {
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
	c.setCorr(req)
	return c.do(req, http.MethodPost, path, out)
}

func (c *httpClient) do(req *http.Request, method, path string, out interface{}) error {
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
		return fmt.Errorf("%s %s: HTTP %d: %s", method, path, resp.StatusCode, string(body))
	}
	if out != nil && len(body) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("%s %s: decode response: %w (body: %s)", method, path, err, string(body))
		}
	}
	return nil
}

// mustGet is like get but fails the test on any error.
func (c *httpClient) mustGet(t *testing.T, path string, out interface{}) {
	t.Helper()
	if err := c.get(path, out); err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
}

// mustPost is like post but fails the test on any error.
func (c *httpClient) mustPost(t *testing.T, path string, in, out interface{}) {
	t.Helper()
	if err := c.post(path, in, out); err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
}

// gatewayLogin obtains a token by calling the entity's API gateway
// POST /api/v1/auth/login, which routes to Keycloak internally.
//
// Retries on HTTP 503 for up to 90 s: the gateway health check passes as soon as
// its HTTP server starts, but the auth service (a separate container) needs a few
// more seconds to establish its Keycloak connection and returns 503 until it does.
// Any other non-200 response (including 401) is a hard credential/config failure.
func gatewayLogin(t *testing.T, name, gatewayURL, clientID, clientSecret string) string {
	t.Helper()
	loginURL := strings.TrimRight(gatewayURL, "/") + "/api/v1/auth/login"
	payload, _ := json.Marshal(map[string]string{
		"clientId":     clientID,
		"clientSecret": clientSecret,
	})

	deadline := time.Now().Add(90 * time.Second)
	for {
		resp, err := http.Post(loginURL, "application/json", bytes.NewReader(payload)) //nolint:noctx
		if err != nil {
			t.Fatalf("gateway login request failed for %s (%s): %v", name, gatewayURL, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var result struct {
				AccessToken string `json:"accessToken"`
			}
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("decode gateway login response for %s: %v (body: %s)", name, err, string(body))
			}
			if result.AccessToken == "" {
				t.Fatalf("empty accessToken from %s (%s)", name, gatewayURL)
			}
			t.Logf("  [%s] authenticated", name)
			return result.AccessToken
		}

		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("gateway login for %s returned HTTP %d (expected 200): %s", name, resp.StatusCode, string(body))
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("gateway login still 503 for %s (%s) after 90s — auth service not ready", name, gatewayURL)
		}
		t.Logf("  [%s] auth service not ready (503), retrying...", name)
		time.Sleep(3 * time.Second)
	}
}
