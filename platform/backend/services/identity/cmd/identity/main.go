package main

import (
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/dataaccessclient"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/keycloak"
	kmsproviders "github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/kms/providers"
)

func main() {
	kmsProvider, err := kmsproviders.New(kmsproviders.Config{
		Provider: getEnv("KMS_PROVIDER", "local"),
	})
	if err != nil {
		log.Fatalf("kms: %v", err)
	}

	kcClient, err := keycloak.New(keycloak.Config{
		BaseURL:        mustEnv("KEYCLOAK_BASE_URL"),
		Realm:          getEnv("KEYCLOAK_REALM", "cbweb3"),
		ClientID:       getEnv("KEYCLOAK_CLIENT_ID", "cbweb3-identity"),
		ClientSecret:   getEnv("KEYCLOAK_CLIENT_SECRET", ""),
		JWKSCacheTTL:   time.Duration(getEnvInt("KEYCLOAK_JWKS_CACHE_TTL_SEC", 300)) * time.Second,
		RequestTimeout: time.Duration(getEnvInt("KEYCLOAK_REQUEST_TIMEOUT_SEC", 10)) * time.Second,
	})
	if err != nil {
		log.Fatalf("keycloak: %v", err)
	}

	dataAccess, err := dataaccessclient.New(
		getEnv("DATA_ACCESS_GRPC_ADDR", "localhost:9092"),
		time.Duration(getEnvInt("DATA_ACCESS_REQUEST_TIMEOUT_SEC", 5))*time.Second,
	)
	if err != nil {
		log.Fatalf("data-access: %v", err)
	}

	grpcServer := server.New(kcClient, kmsProvider, dataAccess)

	port := getEnv("IDENTITY_GRPC_PORT", "9091")
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	log.Printf("identity gRPC server starting on :%s [kms=%s keycloak_realm=%s]",
		port, kmsProvider.Name(), getEnv("KEYCLOAK_REALM", "cbweb3"))

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required env var %q is not set", key)
	}
	return v
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
