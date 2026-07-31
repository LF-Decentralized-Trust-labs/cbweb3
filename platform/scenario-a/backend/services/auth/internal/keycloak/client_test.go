// SPDX-License-Identifier: Apache-2.0

package keycloak

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testRealm    = "cbweb3"
	testAudience = "cbweb3-auth"
	testKID      = "test-key"
)

// newJWKSServer starts an httptest server that serves the Keycloak JWKS
// endpoint for testRealm, exposing the public half of key. It returns the
// server (which the caller must Close) so its URL can be used as the client
// BaseURL.
func newJWKSServer(t *testing.T, key *rsa.PrivateKey) *httptest.Server {
	t.Helper()

	eBytes := big.NewInt(int64(key.PublicKey.E)).Bytes()
	jwks := jwksResponse{Keys: []jwksKey{{
		Kid: testKID,
		Kty: "RSA",
		Alg: "RS256",
		Use: "sig",
		N:   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(eBytes),
	}}}

	mux := http.NewServeMux()
	mux.HandleFunc("/realms/"+testRealm+"/protocol/openid-connect/certs",
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(jwks)
		})
	return httptest.NewServer(mux)
}

// signToken mints an RS256 JWT signed by key with the given issuer and audience.
func signToken(t *testing.T, key *rsa.PrivateKey, issuer, audience string) string {
	t.Helper()

	claims := jwt.MapClaims{
		"sub": "user-123",
		"iss": issuer,
		"aud": audience,
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = testKID
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}
	return signed
}

func newTestClient(t *testing.T, baseURL string) Client {
	t.Helper()

	c, err := New(Config{
		BaseURL:  baseURL,
		Realm:    testRealm,
		ClientID: testAudience,
		Audience: testAudience,
	})
	if err != nil {
		t.Fatalf("keycloak.New: %v", err)
	}
	return c
}

func TestValidateToken_AcceptsCorrectIssuerAndAudience(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	srv := newJWKSServer(t, key)
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	issuer := srv.URL + "/realms/" + testRealm
	token := signToken(t, key, issuer, testAudience)

	claims, err := c.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("expected token to be accepted, got error: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Fatalf("expected subject user-123, got %q", claims.Subject)
	}
	if claims.Issuer != issuer {
		t.Fatalf("expected issuer %q, got %q", issuer, claims.Issuer)
	}
}

func TestValidateToken_RejectsWrongIssuer(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	srv := newJWKSServer(t, key)
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	// Correct audience, but an issuer that does not match the realm URL.
	token := signToken(t, key, "https://evil.example.com/realms/"+testRealm, testAudience)

	if _, err := c.ValidateToken(context.Background(), token); err == nil {
		t.Fatal("expected token with wrong issuer to be rejected, but it was accepted")
	}
}

func TestValidateToken_RejectsWrongAudience(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	srv := newJWKSServer(t, key)
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	issuer := srv.URL + "/realms/" + testRealm
	// Correct issuer, but an audience the auth service is not the intended
	// recipient of.
	token := signToken(t, key, issuer, "some-other-client")

	if _, err := c.ValidateToken(context.Background(), token); err == nil {
		t.Fatal("expected token with wrong audience to be rejected, but it was accepted")
	}
}

func TestValidateToken_RejectsUnsignedAlg(t *testing.T) {
	// Guard against alg confusion: a token using an unexpected signing method
	// must be rejected regardless of iss/aud.
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	srv := newJWKSServer(t, key)
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	issuer := srv.URL + "/realms/" + testRealm

	claims := jwt.MapClaims{
		"sub": "user-123",
		"iss": issuer,
		"aud": testAudience,
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	token.Header["kid"] = testKID
	signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("signing none token: %v", err)
	}

	if _, err := c.ValidateToken(context.Background(), signed); err == nil {
		t.Fatal("expected token with 'none' alg to be rejected, but it was accepted")
	} else if !strings.Contains(err.Error(), "invalid token") {
		t.Fatalf("expected invalid token error, got: %v", err)
	}
}
