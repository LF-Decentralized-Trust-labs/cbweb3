// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	besuAdapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/adapters/besu"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/adapters/cacti"
	dbinit "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/db/init"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/repository"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/workers"
	gormpg "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	port := getEnv("PAYMENT_GRPC_PORT", "9094")

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Interoperability relay.
	cactiURL := os.Getenv("CACTI_API_URL")
	if cactiURL == "" {
		log.Fatal("FATAL: CACTI_API_URL is required — set it to the Cacti HTLC relay REST endpoint (e.g. http://cacti-htlc-relay:4000)")
	}
	relay := cacti.NewCactiRelay(cactiURL, logger)
	logger.Info("cacti relay configured", "url", cactiURL)

	// Shared Besu config — used by HTLC, tCeBM token, and FXAgreement adapters.
	besuRPC := os.Getenv("BESU_RPC_URL")
	operatorKey := os.Getenv("BESU_OPERATOR_KEY")
	if operatorKey == "" {
		if cbKey := os.Getenv("CB_PRIVATE_KEY"); cbKey != "" {
			operatorKey = cbKey
			logger.Warn("BESU_OPERATOR_KEY not set; falling back to CB_PRIVATE_KEY")
		}
	}
	chainIDStr := getEnv("BESU_CHAIN_ID", "1337")
	var chainID int64
	if besuRPC != "" {
		var err error
		chainID, err = strconv.ParseInt(chainIDStr, 10, 64)
		if err != nil {
			log.Fatalf("FATAL: invalid BESU_CHAIN_ID %q: %v", chainIDStr, err)
		}
		if operatorKey == "" {
			log.Fatal("FATAL: BESU_OPERATOR_KEY is required when BESU_RPC_URL is set")
		}
	}

	// tCeBM token client (required for escrow/redeem mint operations).
	var token ports.TCeBMPort
	if tokenAddr := os.Getenv("TOKEN_ADDRESS"); tokenAddr != "" {
		if besuRPC == "" {
			log.Fatal("FATAL: BESU_RPC_URL is required when TOKEN_ADDRESS is set")
		}
		tokenClient, err := besuAdapter.NewTCeBMClient(besuAdapter.TCeBMClientConfig{
			RPCURL:        besuRPC,
			ChainID:       chainID,
			TokenAddress:  tokenAddr,
			PrivateKeyHex: operatorKey,
		}, logger)
		if err != nil {
			log.Fatalf("FATAL: tCeBM client: %v", err)
		}
		token = tokenClient
		logger.Info("tCeBM token client configured", "address", tokenAddr)
	} else {
		logger.Warn("TOKEN_ADDRESS not set — tCeBM mint/burn operations disabled")
	}

	// fCeBM token client (required for deposit approval — mints fCeBM to the bank).
	var fiatToken ports.FiatTokenPort
	if fiatTokenAddr := os.Getenv("FIAT_TOKEN_ADDRESS"); fiatTokenAddr != "" {
		if besuRPC == "" {
			log.Fatal("FATAL: BESU_RPC_URL is required when FIAT_TOKEN_ADDRESS is set")
		}
		fiatClient, err := besuAdapter.NewFiatTokenClient(besuAdapter.FiatTokenClientConfig{
			RPCURL:           besuRPC,
			ChainID:          chainID,
			FiatTokenAddress: fiatTokenAddr,
			PrivateKeyHex:    operatorKey,
		}, logger)
		if err != nil {
			log.Fatalf("FATAL: fCeBM client: %v", err)
		}
		fiatToken = fiatClient
		logger.Info("fCeBM token client configured", "address", fiatTokenAddr)
	} else {
		logger.Warn("FIAT_TOKEN_ADDRESS not set — fCeBM mint/burn operations disabled")
	}

	// On-chain FXAgreement coordination (optional — Besu only).
	var fxAgreementBesu ports.FXAgreementContractPort
	if fxAddr := os.Getenv("FX_AGREEMENT_ADDRESS"); fxAddr != "" && besuRPC != "" {
		fxClient, err := besuAdapter.NewFXAgreementClient(besuAdapter.FXAgreementClientConfig{
			RPCURL:             besuRPC,
			ChainID:            chainID,
			FXAgreementAddress: fxAddr,
			PrivateKeyHex:      operatorKey,
		}, logger)
		if err != nil {
			log.Fatalf("FATAL: fx agreement client: %v", err)
		}
		fxAgreementBesu = fxClient
		logger.Info("FXAgreement Besu client configured", "address", fxAddr)
	} else {
		logger.Warn("FX_AGREEMENT_ADDRESS not set or BESU_RPC_URL empty — Besu FX agreement operations disabled")
	}

	// Escrow repository: in-memory for now (production: GORM + PostgreSQL).
	escrowRepo := repository.NewMemoryEscrowRepository()
	logger.Info("escrow repository configured (in-memory)")

	// FX Agreement repository: PostgreSQL-backed.
	fxDSN := getEnv("FX_POSTGRES_DSN", getEnv("DATABASE_URL", ""))
	if fxDSN == "" {
		log.Fatal("FATAL: FX_POSTGRES_DSN or DATABASE_URL is required for FX agreement persistence")
	}
	fxRepo, err := repository.NewGormFXAgreementRepository(fxDSN)
	if err != nil {
		log.Fatalf("FATAL: fx agreement repository: %v", err)
	}
	logger.Info("fx agreement repository configured", "dsn_source", "FX_POSTGRES_DSN|DATABASE_URL")

	rateTolPctStr := getEnv("FX_RATE_TOLERANCE_PCT", "0.001")
	rateTolPct, err := strconv.ParseFloat(rateTolPctStr, 64)
	if err != nil || rateTolPct <= 0 {
		log.Fatalf("FATAL: invalid FX_RATE_TOLERANCE_PCT %q", rateTolPctStr)
	}
	logger.Info("fx rate tolerance configured", "pct", rateTolPct)

	fxExpiryCheckIntervalStr := getEnv("FX_EXPIRY_CHECK_INTERVAL_S", "300")
	fxExpiryCheckIntervalU, err := strconv.ParseUint(fxExpiryCheckIntervalStr, 10, 64)
	if err != nil || fxExpiryCheckIntervalU == 0 {
		log.Fatalf("FATAL: invalid FX_EXPIRY_CHECK_INTERVAL_S %q", fxExpiryCheckIntervalStr)
	}
	logger.Info("fx expiry check interval configured", "seconds", fxExpiryCheckIntervalU)

	grpcServer, err := server.New(server.Config{
		Token:           token,
		Fiat:            fiatToken,
		Relay:           relay,
		EscrowRepo:      escrowRepo,
		FXAgreementBesu: fxAgreementBesu,
		FXRepo:          fxRepo,
		RateTolPct:      rateTolPct,
		Logger:          logger,
	})
	if err != nil {
		log.Fatalf("FATAL: configure gRPC server: %v", err)
	}

	expiryWorker := workers.NewFXExpirationWorker(fxRepo, time.Duration(fxExpiryCheckIntervalU)*time.Second, logger) // #nosec G115 -- interval parsed from config as bounded positive uint64, fits int64
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go expiryWorker.Start(ctx)

	// Bridge RelayerWorker (Scenario B — FR-031 / FR-039).
	if bridgeDB, bridgeErr := gorm.Open(gormpg.Open(fxDSN), &gorm.Config{}); bridgeErr != nil {
		logger.Warn("bridge relayer worker disabled — could not open bridge DB", "error", bridgeErr)
	} else if migErr := dbinit.RunAutoMigrate(bridgeDB); migErr != nil {
		// Defensive additive migration; the burn/mint confirmation writes must not race ahead of
		// the schema (R2-H-12 #4). Fail closed rather than start the relayer against a stale schema.
		log.Fatalf("FATAL: bridge schema migrate: %v", migErr)
	} else {
		hubRPC := getEnv("HUB_BESU_RPC_URL", "")
		hubKey := getEnv("SIGNER_PRIVATE_KEY", "")
		if hubRPC == "" || hubKey == "" {
			logger.Warn("bridge relayer worker disabled — set HUB_BESU_RPC_URL and SIGNER_PRIVATE_KEY to enable")
		} else {
			hubChainID := resolveHubChainID(logger)
			spokeChainID, _ := strconv.ParseInt(getEnv("SPOKE_CHAIN_ID", "1337"), 10, 64)
			dialCtx, dialCancel := context.WithTimeout(ctx, 15*time.Second)
			hubMintRecipient := getEnv("HUB_MINT_RECIPIENT", "")
			if hubMintRecipient == "" {
				hubMintRecipient = getEnv("ENTITY_BESU_ADDRESS", "")
			}
			executor, execErr := workers.NewBesuRelayerExecutor(dialCtx, bridgeDB, workers.BesuRelayerConfig{
				HubRPCURL:        hubRPC,
				HubChainID:       hubChainID,
				HubSignerKey:     hubKey,
				HubMintRecipient: hubMintRecipient,
				SpokeRPCURL:      getEnv("SPOKE_BESU_RPC_URL", ""),
				SpokeChainID:     spokeChainID,
				SpokeSignerKey:   getEnv("SPOKE_SIGNER_KEY", getEnv("BESU_OPERATOR_KEY", "")),
				SpokeBridgeAddr:  getEnv("SPOKE_BRIDGE_ADDRESS", ""),
				SkipSpokeLock:    getEnv("BRIDGE_SKIP_SPOKE_LOCK", "") == "true",
			})
			dialCancel()
			if execErr != nil {
				log.Fatalf("FATAL: bridge relayer executor init: %v", execErr)
			}
			relayerWorker := workers.NewRelayerWorker(bridgeDB, executor, 5*time.Second, getEnv("BRIDGE_SKIP_SPOKE_LOCK", "") == "true")
			go relayerWorker.Run(ctx)
			logger.Info("bridge relayer worker started",
				"hub_rpc", hubRPC,
				"spoke_configured", getEnv("SPOKE_BESU_RPC_URL", "") != "" && getEnv("SPOKE_BRIDGE_ADDRESS", "") != "",
			)
		}
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigChan
		logger.Info("received signal", "signal", sig)
		cancel()
		grpcServer.GracefulStop()
	}()

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

// resolveHubChainID reads HUB_CHAIN_ID from the environment, defaulting to 1337.
// When the variable is absent, it emits a structured warning so operators can
// detect misconfigured environments without a service failure (FR-009 / Constitution VI).
func resolveHubChainID(logger *slog.Logger) int64 {
	raw := os.Getenv("HUB_CHAIN_ID")
	if raw == "" {
		logger.Warn("HUB_CHAIN_ID not set; defaulting to 1337",
			"variable", "HUB_CHAIN_ID",
			"default", "1337",
		)
		return 1337
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		logger.Warn("HUB_CHAIN_ID invalid; defaulting to 1337",
			"variable", "HUB_CHAIN_ID",
			"value", raw,
			"default", "1337",
		)
		return 1337
	}
	return id
}

func parseBoolEnv(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}
