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
	AppPort            string
	RequestTimeout     time.Duration
	AuthGRPCAddr       string
	ComplianceGRPCAddr string // compliance-orchestrator address (optional; enables governance endpoints)
	PaymentGRPCAddr    string // payment-orchestrator address (optional; enables HTLC + token endpoints)
	CookieSecure       bool   // true for HTTPS (Secure flag); false for plain HTTP
	CentralBankAPIURL  string // when set, this gateway acts as a commercial bank and proxies onboarding calls to the CB
	BankCode           string // commercial bank identifier (e.g. "bank-a"); required when CentralBankAPIURL is set
	PKIDir             string // path to PKI files (CSR, keys); used by the smart proxy to load CSR
	EntityBesuAddress  string // Besu address of this entity; used by the escrow proxy to enrich requests
	PaladinIdentity    string // Paladin identity for this entity; used for Zeto operations
	CBPaladinIdentity  string // Central Bank's Paladin identity; receiver for Zeto transfers in redeem flow
	RelayAuthSecret    string // shared secret for X-Relay-Auth header on internal service-to-service endpoints
	BesuRPCURL          string // Besu JSON-RPC endpoint; when set together with HTLCContractAddress, enables on-chain HTLC scan for supervisors
	HTLCContractAddress string // HTLC contract address on the Besu network (HTLC_ADDRESS env var)
	PaladinURL          string // Paladin JSON-RPC endpoint; enables transaction decryption for supervisors (PALADIN_URL env var)
	FiatSymbol                  string // currency symbol for transfer limit matching (e.g. "BRL", "ARS"); from FIAT_SYMBOL
	TransferLimitComplianceAddr string // when set, limit-enforcement checks are sent to this compliance address instead of ComplianceGRPCAddr; allows commercial-bank gateways to query the central bank's compliance service
	PaladinIdentities           []string // optional static override for the FX party roster; from PALADIN_IDENTITIES (comma-separated). Empty = source live Pente membership.
}

// Load reads environment variables and returns a fully populated Config.
func Load() Config {

	if err := godotenv.Load(".env"); err != nil {
		log.Warnf("no .env (.env) file(s) found, using environment variables: function error: %s", err.Error())
	}

	// PALADIN_IDENTITIES is now only an optional explicit override for the FX
	// agreement party roster. When unset, the roster is sourced from real Pente
	// group membership via the payment-orchestrator. There is no demo fallback,
	// so a misconfigured deployment offers an empty roster ("not configured")
	// instead of silently offering identities from another network.
	if raw := strings.TrimSpace(os.Getenv("PALADIN_IDENTITIES")); raw != "" {
		log.Warnf("PALADIN_IDENTITIES override is set; the FX party roster is pinned to this static list " +
			"instead of live Pente membership. Unset it to track real membership.")
	}

	return Config{
		AppPort:            getEnv("APP_PORT", "8080"),
		RequestTimeout:     time.Duration(getEnvInt("REQUEST_TIMEOUT_SEC", 5)) * time.Second,
		AuthGRPCAddr:       getEnv("AUTH_GRPC_ADDR", "localhost:9091"),
		ComplianceGRPCAddr: getEnv("COMPLIANCE_GRPC_ADDR", "localhost:9093"),
		PaymentGRPCAddr:    getEnv("PAYMENT_GRPC_ADDR", ""),
		CookieSecure:       getEnvBool("COOKIE_SECURE", false),
		CentralBankAPIURL:  getEnv("CENTRAL_BANK_API_URL", ""),
		BankCode:           getEnv("BANK_CODE", ""),
		PKIDir:             getEnv("PKI_DIR", ""),
		EntityBesuAddress:  getEnv("ENTITY_BESU_ADDRESS", ""),
		PaladinIdentity:    getEnv("PALADIN_IDENTITY", ""),
		CBPaladinIdentity:  getEnv("CB_PALADIN_IDENTITY", ""),
		RelayAuthSecret:             getEnv("INTERNAL_RELAY_AUTH_SECRET", ""),
		BesuRPCURL:                  getEnv("BESU_RPC_URL", ""),
		HTLCContractAddress:         getEnv("HTLC_ADDRESS", ""),
		PaladinURL:                  getEnv("PALADIN_URL", ""),
		FiatSymbol:                  getEnv("FIAT_SYMBOL", ""),
		TransferLimitComplianceAddr: getEnv("TRANSFER_LIMIT_COMPLIANCE_ADDR", ""),
		PaladinIdentities:           getEnvList("PALADIN_IDENTITIES", nil),
	}
}

// getEnvList parses a comma-separated environment variable into a trimmed,
// non-empty string slice, returning the fallback when unset or blank.
func getEnvList(name string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
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
