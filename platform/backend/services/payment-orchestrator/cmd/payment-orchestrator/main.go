package main

import (
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/adapters/cacti"
	paladinAdapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/adapters/paladin"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

func main() {
	port := getEnv("PAYMENT_GRPC_PORT", "9094")

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Zeto operator: real Paladin client or no-op based on config.
	var zeto ports.ZetoOperator
	if paladinURL := os.Getenv("PALADIN_URL"); paladinURL != "" {
		identity := getEnv("PALADIN_IDENTITY", "funded_operator@spoke-a-cb")
		zetoAddr := os.Getenv("ZETO_TOKEN_ADDRESS")
		if zetoAddr == "" {
			log.Fatal("FATAL: ZETO_TOKEN_ADDRESS is required when PALADIN_URL is set")
		}
		zeto = paladinAdapter.NewClient(paladinAdapter.ClientConfig{
			BaseURL:          paladinURL,
			Identity:         identity,
			ZetoTokenAddress: zetoAddr,
		}, logger)
		logger.Info("paladin client configured", "url", paladinURL, "identity", identity, "zetoToken", zetoAddr)
	} else {
		log.Println("WARN: PALADIN_URL not set — Zeto operations will fail (dev placeholder)")
		zeto = noopZeto{}
	}

	// Interoperability relay: stub for now, Cacti in future.
	relay := cacti.NewStubRelay(logger)

	grpcServer := server.New(server.Config{
		Zeto:   zeto,
		Relay:  relay,
		Logger: logger,
	})

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("payment-orchestrator: listen: %v", err)
	}

	logger.Info("payment-orchestrator gRPC listening", "port", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("payment-orchestrator: serve: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
