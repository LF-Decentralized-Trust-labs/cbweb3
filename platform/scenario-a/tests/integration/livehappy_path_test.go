// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package integration_test

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

var cfg *Config

func TestMain(m *testing.M) {
	cfg = loadConfig()

	if !cfg.SkipUp {
		fmt.Println("[scenario-a] SKIP_UP=0 — bringing the full stack up via `make spoke-all` (this WIPES the chain)...")
		if err := runMake("spoke-all"); err != nil {
			fmt.Printf("[scenario-a] `make spoke-all` failed: %v\n", err)
			os.Exit(1)
		}
	}

	code := m.Run()

	if !cfg.SkipDown {
		fmt.Println("[scenario-a] SKIP_DOWN=0 — tearing the stack down via `make spoke-all-down`...")
		if err := runMake("spoke-all-down"); err != nil {
			fmt.Printf("[scenario-a] `make spoke-all-down` failed: %v\n", err)
		}
	}

	os.Exit(code)
}

// runMake invokes a Makefile target from the scenario-a root.
func runMake(target string) error {
	cmd := exec.Command("make", target)
	cmd.Dir = scenarioARoot()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// TestFullHappyPath exercises the Scenario A core workflow end-to-end against a
// live stack:
//
//  0. Readiness     — all four gateways report /healthz
//  1. Login         — bank-a, bank-b, cb-a, cb-b obtain tokens
//  2. Onboard       — (opt-in, ONBOARD=1) idempotent PKI onboarding
//  3. Mint          — central banks fund their operators
//  4. FX propose    — bank-a proposes; persistence + audit asserted
//  5. Cross-spoke   — proposal mirrors to bank-b → accept → ACCEPTED mirrors back
//  6. HTLC lock     — initiator (spoke-a) then responder-by-hash (spoke-b)
//  7. Settle        — bank-a reveals the secret; relay settles the responder leg
//  8. Verify        — both legs SETTLED (atomic, no orphaned escrow)
//
// Each phase is a subtest so it can be run selectively with -run, and so a failure
// is attributed to a named step.
func TestFullHappyPath(t *testing.T) {
	gateways := map[string]string{
		"bank-a":         cfg.BankAURL,
		"bank-b":         cfg.BankBURL,
		"bank-d":         cfg.BankDURL,
		"central-bank-a": cfg.CentralBankAURL,
		"central-bank-b": cfg.CentralBankBURL,
	}

	// Shared state threaded across phases.
	//   bankA — originator (spoke-a)        bankB — beneficiary (spoke-b)
	//   bankD — custodian (spoke-b)         cbA/cbB — central banks
	var (
		bankA, bankB, bankD, cbA, cbB *httpClient
		tradeID                       string
		contractIDA                   string
		contractIDB                   string
		secret                        string
	)

	t.Run("Phase0_Readiness", func(t *testing.T) {
		for name, url := range gateways {
			waitForHTTP(t, name, url, 3*time.Minute)
		}
	})

	t.Run("Phase1_Login", func(t *testing.T) {
		requireSecret(t, "bank-a", cfg.BankASecret)
		requireSecret(t, "bank-b", cfg.BankBSecret)
		requireSecret(t, "bank-d", cfg.BankDSecret)
		requireSecret(t, "central-bank-a", cfg.CBASecret)
		requireSecret(t, "central-bank-b", cfg.CBBSecret)

		bankA = newHTTPClient(cfg.BankAURL, gatewayLogin(t, "bank-a", cfg.BankAURL, cfg.BankAClient, cfg.BankASecret))
		bankB = newHTTPClient(cfg.BankBURL, gatewayLogin(t, "bank-b", cfg.BankBURL, cfg.BankBClient, cfg.BankBSecret))
		bankD = newHTTPClient(cfg.BankDURL, gatewayLogin(t, "bank-d", cfg.BankDURL, cfg.BankDClient, cfg.BankDSecret))
		cbA = newHTTPClient(cfg.CentralBankAURL, gatewayLogin(t, "central-bank-a", cfg.CentralBankAURL, cfg.CBAClient, cfg.CBASecret))
		cbB = newHTTPClient(cfg.CentralBankBURL, gatewayLogin(t, "central-bank-b", cfg.CentralBankBURL, cfg.CBBClient, cfg.CBBSecret))
	})

	t.Run("Phase2_Onboard", func(t *testing.T) {
		if !cfg.Onboard {
			t.Skip("onboarding disabled (set ONBOARD=1 to run against a freshly nuked stack)")
		}
		requireLoggedIn(t, bankA)
		onboardBank(t, bankA, cbA, "bank-a", "Bank A", "US")
		onboardBank(t, bankB, cbB, "bank-b", "Bank B", "BR")
	})

	t.Run("Phase3_Mint", func(t *testing.T) {
		if cfg.SkipMint {
			t.Skip("mint disabled (SKIP_MINT=true)")
		}
		// /token/mint requires ROLE_TREASURY, which lives on a dedicated treasury
		// client — the plain central-bank client (used elsewhere) holds only
		// ROLE_GOVERNANCE and would get a 403 here.
		cbATreasury := newHTTPClient(cfg.CentralBankAURL,
			gatewayLogin(t, "cb-a-treasury", cfg.CentralBankAURL, cfg.CBATreasuryClient, cfg.CBATreasurySecret))
		cbBTreasury := newHTTPClient(cfg.CentralBankBURL,
			gatewayLogin(t, "cb-b-treasury", cfg.CentralBankBURL, cfg.CBBTreasuryClient, cfg.CBBTreasurySecret))

		mint := func(name string, c *httpClient, to string) {
			err := c.post("/api/v1/token/mint", map[string]string{"to": to, "amount": cfg.MintAmount}, nil)
			if err != nil {
				t.Fatalf("[%s] mint to %s: %v", name, to, err)
			}
			t.Logf("  [%s] minted %s to %s", name, cfg.MintAmount, to)
		}
		// cb-a funds the originator (bank-a) so it can lock the origin leg.
		// cb-b funds the CUSTODIAN (bank-d) — it locks the counter leg on spoke-b,
		// not the beneficiary bank-b (which only receives on settlement).
		mint("central-bank-a", cbATreasury, cfg.IdentityBankA)
		mint("central-bank-b", cbBTreasury, cfg.IdentityCustodian)
	})

	t.Run("Phase4_FXPropose", func(t *testing.T) {
		requireLoggedIn(t, bankA)
		tradeID = "TRADE-" + strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)

		// Correspondent-banking roles: bank-a originates; the spoke-b counterparty
		// that accepts and settles on-chain is the CUSTODIAN (bank-d); the end
		// recipient is the BENEFICIARY (bank-b). Origin leg lands with the spoke-a
		// financial correspondent (bank-c); counter leg lands with the beneficiary.
		body := map[string]interface{}{
			"trade_id":         tradeID,
			"counterparty_b":   "bank-d",
			"originator":       "bank-a",
			"settlement_agent": "bank-a",
			"custodian":        "bank-d",
			"beneficiary":      "bank-b",
			"origin_amount":    cfg.OriginAmount,
			"counter_amount":   cfg.CounterAmount,
			"origin_currency":  "USD",
			"counter_currency": "BRL",
			"rate":             "5.2",
			"expiry_date":      time.Now().Add(time.Hour).Unix(),
			"spoke_a_receiver": cfg.IdentityCorrespondentA,
			"spoke_b_receiver": cfg.IdentityBankB,
			"on_behalf":        false,
		}
		bankA.mustPost(t, "/api/v1/payments/fx/agreements", body, nil)
		t.Logf("  FX proposed: %s", tradeID)

		// Persistence: the agreement reads back in a PROPOSED state.
		state, _ := fxState(bankA, tradeID)
		if !isFXState(state, "PROPOSED") {
			t.Fatalf("unexpected FX state after propose: %q", state)
		}

		// Audit: at least one lifecycle event was recorded.
		var audit struct {
			Total int `json:"total"`
		}
		bankA.mustGet(t, "/api/v1/payments/fx/agreements/"+tradeID+"/audit", &audit)
		if audit.Total < 1 {
			t.Fatalf("expected >=1 audit event after propose, got %d", audit.Total)
		}
		t.Logf("  persistence + audit OK (%d event(s))", audit.Total)
	})

	t.Run("Phase5_CrossSpokeSync", func(t *testing.T) {
		requireTradeID(t, tradeID)

		// The relay mirrors the proposal to the custodian (bank-d) on spoke-b,
		// which is the counterparty that accepts and will settle on-chain.
		pollUntil(t, 3*time.Second, 60*time.Second, func() (bool, error) {
			state, _ := fxState(bankD, tradeID)
			return isFXState(state, "PROPOSED") || isFXState(state, "ACCEPTED"), nil
		})
		t.Logf("  proposal visible on custodian bank-d")

		bankD.mustPost(t, "/api/v1/payments/fx/agreements/"+tradeID+"/accept", map[string]bool{"on_behalf": false}, nil)

		// Acceptance mirrors back to bank-a.
		pollUntil(t, 3*time.Second, 60*time.Second, func() (bool, error) {
			state, _ := fxState(bankA, tradeID)
			return isFXState(state, "ACCEPTED"), nil
		})
		t.Logf("  cross-spoke sync confirmed: ACCEPTED on both spokes")
	})

	t.Run("Phase6_HTLCLock", func(t *testing.T) {
		requireTradeID(t, tradeID)

		// Initiator lock on spoke-a — the orchestrator generates the secret +
		// hashlock and returns BOTH in the lock response. The secret is a one-time
		// disclosure to the creator (never re-exposed via /htlc/status), so it must
		// be captured here.
		var lockA struct {
			ContractID string `json:"contract_id"`
			HashLock   string `json:"hash_lock"`
			Secret     string `json:"secret"`
		}
		// Originator bank-a locks the origin leg; on settlement it releases to the
		// spoke-a financial correspondent (bank-c).
		bankA.mustPost(t, "/api/v1/htlc/lock", map[string]string{
			"agreement_id": tradeID,
			"receiver":     cfg.IdentityCorrespondentA,
			"amount":       cfg.OriginAmount,
		}, &lockA)
		contractIDA = lockA.ContractID
		if contractIDA == "" {
			t.Fatal("initiator lock returned empty contract_id")
		}
		if lockA.HashLock == "" {
			t.Fatal("hash_lock missing on initiator lock response")
		}
		if lockA.Secret == "" {
			t.Fatal("secret missing on initiator lock response")
		}
		secret = lockA.Secret
		t.Logf("  originator locked on spoke-a: contract=%s", contractIDA)

		// Responder lock on spoke-b: the CUSTODIAN (bank-d) locks the counter leg
		// under the SAME hashlock, with the BENEFICIARY (bank-b) as receiver.
		var lockB struct {
			ContractID string `json:"contract_id"`
		}
		bankD.mustPost(t, "/api/v1/htlc/lock-with-hash", map[string]string{
			"agreement_id": tradeID,
			"receiver":     cfg.IdentityBankB,
			"amount":       cfg.CounterAmount,
			"hash_lock":    lockA.HashLock,
		}, &lockB)
		contractIDB = lockB.ContractID
		if contractIDB == "" {
			t.Fatal("responder lock returned empty contract_id")
		}

		var statusB htlcStatus
		bankD.mustGet(t, "/api/v1/htlc/status/"+contractIDB, &statusB)
		if !isHTLCState(statusB.State, "LOCKED") {
			t.Fatalf("responder leg is not LOCKED: %q", statusB.State)
		}
		t.Logf("  custodian bank-d locked on spoke-b: contract=%s", contractIDB)

		// Atomicity invariant: the responder's timelock must be SHORTER than the
		// initiator's so the initiator always has time to settle after observing
		// the responder lock. Soft-checked — only asserted when both are exposed.
		var statusA htlcStatus
		bankA.mustGet(t, "/api/v1/htlc/status/"+contractIDA, &statusA)
		if !isHTLCState(statusA.State, "LOCKED") {
			t.Fatalf("initiator leg is not LOCKED: %q", statusA.State)
		}
		if statusA.TimeLock > 0 && statusB.TimeLock > 0 {
			if statusB.TimeLock >= statusA.TimeLock {
				t.Fatalf("timelock invariant violated: responder %d >= initiator %d", statusB.TimeLock, statusA.TimeLock)
			}
			t.Logf("  timelock invariant OK: responder %d < initiator %d", statusB.TimeLock, statusA.TimeLock)
		}
	})

	t.Run("Phase7_Settle", func(t *testing.T) {
		requireContracts(t, contractIDA, contractIDB)

		// Reveal the secret on spoke-a. The orchestrator gates this on having seen
		// (via the relay) that the counterparty spoke locked its leg — the relay
		// records lock events on a poll interval, so right after the responder lock
		// the precondition is briefly unmet. Retry until it clears; any other error
		// fails immediately.
		pollUntil(t, 3*time.Second, 90*time.Second, func() (bool, error) {
			err := bankA.post("/api/v1/htlc/settle", map[string]string{
				"contract_id": contractIDA,
				"secret":      secret,
			}, nil)
			if err == nil {
				return true, nil
			}
			if strings.Contains(err.Error(), "counterparty spoke has not yet locked") {
				return false, nil // transient — relay has not confirmed the counterparty lock yet
			}
			return false, err
		})
		t.Logf("  initiator settled on spoke-a")

		// Relay bridges the secret and settles the custodian's leg on spoke-b.
		pollHTLCSettled(t, bankD, contractIDB, "custodian bank-d")
		t.Logf("  relay settled custodian leg on spoke-b")
	})

	t.Run("Phase8_Verify", func(t *testing.T) {
		requireContracts(t, contractIDA, contractIDB)

		var sA, sB htlcStatus
		bankA.mustGet(t, "/api/v1/htlc/status/"+contractIDA, &sA)
		bankD.mustGet(t, "/api/v1/htlc/status/"+contractIDB, &sB)
		if !isHTLCState(sA.State, "SETTLED") {
			t.Fatalf("initiator leg not SETTLED: %q", sA.State)
		}
		if !isHTLCState(sB.State, "SETTLED") {
			t.Fatalf("responder leg not SETTLED: %q", sB.State)
		}
		t.Logf("  both legs SETTLED — atomic cross-spoke settlement complete")
	})
}

func requireSecret(t *testing.T, name, secret string) {
	t.Helper()
	if secret == "" {
		t.Fatalf("missing Keycloak client secret for %s — set KC_%s_SECRET or ensure backend/config/.env.infra.%s has KC_CLIENT_SECRET",
			name, envKey(name), name)
	}
}

func envKey(entity string) string {
	out := make([]byte, 0, len(entity))
	for i := 0; i < len(entity); i++ {
		ch := entity[i]
		if ch == '-' {
			ch = '_'
		} else if ch >= 'a' && ch <= 'z' {
			ch -= 'a' - 'A'
		}
		out = append(out, ch)
	}
	return string(out)
}

func requireLoggedIn(t *testing.T, c *httpClient) {
	t.Helper()
	if c == nil {
		t.Fatal("not logged in — Phase1_Login must run first (do not exclude it with -run)")
	}
}

func requireTradeID(t *testing.T, tradeID string) {
	t.Helper()
	if tradeID == "" {
		t.Fatal("no trade_id — Phase4_FXPropose must run first")
	}
}

func requireContracts(t *testing.T, a, b string) {
	t.Helper()
	if a == "" || b == "" {
		t.Fatal("missing HTLC contract ids — Phase6_HTLCLock must run first")
	}
}
