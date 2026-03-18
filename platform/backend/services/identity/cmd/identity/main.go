package main

import (
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/blockchain"
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

	blockchainClient := newBlockchainClient()

	grpcServer := server.New(kcClient, kmsProvider, dataAccess, blockchainClient)

	port := getEnv("IDENTITY_GRPC_PORT", "9091")
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	log.Printf("identity gRPC server starting on :%s [kms=%s keycloak_realm=%s blockchain=%s]",
		port, kmsProvider.Name(), getEnv("KEYCLOAK_REALM", "cbweb3"), getEnv("BLOCKCHAIN_CLIENT", "noop"))

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

// newBlockchainClient constructs the blockchain client based on the
// BLOCKCHAIN_CLIENT environment variable (default: "noop").
// When set to "besu", BESU_RPC_URL, PARTICIPANT_REGISTRY_ADDRESS, CB_PRIVATE_KEY,
// and BESU_CHAIN_ID must also be set.
func newBlockchainClient() blockchain.Client {
	switch getEnv("BLOCKCHAIN_CLIENT", "noop") {
	case "besu":
		chainID := int64(getEnvInt("BESU_CHAIN_ID", 1337))
		bc, err := blockchain.NewBesuClient(blockchain.BesuConfig{
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
		return blockchain.NoopClient{}
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
