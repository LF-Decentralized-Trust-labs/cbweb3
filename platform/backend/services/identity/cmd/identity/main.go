package main

import (
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/dataaccessclient"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/grpc/server"
	identityproviders "github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/identityprovider/providers"
	kmsproviders "github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/kmsprovider/providers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/tokenissuer"
)

func main() {
	cfg := identityproviders.FactoryConfig{
		Provider:       strings.ToLower(getEnv("IDENTITY_PROVIDER", identityproviders.ProviderLocal)),
		HostURL:        getEnv("IDENTITY_HOST_URL", getEnv("IDENTITY_DWALLET_API_URL", getEnv("IDENTITY_REMOTE_BASE_URL", getEnv("DWALLET_REMOTE_BASE_URL", "")))),
		JWTSecret:      getEnv("IDENTITY_JWT_SECRET", getEnv("DWALLET_JWT_SECRET", "local-identity-secret")),
		AccessTokenTTL: time.Duration(getEnvInt("IDENTITY_ACCESS_TOKEN_TTL_SEC", getEnvInt("DWALLET_ACCESS_TOKEN_TTL_SEC", 3600))) * time.Second,
		RequestTimeout: time.Duration(getEnvInt("IDENTITY_REQUEST_TIMEOUT_SEC", 5)) * time.Second,
	}

	identityProvider, err := identityproviders.NewIdentityProvider(cfg)
	if err != nil {
		log.Fatalf("failed to create identity provider: %v", err)
	}
	kms, err := kmsproviders.New(getEnv("IDENTITY_KMS_PROVIDER", kmsproviders.ProviderLocalKMS))
	if err != nil {
		log.Fatalf("failed to create kms provider: %v", err)
	}
	dataAccess, err := dataaccessclient.New(
		getEnv("DATA_ACCESS_GRPC_ADDR", "localhost:9092"),
		time.Duration(getEnvInt("DATA_ACCESS_REQUEST_TIMEOUT_SEC", 5))*time.Second,
	)
	if err != nil {
		log.Fatalf("failed to create data-access client: %v", err)
	}

	internalTokenIssuer := newTokenIssuer()

	port := getEnv("IDENTITY_GRPC_PORT", "9091")
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("failed to listen on port %s: %v", port, err)
	}

	grpcServer := server.New(identityProvider, dataAccess, kms, internalTokenIssuer)
	log.Printf(
		"identity gRPC listening on :%s (provider=%s, kms=%s, jwt=%s)",
		port,
		identityProvider.Name(),
		kms.Name(),
		internalTokenIssuer.Name(),
	)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("identity gRPC stopped with error: %v", err)
	}
}

func getEnv(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(name string, fallback int) int {
	raw := getEnv(name, "")
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}

func newTokenIssuer() tokenissuer.Issuer {
	mode := strings.ToLower(strings.TrimSpace(getEnv("INTERNAL_JWT_PROVIDER", "local")))
	switch mode {
	case "keycloak":
		return tokenissuer.NewKeycloakIssuer(
			getEnv("INTERNAL_KEYCLOAK_TOKEN_URL", ""),
			getEnv("INTERNAL_KEYCLOAK_CLIENT_ID", ""),
			getEnv("INTERNAL_KEYCLOAK_CLIENT_SECRET", ""),
			time.Duration(getEnvInt("INTERNAL_KEYCLOAK_TIMEOUT_SEC", 5))*time.Second,
		)
	default:
		return tokenissuer.NewLocalIssuer(
			getEnv("INTERNAL_JWT_SECRET", "identity-internal-secret"),
			getEnv("INTERNAL_JWT_ISSUER", "identity-internal"),
			getEnv("INTERNAL_JWT_AUDIENCE", "cbweb3-internal"),
			time.Duration(getEnvInt("INTERNAL_JWT_TTL_SEC", 3600))*time.Second,
		)
	}
}
