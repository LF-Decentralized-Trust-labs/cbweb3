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
	paladinAdapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/adapters/paladin"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/identity"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/repository"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/workers"
)

func main() {
	port := getEnv("PAYMENT_GRPC_PORT", "9094")

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	paladinIdentity := getEnv("PALADIN_IDENTITY", "funded_operator@spoke-a-cb")

	// Zeto operator: real Paladin client or no-op based on config.
	var zeto ports.ZetoOperator
	if paladinURL := os.Getenv("PALADIN_URL"); paladinURL != "" {
		identity := paladinIdentity
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

	// Shared Besu config — used by HTLC, FiatToken, and FXAgreement adapters.
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

	// On-chain HTLC coordination (optional — requires BESU_RPC_URL + HTLC_ADDRESS).
	var htlc ports.HTLCContractPort
	if besuRPC != "" {
		htlcAddr := os.Getenv("HTLC_ADDRESS")
		if htlcAddr == "" {
			log.Fatal("FATAL: HTLC_ADDRESS is required when BESU_RPC_URL is set")
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

	// On-chain FXAgreement coordination (optional — can route to Besu and/or Pente).
	var fxAgreementBesu ports.FXAgreementContractPort
	var fxAgreementPente ports.FXAgreementContractPort
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

	// Pente bilateral private context integration (optional; controlled via env).
	var pente ports.PenteClientPort
	penteEnabled := parseBoolEnv("PENTE_ENABLED", false)
	if penteEnabled {
		penteURL := getEnv("PENTE_BASE_URL", os.Getenv("PALADIN_URL"))
		if penteURL == "" {
			log.Fatal("FATAL: PENTE_BASE_URL (or PALADIN_URL) is required when PENTE_ENABLED=true")
		}
		penteClient := paladinAdapter.NewPenteClient(paladinAdapter.PenteClientConfig{
			BaseURL:            penteURL,
			FXAgreementAddress: getEnv("FX_AGREEMENT_PENTE_CONTRACT_ADDRESS", os.Getenv("FX_AGREEMENT_ADDRESS")),
			Identity:           getEnv("PALADIN_IDENTITY", "funded_operator@spoke-a-cb"),
		})
		pente = penteClient
		fxAgreementPente = penteClient
		logger.Info("pente client configured", "url", penteURL)
	} else {
		logger.Warn("Pente integration disabled (PENTE_ENABLED=false)")
	}

	// Fiat token client: real Besu adapter when FIAT_TOKEN_ADDRESS is set.
	var fiat ports.FiatTokenPort
	if fiatAddr := os.Getenv("FIAT_TOKEN_ADDRESS"); fiatAddr != "" {
		if besuRPC == "" {
			log.Fatal("FATAL: BESU_RPC_URL is required when FIAT_TOKEN_ADDRESS is set")
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

	// FX Agreement repository: PostgreSQL-backed by default, fallback to in-memory map in server when absent.
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

	strictHTLC := parseBoolEnv("FX_AGREEMENT_HTLC_STRICT", true)
	logger.Info("fx htlc strict mode configured", "strict", strictHTLC)

	// Parse FX expiration check interval (default 300 seconds = 5 minutes)
	fxExpiryCheckIntervalStr := getEnv("FX_EXPIRY_CHECK_INTERVAL_S", "300")
	fxExpiryCheckIntervalU, err := strconv.ParseUint(fxExpiryCheckIntervalStr, 10, 64)
	if err != nil || fxExpiryCheckIntervalU == 0 {
		log.Fatalf("FATAL: invalid FX_EXPIRY_CHECK_INTERVAL_S %q", fxExpiryCheckIntervalStr)
	}
	logger.Info("fx expiry check interval configured", "seconds", fxExpiryCheckIntervalU)

	// Extract spoke prefix from PALADIN_IDENTITY for receiver validation.
	// e.g. "funded_operator@spoke-a-bank-a" → "spoke-a"
	spokePrefix := identity.SpokePrefix(paladinIdentity)
	if spokePrefix != "" {
		logger.Info("spoke prefix configured for receiver validation", "spokePrefix", spokePrefix)
	} else {
		logger.Warn("could not extract spoke prefix from PALADIN_IDENTITY — receiver locality check disabled")
	}

	grpcServer, startRelayWorkers := server.New(server.Config{
		Zeto:             zeto,
		HTLC:             htlc,
		Relay:            relay,
		Fiat:             fiat,
		EscrowRepo:       escrowRepo,
		FXAgreementBesu:  fxAgreementBesu,
		FXAgreementPente: fxAgreementPente,
		FXRepo:           fxRepo,
		Pente:            pente,
		RateTolPct:       rateTolPct,
		StrictHTLC:       strictHTLC,
		SpokePrefix:      spokePrefix,
		PaladinIdentity:  paladinIdentity,
		Logger:           logger,
	})

	// Initialize FX expiration worker
	expiryWorker := workers.NewFXExpirationWorker(fxRepo, time.Duration(fxExpiryCheckIntervalU)*time.Second, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go expiryWorker.Start(ctx)
	go startRelayWorkers(ctx)

	// Set up graceful shutdown on SIGTERM/SIGINT
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

func parseBoolEnv(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}
