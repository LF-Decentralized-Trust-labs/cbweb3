package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/bootstrap"
	compliancepki "github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/pki"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/repository"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/registry"
)

func main() {
	port := getEnv("COMPLIANCE_GRPC_PORT", "9093")
	dsn := os.Getenv("POSTGRES_DSN")

	var repo repository.Repository
	if dsn == "" {
		log.Println("WARN: POSTGRES_DSN not set — using in-memory repository (dev mode)")
		repo = repository.NewMemoryRepository()
	} else {
		var err error
		repo, err = repository.NewGormRepository(dsn)
		if err != nil {
			log.Fatalf("compliance: connect postgres: %v", err)
		}
	}

	var ca *compliancepki.CA
	if os.Getenv("CA_CERT_FILE") != "" {
		var err error
		ca, err = compliancepki.NewCAFromEnv()
		if err != nil {
			log.Fatalf("compliance: load CA: %v", err)
		}
		log.Println("compliance: CA loaded from disk")
	} else {
		log.Println("WARN: CA_CERT_FILE not set — certificate issuance disabled (dev mode)")
	}

	bc := newBlockchainClient()

	if ca != nil {
		ctx := context.Background()
		if err := bootstrap.EnsureGovernanceParticipant(ctx, repo, bc, ca); err != nil {
			log.Printf("WARN: governance participant bootstrap: %v", err)
		}
	}

	grpcServer := server.New(repo, ca, bc)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("compliance: listen: %v", err)
	}

	log.Printf("compliance-orchestrator gRPC listening on :%s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("compliance: serve: %v", err)
	}
}

func newBlockchainClient() registry.RegistryWriter {
	switch getEnv("BLOCKCHAIN_CLIENT", "noop") {
	case "besu":
		chainID, _ := strconv.ParseInt(getEnv("BESU_CHAIN_ID", "1337"), 10, 64)
		bc, err := registry.NewBesuClient(registry.BesuConfig{
			RPCURL:          os.Getenv("BESU_RPC_URL"),
			RegistryAddress: os.Getenv("PARTICIPANT_REGISTRY_ADDRESS"),
			CBPrivateKeyHex: os.Getenv("CB_PRIVATE_KEY"),
			ChainID:         chainID,
			RequestTimeout:  time.Duration(15) * time.Second,
		})
		if err != nil {
			log.Printf("WARN: blockchain client init failed: %v — using noop mode", err)
			return registry.NoopRegistryClient{}
		}
		log.Println("compliance: blockchain client connected to", os.Getenv("BESU_RPC_URL"))
		return bc
	default:
		log.Println("compliance: blockchain noop mode (no on-chain writes)")
		return registry.NoopRegistryClient{}
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
