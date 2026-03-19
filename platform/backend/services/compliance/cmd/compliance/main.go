package main

import (
	"fmt"
	"log"
	"net"
	"os"

	compliancepki "github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/pki"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/repository"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/grpc/server"
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

	grpcServer := server.New(repo, ca)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("compliance: listen: %v", err)
	}

	log.Printf("compliance-orchestrator gRPC listening on :%s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("compliance: serve: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
