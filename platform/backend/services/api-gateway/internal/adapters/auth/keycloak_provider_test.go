// This file tests Keycloak provider authentication success and failure cases.
package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestKeycloakProviderAuthenticate(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "kc-token",
			"expires_in":   120,
			"token_type":   "Bearer",
		})
	}))
	defer server.Close()

	provider := NewKeycloakAuthProvider(server.URL, time.Second)
	token, err := provider.Authenticate(context.Background(), "client", "secret")
	if err != nil {
		t.Fatalf("expected successful auth, got %v", err)
	}
	if token.AccessToken != "kc-token" {
		t.Fatalf("unexpected token value: %+v", token)
	}
}

func TestKeycloakProviderAuthenticateFail(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	provider := NewKeycloakAuthProvider(server.URL, time.Second)
	if _, err := provider.Authenticate(context.Background(), "client", "wrong"); err == nil {
		t.Fatal("expected invalid credentials error")
	}
}

func TestKeycloakProviderEdgeCases(t *testing.T) {
	t.Parallel()

	providerWithoutURL := NewKeycloakAuthProvider("", time.Second)
	if _, err := providerWithoutURL.Authenticate(context.Background(), "client", "secret"); err == nil {
		t.Fatal("expected error when token URL is empty")
	}

	providerInvalidURL := NewKeycloakAuthProvider("://bad-url", time.Second)
	if _, err := providerInvalidURL.Authenticate(context.Background(), "client", "secret"); err == nil {
		t.Fatal("expected invalid URL error")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{"))
	}))
	defer server.Close()
	providerBadJSON := NewKeycloakAuthProvider(server.URL, time.Second)
	if _, err := providerBadJSON.Authenticate(context.Background(), "client", "secret"); err == nil {
		t.Fatal("expected JSON decode error")
	}
}

