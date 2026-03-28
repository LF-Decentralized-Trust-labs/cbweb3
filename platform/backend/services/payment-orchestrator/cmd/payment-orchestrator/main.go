package main

import (
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"strconv"

	besuAdapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/adapters/besu"
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

	// On-chain HTLC coordination (optional — requires BESU_RPC_URL + HTLC_ADDRESS).
	var htlc ports.HTLCContractPort
	if besuRPC := os.Getenv("BESU_RPC_URL"); besuRPC != "" {
		htlcAddr := os.Getenv("HTLC_ADDRESS")
		operatorKey := os.Getenv("BESU_OPERATOR_KEY")
		chainIDStr := getEnv("BESU_CHAIN_ID", "1337")
		chainID, err := strconv.ParseInt(chainIDStr, 10, 64)
		if err != nil {
			log.Fatalf("FATAL: invalid BESU_CHAIN_ID %q: %v", chainIDStr, err)
		}
		if htlcAddr == "" || operatorKey == "" {
			log.Fatal("FATAL: HTLC_ADDRESS and BESU_OPERATOR_KEY are required when BESU_RPC_URL is set")
		}
		besuClient, err := besuAdapter.NewClient(besuAdapter.ClientConfig{
			RPCURL:        besuRPC,
			ChainID:       chainID,
			HTLCAddress:   htlcAddr,
			PrivateKeyHex: operatorKey,
		}, logger)
		if err != nil {
			log.Fatalf("FATAL: besu client: %v", err)
		}
		htlc = besuClient
		logger.Info("besu HTLC client configured", "rpc", besuRPC, "htlcAddress", htlcAddr, "chainID", chainID)
	} else {
		logger.Warn("BESU_RPC_URL not set — on-chain HTLC coordination disabled")
	}

	grpcServer := server.New(server.Config{
		Zeto:   zeto,
		HTLC:   htlc,
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
