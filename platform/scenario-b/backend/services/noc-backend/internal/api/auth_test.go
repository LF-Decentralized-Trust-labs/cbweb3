// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/backend/services/noc-backend/internal/keycloak"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/backend/services/noc-backend/internal/middleware"
)

// The login the NOC portal never had. It used to call Keycloak from the browser and keep the
// tokens in localStorage, where any script on the page can read them. These tests pin the
// three properties that make the server-side version worth the change: the session lands in
// an HttpOnly cookie, a readable CSRF token lands beside it, and a wrong password is not
// reported the same way as a Keycloak outage.

type grantStub struct {
	tokens   keycloak.Tokens
	err      error
	gotUser  string
	gotPass  string
	gotRefTk string
}

func (g *grantStub) ValidateToken(context.Context, string) (keycloak.TokenClaims, error) {
	return keycloak.TokenClaims{Subject: "sub", Username: "noc-admin", Roles: []string{"ROLE_NOC_ADMIN"}}, nil
}

func (g *grantStub) PasswordGrant(_ context.Context, user, pass string) (keycloak.Tokens, error) {
	g.gotUser, g.gotPass = user, pass
	return g.tokens, g.err
}

func (g *grantStub) RefreshGrant(_ context.Context, refreshToken string) (keycloak.Tokens, error) {
	g.gotRefTk = refreshToken
	return g.tokens, g.err
}

func authTestApp(kc keycloak.Client, secure bool) *fiber.App {
	app := fiber.New()
	NewAuthHandler(kc, secure, []byte("test-secret")).Register(app.Group("/api/v1/auth"))
	return app
}

func cookiesByName(resp *http.Response) map[string]*http.Cookie {
	out := map[string]*http.Cookie{}
	for _, c := range resp.Cookies() {
		out[c.Name] = c
	}
	return out
}

func postJSON(t *testing.T, app *fiber.App, path, body string, reqCookies ...*http.Cookie) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range reqCookies {
		req.AddCookie(c)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request %s: %v", path, err)
	}
	return resp
}

func TestLogin_PutsTheSessionInAnHttpOnlyCookie(t *testing.T) {
	kc := &grantStub{tokens: keycloak.Tokens{
		AccessToken: "at", RefreshToken: "rt", ExpiresIn: 300, RefreshExpiresIn: 1800,
	}}
	resp := postJSON(t, authTestApp(kc, false), "/api/v1/auth/login",
		`{"username":"noc-admin","password":"s3cr3t"}`)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if kc.gotUser != "noc-admin" || kc.gotPass != "s3cr3t" {
		t.Errorf("credentials not forwarded to the realm: %q/%q", kc.gotUser, kc.gotPass)
	}

	jar := cookiesByName(resp)
	session, ok := jar["access_token"]
	if !ok {
		t.Fatal("no access_token cookie — there is no session for the browser to use")
	}
	if !session.HttpOnly {
		t.Error("the session cookie is NOT HttpOnly, which is the entire point of this change: " +
			"a readable cookie is no better than localStorage")
	}
	if session.Value != "at" {
		t.Errorf("session cookie = %q, want the access token", session.Value)
	}
	if session.MaxAge != 300 {
		t.Errorf("session MaxAge = %d, want the realm's expires_in (300)", session.MaxAge)
	}

	refresh, ok := jar["refresh_token"]
	if !ok {
		t.Fatal("no refresh_token cookie — silent refresh is impossible without it")
	}
	if !refresh.HttpOnly {
		t.Error("the refresh cookie must be HttpOnly too; it is the longer-lived credential")
	}
	if refresh.MaxAge <= session.MaxAge {
		t.Errorf("refresh MaxAge (%d) must outlive the session (%d), or the session cannot be renewed",
			refresh.MaxAge, session.MaxAge)
	}
}

// TestLogin_AlsoIssuesAReadableCSRFToken is the half a reviewer forgets. The session cookie
// being HttpOnly means the browser needs a SECOND, readable value to echo in a header —
// that is the double-submit mechanism, and without it every mutating request is refused.
func TestLogin_AlsoIssuesAReadableCSRFToken(t *testing.T) {
	kc := &grantStub{tokens: keycloak.Tokens{AccessToken: "at", ExpiresIn: 300}}
	resp := postJSON(t, authTestApp(kc, false), "/api/v1/auth/login", `{"username":"u","password":"p"}`)

	csrf, ok := cookiesByName(resp)[middleware.CSRFCookieName]
	if !ok {
		t.Fatal("no XSRF-TOKEN cookie — every mutating request would answer 403")
	}
	if csrf.HttpOnly {
		t.Error("the CSRF cookie must NOT be HttpOnly: the browser has to read it to echo the header")
	}
	if !middleware.ValidCSRFToken([]byte("test-secret"), csrf.Value, "at") {
		t.Error("the CSRF token is not bound to this session, so a token minted elsewhere would validate")
	}
}

func TestLogin_MarksCookiesSecureWhenConfigured(t *testing.T) {
	kc := &grantStub{tokens: keycloak.Tokens{AccessToken: "at", ExpiresIn: 300}}
	resp := postJSON(t, authTestApp(kc, true), "/api/v1/auth/login", `{"username":"u","password":"p"}`)

	for _, name := range []string{"access_token", middleware.CSRFCookieName} {
		if c, ok := cookiesByName(resp)[name]; !ok || !c.Secure {
			t.Errorf("%s cookie is not Secure with COOKIE_SECURE on", name)
		}
	}
}

