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
// The five Scenario B portals all rendered `error.message` from axios, so an operator saw
// "Request failed with status code 400" and could not tell a wrong secret from a missing field
// from a gateway that was down. The message the gateway already writes was never read.
//
// The frontend cannot key on the prose — reword "clientId is required" and every portal silently
// falls back to the raw status — and it cannot key on the status either, since 400 covers both a
// missing field and a malformed body. So the body carries a stable `code`, the same convention the
// relay-rejection classifier already relies on. `error` is left exactly as it was: it is what the
// existing clients and logs read.

func decodeErrorBody(t *testing.T, resp *http.Response) (code string, message string) {
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

func postLogin(t *testing.T, handler *AuthHandler, payload []byte) *http.Response {
	t.Helper()
	app := fiber.New()
	app.Post("/auth/login", handler.Login)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestLogin_CarriesAStableCodeForEachRefusal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		handler  func() *AuthHandler
		payload  []byte
		status   int
		wantCode string
		why      string
	}{
		{
			name:    "missing_client_id",
			handler: func() *AuthHandler { return NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false) },
			payload: mustJSON(t, map[string]string{"clientId": "", "clientSecret": "s"}),
			status:  http.StatusBadRequest, wantCode: CodeMissingCredentials,
			why: "an empty field is the operator's own omission, not a rejected credential",
		},
		{
			name:    "missing_client_secret",
			handler: func() *AuthHandler { return NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false) },
			payload: mustJSON(t, map[string]string{"clientId": "bank", "clientSecret": ""}),
			status:  http.StatusBadRequest, wantCode: CodeMissingCredentials,
			why: "same code for either field: naming which one is missing is fine, but the frontend must not need to parse prose to know",
		},
		{
			name:    "malformed_body",
			handler: func() *AuthHandler { return NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false) },
			payload: []byte("{"),
			status:  http.StatusBadRequest, wantCode: CodeInvalidRequest,
			why: "a malformed body is a client bug, not something the operator can fix by typing more carefully — it must not read as a credential problem",
		},
		{
			name: "rejected_credentials",
			handler: func() *AuthHandler {
				return NewAuthHandler(authProviderStub{err: errors.New("invalid")}, kycCheckerStub{}, false)
			},
			payload: mustJSON(t, map[string]string{"clientId": "bank", "clientSecret": "bad"}),
			status:  http.StatusUnauthorized, wantCode: CodeInvalidCredentials,
			why: "one code for both an unknown client and a wrong secret — telling them apart is user enumeration",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := postLogin(t, tc.handler(), tc.payload)
			if resp.StatusCode != tc.status {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.status)
			}
			code, message := decodeErrorBody(t, resp)
			if code != tc.wantCode {
				t.Fatalf("code = %q, want %q — %s", code, tc.wantCode, tc.why)
			}
			// The human-readable field stays populated: logs and existing clients read it.
			if message == "" {
				t.Fatal("error message must not be dropped when a code is added")
			}
		})
	}
}

func TestRefresh_DistinguishesNoSessionFromARejectedOne(t *testing.T) {
	t.Parallel()

	// This is the pair behind the "refreshToken is required" an operator reported on a LOGIN screen:
	// a session-restore probe with no cookie yet answers 400, and rendering that as a login failure
	// tells the operator their credentials were wrong when nothing was even submitted. A code lets
	// the caller recognise "no session to restore" and stay silent.
	newApp := func(h *AuthHandler) *fiber.App {
		app := fiber.New()
		app.Post("/auth/refresh", h.Refresh)
		return app
	}

	t.Run("no_token_at_all", func(t *testing.T) {
		app := newApp(NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false))
		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", resp.StatusCode)
		}
		if code, _ := decodeErrorBody(t, resp); code != CodeMissingRefreshToken {
			t.Fatalf("code = %q, want %q — a caller cannot otherwise tell this from a rejected credential", code, CodeMissingRefreshToken)
		}
	})

	t.Run("token_rejected", func(t *testing.T) {
		app := newApp(NewAuthHandler(authProviderStub{err: errors.New("expired")}, kycCheckerStub{}, false))
		body := mustJSON(t, map[string]string{"refreshToken": "stale"})
		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", resp.StatusCode)
		}
		if code, _ := decodeErrorBody(t, resp); code != CodeInvalidRefreshToken {
			t.Fatalf("code = %q, want %q", code, CodeInvalidRefreshToken)
		}
	})
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return out
}
