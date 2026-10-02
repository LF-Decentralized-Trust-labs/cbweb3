// SPDX-License-Identifier: Apache-2.0

package middleware_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/gofiber/fiber/v2"
)

// The relay-authentication refusals must name themselves, for the same reason the caller-identity
// ones do: the bank portal decides what a 401 MEANS from its code, and an unclassified one is taken
// for an expired session — refresh, retry, same 401, forceLogout(). These reach the browser because
// PaymentProxyHandler.proxy relays the central bank's status and body verbatim, so a bank whose
// INTERNAL_RELAY_AUTH_SECRET diverged from its central bank's ejected the operator to the login
// screen with nothing said about the cause.
//
// They are NOT caller-identity refusals and must not borrow that code: there, the request
// authenticated and the identity was not one the central bank can act for; here the request never
// authenticated at all. Telling the operator to finish onboarding over a configuration fault would
// send them down a dead end, so the codes are distinct and the portal presents them differently.
const (
	codeRelayAuthRequired      = "RELAY_AUTH_REQUIRED"
	codeRelayAuthInvalid       = "RELAY_AUTH_INVALID"
	codeRelayAuthNotConfigured = "RELAY_AUTH_NOT_CONFIGURED"
)

// refusal returns the status and the "code" of a refusal, empty when the body names none.
func refusal(t *testing.T, app *fiber.App, req *http.Request) (int, string) {
	t.Helper()
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var body struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
	return resp.StatusCode, body.Code
}

func TestMigrating_RefusalsCarryARelayAuthCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		cfg        middleware.RelayAuthConfig
		secret     string // X-Relay-Auth to send; empty sends no header
		wantStatus int
		wantCode   string
	}{
		{
			name:       "no signature and no secret header",
			cfg:        middleware.RelayAuthConfig{LegacySecret: "shh"},
			wantStatus: http.StatusUnauthorized,
			wantCode:   codeRelayAuthRequired,
		},
		{
			name:       "the secret does not match",
			cfg:        middleware.RelayAuthConfig{LegacySecret: "shh"},
			secret:     "nope",
			wantStatus: http.StatusUnauthorized,
			wantCode:   codeRelayAuthInvalid,
		},
		{
			// Not reachable from a browser — the gateway fails closed before authenticating anyone
			// — but it is the same operator-facing condition as a divergent secret, and the portal
			// can only say so if it is named.
			name:       "nothing configured to authenticate against",
			cfg:        middleware.RelayAuthConfig{},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   codeRelayAuthNotConfigured,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPost, sigPath, strings.NewReader(`{}`))
			if tc.secret != "" {
				req.Header.Set("X-Relay-Auth", tc.secret)
			}

			status, code := refusal(t, newApp(tc.cfg), req)

			if status != tc.wantStatus {
				t.Errorf("status = %d, want %d", status, tc.wantStatus)
			}
			if code != tc.wantCode {
				t.Errorf("code = %q, want %q", code, tc.wantCode)
			}
		})
	}
}

// RequireRelayAuth is the older secret-only middleware, still guarding the hub's
// /internal/v1/spokes/* routes. Those callers are the toolkit and the relay rather than a browser,
// so no operator is ejected by them — but a refusal that names itself is what lets any caller tell
// "you sent no credential" from "the server has none configured", and holding one middleware to a
// rule the other is exempt from is how the exempt one becomes the next surprise.
func TestRequireRelayAuth_RefusalsCarryARelayAuthCode(t *testing.T) {
	t.Parallel()

	const path = "/internal/v1/spokes/register"

	cases := []struct {
		name       string
		configured string
		sent       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "no secret configured on the server",
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   codeRelayAuthNotConfigured,
		},
		{
			name:       "no header sent",
			configured: "shh",
			wantStatus: http.StatusUnauthorized,
			wantCode:   codeRelayAuthRequired,
		},
		{
			name:       "the secret does not match",
			configured: "shh",
			sent:       "nope",
			wantStatus: http.StatusUnauthorized,
			wantCode:   codeRelayAuthInvalid,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Post(path, middleware.RequireRelayAuth(tc.configured), func(c *fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
			if tc.sent != "" {
				req.Header.Set("X-Relay-Auth", tc.sent)
			}

			status, code := refusal(t, app, req)

			if status != tc.wantStatus {
				t.Errorf("status = %d, want %d", status, tc.wantStatus)
			}
			if code != tc.wantCode {
				t.Errorf("code = %q, want %q", code, tc.wantCode)
			}
		})
	}
}
