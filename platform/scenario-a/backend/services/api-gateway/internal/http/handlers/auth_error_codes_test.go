// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// --- stable error codes on the auth routes ---
//
// The five Scenario A portals all rendered `error.message` from axios, so an operator saw "Request
// failed with status code 400" whether they mistyped a password, left a field empty, or hit a
// gateway that was down. The message the gateway already writes was never read.
//
// The frontend cannot key on the prose — reword one string and every portal silently falls back to
// the raw status — and the status is not enough either, since 400 covers both a missing field and a
// malformed body. So the body carries a stable `code`. `error` is untouched: logs and existing
// clients read it.

func decodeAuthError(t *testing.T, resp *http.Response) (code string, message string) {
	t.Helper()
	var body struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return body.Code, body.Error
}

func postTo(t *testing.T, route string, register func(*fiber.App), payload []byte) *http.Response {
	t.Helper()
	app := fiber.New()
	register(app)
	req := httptest.NewRequest(http.MethodPost, route, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func jsonBody(t *testing.T, v any) []byte {
	t.Helper()
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return out
}

func TestLogin_CarriesAStableCodeForEachRefusal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider authProviderStub
		payload  []byte
		status   int
		wantCode string
		why      string
	}{
		{
			name:    "missing_username",
			payload: jsonBody(t, map[string]string{"clientId": "", "clientSecret": "p"}),
			status:  http.StatusBadRequest, wantCode: CodeMissingCredentials,
			why: "an empty field is the operator's own omission, not a rejected credential",
		},
		{
			name:    "missing_password",
			payload: jsonBody(t, map[string]string{"clientId": "admin@cb6", "clientSecret": ""}),
			status:  http.StatusBadRequest, wantCode: CodeMissingCredentials,
			why: "same code for either field: no caller should need to parse prose to know which",
		},
		{
			name:    "malformed_body",
			payload: []byte("{"),
			status:  http.StatusBadRequest, wantCode: CodeInvalidRequest,
			why: "a malformed body is a client bug and must not read as a credential problem",
		},
		{
			name:     "rejected_credentials",
			provider: authProviderStub{err: errors.New("invalid")},
			payload:  jsonBody(t, map[string]string{"clientId": "admin@cb6", "clientSecret": "bad"}),
			status:   http.StatusUnauthorized, wantCode: CodeInvalidCredentials,
			why: "one code for both an unknown user and a wrong password — telling them apart is user enumeration",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewAuthHandler(tc.provider, kycCheckerStub{}, false)
			resp := postTo(t, "/auth/login", func(app *fiber.App) { app.Post("/auth/login", handler.Login) }, tc.payload)
			if resp.StatusCode != tc.status {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.status)
			}
			code, message := decodeAuthError(t, resp)
			if code != tc.wantCode {
				t.Fatalf("code = %q, want %q — %s", code, tc.wantCode, tc.why)
			}
			if message == "" {
				t.Fatal("the human-readable message must survive adding a code")
			}
		})
	}
}

// The pair behind the "refreshToken is required" a BCRP tester reported on a LOGIN screen: a
// session-restore probe with no cookie answers 400, and rendering that as a login failure tells the
// operator their password was wrong when nothing was submitted.
func TestRefresh_DistinguishesNoSessionFromARejectedOne(t *testing.T) {
	t.Parallel()

	t.Run("no_token_at_all", func(t *testing.T) {
		handler := NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false)
		resp := postTo(t, "/auth/refresh", func(app *fiber.App) { app.Post("/auth/refresh", handler.Refresh) }, []byte("{}"))
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", resp.StatusCode)
		}
		if code, _ := decodeAuthError(t, resp); code != CodeMissingRefreshToken {
			t.Fatalf("code = %q, want %q — a caller cannot otherwise tell this from a refused credential", code, CodeMissingRefreshToken)
		}
	})

	t.Run("token_rejected", func(t *testing.T) {
		handler := NewAuthHandler(authProviderStub{err: errors.New("expired")}, kycCheckerStub{}, false)
		body := jsonBody(t, map[string]string{"refreshToken": "stale"})
		resp := postTo(t, "/auth/refresh", func(app *fiber.App) { app.Post("/auth/refresh", handler.Refresh) }, body)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", resp.StatusCode)
		}
		if code, _ := decodeAuthError(t, resp); code != CodeInvalidRefreshToken {
			t.Fatalf("code = %q, want %q", code, CodeInvalidRefreshToken)
		}
	})
}
