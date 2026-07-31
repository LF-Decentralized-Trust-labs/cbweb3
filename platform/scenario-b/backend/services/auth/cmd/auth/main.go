// SPDX-License-Identifier: Apache-2.0

package main

import (
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/blockchain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/complianceclient"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/keycloak"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/kms"
	kmsproviders "github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/kms/providers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/noncestore"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/registry"
)

func main() {
	kmsProvider, err := kmsproviders.New(kmsproviders.Config{
		Provider: getEnv("KMS_PROVIDER", "local"),
	})
	if err != nil {
		log.Fatalf("kms: %v", err)
	}

	// Pre-seed the KMS with a known operator key so that CreateOnboardingKey returns
	// a deterministic address (the bank's BESU_OPERATOR_KEY address) instead of a
	// random ephemeral one. This ensures bridge-out mints land at the same address
	// that GetBalance queries. Only applies to KMSLocal (dev); silently ignored otherwise.
	if keyID := os.Getenv("KMS_SEED_KEY_ID"); keyID != "" {
		if privKey := os.Getenv("KMS_SEED_PRIVATE_KEY"); privKey != "" {
			if local, ok := kmsProvider.(*kmsproviders.KMSLocal); ok {
				if seedErr := local.SeedKey(keyID, privKey); seedErr != nil {
					log.Fatalf("kms seed: %v", seedErr)
				}
				log.Printf("kms: seeded key for %q (KMS_SEED_KEY_ID)", keyID)
			}
		}
	}

	kcBaseURL := mustEnv("KEYCLOAK_BASE_URL")
	kcRealm := getEnv("KEYCLOAK_REALM", "cbweb3")
	// Audience is opt-in: the "aud" check runs only when KEYCLOAK_AUDIENCE is set
	// explicitly. There is no client-id fallback, because Keycloak does not stamp
	// the client id into "aud" without an audience mapper, so defaulting would
	// reject every real token. The issuer is always derived and enforced.
	kcAudience := getEnv("KEYCLOAK_AUDIENCE", "")
	kcClient, err := keycloak.New(keycloak.Config{
		BaseURL:        kcBaseURL,
		Realm:          kcRealm,
		ClientID:       getEnv("KEYCLOAK_CLIENT_ID", "cbweb3-auth"),
		ClientSecret:   getEnv("KEYCLOAK_CLIENT_SECRET", ""),
		Audience:       kcAudience,
		JWKSCacheTTL:   time.Duration(getEnvInt("KEYCLOAK_JWKS_CACHE_TTL_SEC", 300)) * time.Second,
		RequestTimeout: time.Duration(getEnvInt("KEYCLOAK_REQUEST_TIMEOUT_SEC", 10)) * time.Second,
	})
	if err != nil {
		log.Fatalf("keycloak: %v", err)
	}
	kcIssuer := strings.TrimRight(kcBaseURL, "/") + "/realms/" + kcRealm
	if kcAudience == "" {
		log.Printf("keycloak: token validation — issuer=%q, audience enforcement DISABLED (KEYCLOAK_AUDIENCE unset)", kcIssuer)
	} else {
		log.Printf("keycloak: token validation — issuer=%q, audience=%q", kcIssuer, kcAudience)
	}

	complianceClient, err := complianceclient.New(
		getEnv("COMPLIANCE_GRPC_ADDR", "localhost:9093"),
		time.Duration(getEnvInt("COMPLIANCE_REQUEST_TIMEOUT_SEC", 5))*time.Second,
	)
	if err != nil {
		log.Fatalf("compliance: %v", err)
	}

	caCertPEM := os.Getenv("CA_CERT_PEM")

	blockchainClient := newBlockchainClient(kmsProvider)

	ns := newNonceStore()

	grpcServer, err := server.New(kcClient, kmsProvider, complianceClient, blockchainClient, caCertPEM, ns)
	if err != nil {
		log.Fatalf("configure gRPC server: %v", err)
	}

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

func newNonceStore() noncestore.NonceStore {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		log.Println("nonce store: REDIS_ADDR not set, using in-memory store (not suitable for production)")
		return noncestore.NewInMemoryStore()
	}
	log.Printf("nonce store: connecting to Redis at %s (db=%d)", addr, getEnvInt("REDIS_DB", 0))
	return noncestore.NewRedisStore(addr, os.Getenv("REDIS_PASSWORD"), getEnvInt("REDIS_DB", 0))
}

type blockchainRegistryClient interface {
	registry.RegistryWriter
	registry.RegistryReader
}

// newBlockchainClient constructs the blockchain client based on BLOCKCHAIN_CLIENT.
//
// When set to "besu", two modes are supported:
//   - Central Bank: CB_PRIVATE_KEY is set → uses StaticKeySigner for on-chain writes.
//   - Commercial Bank: CB_PRIVATE_KEY absent → uses KMSSigner backed by the
//     participant's key in the KMS (created during onboarding). The signer
//     user ID is resolved per-request from the context; callers must set it
//     with blockchain.WithSignerUserID before invoking write operations.
func newBlockchainClient(kmsProvider kms.Provider) blockchainRegistryClient {
	switch getEnv("BLOCKCHAIN_CLIENT", "noop") {
	case "besu":
		cfg := registry.BesuConfig{
			RPCURL:          mustEnv("BESU_RPC_URL"),
			RegistryAddress: mustEnv("PARTICIPANT_REGISTRY_ADDRESS"),
			ChainID:         int64(getEnvInt("BESU_CHAIN_ID", 1337)),
			RequestTimeout:  time.Duration(getEnvInt("BLOCKCHAIN_REQUEST_TIMEOUT_SEC", 15)) * time.Second,
		}

		var signer registry.TransactionSigner

		if cbKey := os.Getenv("CB_PRIVATE_KEY"); cbKey != "" {
			s, err := registry.NewStaticKeySigner(cbKey)
			if err != nil {
				log.Fatalf("blockchain: invalid CB_PRIVATE_KEY: %v", err)
			}
			signer = s
			log.Println("blockchain: central bank mode (static key signer)")
		} else {
			signer = &blockchain.KMSSigner{KMS: kmsProvider}
			log.Println("blockchain: commercial bank mode (KMS signer, per-request user ID from context)")
		}

		bc, err := registry.NewBesuClient(cfg, signer)
		if err != nil {
			log.Fatalf("blockchain: %v", err)
		}
		return bc
	default:
		log.Println("blockchain: using noop client (no on-chain operations)")
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
