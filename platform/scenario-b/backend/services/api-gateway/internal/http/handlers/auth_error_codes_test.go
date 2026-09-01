// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
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

// --- the wire values themselves ---
//
// Review of #188: renaming CodeMissingCredentials's VALUE (not its identifier) to
// "CREDENTIALS_MISSING" left the whole Go suite green, because every test here compares against
// the constant. The frontend's CODE_TO_MESSAGE hardcodes the literal, so the two lists are
// independent and nothing checked that they agree — a rename would keep both suites green and
// silently degrade the 400 case to "Sign-in failed (HTTP 400)" in all five portals, which is the
// status dump this work exists to remove.
//
// Pinning the literals makes a rename a deliberate two-file act. INVALID_CREDENTIALS is the benign
// one — a 401 still falls through to the right copy — but it is pinned too, so the rule has no
// exceptions to remember.
func TestAuthErrorCodes_WireValuesArePinned(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ got, want string }{
		{CodeInvalidRequest, "INVALID_REQUEST"},
		{CodeMissingCredentials, "MISSING_CREDENTIALS"},
		{CodeInvalidCredentials, "INVALID_CREDENTIALS"},
		{CodeAuthServiceUnavailable, "AUTH_SERVICE_UNAVAILABLE"},
		{CodeMissingRefreshToken, "MISSING_REFRESH_TOKEN"},
		{CodeInvalidRefreshToken, "INVALID_REFRESH_TOKEN"},
	} {
		if tc.got != tc.want {
			t.Errorf("wire value is %q, want %q — the portals' CODE_TO_MESSAGE keys on the literal, so changing it here alone degrades every portal to a bare status", tc.got, tc.want)
		}
	}
}

// --- the 503 branch ---
//
// Also from the review: dropping the code from BOTH service-unavailable responses left the suite
// green. That is the path behind this work's own headline — "503 does not blame the credential" —
// so it was the one branch the frontend was written against and the backend did not hold itself to.
//
// Reaching it needs a PKI provider: Login tries IssueLoginNonce first, and a NON-gRPC error there
// (a network failure, a timeout, a cancelled context) is what produces the 503.

type pkiProviderStub struct {
	authProviderStub
	nonceErr error
}

func (s pkiProviderStub) IssueLoginNonce(_ context.Context, _, _ string) (string, error) {
	return "", s.nonceErr
}

func (s pkiProviderStub) VerifyPKILogin(_ context.Context, _, _, _ string) (domain.AuthToken, error) {
	return domain.AuthToken{}, s.nonceErr
}

func TestLogin_ServiceUnavailableCarriesItsOwnCode(t *testing.T) {
	t.Parallel()

	// A plain error, not a gRPC status: the handler reads that as "the auth service could not be
	// reached", which must never be reported to an operator as a rejected credential.
	handler := NewAuthHandler(pkiProviderStub{nonceErr: errors.New("dial tcp: connection refused")}, kycCheckerStub{}, false)
	resp := postLogin(t, handler, mustJSON(t, map[string]string{"clientId": "bank", "clientSecret": "s"}))

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
	code, message := decodeErrorBody(t, resp)
	if code != CodeAuthServiceUnavailable {
		t.Fatalf("code = %q, want %q — without it the portals fall back to a bare status and tell the operator to check credentials that were fine", code, CodeAuthServiceUnavailable)
	}
	if code == CodeInvalidCredentials {
		t.Fatal("a service outage must never be reported as a rejected credential")
	}
	if message == "" {
		t.Fatal("the human-readable message must survive adding a code")
	}
}
