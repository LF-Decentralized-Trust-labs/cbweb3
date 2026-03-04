// This file tests Keycloak token introspection and claims parsing behavior.
package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestKeycloakValidatorValidate(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"active": true,
			"sub":    "bank-a",
			"iss":    "http://keycloak",
			"scope":  "openid profile",
			"aud":    []string{"cbweb3-pilot"},
		})
	}))
	defer server.Close()

	validator := NewKeycloakTokenValidator(server.URL, "client", "secret", time.Second)
	claims, err := validator.Validate(context.Background(), "token")
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if claims.Subject != "bank-a" {
		t.Fatalf("expected bank-a subject, got %s", claims.Subject)
	}
}

func TestKeycloakValidatorInactiveToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"active": false,
		})
	}))
	defer server.Close()

	validator := NewKeycloakTokenValidator(server.URL, "client", "secret", time.Second)
	if _, err := validator.Validate(context.Background(), "token"); err == nil {
		t.Fatal("expected inactive token error")
	}
}

func TestKeycloakValidatorEdgeCases(t *testing.T) {
	t.Parallel()

	validatorNoURL := NewKeycloakTokenValidator("", "client", "secret", time.Second)
	if _, err := validatorNoURL.Validate(context.Background(), "token"); err == nil {
		t.Fatal("expected error without introspection URL")
	}

	validatorInvalidURL := NewKeycloakTokenValidator("://bad-url", "client", "secret", time.Second)
	if _, err := validatorInvalidURL.Validate(context.Background(), "token"); err == nil {
		t.Fatal("expected invalid URL error")
	}

	notOKServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer notOKServer.Close()
	validatorStatus := NewKeycloakTokenValidator(notOKServer.URL, "client", "secret", time.Second)
	if _, err := validatorStatus.Validate(context.Background(), "token"); err == nil {
		t.Fatal("expected unauthorized status error")
	}

	badJSONServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{"))
	}))
	defer badJSONServer.Close()
	validatorJSON := NewKeycloakTokenValidator(badJSONServer.URL, "client", "secret", time.Second)
	if _, err := validatorJSON.Validate(context.Background(), "token"); err == nil {
		t.Fatal("expected JSON decode error")
	}

	audString := audiencesToRoles("aud1")
	if len(audString) != 1 || audString[0] != "aud1" {
		t.Fatalf("unexpected audience conversion for string: %v", audString)
	}
	audSlice := audiencesToRoles([]interface{}{"a", float64(2)})
	if len(audSlice) != 2 || audSlice[1] != "2" {
		t.Fatalf("unexpected audience conversion for slice: %v", audSlice)
	}
}

