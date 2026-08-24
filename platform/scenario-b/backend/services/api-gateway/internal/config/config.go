// SPDX-License-Identifier: Apache-2.0

// This file loads environment variables into a typed runtime configuration.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2/log"
	"github.com/joho/godotenv"
)

// Config holds runtime settings loaded from environment variables.
type Config struct {
	AppPort                 string
	RequestTimeout          time.Duration
	AuthGRPCAddr            string
	ComplianceGRPCAddr      string // compliance-orchestrator address (optional; enables governance endpoints)
	PaymentGRPCAddr         string // payment-orchestrator address (optional; enables HTLC + token endpoints)
	CookieSecure            bool   // true for HTTPS (Secure flag); false for plain HTTP
	CentralBankAPIURL       string // when set, this gateway acts as a commercial bank and proxies onboarding calls to the CB
	BankCode                string // commercial bank identifier (e.g. "bank-a"); required when CentralBankAPIURL is set
	PKIDir                  string // path to PKI files (CSR, keys); used by the smart proxy to load CSR
	EntityBesuAddress       string // Besu address of this entity; used by the escrow proxy to enrich requests
	CBTokenRecipientAddress string // Central Bank's token recipient address (Besu address); receiver for token transfers in redeem flow
	RelayAuthSecret         string // shared secret for X-Relay-Auth header on internal service-to-service endpoints (legacy fallback)
	// RelayKeyID identifies this entity in service-to-service authentication: the sender puts it in
	// X-Relay-Key-Id and the receiver pins one public key per id, so it is also what attributes an
	// internal act to a specific sovereign. It must be UNIQUE across the deployment.
	//
	// Separate from BankCode on purpose. The compose template sets BANK_CODE to the entity's ROLE,
	// so every central bank is "central-bank" — two of them sharing an id means the registry can pin
	// only one key, and "signed by central-bank" would not say which one. BankCode cannot simply be
	// changed: it flows into owner_bank_id on bridge positions and into the reconciliation's
	// self-exclusion, so that would be a data migration. Defaults to BankCode, which is already
	// unique for a commercial bank.
	RelayKeyID            string
	RelayRequireSignature bool // when true, internal relay endpoints reject requests without a valid per-CB signature (post-cutover enforcement)
	// Simplified bridge/liquidity config (008-fix-cb-liquidity API simplification)
	SpokeNetwork      string // spoke-a, spoke-b (for bridge lock-mint derivation)
	NativeAssetSymbol string // tCeBM_BRL, tCeBM_ARS (for bridge lock-mint derivation)
	FiatSymbol        string // human-readable currency symbol used for transfer-limit matching (e.g. "BRL", "ARS")
	WTokenAddress     string // Hub W-tCeBM token address (for bridge + commit derivation)
	CommitSide        string // A or B (derived from BANK_CODE for commit derivation)
	ApproveSide       string // A or B (derived from BANK_CODE for approve-amm; FR-013)
}

// Load reads environment variables and returns a fully populated Config.
func Load() Config {

	if err := godotenv.Load(".env"); err != nil {
		log.Warnf("no .env (.env) file(s) found, using environment variables: function error: %s", err.Error())
	}

	bankCode := getEnv("BANK_CODE", "")
	if bankCode == "" {
		bankCode = getEnv("GOVERNANCE_BANK_CODE", "")
	}

	return Config{
		AppPort:                 getEnv("APP_PORT", "8080"),
		RequestTimeout:          time.Duration(getEnvInt("REQUEST_TIMEOUT_SEC", 5)) * time.Second,
		AuthGRPCAddr:            getEnv("AUTH_GRPC_ADDR", "localhost:9091"),
		ComplianceGRPCAddr:      getEnv("COMPLIANCE_GRPC_ADDR", "localhost:9093"),
		PaymentGRPCAddr:         getEnv("PAYMENT_GRPC_ADDR", ""),
		CookieSecure:            getEnvBool("COOKIE_SECURE", false),
		CentralBankAPIURL:       getEnv("CENTRAL_BANK_API_URL", ""),
		BankCode:                bankCode,
		PKIDir:                  getEnv("PKI_DIR", ""),
		EntityBesuAddress:       getEnv("ENTITY_BESU_ADDRESS", ""),
		CBTokenRecipientAddress: getEnv("CB_TOKEN_RECIPIENT_ADDRESS", ""),
		RelayAuthSecret:         getEnv("INTERNAL_RELAY_AUTH_SECRET", ""),
		RelayRequireSignature:   getEnvBool("RELAY_REQUIRE_SIGNATURE", false),
		RelayKeyID:              getEnv("RELAY_KEY_ID", bankCode),
		SpokeNetwork:            getEnv("SPOKE_NETWORK", ""),
		NativeAssetSymbol:       getEnv("NATIVE_ASSET_SYMBOL", ""),
		FiatSymbol:              getEnv("FIAT_SYMBOL", ""),
		WTokenAddress:           getEnv("W_TOKEN_ADDRESS", ""),
		CommitSide:              deriveCommitSide(bankCode),
		ApproveSide:             deriveApproveSide(bankCode),
	}
}

// getEnv returns an environment variable or a fallback if missing/blank.
func getEnv(name, fallback string) string {
	value, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// getEnvInt parses an integer environment variable with fallback behavior.
func getEnvInt(name string, fallback int) int {
	raw := getEnv(name, strconv.Itoa(fallback))
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

// getEnvBool parses a boolean environment variable ("true"/"1" → true) with fallback.
func getEnvBool(name string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	return raw == "true" || raw == "1"
}

// deriveCommitSide maps BANK_CODE to commit side (A/B) for sovereign liquidity.
// Used by 008-fix-cb-liquidity API simplification.
func deriveCommitSide(bankCode string) string {
	switch bankCode {
	case "central-bank-a":
		return "A"
	case "central-bank-b":
		return "B"
	default:
		return ""
	}
}

// deriveApproveSide maps BANK_CODE to the token side used in approve-amm (FR-013).
// Commercial bank codes ending in "-a" → TOKEN_A (side "A"); ending in "-b" → TOKEN_B (side "B").
// Central banks (prefix "central-") return "" so they keep CENTRAL_BANK_ROLE auto-detection
// and can still override side explicitly for the G5-cross pattern.
func deriveApproveSide(bankCode string) string {
	if bankCode == "" {
		return ""
	}
	lower := strings.ToLower(strings.TrimSpace(bankCode))
	// CBs use CENTRAL_BANK_ROLE on-chain; must NOT have approveSide forced.
	if strings.HasPrefix(lower, "central-") {
		return ""
	}
	if strings.HasSuffix(lower, "-a") {
		return "A"
	}
	if strings.HasSuffix(lower, "-b") {
		return "B"
	}
	return ""
}
