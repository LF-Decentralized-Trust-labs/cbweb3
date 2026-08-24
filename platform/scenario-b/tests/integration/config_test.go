// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package integration_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Config holds all environment-driven settings for the integration test suite.
//
// Every field comes from the environment. tests/integration/toolkit-env.sh derives the
// whole set from the toolkit manifests a stack was provisioned with, and the make
// target sources it — so the values follow the topology under test instead of being
// pinned to one bring-up's ports and entity names.
type Config struct {
	BankAURL        string
	BankBURL        string
	CentralBankAURL string
	CentralBankBURL string

	// Besu JSON-RPC endpoints for on-chain evidence (eth_getTransactionReceipt).
	// Hub hosts the AMM swap; Spoke-A (BRL — bank-a/cb-a) hosts the fiat/reserve
	// token mints; Spoke-B (ARS) hosts the beneficiary bridge-out.
	BesuHubURL    string
	BesuSpokeAURL string
	BesuSpokeBURL string

	// Despite the *Client/*Secret names, these carry a Keycloak USERNAME and that
	// user's PASSWORD. The login endpoint's clientId/clientSecret JSON fields are the
	// wire contract's names, not the credential type — the auth service accepts only
	// the OIDC password grant, so a realm client id/secret is refused on purpose.
	//
	// The Realm fields that used to sit here are gone: nothing read them, and their
	// defaults (bank-a, central-bank-a) named realms no toolkit stack has.
	BankAClient        string
	BankASecret        string
	BankBClient        string
	BankBSecret        string
	CentralBankAClient string
	CentralBankASecret string
	CentralBankBClient string
	CentralBankBSecret string

	// Corridor identity. The hub names a pool by BOTH wrapped sides
	// ("W-BRL-W-ARS"); the suite used to hardcode the short form "W-BRL-ARS", which
	// resolved to nothing. Derived from the two central banks' spoke currencies.
	PoolPair       string
	SourceCurrency string
	TargetCurrency string

	// Participant codes, from each bank manifest's spec.bankId. The registry keys
	// banks by these; "bank-a"/"bank-b" name nothing in a toolkit topology.
	BankACode string
	BankBCode string

	SkipUp   bool
	SkipDown bool
}

func loadConfig() *Config {
	return &Config{
		// No fallback ports. The defaults here used to be the legacy deploy/local
		// host-port mappings, which now answer nothing — so a bare `go test` failed by
		// connection-refused against an address that stopped meaning anything. Unset is
		// an explicit error instead (see requireComplete).
		BankAURL:        os.Getenv("API_GW_BANK_A_URL"),
		BankBURL:        os.Getenv("API_GW_BANK_B_URL"),
		CentralBankAURL: os.Getenv("API_GW_CENTRAL_BANK_A_URL"),
		CentralBankBURL: os.Getenv("API_GW_CENTRAL_BANK_B_URL"),

		BesuHubURL:    os.Getenv("BESU_HUB_RPC"),
		BesuSpokeAURL: os.Getenv("BESU_SPOKE_A_RPC"),
		BesuSpokeBURL: os.Getenv("BESU_SPOKE_B_RPC"),

		// These used to fall back to reading KC_CLIENT_SECRET out of
		// backend/config/.env.infra.<entity> — files the legacy deploy/local bring-up
		// wrote and the toolkit deliberately does not. The fallback is gone; credentials
		// come from the environment, which toolkit-env.sh derives from the manifests the
		// stack was actually provisioned with.
		BankAClient:        os.Getenv("KC_BANK_A_CLIENT"),
		BankASecret:        os.Getenv("KC_BANK_A_SECRET"),
		BankBClient:        os.Getenv("KC_BANK_B_CLIENT"),
		BankBSecret:        os.Getenv("KC_BANK_B_SECRET"),
		CentralBankAClient: os.Getenv("KC_CENTRAL_BANK_A_CLIENT"),
		CentralBankASecret: os.Getenv("KC_CENTRAL_BANK_A_SECRET"),
		CentralBankBClient: os.Getenv("KC_CENTRAL_BANK_B_CLIENT"),
		CentralBankBSecret: os.Getenv("KC_CENTRAL_BANK_B_SECRET"),

		PoolPair:       os.Getenv("POOL_PAIR"),
		SourceCurrency: os.Getenv("SOURCE_CURRENCY"),
		TargetCurrency: os.Getenv("TARGET_CURRENCY"),
		BankACode:      os.Getenv("BANK_A_CODE"),
		BankBCode:      os.Getenv("BANK_B_CODE"),

		// Default: assume a live stack. Setting either to 0 is an error, not a
		// lifecycle request — the suite does not provision (see TestMain).
		SkipUp:   os.Getenv("SKIP_UP") != "0",
		SkipDown: os.Getenv("SKIP_DOWN") != "0",
	}
}

// requireComplete fails the run when an endpoint or an operator login is missing,
// rather than letting an empty string reach Keycloak and come back as a bare 401.
func (c *Config) requireComplete(t *testing.T) {
	t.Helper()
	required := []struct{ name, val string }{
		{"API_GW_BANK_A_URL", c.BankAURL},
		{"API_GW_BANK_B_URL", c.BankBURL},
		{"API_GW_CENTRAL_BANK_A_URL", c.CentralBankAURL},
		{"API_GW_CENTRAL_BANK_B_URL", c.CentralBankBURL},
		{"BESU_HUB_RPC", c.BesuHubURL},
		{"BESU_SPOKE_A_RPC", c.BesuSpokeAURL},
		{"BESU_SPOKE_B_RPC", c.BesuSpokeBURL},
		{"POOL_PAIR", c.PoolPair},
		{"SOURCE_CURRENCY", c.SourceCurrency},
		{"TARGET_CURRENCY", c.TargetCurrency},
		{"BANK_A_CODE", c.BankACode},
		{"BANK_B_CODE", c.BankBCode},
	}
	var unset []string
	for _, e := range required {
		if e.val == "" {
			unset = append(unset, e.name)
		}
	}
	if len(unset) > 0 {
		t.Fatalf("unset required setting(s): %s.\n%s", strings.Join(unset, ", "), configHelp)
	}

	pairs := []struct{ name, user, pass string }{
		{"bank-a", c.BankAClient, c.BankASecret},
		{"bank-b", c.BankBClient, c.BankBSecret},
		{"central-bank-a", c.CentralBankAClient, c.CentralBankASecret},
		{"central-bank-b", c.CentralBankBClient, c.CentralBankBSecret},
	}
	var missing []string
	for _, p := range pairs {
		if p.user == "" || p.pass == "" {
			missing = append(missing, p.name)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("no operator credentials for %s.\n%s", strings.Join(missing, ", "), configHelp)
	}
}

const configHelp = "" +
	"This suite takes its whole configuration from the environment — it has no\n" +
	"built-in topology. Derive it from the manifests the stack was provisioned with:\n" +
	"    make scenario-b.test-integration        # derives and runs\n" +
	"    make scenario-b.test-integration-env    # prints what it would use\n" +
	"Bring a stack up first with the single provisioning path:\n" +
	"    cd samples && ./deploy-all.sh"

// scenarioBRoot returns the absolute path to scenario-b/ by walking up from
// this test file's location (tests/integration/ → scenario-b/).
func scenarioBRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "../.."
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}
