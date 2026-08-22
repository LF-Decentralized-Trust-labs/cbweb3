// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package integration_test

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

var cfg *Config

// evidence records per-step on-chain artifacts (tx_hash, block_number, gas) for
// the machine-readable evidence bundle. Initialized in TestMain.
var evidence *evidenceRecorder

func TestMain(m *testing.M) {
	cfg = loadConfig()
	evidence = newEvidenceRecorder(map[string]string{
		"spoke-a": cfg.BesuSpokeAURL,
		"spoke-b": cfg.BesuSpokeBURL,
	})

	// The suite no longer provisions. SKIP_UP=0 used to run `make spoke-all`, the
	// legacy deploy/local bring-up; that target is gone, so the branch could only
	// fail with "No rule to make target". The toolkit is the single provisioning
	// path and it is driven from samples/, never from a test.
	if !cfg.SkipUp {
		fmt.Println("[scenario-a] SKIP_UP=0 is no longer supported: this suite does not provision.")
		fmt.Println("[scenario-a] Bring a stack up first:  cd samples && ./deploy-all.sh")
		os.Exit(1)
	}

	code := m.Run()

	// Emit the harness evidence file (tx_hash/block_number/gas per step) for
	// tools/gen_evidence_bundles.py to fold into evidence-bundle-<run-id>/.
	if path, err := evidence.flush("e2e-scenario-a",
		"scenario-a TestFullHappyPath (go test -tags integration)", code == 0); err != nil {
		fmt.Printf("[scenario-a] evidence flush failed: %v\n", err)
	} else {
		fmt.Printf("[scenario-a] wrote on-chain evidence: %s\n", path)
	}

	// Teardown is the operator's call for the same reason: `make spoke-all-down` was
	// the legacy path's, and the toolkit's stack is torn down from samples/.
	if !cfg.SkipDown {
		fmt.Println("[scenario-a] SKIP_DOWN=0 is no longer supported: this suite does not tear stacks down.")
	}

	os.Exit(code)
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
	cfg.requireComplete(t)

	gateways := map[string]string{
		"bank-a":         cfg.BankAURL,
		"bank-b":         cfg.BankBURL,
		"bank-c":         cfg.BankCURL,
		"bank-d":         cfg.BankDURL,
		"central-bank-a": cfg.CentralBankAURL,
		"central-bank-b": cfg.CentralBankBURL,
	}

	// Shared state threaded across phases.
	//   bankA — originator (spoke-a)        bankB — beneficiary (spoke-b)
	//   bankC — correspondent (spoke-a)     bankD — custodian (spoke-b)
	//   cbA/cbB — central banks
	var (
		bankA, bankB, bankC, bankD, cbA, cbB *httpClient
		tradeID                              string
		contractIDA                          string
		contractIDB                          string
		secret                               string
	)

	t.Run("Phase0_Readiness", func(t *testing.T) {
		start := time.Now()
		for name, url := range gateways {
			waitForHTTP(t, name, url, 3*time.Minute)
		}
		evidence.record("E2E-A-03", "phase_0_readiness", 200, time.Since(start), !t.Failed(), "")
	})

	t.Run("Phase1_Login", func(t *testing.T) {
		start := time.Now()
		defer func() { evidence.record("E2E-A-03", "phase_1_login", 200, time.Since(start), !t.Failed(), "") }()
		requireSecret(t, "bank-a", cfg.BankASecret)
		requireSecret(t, "bank-b", cfg.BankBSecret)
		requireSecret(t, "bank-c", cfg.BankCSecret)
		requireSecret(t, "bank-d", cfg.BankDSecret)
		requireSecret(t, "central-bank-a", cfg.CBASecret)
		requireSecret(t, "central-bank-b", cfg.CBBSecret)

		bankA = newHTTPClient(cfg.BankAURL, gatewayLogin(t, "bank-a", cfg.BankAURL, cfg.BankAClient, cfg.BankASecret))
		bankB = newHTTPClient(cfg.BankBURL, gatewayLogin(t, "bank-b", cfg.BankBURL, cfg.BankBClient, cfg.BankBSecret))
		bankC = newHTTPClient(cfg.BankCURL, gatewayLogin(t, "bank-c", cfg.BankCURL, cfg.BankCClient, cfg.BankCSecret))
		bankD = newHTTPClient(cfg.BankDURL, gatewayLogin(t, "bank-d", cfg.BankDURL, cfg.BankDClient, cfg.BankDSecret))
		cbA = newHTTPClient(cfg.CentralBankAURL, gatewayLogin(t, "central-bank-a", cfg.CentralBankAURL, cfg.CBAClient, cfg.CBASecret))
		cbB = newHTTPClient(cfg.CentralBankBURL, gatewayLogin(t, "central-bank-b", cfg.CentralBankBURL, cfg.CBBClient, cfg.CBBSecret))
	})

	t.Run("Phase2_Onboard", func(t *testing.T) {
		if !cfg.Onboard {
			evidence.recordSkipped("E2E-A-03", "phase_2_onboard")
			t.Skip("onboarding disabled (ONBOARD=0)")
		}
		start := time.Now()
		var refs []txRef
		defer func() {
			evidence.record("E2E-A-03", "phase_2_onboard", 201, time.Since(start), !t.Failed(), "", refs...)
		}()
		requireLoggedIn(t, bankA)

		// All four commercial banks must be verified in the IdentityRegistry: the
		// HTLC lock requires onlyVerified(msg.sender) AND onlyVerified(receiver), so
		// each leg needs both its locker and its receiver registered.
		//   spoke-a: bank-a (origin locker) + bank-c (origin receiver) via cb-a
		//   spoke-b: bank-d (counter locker) + bank-b (counter receiver) via cb-b
		// Onboarding (with KMS seeded to the operator key) registers exactly the
		// address the orchestrator signs with. Idempotent: ACTIVE banks are skipped.
		// Each approve-kyc mines an IdentityRegistry.setParticipant tx on the home
		// CB's spoke; capture it for the evidence bundle (empty when already ACTIVE).
		addReg := func(network, bankCode, hash string) {
			if hash != "" {
				refs = append(refs, txRef{network: network, label: "kyc_register_" + bankCode, hash: hash})
			}
		}
		addReg("spoke-a", "bank-a", onboardBank(t, bankA, cbA, "bank-a", "Bank A", "US"))
		addReg("spoke-a", "bank-c", onboardBank(t, bankC, cbA, "bank-c", "Bank C", "US"))
		addReg("spoke-b", "bank-b", onboardBank(t, bankB, cbB, "bank-b", "Bank B", "BR"))
		addReg("spoke-b", "bank-d", onboardBank(t, bankD, cbB, "bank-d", "Bank D", "BR"))
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

		start := time.Now()
		corr := newCorrID()
		var refs []txRef
		mint := func(name, network string, c *httpClient, to string) {
			var res struct {
				TxHash string `json:"tx_hash"`
			}
			err := c.withCorr(corr).post("/api/v1/token/mint", map[string]string{"to": to, "amount": cfg.MintAmount}, &res)
			if err != nil {
				t.Fatalf("[%s] mint to %s: %v", name, to, err)
			}
			refs = append(refs, txRef{network: network, label: name + "_mint", hash: res.TxHash})
			t.Logf("  [%s] minted %s to %s (tx=%s)", name, cfg.MintAmount, to, res.TxHash)
		}
		// cb-a funds the originator (bank-a) so it can lock the origin leg.
		// cb-b funds the CUSTODIAN (bank-d) — it locks the counter leg on spoke-b,
		// not the beneficiary bank-b (which only receives on settlement).
		mint("central-bank-a", "spoke-a", cbATreasury, cfg.IdentityBankA)
		mint("central-bank-b", "spoke-b", cbBTreasury, cfg.IdentityCustodian)
		evidence.record("E2E-A-03", "phase_3_mint", 201, time.Since(start), !t.Failed(), corr, refs...)
	})

	t.Run("Phase4_FXPropose", func(t *testing.T) {
		start := time.Now()
		var refs []txRef
		defer func() {
			evidence.record("E2E-A-03", "phase_4_fx_propose", 201, time.Since(start), !t.Failed(), "", refs...)
		}()
		requireLoggedIn(t, bankA)
		tradeID = "TRADE-" + strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)

		// Correspondent-banking roles: bank-a originates; the spoke-b counterparty
		// that accepts and settles on-chain is the CUSTODIAN (bank-d); the end
		// recipient is the BENEFICIARY (bank-b). Origin leg lands with the spoke-a
		// financial correspondent (bank-c); counter leg lands with the beneficiary.
		body := map[string]interface{}{
			"trade_id": tradeID,
			// Every party is a PALADIN IDENTITY, not a bank code. The api-gateway
			// checks counterparty_b / settlement_agent / custodian / beneficiary /
			// source_receiver / dest_receiver against the Pente roster before
			// proposing, and the orchestrator resolves the bilateral Pente group by
			// {originator, counterparty} membership — a bank code matches neither.
			// These were bank-a/bank-b/bank-d, the legacy stack's codes, which the
			// roster rejected with HTTP 400 "not members of the Paladin roster".
			"counterparty_b":   cfg.IdentityCustodian,
			"originator":       cfg.IdentityBankA,
			"settlement_agent": cfg.IdentitySettlementAgent,
			"custodian":        cfg.IdentityCustodian,
			"beneficiary":      cfg.IdentityBankB,
			"origin_amount":    cfg.OriginAmount,
			"counter_amount":   cfg.CounterAmount,
			"origin_currency":  cfg.OriginCurrency,
			"counter_currency": cfg.CounterCurrency,
			"rate":             "5.2",
			"expiry_date":      time.Now().Add(time.Hour).Unix(),
			"source_spoke_id":  cfg.SpokeAID,
			"dest_spoke_id":    cfg.SpokeBID,
			"source_receiver":  cfg.IdentityCorrespondentA,
			"dest_receiver":    cfg.IdentityBankB,
			"on_behalf":        false,
		}
		bankA.mustPost(t, "/api/v1/payments/fx/agreements", body, nil)
		t.Logf("  FX proposed: %s", tradeID)

		// Persistence: the agreement reads back in a PROPOSED state.
		state, _ := fxState(bankA, tradeID)
		if !isFXState(state, "PROPOSED") {
			t.Fatalf("unexpected FX state after propose: %q", state)
		}

		// Audit: at least one lifecycle event was recorded. Each event carries the
		// on-chain FXAgreement tx hash; the PROPOSED transition is the propose() tx
		// mined on spoke-a — capture it for the evidence bundle (best-effort).
		var audit fxAuditResponse
		bankA.mustGet(t, "/api/v1/payments/fx/agreements/"+tradeID+"/audit", &audit)
		if audit.Total < 1 {
			t.Fatalf("expected >=1 audit event after propose, got %d", audit.Total)
		}
		if h := audit.txHashForState("PROPOSED"); h != "" {
			refs = append(refs, txRef{network: "spoke-a", label: "fx_propose", hash: h})
		}
		t.Logf("  persistence + audit OK (%d event(s))", audit.Total)
	})

	t.Run("Phase5_CrossSpokeSync", func(t *testing.T) {
		start := time.Now()
		var refs []txRef
		defer func() {
			evidence.record("E2E-A-03", "phase_5_cross_spoke_sync", 200, time.Since(start), !t.Failed(), "", refs...)
		}()
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

		// The custodian's accept() is mined on spoke-b; its tx hash surfaces in the
		// bank-d audit trail as the ACCEPTED transition — capture it (best-effort).
		var audit fxAuditResponse
		if err := bankD.get("/api/v1/payments/fx/agreements/"+tradeID+"/audit", &audit); err == nil {
			if h := audit.txHashForState("ACCEPTED"); h != "" {
				refs = append(refs, txRef{network: "spoke-b", label: "fx_accept", hash: h})
			}
		}
	})

	t.Run("Phase6_HTLCLock", func(t *testing.T) {
		start := time.Now()
		corr := newCorrID()
		var refs []txRef
		defer func() {
			evidence.record("E2E-A-03", "phase_6_htlc_lock", 201, time.Since(start), !t.Failed(), corr, refs...)
			evidence.setContracts("phase_6_htlc_lock", contractIDA, contractIDB)
		}()
		requireTradeID(t, tradeID)

		// Initiator lock on spoke-a — the orchestrator generates the secret +
		// hashlock and returns BOTH in the lock response. The secret is a one-time
		// disclosure to the creator (never re-exposed via /htlc/status), so it must
		// be captured here. htlc_tx_hash is the Besu EVM tx; zeto_tx_hash is the
		// Paladin/Zeto private-transfer id (no EVM receipt).
		var lockA struct {
			ContractID string `json:"contract_id"`
			HashLock   string `json:"hash_lock"`
			Secret     string `json:"secret"`
			HTLCTxHash string `json:"htlc_tx_hash"`
			ZetoTxHash string `json:"zeto_tx_hash"`
		}
		// Originator bank-a locks the origin leg; on settlement it releases to the
		// spoke-a financial correspondent (bank-c).
		bankA.withCorr(corr).mustPost(t, "/api/v1/htlc/lock", map[string]string{
			"agreement_id": tradeID,
			"receiver":     cfg.IdentityCorrespondentA,
			"amount":       cfg.OriginAmount,
		}, &lockA)
		refs = append(refs,
			txRef{network: "spoke-a", label: "htlc_lock_origin", hash: lockA.HTLCTxHash},
			txRef{network: "spoke-a", label: "zeto_lock_origin", hash: lockA.ZetoTxHash})
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
			HTLCTxHash string `json:"htlc_tx_hash"`
			ZetoTxHash string `json:"zeto_tx_hash"`
		}
		bankD.withCorr(corr).mustPost(t, "/api/v1/htlc/lock-with-hash", map[string]string{
			"agreement_id": tradeID,
			"receiver":     cfg.IdentityBankB,
			"amount":       cfg.CounterAmount,
			"hash_lock":    lockA.HashLock,
		}, &lockB)
		refs = append(refs,
			txRef{network: "spoke-b", label: "htlc_lock_counter", hash: lockB.HTLCTxHash},
			txRef{network: "spoke-b", label: "zeto_lock_counter", hash: lockB.ZetoTxHash})
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
		start := time.Now()
		corr := newCorrID()
		var refs []txRef
		defer func() {
			evidence.record("E2E-A-03", "phase_7_settle", 200, time.Since(start), !t.Failed(), corr, refs...)
		}()
		requireContracts(t, contractIDA, contractIDB)

		// Reveal the secret on spoke-a. The orchestrator gates this on having seen
		// (via the relay) that the counterparty spoke locked its leg — the relay
		// records lock events on a poll interval, so right after the responder lock
		// the precondition is briefly unmet. Retry until it clears; any other error
		// fails immediately.
		var settle struct {
			HTLCTxHash string `json:"htlc_tx_hash"`
			ZetoTxHash string `json:"zeto_tx_hash"`
		}
		pollUntil(t, 3*time.Second, 90*time.Second, func() (bool, error) {
			err := bankA.withCorr(corr).post("/api/v1/htlc/settle", map[string]string{
				"contract_id": contractIDA,
				"secret":      secret,
			}, &settle)
			if err == nil {
				return true, nil
			}
			if strings.Contains(err.Error(), "counterparty spoke has not yet locked") {
				return false, nil // transient — relay has not confirmed the counterparty lock yet
			}
			return false, err
		})
		refs = append(refs,
			txRef{network: "spoke-a", label: "htlc_settle_origin", hash: settle.HTLCTxHash},
			txRef{network: "spoke-a", label: "zeto_settle_origin", hash: settle.ZetoTxHash})
		t.Logf("  initiator settled on spoke-a")

		// Relay bridges the secret and settles the custodian's leg on spoke-b.
		pollHTLCSettled(t, bankD, contractIDB, "custodian bank-d")
		t.Logf("  relay settled custodian leg on spoke-b")
	})

	t.Run("Phase8_Verify", func(t *testing.T) {
		start := time.Now()
		defer func() { evidence.record("E2E-A-03", "phase_8_verify", 200, time.Since(start), !t.Failed(), "") }()
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
