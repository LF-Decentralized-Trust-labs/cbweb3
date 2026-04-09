package main

import (
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"

	besuAdapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/adapters/besu"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/adapters/cacti"
	paladinAdapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/adapters/paladin"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/repository"
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

	// Interoperability relay: CactiRelay when CACTI_API_URL is set (required in production).
	cactiURL := os.Getenv("CACTI_API_URL")
	if cactiURL == "" {
		log.Fatal("FATAL: CACTI_API_URL is required — set it to the Cacti HTLC relay REST endpoint (e.g. http://cacti-htlc-relay:4000)")
	}
	relay := cacti.NewCactiRelay(cactiURL, logger)
	logger.Info("cacti relay configured", "url", cactiURL)

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

	// Fiat token client: real Besu adapter when FIAT_TOKEN_ADDRESS is set.
	var fiat ports.FiatTokenPort
	if fiatAddr := os.Getenv("FIAT_TOKEN_ADDRESS"); fiatAddr != "" {
		besuRPC := getEnv("BESU_RPC_URL", "")
		operatorKey := os.Getenv("BESU_OPERATOR_KEY")
		chainIDStr := getEnv("BESU_CHAIN_ID", "1337")
		chainID, err := strconv.ParseInt(chainIDStr, 10, 64)
		if err != nil {
			log.Fatalf("FATAL: invalid BESU_CHAIN_ID %q: %v", chainIDStr, err)
		}
		if besuRPC == "" || operatorKey == "" {
			log.Fatal("FATAL: BESU_RPC_URL and BESU_OPERATOR_KEY are required when FIAT_TOKEN_ADDRESS is set")
		}
		fiatClient, err := besuAdapter.NewFiatClient(besuAdapter.FiatClientConfig{
			RPCURL:           besuRPC,
			ChainID:          chainID,
			FiatTokenAddress: fiatAddr,
			PrivateKeyHex:    operatorKey,
		}, logger)
		if err != nil {
			log.Fatalf("FATAL: fiat client: %v", err)
		}
		fiat = fiatClient
		logger.Info("fiat token client configured", "address", fiatAddr)
	} else {
		logger.Warn("FIAT_TOKEN_ADDRESS not set — fCeBM operations disabled")
	}

	// Escrow repository: in-memory for now (production: GORM + PostgreSQL).
	escrowRepo := repository.NewMemoryEscrowRepository()
	logger.Info("escrow repository configured (in-memory)")

	// Extract spoke prefix from PALADIN_IDENTITY for receiver validation.
	// e.g. "funded_operator@spoke-a-bank-a" → "spoke-a"
	spokePrefix := extractSpokePrefix(getEnv("PALADIN_IDENTITY", ""))
	if spokePrefix != "" {
		logger.Info("spoke prefix configured for receiver validation", "spokePrefix", spokePrefix)
	} else {
		logger.Warn("could not extract spoke prefix from PALADIN_IDENTITY — receiver locality check disabled")
	}

	grpcServer := server.New(server.Config{
		Zeto:        zeto,
		HTLC:        htlc,
		Relay:       relay,
		Fiat:        fiat,
		EscrowRepo:  escrowRepo,
		SpokePrefix: spokePrefix,
		Logger:      logger,
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

// extractSpokePrefix extracts the spoke identifier from a Paladin identity.
// e.g. "funded_operator@spoke-a-bank-a" → "spoke-a"
func extractSpokePrefix(identity string) string {
	parts := strings.SplitN(identity, "@", 2)
	if len(parts) < 2 {
		return ""
	}
	segs := strings.SplitN(parts[1], "-", 3)
	if len(segs) < 2 {
		return ""
	}
	return segs[0] + "-" + segs[1]
}
