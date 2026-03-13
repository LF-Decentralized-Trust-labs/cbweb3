package main

import (
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/data-access/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/data-access/internal/repository"
)

func main() {
	repo, err := newParticipantsRepository()
	if err != nil {
		log.Fatalf("failed to initialize participants repository: %v", err)
	}

	port := getEnv("DATA_ACCESS_GRPC_PORT", "9092")
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("failed to listen on :%s: %v", port, err)
	}

	grpcServer := server.New(repo)
	log.Printf("data-access gRPC listening on :%s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("data-access server stopped with error: %v", err)
	}
}

func newParticipantsRepository() (repository.ParticipantsRepository, error) {
	dsn := strings.TrimSpace(getEnv("POSTGRES_DSN", ""))
	if dsn == "" {
		return repository.NewMemoryParticipantsRepository(), nil
	}
	return repository.NewGormParticipantsRepositoryWithConfig(dsn, repository.PostgresConfig{
		MaxOpenConns:    getEnvInt("DATA_ACCESS_DB_MAX_OPEN_CONNS", 10),
		MaxIdleConns:    getEnvInt("DATA_ACCESS_DB_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime: time.Duration(getEnvInt("DATA_ACCESS_DB_CONN_MAX_LIFETIME_SEC", 300)) * time.Second,
	})
}

func getEnv(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
