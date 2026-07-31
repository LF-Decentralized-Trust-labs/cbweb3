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
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testRealm    = "cbweb3"
	testKID      = "test-key"
	testAudience = "noc-portal"
)

// jwksKeyFor builds a JWKS entry exposing the public half of key under kid.
func jwksKeyFor(key *rsa.PrivateKey, kid string) jwksKey {
	return jwksKey{
		Kid: kid,
		Kty: "RSA",
		N:   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
	}
}

// jwksServer serves a JWKS whose keys can be swapped at runtime, exercising
// realm signing-key rotation.
type jwksServer struct {
	*httptest.Server
	mu   sync.Mutex
	body jwksResponse
}

func newJWKSServer(t *testing.T, keys ...jwksKey) *jwksServer {
	t.Helper()
	s := &jwksServer{body: jwksResponse{Keys: keys}}
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

func (s *jwksServer) setKeys(keys ...jwksKey) {
	s.mu.Lock()
	s.body = jwksResponse{Keys: keys}
	s.mu.Unlock()
}

// signToken mints an RS256 JWT signed by key with an explicit kid. A nil issuer
// or aud omits that claim; aud may be a string or []string; roles populate
// realm_access.roles.
func signToken(t *testing.T, key *rsa.PrivateKey, kid, issuer string, aud any, roles []string) string {
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
	if roles != nil {
		claims["realm_access"] = map[string]any{"roles": roles}
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}
	return signed
}

func newClientT(t *testing.T, cfg Config) Client {
	t.Helper()
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("keycloak.New: %v", err)
	}
	return c
}

func TestValidateToken_AcceptsCorrectIssuerAndExtractsRoles(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	srv := newJWKSServer(t, jwksKeyFor(key, testKID))
	defer srv.Close()

	c := newClientT(t, Config{BaseURL: srv.URL, Realm: testRealm})
	issuer := srv.URL + "/realms/" + testRealm
	token := signToken(t, key, testKID, issuer, "account", []string{"ROLE_NOC_ADMIN", "ROLE_NOC_VIEWER"})

	claims, err := c.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("expected token accepted, got: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Fatalf("expected subject user-123, got %q", claims.Subject)
	}
	if len(claims.Roles) != 2 || claims.Roles[0] != "ROLE_NOC_ADMIN" {
		t.Fatalf("expected realm roles extracted, got %v", claims.Roles)
	}
}

func TestValidateToken_RejectsWrongIssuer(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	srv := newJWKSServer(t, jwksKeyFor(key, testKID))
	defer srv.Close()

	c := newClientT(t, Config{BaseURL: srv.URL, Realm: testRealm})
	// A token signed with the realm's key but minted for a DIFFERENT issuer — the
	// cross-realm token-reuse this fix closes.
	token := signToken(t, key, testKID, "https://evil.example.com/realms/"+testRealm, "account", nil)

	if _, err := c.ValidateToken(context.Background(), token); err == nil {
		t.Fatal("expected token with wrong issuer to be rejected, but it was accepted")
	}
}

func TestValidateToken_RejectsMissingIssuer(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	srv := newJWKSServer(t, jwksKeyFor(key, testKID))
	defer srv.Close()

	c := newClientT(t, Config{BaseURL: srv.URL, Realm: testRealm})
	token := signToken(t, key, testKID, "", "account", nil) // no iss claim

	if _, err := c.ValidateToken(context.Background(), token); err == nil {
		t.Fatal("expected token with no issuer to be rejected, but it was accepted")
	}
}

func TestValidateToken_RejectsUnsignedAlg(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	srv := newJWKSServer(t, jwksKeyFor(key, testKID))
	defer srv.Close()

	c := newClientT(t, Config{BaseURL: srv.URL, Realm: testRealm})
	issuer := srv.URL + "/realms/" + testRealm
	claims := jwt.MapClaims{
		"sub": "user-123",
		"iss": issuer,
		"aud": "account",
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

func TestValidateToken_EnforcesAudienceWhenConfigured(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	srv := newJWKSServer(t, jwksKeyFor(key, testKID))
	defer srv.Close()

	c := newClientT(t, Config{BaseURL: srv.URL, Realm: testRealm, Audience: testAudience})
	issuer := srv.URL + "/realms/" + testRealm

	// Correct issuer, wrong audience → rejected.
	wrong := signToken(t, key, testKID, issuer, "some-other-client", nil)
	if _, err := c.ValidateToken(context.Background(), wrong); err == nil {
		t.Fatal("expected token with wrong audience to be rejected, but it was accepted")
	}

	// Correct issuer and audience (as an array, Keycloak's normal encoding) → ok.
	ok := signToken(t, key, testKID, issuer, []string{"account", testAudience}, nil)
	if _, err := c.ValidateToken(context.Background(), ok); err != nil {
		t.Fatalf("expected token with matching audience accepted, got: %v", err)
	}
}

func TestValidateToken_SkipsAudienceWhenNotConfigured(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	srv := newJWKSServer(t, jwksKeyFor(key, testKID))
	defer srv.Close()

	// No Audience configured → aud check skipped, issuer still enforced.
	c := newClientT(t, Config{BaseURL: srv.URL, Realm: testRealm})
	issuer := srv.URL + "/realms/" + testRealm
	token := signToken(t, key, testKID, issuer, "some-other-client", nil)

	if _, err := c.ValidateToken(context.Background(), token); err != nil {
		t.Fatalf("expected token accepted when audience is unconfigured, got: %v", err)
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
	srv := newJWKSServer(t, jwksKeyFor(keyA, "kid-A"))
	defer srv.Close()

	// Long TTL: the refetch must be driven by the unknown kid, not by TTL expiry.
	c := newClientT(t, Config{BaseURL: srv.URL, Realm: testRealm, JWKSCacheTTL: time.Hour})
	issuer := srv.URL + "/realms/" + testRealm

	tokenA := signToken(t, keyA, "kid-A", issuer, "account", nil)
	if _, err := c.ValidateToken(context.Background(), tokenA); err != nil {
		t.Fatalf("expected token signed by key A accepted, got: %v", err)
	}

	srv.setKeys(jwksKeyFor(keyB, "kid-B")) // realm rotates its signing key
	tokenB := signToken(t, keyB, "kid-B", issuer, "account", nil)
	if _, err := c.ValidateToken(context.Background(), tokenB); err != nil {
		t.Fatalf("expected token signed by rotated key B accepted after refetch, got: %v", err)
	}
}
