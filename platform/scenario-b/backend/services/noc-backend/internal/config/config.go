// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration for noc-backend.
type Config struct {
	DatabaseURL          string
	Port                 string
	KeycloakURL          string
	KeycloakRealm        string
	KeycloakClientID     string
	KeycloakClientSecret string
	JWKSCacheTTL         time.Duration
	AgentGraceMultiplier int
	FrontendOrigin       string
	// SkipAuth disables Keycloak JWT validation for local development.
	// Set NOC_SKIP_AUTH=true — NEVER use in production.
	SkipAuth bool
	// AMMGatewayURL is the base URL of the api-gateway exposing AMM pool status.
	AMMGatewayURL string
	// AMMPairs is the list of pool pairs to proxy from the AMM gateway.
	AMMPairs []string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:          getEnv("DATABASE_URL", ""),
		Port:                 getEnv("NOC_BACKEND_PORT", "8090"),
		KeycloakURL:          getEnv("KEYCLOAK_URL", "http://keycloak:8080"),
		KeycloakRealm:        getEnv("KEYCLOAK_REALM", "cbweb3"),
		KeycloakClientID:     getEnv("KEYCLOAK_CLIENT_ID", "noc-portal"),
		KeycloakClientSecret: getEnv("KEYCLOAK_CLIENT_SECRET", ""),
		JWKSCacheTTL:         5 * time.Minute,
		FrontendOrigin:       getEnv("NOC_FRONTEND_ORIGIN", "http://localhost:5173"),
		SkipAuth:             getEnv("NOC_SKIP_AUTH", "") == "true",
		AMMGatewayURL:        getEnv("AMM_GATEWAY_URL", "http://api-gateway-cb-a:8080"),
		AMMPairs:             parsePairs(getEnv("AMM_PAIRS", "W-BRL-ARS")),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("config: DATABASE_URL is required")
	}

	mult, err := strconv.Atoi(getEnv("AGENT_GRACE_MULTIPLIER", "3"))
	if err != nil || mult < 1 {
		mult = 3
	}
	cfg.AgentGraceMultiplier = mult

	return cfg, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parsePairs(raw string) []string {
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
