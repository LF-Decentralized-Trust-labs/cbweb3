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

// Config holds the environment-driven settings for the live happy-path suite.
//
// Every field comes from the environment. tests/integration/toolkit-env.sh derives
// the whole set from the toolkit manifests a stack was provisioned with, and the
// make target sources it — so the values follow the topology under test instead of
// being pinned to one bring-up's ports and entity names.
type Config struct {
	BankAURL        string
	BankBURL        string
	BankCURL        string // spoke-a financial correspondent (origin-leg receiver)
	BankDURL        string // custodian (financial correspondent) on spoke-b
	CentralBankAURL string
	CentralBankBURL string

	// Besu JSON-RPC endpoints used to resolve on-chain evidence
	// (eth_getTransactionReceipt / eth_getLogs). Any validator on a given chain
	// can answer; toolkit-env.sh derives the always-up central-bank node per spoke.
	BesuSpokeAURL string
	BesuSpokeBURL string

	BankAClient string
	BankASecret string
	BankBClient string
	BankBSecret string
	BankCClient string
	BankCSecret string
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

	// Correspondent-banking roles (each entity's Paladin funded operator):
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
	// The settlement agent is the SOURCE central bank, not the originating bank.
	IdentitySettlementAgent string

	// Spoke identity and currencies of the topology under test. These used to be
	// literals in the FX payload (spoke-a/spoke-b, USD/BRL) — names no toolkit
	// spoke carries, so the propose was rejected before reaching the chain.
	SpokeAID        string
	SpokeBID        string
	OriginCurrency  string
	CounterCurrency string

	// FX/HTLC amounts (origin leg on spoke-a, counter leg on spoke-b).
	OriginAmount  string
	CounterAmount string
	MintAmount    string

	// The suite neither provisions nor tears down: the toolkit is the single
	// provisioning path and it is driven from samples/. Both flags exist only to
	// reject the removed behaviour with an explanation instead of a stale target.
	SkipUp   bool
	SkipDown bool
	SkipMint bool // skip the CB mint step (operators already funded)
	Onboard  bool // run 3-phase PKI onboarding (ON by default; ONBOARD=0 to skip). Commercial banks are no longer pre-registered, so onboarding is what verifies them.
}

