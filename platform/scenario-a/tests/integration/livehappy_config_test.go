// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package integration_test

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Config holds the environment-driven settings for the live happy-path suite.
// Defaults match the deploy/local docker-compose host-port mappings and the
// per-entity Keycloak client IDs used by the tryout scripts.
type Config struct {
	BankAURL        string
	BankBURL        string
	BankDURL        string // custodian (financial correspondent) on spoke-b
	CentralBankAURL string
	CentralBankBURL string

	BankAClient string
	BankASecret string
	BankBClient string
	BankBSecret string
	BankDClient string
	BankDSecret string
	CBAClient   string
	CBASecret   string
	CBBClient   string
	CBBSecret   string

	// Treasury clients hold ROLE_TREASURY (required by /token/mint); the plain
	// central-bank clients above hold ROLE_GOVERNANCE and cannot mint.
	CBATreasuryClient string
	CBATreasurySecret string
	CBBTreasuryClient string
	CBBTreasurySecret string

	// Correspondent-banking roles (funded operators provisioned by `make spoke-all`):
	//   IdentityBankA       — originator on spoke-a (locks the origin leg)
	//   IdentityCorrespondentA — spoke-a financial correspondent (receives the origin leg)
	//   IdentityCustodian   — custodian/financial-correspondent bank-d on spoke-b (locks the counter leg)
	//   IdentityBankB       — beneficiary commercial bank-b on spoke-b (receives the counter leg)
	// The spoke-b leg is locked by the CUSTODIAN (bank-d) and released to the
	// BENEFICIARY (bank-b); the relay coordinates bank-a ↔ bank-d.
	IdentityBankA          string
	IdentityCorrespondentA string
	IdentityCustodian      string
	IdentityBankB          string

	// FX/HTLC amounts (origin leg on spoke-a, counter leg on spoke-b).
	OriginAmount  string
	CounterAmount string
	MintAmount    string

	SkipUp   bool // bring the stack up via `make spoke-all` before the suite
	SkipDown bool // tear the stack down after the suite
	SkipMint bool // skip the CB mint step (operators already funded)
	Onboard  bool // run runtime PKI onboarding (off by default; operators are pre-registered)
}

func loadConfig() *Config {
	root := scenarioARoot()
	return &Config{
		BankAURL:        envOr("API_GW_BANK_A_URL", "http://localhost:18080"),
		BankBURL:        envOr("API_GW_BANK_B_URL", "http://localhost:28080"),
		BankDURL:        envOr("API_GW_BANK_D_URL", "http://localhost:58080"),
		CentralBankAURL: envOr("API_GW_CENTRAL_BANK_A_URL", "http://localhost:38080"),
		CentralBankBURL: envOr("API_GW_CENTRAL_BANK_B_URL", "http://localhost:60080"),

		BankAClient: envOr("KC_BANK_A_CLIENT", "bank-a-client"),
		BankASecret: envOrEnvFile("KC_BANK_A_SECRET",
			filepath.Join(root, "backend/config/.env.infra.bank-a"), "KC_CLIENT_SECRET", ""),
		BankBClient: envOr("KC_BANK_B_CLIENT", "bank-b-client"),
		BankBSecret: envOrEnvFile("KC_BANK_B_SECRET",
			filepath.Join(root, "backend/config/.env.infra.bank-b"), "KC_CLIENT_SECRET", ""),
		BankDClient: envOr("KC_BANK_D_CLIENT", "bank-d-client"),
		BankDSecret: envOrEnvFile("KC_BANK_D_SECRET",
			filepath.Join(root, "backend/config/.env.infra.bank-d"), "KC_CLIENT_SECRET", ""),
		CBAClient: envOr("KC_CENTRAL_BANK_A_CLIENT", "central-bank-a-client"),
		CBASecret: envOrEnvFile("KC_CENTRAL_BANK_A_SECRET",
			filepath.Join(root, "backend/config/.env.infra.central-bank-a"), "KC_CLIENT_SECRET", ""),
		CBBClient: envOr("KC_CENTRAL_BANK_B_CLIENT", "central-bank-b-client"),
		CBBSecret: envOrEnvFile("KC_CENTRAL_BANK_B_SECRET",
			filepath.Join(root, "backend/config/.env.infra.central-bank-b"), "KC_CLIENT_SECRET", ""),

		// Treasury clients are provisioned by deploy/local/keycloak/init.sh with
		// fixed local-dev secrets (not stored in the .env.infra files).
		CBATreasuryClient: envOr("KC_CENTRAL_BANK_A_TREASURY_CLIENT", "central-bank-a-treasury-client"),
		CBATreasurySecret: envOr("KC_CENTRAL_BANK_A_TREASURY_SECRET", "central-bank-a-treasury-local-secret"),
		CBBTreasuryClient: envOr("KC_CENTRAL_BANK_B_TREASURY_CLIENT", "central-bank-b-treasury-client"),
		CBBTreasurySecret: envOr("KC_CENTRAL_BANK_B_TREASURY_SECRET", "central-bank-b-treasury-local-secret"),

		IdentityBankA:          envOr("IDENTITY_BANK_A", "funded_operator@spoke-a-bank-a"),
		IdentityCorrespondentA: envOr("IDENTITY_CORRESPONDENT_A", "funded_operator@spoke-a-bank-c"),
		IdentityCustodian:      envOr("IDENTITY_CUSTODIAN", "funded_operator@spoke-b-bank-d"),
		IdentityBankB:          envOr("IDENTITY_BANK_B", "funded_operator@spoke-b-bank-b"),

		OriginAmount:  envOr("FX_ORIGIN_AMOUNT", "100000"),
		CounterAmount: envOr("FX_COUNTER_AMOUNT", "520000"),
		MintAmount:    envOr("MINT_AMOUNT", "5000000"),

		// Stack lifecycle is opt-in: by default the suite assumes a live stack and
		// neither brings it up nor tears it down. `make spoke-all` regenerates
		// genesis and wipes the chain, so we never trigger it implicitly.
		SkipUp:   os.Getenv("SKIP_UP") != "0",
		SkipDown: os.Getenv("SKIP_DOWN") != "0",
		SkipMint: os.Getenv("SKIP_MINT") == "true",
		Onboard:  os.Getenv("ONBOARD") == "1",
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envOrEnvFile returns os.Getenv(envKey) if set, otherwise reads fileKey from the
// .env file at filePath, otherwise returns fallback.
func envOrEnvFile(envKey, filePath, fileKey, fallback string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	if v := readEnvFile(filePath, fileKey); v != "" {
		return v
	}
	return fallback
}

// readEnvFile parses a KEY=VALUE .env file and returns the value for the key.
func readEnvFile(path, key string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	prefix := key + "="
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	return ""
}

// scenarioARoot resolves scenario-a/ by walking up from this test file's location
// (tests/integration/ → scenario-a/).
func scenarioARoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "../.."
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}
