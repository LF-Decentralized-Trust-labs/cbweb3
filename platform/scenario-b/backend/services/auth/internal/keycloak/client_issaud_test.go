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
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// jwksKeyFor builds a JWKS entry exposing the public half of key under kid.
func jwksKeyFor(key *rsa.PrivateKey, kid string) jwksKey {
	return jwksKey{
		Kid: kid,
		Kty: "RSA",
		Alg: "RS256",
		Use: "sig",
		N:   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
	}
}

// mutableJWKSServer serves a JWKS whose keys can be swapped at runtime, to
// exercise realm signing-key rotation.
type mutableJWKSServer struct {
	*httptest.Server
	mu   sync.Mutex
	body jwksResponse
}

func newMutableJWKSServer(t *testing.T, initial ...jwksKey) *mutableJWKSServer {
	t.Helper()
	s := &mutableJWKSServer{body: jwksResponse{Keys: initial}}
	mux := http.NewServeMux()
	mux.HandleFunc("/realms/"+testRealm+"/protocol/openid-connect/certs",
		func(w http.ResponseWriter, _ *http.Request) {
			s.mu.Lock()
			body := s.body
			s.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(body)
		})
	s.Server = httptest.NewServer(mux)
	return s
}

func (s *mutableJWKSServer) setKeys(keys ...jwksKey) {
	s.mu.Lock()
	s.body = jwksResponse{Keys: keys}
	s.mu.Unlock()
}

// signTokenWithKID mints an RS256 JWT signed by key with an explicit kid header.
// A nil issuer or aud omits that claim entirely (to test absent claims); aud may
// be a string or a []string (Keycloak's normal array encoding).
func signTokenWithKID(t *testing.T, key *rsa.PrivateKey, kid, issuer string, aud any) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	if issuer != "" {
		claims["iss"] = issuer
	}
	if aud != nil {
		claims["aud"] = aud
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}
	return signed
}

func TestValidateToken_AcceptsAudienceAsArray(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	srv := newJWKSServer(t, key)
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	issuer := srv.URL + "/realms/" + testRealm
	// Keycloak encodes "aud" as a JSON array; the expected audience is one entry.
	token := signTokenWithKID(t, key, testKID, issuer, []string{"account", testAudience})

	if _, err := c.ValidateToken(context.Background(), token); err != nil {
		t.Fatalf("expected token with aud array containing %q to be accepted, got: %v", testAudience, err)
	}
}

func TestValidateToken_SkipsAudienceWhenNotConfigured(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	srv := newJWKSServer(t, key)
	defer srv.Close()

	// No Audience configured → the aud check is skipped (issuer still enforced).
	c, err := New(Config{BaseURL: srv.URL, Realm: testRealm})
	if err != nil {
		t.Fatalf("keycloak.New: %v", err)
	}
	issuer := srv.URL + "/realms/" + testRealm
	token := signTokenWithKID(t, key, testKID, issuer, "some-other-client")

	if _, err := c.ValidateToken(context.Background(), token); err != nil {
		t.Fatalf("expected token accepted when audience is unconfigured, got: %v", err)
	}
}

func TestValidateToken_RejectsMissingIssuer(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	srv := newJWKSServer(t, key)
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	// No "iss" claim at all: a configured expected issuer is required, so an
	// absent issuer must be rejected (fails closed).
	token := signTokenWithKID(t, key, testKID, "", testAudience)

	if _, err := c.ValidateToken(context.Background(), token); err == nil {
		t.Fatal("expected token with no issuer to be rejected, but it was accepted")
	}
}

func TestValidateToken_NormalizesTrailingSlashBaseURL(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	srv := newJWKSServer(t, key)
	defer srv.Close()

	// A trailing slash on BaseURL must not corrupt the derived issuer (otherwise
	// "…//realms/…" never matches the "iss" Keycloak emits).
	c, err := New(Config{BaseURL: srv.URL + "/", Realm: testRealm, Audience: testAudience})
	if err != nil {
		t.Fatalf("keycloak.New: %v", err)
	}
	issuer := srv.URL + "/realms/" + testRealm // single slash
	token := signTokenWithKID(t, key, testKID, issuer, testAudience)

	if _, err := c.ValidateToken(context.Background(), token); err != nil {
		t.Fatalf("expected trailing-slash BaseURL normalised and token accepted, got: %v", err)
	}
}

func TestValidateToken_RefetchesJWKSOnKeyRotation(t *testing.T) {
	keyA, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key A: %v", err)
	}
	keyB, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key B: %v", err)
	}
	srv := newMutableJWKSServer(t, jwksKeyFor(keyA, "kid-A"))
	defer srv.Close()

	// A long TTL so the cache does NOT expire on its own: the refetch must be
	// driven by the unknown kid, not by the cache TTL elapsing.
	c, err := New(Config{BaseURL: srv.URL, Realm: testRealm, Audience: testAudience, JWKSCacheTTL: time.Hour})
	if err != nil {
		t.Fatalf("keycloak.New: %v", err)
	}
	issuer := srv.URL + "/realms/" + testRealm

	// Prime the cache with key A.
	tokenA := signTokenWithKID(t, keyA, "kid-A", issuer, testAudience)
	if _, err := c.ValidateToken(context.Background(), tokenA); err != nil {
		t.Fatalf("expected token signed by key A to be accepted, got: %v", err)
	}

	// The realm rotates its signing key: only key B is now published, new kid.
	srv.setKeys(jwksKeyFor(keyB, "kid-B"))
	tokenB := signTokenWithKID(t, keyB, "kid-B", issuer, testAudience)

	// Without the on-miss refetch the cached JWKS (kid-A only) would reject this.
	if _, err := c.ValidateToken(context.Background(), tokenB); err != nil {
		t.Fatalf("expected token signed by rotated key B to be accepted after refetch, got: %v", err)
	}
}
