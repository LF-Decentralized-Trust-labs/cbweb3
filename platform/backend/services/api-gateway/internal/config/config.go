// This file loads environment variables into a typed runtime configuration.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// Supported authentication modes.
	ModeMock     = "mock"
	ModeKeycloak = "keycloak"
	ModeHybrid   = "hybrid"
)

// Config holds runtime settings loaded from environment variables.
type Config struct {
	AppPort           string
	AuthMode          string
	TokenTTL          time.Duration
	TokenIssuer       string
	TokenAudience     string
	MockClients       map[string]string
	RequestTimeout    time.Duration
	KeycloakTokenURL  string
	KeycloakIntrospect string
	KeycloakClientID  string
	KeycloakSecret    string
	PostgresDSN       string
}

// Load reads environment variables and returns a fully populated Config.
func Load() Config {
	return Config{
		AppPort:            getEnv("APP_PORT", "8080"),
		AuthMode:           strings.ToLower(getEnv("AUTH_MODE", ModeMock)),
		TokenTTL:           time.Duration(getEnvInt("AUTH_TOKEN_TTL_SEC", 900)) * time.Second,
		TokenIssuer:        getEnv("AUTH_TOKEN_ISSUER", "cbweb3-api-gateway"),
		TokenAudience:      getEnv("AUTH_TOKEN_AUDIENCE", "cbweb3-pilot"),
		MockClients:        parseClientSecrets(getEnv("AUTH_MOCK_CLIENTS", "bank-a:secret-a,bank-b:secret-b")),
		RequestTimeout:     time.Duration(getEnvInt("REQUEST_TIMEOUT_SEC", 5)) * time.Second,
		KeycloakTokenURL:   getEnv("KEYCLOAK_TOKEN_URL", ""),
		KeycloakIntrospect: getEnv("KEYCLOAK_INTROSPECTION_URL", ""),
		KeycloakClientID:   getEnv("KEYCLOAK_CLIENT_ID", ""),
		KeycloakSecret:     getEnv("KEYCLOAK_CLIENT_SECRET", ""),
		PostgresDSN:        getEnv("POSTGRES_DSN", ""),
	}
}

// getEnv returns an environment variable or a fallback if missing/blank.
func getEnv(name, fallback string) string {
	value, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// getEnvInt parses an integer environment variable with fallback behavior.
func getEnvInt(name string, fallback int) int {
	raw := getEnv(name, strconv.Itoa(fallback))
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

// parseClientSecrets parses "client:secret" pairs separated by commas.
func parseClientSecrets(raw string) map[string]string {
	result := make(map[string]string)
	for _, entry := range strings.Split(raw, ",") {
		parts := strings.Split(strings.TrimSpace(entry), ":")
		if len(parts) != 2 {
			continue
		}
		clientID := strings.TrimSpace(parts[0])
		secret := strings.TrimSpace(parts[1])
		if clientID == "" || secret == "" {
			continue
		}
		result[clientID] = secret
	}
	return result
}

