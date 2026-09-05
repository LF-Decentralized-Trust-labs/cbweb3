// SPDX-License-Identifier: Apache-2.0

package keycloak

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// The NOC portals kept their tokens in localStorage because the login happened in the
// browser: it called Keycloak's token endpoint directly. A cookie can only be set by the
// server, so the grant has to move here — and until now this client could only validate a
// token someone else had obtained.

func grantServer(t *testing.T, status int, body string, capture *url.Values) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/x-www-form-urlencoded") {
			t.Errorf("Content-Type = %q, want form-urlencoded", got)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if capture != nil {
			*capture = r.PostForm
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func newGrantClient(t *testing.T, baseURL string) Client {
	t.Helper()
	kc, err := New(Config{BaseURL: baseURL, Realm: "cbweb3", ClientID: "noc-portal"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return kc
}

func TestPasswordGrant_ExchangesCredentialsForTokens(t *testing.T) {
	var form url.Values
	srv := grantServer(t, http.StatusOK,
		`{"access_token":"at","refresh_token":"rt","expires_in":300,"refresh_expires_in":1800}`, &form)
	defer srv.Close()

	tok, err := newGrantClient(t, srv.URL).PasswordGrant(context.Background(), "noc-admin", "s3cr3t")
	if err != nil {
		t.Fatalf("PasswordGrant: %v", err)
	}

	if tok.AccessToken != "at" || tok.RefreshToken != "rt" {
		t.Errorf("tokens = %+v, want at/rt", tok)
	}
	if tok.ExpiresIn != 300 || tok.RefreshExpiresIn != 1800 {
		t.Errorf("lifetimes = %d/%d, want 300/1800 — the cookie MaxAge comes from these",
			tok.ExpiresIn, tok.RefreshExpiresIn)
	}
	if form.Get("grant_type") != "password" {
		t.Errorf("grant_type = %q, want password", form.Get("grant_type"))
	}
	if form.Get("username") != "noc-admin" || form.Get("password") != "s3cr3t" {
		t.Errorf("credentials not forwarded: %v", form)
	}
	if form.Get("client_id") != "noc-portal" {
		t.Errorf("client_id = %q, want the configured noc-portal", form.Get("client_id"))
	}
}

// TestPasswordGrant_SendsNoEmptyClientSecret guards the public-client case. noc-portal is a
// public client today, and posting client_secret="" is not the same as omitting it — some
// Keycloak configurations reject the empty value outright.
func TestPasswordGrant_SendsNoEmptyClientSecret(t *testing.T) {
	var form url.Values
	srv := grantServer(t, http.StatusOK, `{"access_token":"at","expires_in":300}`, &form)
	defer srv.Close()

	if _, err := newGrantClient(t, srv.URL).PasswordGrant(context.Background(), "u", "p"); err != nil {
		t.Fatalf("PasswordGrant: %v", err)
	}
	if _, present := form["client_secret"]; present {
		t.Errorf("client_secret was sent as %q with none configured; it must be omitted",
			form.Get("client_secret"))
	}
}

func TestPasswordGrant_SendsAConfiguredClientSecret(t *testing.T) {
	var form url.Values
	srv := grantServer(t, http.StatusOK, `{"access_token":"at","expires_in":300}`, &form)
	defer srv.Close()

	kc, err := New(Config{BaseURL: srv.URL, Realm: "cbweb3", ClientID: "noc-portal", ClientSecret: "shh"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := kc.PasswordGrant(context.Background(), "u", "p"); err != nil {
		t.Fatalf("PasswordGrant: %v", err)
	}
	if form.Get("client_secret") != "shh" {
		t.Errorf("client_secret = %q, want shh", form.Get("client_secret"))
	}
}

// TestPasswordGrant_DistinguishesBadCredentialsFromAnOutage is what keeps a login screen
// honest. Telling an operator their password is wrong when Keycloak is down sends them to
// rotate a credential that was fine — the same finding the gateways already fixed.
func TestPasswordGrant_DistinguishesBadCredentialsFromAnOutage(t *testing.T) {
	bad := grantServer(t, http.StatusUnauthorized, `{"error":"invalid_grant"}`, nil)
	defer bad.Close()
	if _, err := newGrantClient(t, bad.URL).PasswordGrant(context.Background(), "u", "wrong"); !IsInvalidCredentials(err) {
		t.Errorf("a 401 from Keycloak must report invalid credentials, got %v", err)
	}

	down := grantServer(t, http.StatusInternalServerError, `{"error":"boom"}`, nil)
	defer down.Close()
	_, err := newGrantClient(t, down.URL).PasswordGrant(context.Background(), "u", "p")
	if err == nil {
		t.Fatal("a 500 from Keycloak must be an error")
	}
	if IsInvalidCredentials(err) {
		t.Errorf("a 500 must NOT be reported as invalid credentials, got %v", err)
	}
}

func TestRefreshGrant_ExchangesTheRefreshToken(t *testing.T) {
	var form url.Values
	srv := grantServer(t, http.StatusOK,
		`{"access_token":"at2","refresh_token":"rt2","expires_in":300}`, &form)
	defer srv.Close()

	tok, err := newGrantClient(t, srv.URL).RefreshGrant(context.Background(), "rt1")
	if err != nil {
		t.Fatalf("RefreshGrant: %v", err)
	}
	if tok.AccessToken != "at2" || tok.RefreshToken != "rt2" {
		t.Errorf("tokens = %+v, want at2/rt2", tok)
	}
	if form.Get("grant_type") != "refresh_token" {
		t.Errorf("grant_type = %q, want refresh_token", form.Get("grant_type"))
	}
	if form.Get("refresh_token") != "rt1" {
		t.Errorf("refresh_token = %q, want rt1", form.Get("refresh_token"))
	}
}

// TestRefreshGrant_ReportsAnExpiredSessionAsInvalidCredentials lets the portal tell a real
// expiry apart from a broken realm: the first sends the operator to the login screen, the
// second must not.
func TestRefreshGrant_ReportsAnExpiredSessionAsInvalidCredentials(t *testing.T) {
	srv := grantServer(t, http.StatusBadRequest, `{"error":"invalid_grant"}`, nil)
	defer srv.Close()

	if _, err := newGrantClient(t, srv.URL).RefreshGrant(context.Background(), "stale"); !IsInvalidCredentials(err) {
		t.Errorf("an expired refresh token must report invalid credentials, got %v", err)
	}
}

func TestNoOpClient_GrantsWithoutAKeycloak(t *testing.T) {
	// NOC_SKIP_AUTH deployments have no realm to talk to. The no-op client must still
	// answer a login, or turning auth off would break the portal instead of loosening it.
	tok, err := NewNoOp().PasswordGrant(context.Background(), "anyone", "anything")
	if err != nil {
		t.Fatalf("no-op PasswordGrant: %v", err)
	}
	if strings.TrimSpace(tok.AccessToken) == "" {
		t.Error("the no-op client must hand back some token, or there is no session to set a cookie from")
	}
}
