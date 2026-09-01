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

// --- the wire values themselves ---
//
// Review of the Scenario B twin (#188): renaming a code's VALUE (not its identifier) left the whole
// Go suite green, because every test compares against the constant while the portals' CODE_TO_MESSAGE
// hardcodes the literal. Two independent lists with nothing checking they agree — a rename would keep
// both suites green and silently degrade the 400 case to a bare status in every portal, which is the
// status dump this work exists to remove.
//
// Pinning makes a rename a deliberate two-file act. INVALID_CREDENTIALS is pinned too even though a
// 401 would still fall through to the right copy: a rule with an exception is a rule to remember.
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
			t.Errorf("wire value is %q, want %q — the portals key on the literal, so changing it here alone degrades every portal to a bare status", tc.got, tc.want)
		}
	}
}

// --- the 503 branch ---
//
// The path behind this work's own headline, "503 does not blame the credential", and the one branch
// with no test: dropping the code from both service-unavailable responses left the suite green, so
// the frontend was written against a contract the backend did not hold itself to.
//
// Reaching it needs a PKI provider: Login tries IssueLoginNonce first, and a NON-gRPC error there —
// a network failure, a timeout, a cancelled context — is what produces the 503.

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

	handler := NewAuthHandler(pkiProviderStub{nonceErr: errors.New("dial tcp: connection refused")}, kycCheckerStub{}, false)
	resp := postTo(t, "/auth/login", func(app *fiber.App) { app.Post("/auth/login", handler.Login) },
		jsonBody(t, map[string]string{"clientId": "admin@cb.test", "clientSecret": "s"}))

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
	code, message := decodeAuthError(t, resp)
	if code != CodeAuthServiceUnavailable {
		t.Fatalf("code = %q, want %q — without it the portals fall back to a bare status and send an operator to reset a password that was fine", code, CodeAuthServiceUnavailable)
	}
	if message == "" {
		t.Fatal("the human-readable message must survive adding a code")
	}
}
