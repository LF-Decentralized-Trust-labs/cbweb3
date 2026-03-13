// This file loads environment variables into a typed runtime configuration.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime settings loaded from environment variables.
type Config struct {
	AppPort          string
	RequestTimeout   time.Duration
	IdentityGRPCAddr string
}

// Load reads environment variables and returns a fully populated Config.
func Load() Config {
	return Config{
		AppPort:          getEnv("APP_PORT", "8080"),
		RequestTimeout:   time.Duration(getEnvInt("REQUEST_TIMEOUT_SEC", 5)) * time.Second,
		IdentityGRPCAddr: getEnv("IDENTITY_GRPC_ADDR", "localhost:9091"),
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
