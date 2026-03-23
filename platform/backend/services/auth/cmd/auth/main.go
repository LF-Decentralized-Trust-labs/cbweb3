package main

import (
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/registry"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/complianceclient"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/keycloak"
	kmsproviders "github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/kms/providers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/noncestore"
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
		ClientID:       getEnv("KEYCLOAK_CLIENT_ID", "cbweb3-auth"),
		ClientSecret:   getEnv("KEYCLOAK_CLIENT_SECRET", ""),
		JWKSCacheTTL:   time.Duration(getEnvInt("KEYCLOAK_JWKS_CACHE_TTL_SEC", 300)) * time.Second,
		RequestTimeout: time.Duration(getEnvInt("KEYCLOAK_REQUEST_TIMEOUT_SEC", 10)) * time.Second,
	})
	if err != nil {
		log.Fatalf("keycloak: %v", err)
	}

	complianceClient, err := complianceclient.New(
		getEnv("COMPLIANCE_GRPC_ADDR", "localhost:9093"),
		time.Duration(getEnvInt("COMPLIANCE_REQUEST_TIMEOUT_SEC", 5))*time.Second,
	)
	if err != nil {
		log.Fatalf("compliance: %v", err)
	}

	caCertPEM := os.Getenv("CA_CERT_PEM") // PEM string injected directly (optional)

	blockchainClient := newBlockchainClient()

	ns := newNonceStore()

	grpcServer := server.New(kcClient, kmsProvider, complianceClient, blockchainClient, caCertPEM, ns)

	port := getEnv("AUTH_GRPC_PORT", "9091")
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	log.Printf("auth gRPC server starting on :%s [kms=%s keycloak_realm=%s blockchain=%s]",
		port, kmsProvider.Name(), getEnv("KEYCLOAK_REALM", "cbweb3"), getEnv("BLOCKCHAIN_CLIENT", "noop"))

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

// newNonceStore returns a Redis-backed NonceStore when REDIS_ADDR is set,
// otherwise falls back to the in-memory store (dev only).
func newNonceStore() noncestore.NonceStore {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		log.Println("nonce store: REDIS_ADDR not set, using in-memory store (not suitable for production)")
		return noncestore.NewInMemoryStore()
	}
	log.Printf("nonce store: connecting to Redis at %s", addr)
	return noncestore.NewRedisStore(addr, os.Getenv("REDIS_PASSWORD"), 0)
}

// blockchainRegistry combines read and write access for use in auth-service.
type blockchainRegistryClient interface {
	registry.RegistryWriter
	registry.RegistryReader
}

// newBlockchainClient constructs the blockchain client based on the
// BLOCKCHAIN_CLIENT environment variable (default: "noop").
// When set to "besu", BESU_RPC_URL, PARTICIPANT_REGISTRY_ADDRESS, CB_PRIVATE_KEY,
// and BESU_CHAIN_ID must also be set.
func newBlockchainClient() blockchainRegistryClient {
	switch getEnv("BLOCKCHAIN_CLIENT", "noop") {
	case "besu":
		chainID := int64(getEnvInt("BESU_CHAIN_ID", 1337))
		bc, err := registry.NewBesuClient(registry.BesuConfig{
			RPCURL:          mustEnv("BESU_RPC_URL"),
			RegistryAddress: mustEnv("PARTICIPANT_REGISTRY_ADDRESS"),
			CBPrivateKeyHex: mustEnv("CB_PRIVATE_KEY"),
			ChainID:         chainID,
			RequestTimeout:  time.Duration(getEnvInt("BLOCKCHAIN_REQUEST_TIMEOUT_SEC", 15)) * time.Second,
		})
		if err != nil {
			log.Fatalf("blockchain: %v", err)
		}
		return bc
	default:
		log.Println("blockchain: using noop client (no on-chain registration)")
		return registry.NoopRegistryClient{}
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
