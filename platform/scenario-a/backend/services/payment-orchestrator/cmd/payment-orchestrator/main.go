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
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/adapters/cbreport"
	paladinAdapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/adapters/paladin"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/identity"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/repository"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/workers"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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
	cactiAuthSecret := os.Getenv("INTERNAL_RELAY_AUTH_SECRET")
	if cactiAuthSecret == "" {
		log.Fatal("FATAL: INTERNAL_RELAY_AUTH_SECRET is required")
	}
	// The relay is constructed below, after the shared DB is opened, so it can be
	// given a durable watermark store (finding R2-H-11).

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
	var fxChainReader ports.FXChainReaderPort
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
		fxChainReader = penteClient
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

	// Open shared Postgres connection for all repositories.
	dbDSN := getEnv("DATABASE_URL", getEnv("FX_POSTGRES_DSN", ""))
	if dbDSN == "" {
		log.Fatal("FATAL: DATABASE_URL or FX_POSTGRES_DSN is required")
	}
	sharedDB, err := gorm.Open(postgres.Open(dbDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("FATAL: database connection: %v", err)
	}
	{
		sqlDB, err := sharedDB.DB()
		if err != nil {
			log.Fatalf("FATAL: get sql.DB: %v", err)
		}
		sqlDB.SetMaxOpenConns(25)
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetConnMaxLifetime(5 * time.Minute)
	}
	logger.Info("database connection established", "dsn_source", "DATABASE_URL|FX_POSTGRES_DSN")

	// Durable relay event-cursor store: lets the Cacti poller resume from the last
	// processed timestamp after a restart instead of resetting to now (R2-H-11).
	watermarkStore, err := repository.NewGormRelayWatermarkStoreFromDB(sharedDB)
	if err != nil {
		log.Fatalf("FATAL: relay watermark store: %v", err)
	}
	relay := cacti.NewCactiRelay(cactiURL, cactiAuthSecret, watermarkStore, logger)
	logger.Info("cacti relay configured", "url", cactiURL)

	fxRepo, err := repository.NewGormFXAgreementRepositoryFromDB(sharedDB)
	if err != nil {
		log.Fatalf("FATAL: fx agreement repository: %v", err)
	}
	logger.Info("fx agreement repository configured")

	escrowRepo, err := repository.NewGormEscrowRepositoryFromDB(sharedDB)
	if err != nil {
		log.Fatalf("FATAL: escrow repository: %v", err)
	}
	logger.Info("escrow repository configured (postgres)")

	htlcRepo, err := repository.NewGormHTLCRepositoryFromDB(sharedDB)
	if err != nil {
		log.Fatalf("FATAL: htlc repository: %v", err)
	}
	logger.Info("htlc repository configured (postgres)")

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

	// The spoke this orchestrator belongs to, for the receiver-locality check.
	//
	// SPOKE_ID is authoritative and is the only correct source: a node name is
	// <spokeId>-<bankId> and both halves may contain hyphens, so the spoke id
	// cannot be parsed back out of PALADIN_IDENTITY —
	// "spoke-costa-rica-cb1" is indistinguishable from a spoke "spoke-costa"
	// with a bank "rica-cb1".
	//
	// identity.SpokePrefix remains the fallback for an environment deployed
	// before SPOKE_ID was wired into the compose template. It guesses two
	// segments, which conflates any two spokes sharing them; the warning says so
	// rather than reporting a healthy configuration.
	spokePrefix := strings.TrimSpace(os.Getenv("SPOKE_ID"))
	switch {
	case spokePrefix != "":
		logger.Info("spoke configured for receiver validation", "spokeID", spokePrefix, "source", "SPOKE_ID")
	default:
		spokePrefix = identity.SpokePrefix(paladinIdentity)
		if spokePrefix != "" {
			logger.Warn("SPOKE_ID is not set — falling back to a two-segment guess from "+
				"PALADIN_IDENTITY, which cannot distinguish spokes sharing their first two "+
				"segments; re-apply the entity to have the toolkit set SPOKE_ID",
				"guessedSpokeID", spokePrefix, "paladinIdentity", paladinIdentity)
		} else {
			logger.Warn("neither SPOKE_ID nor a parseable PALADIN_IDENTITY — receiver locality check disabled")
		}
	}

	// Settlement reporter: forwards settled PvP legs to the Central Bank gateway
	// so the receiving bank sees the incoming credit on its statement. A receiving
	// bank's own orchestrator holds no record of an incoming leg — the counterparty
	// locked it elsewhere and the amount is private — so this report is the ONLY
	// source of that movement.
	//
	// It defaults to CENTRAL_BANK_API_URL, which the toolkit already renders for
	// every entity and which already has exactly the right shape: a commercial
	// bank gets its own spoke's CB gateway, and a central bank gets an empty value
	// (a CB receives these reports, it does not send them). CB_INTERNAL_API_URL
	// stays as an explicit override for the rare case of pointing the reporter
	// somewhere else.
	//
	// Reading only CB_INTERNAL_API_URL is what left this switched off everywhere:
	// that name appears nowhere but here — not in a template, not in the toolkit,
	// not in deploy-lnet — so the reporter was never constructed, pvp_settled_legs
	// was never written by the product, and the credit side of every bank's
	// statement was permanently empty.
	var settlementReporter ports.SettlementReporter
	if cbURL := settlementReportURL(os.Getenv); cbURL != "" {
		settlementReporter = cbreport.New(cbURL, os.Getenv("INTERNAL_RELAY_AUTH_SECRET"))
		logger.Info("settlement reporter configured", "cb_url", cbURL)
	} else {
		// Expected on a central-bank orchestrator. On a commercial bank it means
		// incoming credits will not appear on any statement.
		logger.Warn("no central bank URL (CB_INTERNAL_API_URL / CENTRAL_BANK_API_URL) — settled PvP legs will not be reported (receiver credits will not appear)")
	}

	grpcServer, startRelayWorkers, err := server.New(server.Config{
		Zeto:               zeto,
		HTLC:               htlc,
		Relay:              relay,
		Fiat:               fiat,
		EscrowRepo:         escrowRepo,
		HTLCRepo:           htlcRepo,
		FXAgreementBesu:    fxAgreementBesu,
		FXAgreementPente:   fxAgreementPente,
		FXRepo:             fxRepo,
		Pente:              pente,
		FXChainReader:      fxChainReader,
		FXContextsFile:     getEnv("FX_CONTEXTS_FILE", "/workspace/backend/config/pki/fx-contexts.json"),
		RateTolPct:         rateTolPct,
		CrossSpokeMode:     true, // relay is always active in production (CACTI_API_URL is required)
		StrictHTLC:         strictHTLC,
		SpokePrefix:        spokePrefix,
		PaladinIdentity:    paladinIdentity,
		SettlementReporter: settlementReporter,
		Logger:             logger,
	})
	if err != nil {
		log.Fatalf("FATAL: payment-orchestrator server: %v", err)
	}

	// Initialize FX expiration worker
	expiryWorker := workers.NewFXExpirationWorker(fxRepo, time.Duration(fxExpiryCheckIntervalU)*time.Second, logger) //#nosec G115 -- check interval is a small positive uint64 from config, fits int64 Duration
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go expiryWorker.Start(ctx)
	go startRelayWorkers(ctx)

	// FX aggregation indexer — central-bank only. Projects on-chain FX agreements from every
	// bilateral Pente group the node belongs to into fx_agreements, so the CB exposes an
	// aggregate the relay polls. Chain-driven; no context file. Gated so commercial-bank nodes
	// (which are members of a single group and don't aggregate) do not run it.
	if parseBoolEnv("FX_INDEXER_ENABLED", false) {
		if fxChainReader == nil {
			logger.Warn("FX_INDEXER_ENABLED=true but Pente is disabled — indexer not started")
		} else {
			indexerSec := uint64(15)
			if v, err := strconv.ParseUint(getEnv("FX_INDEXER_INTERVAL_SEC", "15"), 10, 32); err == nil && v > 0 {
				indexerSec = v
			}
			//#nosec G115 -- interval is a small positive value from config, fits int64 Duration
			go workers.NewFXIndexer(fxChainReader, fxRepo, time.Duration(indexerSec)*time.Second, logger).Start(ctx)
			logger.Info("fx indexer enabled", "interval_sec", indexerSec)
		}
	}

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

// settlementReportURL resolves the central-bank base URL the settlement reporter
// posts to. Extracted from main so it can be tested: the defect it fixes was not a
// logic error but a wiring one — the reporter read a name that nothing set — and a
// wiring error is only catchable by asserting which names are consulted.
//
// CB_INTERNAL_API_URL is the explicit override. CENTRAL_BANK_API_URL is the default
// because the toolkit already renders it per entity, with exactly the right shape:
// a commercial bank gets its own spoke's central bank, and a central bank gets an
// empty value, which correctly leaves the reporter off (a CB receives these
// reports, it does not send them).
func settlementReportURL(getenv func(string) string) string {
	if v := strings.TrimSpace(getenv("CB_INTERNAL_API_URL")); v != "" {
		return v
	}
	return strings.TrimSpace(getenv("CENTRAL_BANK_API_URL"))
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