func loadConfig() *Config {
	return &Config{
		// No fallback ports. The defaults here used to be the legacy deploy/local
		// host-port mappings, which now answer nothing — so a bare `go test` failed
		// by connection-refused against an address that no longer means anything.
		// Unset is an explicit error instead (see requireComplete).
		BankAURL:        os.Getenv("API_GW_BANK_A_URL"),
		BankBURL:        os.Getenv("API_GW_BANK_B_URL"),
		BankCURL:        os.Getenv("API_GW_BANK_C_URL"),
		BankDURL:        os.Getenv("API_GW_BANK_D_URL"),
		CentralBankAURL: os.Getenv("API_GW_CENTRAL_BANK_A_URL"),
		CentralBankBURL: os.Getenv("API_GW_CENTRAL_BANK_B_URL"),

		// Any validator on a spoke can answer an evidence query; the always-up
		// central-bank node is what toolkit-env.sh derives.
		BesuSpokeAURL: os.Getenv("BESU_SPOKE_A_RPC"),
		BesuSpokeBURL: os.Getenv("BESU_SPOKE_B_RPC"),

		// Operator logins. These six pairs used to fall back to reading
		// KC_CLIENT_SECRET out of backend/config/.env.infra.<entity> — files the
		// legacy deploy/local bring-up wrote and the toolkit deliberately does not
		// (they only ever covered six reference entities; see
		// toolkit/engine/orchestrator/entityenv.go). The fallback is gone: the suite
		// now takes credentials only from the environment, which toolkit-env.sh
		// derives from the manifests the stack was actually provisioned with.
		//
		// Despite the KC_*_CLIENT / KC_*_SECRET names, what belongs here is a
		// Keycloak USERNAME and that user's PASSWORD. The login endpoint's
		// clientId/clientSecret JSON fields are the wire contract's names, not the
		// credential type: since f55ade5b the auth service accepts only the OIDC
		// password grant, and a realm client id/secret is refused on purpose.
		BankAClient: os.Getenv("KC_BANK_A_CLIENT"),
		BankASecret: os.Getenv("KC_BANK_A_SECRET"),
		BankBClient: os.Getenv("KC_BANK_B_CLIENT"),
		BankBSecret: os.Getenv("KC_BANK_B_SECRET"),
		BankCClient: os.Getenv("KC_BANK_C_CLIENT"),
		BankCSecret: os.Getenv("KC_BANK_C_SECRET"),
		BankDClient: os.Getenv("KC_BANK_D_CLIENT"),
		BankDSecret: os.Getenv("KC_BANK_D_SECRET"),
		CBAClient:   os.Getenv("KC_CENTRAL_BANK_A_CLIENT"),
		CBASecret:   os.Getenv("KC_CENTRAL_BANK_A_SECRET"),
		CBBClient:   os.Getenv("KC_CENTRAL_BANK_B_CLIENT"),
		CBBSecret:   os.Getenv("KC_CENTRAL_BANK_B_SECRET"),

		// Minting requires ROLE_TREASURY; the governance operators above cannot mint.
		CBATreasuryClient: os.Getenv("KC_CENTRAL_BANK_A_TREASURY_CLIENT"),
		CBATreasurySecret: os.Getenv("KC_CENTRAL_BANK_A_TREASURY_SECRET"),
		CBBTreasuryClient: os.Getenv("KC_CENTRAL_BANK_B_TREASURY_CLIENT"),
		CBBTreasurySecret: os.Getenv("KC_CENTRAL_BANK_B_TREASURY_SECRET"),

		// Also required, and for the same reason as the endpoints: the old defaults
		// named the legacy stack's Paladin nodes (spoke-a-bank-a …), which a toolkit
		// stack does not have. Left to default they would fail deep in a lock, far
		// from the cause.
		IdentityBankA:           os.Getenv("IDENTITY_BANK_A"),
		IdentityCorrespondentA:  os.Getenv("IDENTITY_CORRESPONDENT_A"),
		IdentityCustodian:       os.Getenv("IDENTITY_CUSTODIAN"),
		IdentityBankB:           os.Getenv("IDENTITY_BANK_B"),
		IdentitySettlementAgent: os.Getenv("IDENTITY_SETTLEMENT_AGENT"),

		SpokeAID:        os.Getenv("SPOKE_A_ID"),
		SpokeBID:        os.Getenv("SPOKE_B_ID"),
		OriginCurrency:  os.Getenv("FX_ORIGIN_CURRENCY"),
		CounterCurrency: os.Getenv("FX_COUNTER_CURRENCY"),

		OriginAmount:  envOr("FX_ORIGIN_AMOUNT", "100000"),
		CounterAmount: envOr("FX_COUNTER_AMOUNT", "520000"),
		MintAmount:    envOr("MINT_AMOUNT", "5000000"),

		// Default: assume a live stack. Setting either to 0 is now an error, not a
		// lifecycle request (see TestMain).
		SkipUp:   os.Getenv("SKIP_UP") != "0",
		SkipDown: os.Getenv("SKIP_DOWN") != "0",
		SkipMint: os.Getenv("SKIP_MINT") == "true",
		// Onboarding is part of the happy path and runs by default (idempotent: it
		// skips banks already ACTIVE). Set ONBOARD=0 to skip (e.g. faster reruns).
		Onboard: os.Getenv("ONBOARD") != "0",
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// requireComplete fails the run when an endpoint or an operator login is missing,
// rather than letting an empty string reach Keycloak and come back as a bare 401 —
// the failure mode that cost real debugging time against the old bring-up.
func (c *Config) requireComplete(t *testing.T) {
	t.Helper()
	required := []struct{ name, url string }{
		{"API_GW_BANK_A_URL", c.BankAURL},
		{"API_GW_BANK_B_URL", c.BankBURL},
		{"API_GW_BANK_C_URL", c.BankCURL},
		{"API_GW_BANK_D_URL", c.BankDURL},
		{"API_GW_CENTRAL_BANK_A_URL", c.CentralBankAURL},
		{"API_GW_CENTRAL_BANK_B_URL", c.CentralBankBURL},
		{"BESU_SPOKE_A_RPC", c.BesuSpokeAURL},
		{"BESU_SPOKE_B_RPC", c.BesuSpokeBURL},
		{"IDENTITY_BANK_A", c.IdentityBankA},
		{"IDENTITY_CORRESPONDENT_A", c.IdentityCorrespondentA},
		{"IDENTITY_CUSTODIAN", c.IdentityCustodian},
		{"IDENTITY_BANK_B", c.IdentityBankB},
		{"IDENTITY_SETTLEMENT_AGENT", c.IdentitySettlementAgent},
		{"SPOKE_A_ID", c.SpokeAID},
		{"SPOKE_B_ID", c.SpokeBID},
		{"FX_ORIGIN_CURRENCY", c.OriginCurrency},
		{"FX_COUNTER_CURRENCY", c.CounterCurrency},
	}
	var unset []string
	for _, e := range required {
		if e.url == "" {
			unset = append(unset, e.name)
		}
	}
	if len(unset) > 0 {
		t.Fatalf("unset required setting(s): %s.\n%s", strings.Join(unset, ", "), configHelp)
	}

	pairs := []struct{ name, user, pass string }{
		{"bank-a", c.BankAClient, c.BankASecret},
		{"bank-b", c.BankBClient, c.BankBSecret},
		{"bank-c", c.BankCClient, c.BankCSecret},
		{"bank-d", c.BankDClient, c.BankDSecret},
		{"central-bank-a", c.CBAClient, c.CBASecret},
		{"central-bank-b", c.CBBClient, c.CBBSecret},
		{"central-bank-a treasury", c.CBATreasuryClient, c.CBATreasurySecret},
		{"central-bank-b treasury", c.CBBTreasuryClient, c.CBBTreasurySecret},
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
	"    make scenario-a.test-integration        # derives and runs\n" +
	"    make scenario-a.test-integration-env    # prints what it would use\n" +
	"Bring a stack up first with the single provisioning path:\n" +
	"    cd samples && ./deploy-all.sh"

// scenarioARoot resolves scenario-a/ by walking up from this test file's location
// (tests/integration/ → scenario-a/).
func scenarioARoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "../.."
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}