// TestLogin_TellsBadCredentialsApartFromAnOutage repeats a lesson the gateways already
// learned: an operator told "invalid credentials" when the identity provider is down goes
// and rotates a secret that was fine.
func TestLogin_TellsBadCredentialsApartFromAnOutage(t *testing.T) {
	bad := &grantStub{err: keycloak.ErrInvalidCredentials}
	resp := postJSON(t, authTestApp(bad, false), "/api/v1/auth/login", `{"username":"u","password":"wrong"}`)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong password status = %d, want 401", resp.StatusCode)
	}

	down := &grantStub{err: errors.New("keycloak: token endpoint unreachable")}
	resp = postJSON(t, authTestApp(down, false), "/api/v1/auth/login", `{"username":"u","password":"p"}`)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("outage status = %d, want 503 — a 401 here sends the operator to rotate a good secret",
			resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["code"] == nil {
		t.Error("the refusal carries no stable code, so the portal has only prose to key on")
	}
}

func TestLogin_RejectsAnEmptyCredential(t *testing.T) {
	kc := &grantStub{tokens: keycloak.Tokens{AccessToken: "at", ExpiresIn: 300}}
	resp := postJSON(t, authTestApp(kc, false), "/api/v1/auth/login", `{"username":"","password":""}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 — an empty field is not a credential to try against the realm",
			resp.StatusCode)
	}
	if kc.gotUser != "" {
		t.Error("empty credentials were forwarded to Keycloak")
	}
}

func TestRefresh_RenewsFromTheCookieAndNotTheBody(t *testing.T) {
	kc := &grantStub{tokens: keycloak.Tokens{AccessToken: "at2", RefreshToken: "rt2", ExpiresIn: 300}}
	app := authTestApp(kc, false)

	resp := postJSON(t, app, "/api/v1/auth/refresh", `{"refresh_token":"attacker-supplied"}`,
		&http.Cookie{Name: "refresh_token", Value: "rt1"})

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if kc.gotRefTk != "rt1" {
		t.Errorf("refreshed with %q; it must come from the HttpOnly cookie, never the body", kc.gotRefTk)
	}
	if jar := cookiesByName(resp); jar["access_token"].Value != "at2" {
		t.Errorf("session cookie = %q, want the renewed token", jar["access_token"].Value)
	}
}

func TestRefresh_WithoutACookieIsRefused(t *testing.T) {
	kc := &grantStub{tokens: keycloak.Tokens{AccessToken: "at", ExpiresIn: 300}}
	resp := postJSON(t, authTestApp(kc, false), "/api/v1/auth/refresh", `{}`)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestLogout_ExpiresEveryAuthCookie(t *testing.T) {
	kc := &grantStub{tokens: keycloak.Tokens{AccessToken: "at", ExpiresIn: 300}}
	resp := postJSON(t, authTestApp(kc, false), "/api/v1/auth/logout", `{}`,
		&http.Cookie{Name: "access_token", Value: "at"})

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	jar := cookiesByName(resp)
	for _, name := range []string{"access_token", "refresh_token", middleware.CSRFCookieName} {
		c, ok := jar[name]
		if !ok {
			t.Errorf("logout did not clear %s — it would survive in the browser", name)
			continue
		}
		// Expiry, not an empty value. An empty cookie fails authentication, so logout
		// "works" either way — but the cookie would sit in the jar until the browser
		// closed. Fiber drops a negative MaxAge silently, so Expires is what deletes it.
		if c.Value != "" {
			t.Errorf("%s still carries a value after logout: %q", name, c.Value)
		}
		if c.Expires.IsZero() || c.Expires.After(time.Now()) {
			t.Errorf("%s is not expired (Expires = %v); the browser would keep it", name, c.Expires)
		}
	}
}

// With the session in an HttpOnly cookie the portal can no longer decode the token to learn
// who it is — that was the old model, and it only worked because the token was readable.
// The profile has to come from the server that CAN read it.

func TestLogin_ReturnsTheProfileTheTokenNoLongerReveals(t *testing.T) {
	kc := &grantStub{tokens: keycloak.Tokens{AccessToken: "at", ExpiresIn: 300}}
	resp := postJSON(t, authTestApp(kc, false), "/api/v1/auth/login", `{"username":"u","password":"p"}`)

	var body struct {
		User struct {
			ID    string   `json:"id"`
			Name  string   `json:"name"`
			Roles []string `json:"roles"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.User.Name != "noc-admin" {
		t.Errorf("user.name = %q, want the realm's preferred_username", body.User.Name)
	}
	if len(body.User.Roles) == 0 {
		t.Error("no roles returned — the portal decides what to render from these")
	}
}

func TestMe_AnswersFromTheSessionCookie(t *testing.T) {
	kc := &grantStub{}
	app := fiber.New()
	h := NewAuthHandler(kc, false, []byte("test-secret"))
	g := app.Group("/api/v1/auth")
	h.Register(g)
	h.RegisterProtected(g, middleware.RequireAuth(kc))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: "at"})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 — reopening the portal must restore the session", resp.StatusCode)
	}
	var body struct {
		User struct {
			Name string `json:"name"`
		} `json:"user"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.User.Name != "noc-admin" {
		t.Errorf("user.name = %q, want noc-admin", body.User.Name)
	}
}

func TestMe_WithoutASessionIsRefused(t *testing.T) {
	kc := &grantStub{}
	app := fiber.New()
	h := NewAuthHandler(kc, false, []byte("test-secret"))
	g := app.Group("/api/v1/auth")
	h.Register(g)
	h.RegisterProtected(g, middleware.RequireAuth(kc))

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}
