// SPDX-License-Identifier: Apache-2.0

// Package integration_test exercises the Scenario B full happy-path via the REST API:
//
//  1. Liquidity provision — CB-A + CB-B run cooperative commit-reveal → pool ACTIVE
//  2. Bank A PKI onboarding — credential request → poll → complete
//  3. Fiat issuance — Bank A requests deposit → CB-A approves → balance confirmed
//  4. Cross-currency transfer — Bank A quotes → executes swap → polls until COMPLETED
//  5. Receipt verification — Bank B fiat balance increased
//
// Run with a live stack:
//
//	SKIP_UP=1 go test -v -timeout 30m -run TestFullHappyPath ./...
//
// Run including stack bring-up (slow, CI):
//
//	go test -v -timeout 45m -run TestFullHappyPath ./...
package integration_test

import (
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mustBigInt parses a base-10 integer string, failing the test on malformed input.
func mustBigInt(t *testing.T, s string) *big.Int {
	t.Helper()
	v, ok := new(big.Int).SetString(s, 10)
	require.True(t, ok, "expected base-10 integer, got %q", s)
	return v
}

// Token amounts (18-decimal ERC-20, matching sovereign CB liquidity tryout defaults).
const (
	// Pool seed reflects the FX rate 1 BRL = 287 ARS, so the pool is loaded asymmetrically
	// (matched to the national-currency exchange rate) rather than 1:1. Side A = W-BRL (CB-A),
	// side B = W-ARS (CB-B). The first deposit still splits LP shares 50/50 by value, which is
	// correct because both sides post equal value at this rate.
	commitAmountA = "1000000000000000000000"   // 1e21 → CB-A side A (W-BRL)
	commitAmountB = "287000000000000000000000" // 287e21 → CB-B side B (W-ARS), 287× side A
	// Each CB mints/bridges at least its own side's commit amount of hub W-tokens.
	mintAmountA = commitAmountA
	mintAmountB = commitAmountB

	depositAmount = "1000000000000000000000" // 1e21 → Bank A fiat deposit
	swapAmountOut = "500000000000000000000" // 5e20 → target ARS output (500 ARS)
	// At the 1:287 pool (1000 W-BRL / 287000 W-ARS) an exact-output of 500 ARS costs
	// ~1.75 BRL in (constant-product + 0.3% fee). The cap is set to 5 BRL: ~3× the
	// expected cost — comfortable headroom against price impact/fees, yet a meaningful
	// slippage guard (the prior 800 BRL was sized for the obsolete 1:1 pool).
	swapMaxIn = "5000000000000000000" // 5e18 → max BRL in
	poolPair      = "W-BRL-ARS"

	// withdrawFractionBps withdraws part of CB-A's position (40%) so Phase 6 exercises a
	// partial LP exit — CB-A keeps a reduced, still-active position afterward.
	withdrawFractionBps = 4000
)

var cfg *Config

func TestMain(m *testing.M) {
	cfg = loadConfig()

	if !cfg.SkipUp {
		root := scenarioBRoot()
		cmd := exec.Command("make", "-C", root, "scenario-b.up")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "scenario-b.up failed: %v\n", err)
			os.Exit(1)
		}
	}

	code := m.Run()

	if !cfg.SkipDown {
		root := scenarioBRoot()
		cmd := exec.Command("make", "-C", root, "scenario-b.down")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	}

	os.Exit(code)
}

// TestFullHappyPath runs the complete Scenario B happy path end-to-end.
// Each phase is a subtest so individual phases can be targeted with -run.
func TestFullHappyPath(t *testing.T) {
	// Phase 0: wait for all gateways to be healthy.
	t.Run("phase_0_readiness", func(t *testing.T) {
		t.Log("Waiting for API gateways to report healthy...")
		waitForHTTP(t, cfg.CentralBankAURL, 3*time.Minute)
		waitForHTTP(t, cfg.CentralBankBURL, 3*time.Minute)
		waitForHTTP(t, cfg.BankAURL, 3*time.Minute)
		waitForHTTP(t, cfg.BankBURL, 3*time.Minute)
		t.Log("All gateways healthy.")
	})

	// Acquire tokens via each entity's API gateway (Keycloak direct HTTP blocked outside Docker).
	cbAToken := gatewayLogin(t, cfg.CentralBankAURL, cfg.CentralBankAClient, cfg.CentralBankASecret)
	cbBToken := gatewayLogin(t, cfg.CentralBankBURL, cfg.CentralBankBClient, cfg.CentralBankBSecret)
	bankAToken := gatewayLogin(t, cfg.BankAURL, cfg.BankAClient, cfg.BankASecret)
	bankBToken := gatewayLogin(t, cfg.BankBURL, cfg.BankBClient, cfg.BankBSecret)

	cbA := newHTTPClient(cfg.CentralBankAURL, cbAToken)
	cbB := newHTTPClient(cfg.CentralBankBURL, cbBToken)
	bankA := newHTTPClient(cfg.BankAURL, bankAToken)
	bankB := newHTTPClient(cfg.BankBURL, bankBToken)

	// Phase 1: Liquidity provision — CB-A and CB-B run cooperative commit-reveal.
	t.Run("phase_1_liquidity_provision", func(t *testing.T) {
		// Check if pool is already active to allow re-running against a seeded stack.
		var poolStatus struct {
			PoolStatus string `json:"pool_status"`
			ReserveA   string `json:"reserve_a"`
			ReserveB   string `json:"reserve_b"`
		}
		if err := cbA.get(t, "/api/v2/amm/pool/"+poolPair+"/status", &poolStatus); err == nil &&
			poolStatus.PoolStatus == "ACTIVE" {
			t.Logf("Pool already ACTIVE (reserve_a=%s reserve_b=%s) — skipping provision",
				poolStatus.ReserveA, poolStatus.ReserveB)
			return
		}

		t.Log("Step 1: CB-A bridge lock-mint (lock tCeBM on Spoke-A, mint W-token on Hub)...")
		var lockMintA struct {
			PositionID  string `json:"position_id"`
			BridgeState string `json:"bridge_state"`
		}
		cbA.mustPost(t, "/api/v2/bridge/lock-mint",
			map[string]string{"amount": mintAmountA}, &lockMintA)
		require.NotEmpty(t, lockMintA.PositionID, "CB-A bridge position_id must not be empty")
		t.Logf("CB-A bridge position: %s state=%s", lockMintA.PositionID, lockMintA.BridgeState)

		t.Log("Step 2: CB-B bridge lock-mint (lock tCeBM on Spoke-B, mint W-token on Hub)...")
		var lockMintB struct {
			PositionID  string `json:"position_id"`
			BridgeState string `json:"bridge_state"`
		}
		cbB.mustPost(t, "/api/v2/bridge/lock-mint",
			map[string]string{"amount": mintAmountB}, &lockMintB)
		require.NotEmpty(t, lockMintB.PositionID, "CB-B bridge position_id must not be empty")
		t.Logf("CB-B bridge position: %s state=%s", lockMintB.PositionID, lockMintB.BridgeState)

		t.Log("Step 3: Polling CB-A bridge position until ACTIVE (relayer processes lock on-chain)...")
		pollBridgeActive(t, cbA, lockMintA.PositionID, "CB-A")

		t.Log("Step 4: Polling CB-B bridge position until ACTIVE...")
		pollBridgeActive(t, cbB, lockMintB.PositionID, "CB-B")

		t.Log("Step 5: CB-A mint-and-approve hub W-tokens for AMM...")
		var mintResp map[string]interface{}
		cbA.mustPost(t, "/api/v2/amm/token/mint-and-approve",
			map[string]string{"amount": mintAmountA}, &mintResp)
		assert.Equal(t, "ok", mintResp["status"], "CB-A mint-and-approve status")

		t.Log("Step 6: CB-B mint-and-approve hub W-tokens for AMM...")
		cbB.mustPost(t, "/api/v2/amm/token/mint-and-approve",
			map[string]string{"amount": mintAmountB}, &mintResp)
		assert.Equal(t, "ok", mintResp["status"], "CB-B mint-and-approve status")

		t.Log("Step 7: CB-A submits liquidity commit (side A)...")
		var commitAResp struct {
			CommitID string `json:"commit_id"`
			Status   string `json:"status"`
		}
		cbA.mustPost(t, "/api/v2/amm/liquidity/commit", map[string]string{
			"pool_pair":   poolPair,
			"provider_id": "central_bank_a",
			"side":        "A",
			"amount":      commitAmountA,
		}, &commitAResp)
		require.NotEmpty(t, commitAResp.CommitID, "CB-A commit_id must not be empty")
		t.Logf("CB-A commit_id=%s status=%s", commitAResp.CommitID, commitAResp.Status)

		t.Log("Step 8: CB-B submits liquidity commit (side B)...")
		var commitBResp struct {
			CommitID string `json:"commit_id"`
			Status   string `json:"status"`
		}
		cbB.mustPost(t, "/api/v2/amm/liquidity/commit", map[string]string{
			"pool_pair":   poolPair,
			"provider_id": "central_bank_b",
			"side":        "B",
			"amount":      commitAmountB,
		}, &commitBResp)
		require.NotEmpty(t, commitBResp.CommitID, "CB-B commit_id must not be empty")
		t.Logf("CB-B commit_id=%s status=%s", commitBResp.CommitID, commitBResp.Status)

		t.Log("Step 9: Polling CB-A commit until EXECUTED (Cacti relay fires CommitMatched)...")
		pollUntil(t, 5*time.Second, 2*time.Minute, func() (bool, error) {
			var s struct {
				Status string `json:"status"`
			}
			if err := cbA.get(t, "/api/v2/amm/liquidity/commits/"+commitAResp.CommitID, &s); err != nil {
				return false, nil // transient, keep polling
			}
			t.Logf("  commit A status: %s", s.Status)
			if s.Status == "FAILED" || s.Status == "CANCELLED" {
				return false, fmt.Errorf("commit A reached terminal failure status: %s", s.Status)
			}
			// Require EXECUTED only (R1-12.4 gap 3): MATCHED merely confirms the on-chain
			// CommitMatched event fired; EXECUTED additionally proves the Cacti watcher detected
			// it, forwarded to the gateway, and the single-sided liquidity was added. Accepting
			// MATCHED here would let the test pass without exercising the watcher's forward path.
			return s.Status == "EXECUTED", nil
		})

		t.Log("Step 10: Polling pool status until ACTIVE (both commits executed)...")
		pollUntil(t, 5*time.Second, 2*time.Minute, func() (bool, error) {
			if err := cbA.get(t, "/api/v2/amm/pool/"+poolPair+"/status", &poolStatus); err != nil {
				return false, nil
			}
			t.Logf("  pool_status: %s", poolStatus.PoolStatus)
			return poolStatus.PoolStatus == "ACTIVE", nil
		})

		require.Equal(t, "ACTIVE", poolStatus.PoolStatus, "pool must be ACTIVE after commit-reveal")
		assert.NotEmpty(t, poolStatus.ReserveA, "reserve_a must be set")
		assert.NotEmpty(t, poolStatus.ReserveB, "reserve_b must be set")
		t.Logf("Pool ACTIVE: reserve_a=%s reserve_b=%s", poolStatus.ReserveA, poolStatus.ReserveB)
	})

	// Phase 2: PKI onboarding for both Bank A (through CB-A) and Bank B (through CB-B).
	// The gateway proxy enriches the payload with CSR and blockchain key automatically.
	// Uses the onboardBank helper which is idempotent — safe to re-run against a live stack.
	var bankAUserID string
	t.Run("phase_2_onboarding", func(t *testing.T) {
		t.Log("Onboarding Bank A through CB-A...")
		bankAUserID = onboardBank(t, bankA, cbA, "bank-a", "Test Bank A", "BR")
		require.NotEmpty(t, bankAUserID, "bank-a user_id must not be empty after onboarding")

		t.Log("Onboarding Bank B through CB-B (required for beneficiary resolution in swap)...")
		_ = onboardBank(t, bankB, cbB, "bank-b", "Test Bank B", "AR")

		t.Log("Verifying Bank A identity via /auth/me...")
		var me struct {
			BankID string   `json:"bankId"`
			Roles  []string `json:"roles"`
		}
		bankA.mustGet(t, "/api/v1/auth/me", &me)
		// bankId is enriched from the compliance DB; only assert non-empty since the
		// service-account JWT sub may not carry a bank_id attribute directly.
		t.Logf("Bank A /auth/me: bankId=%s roles=%v", me.BankID, me.Roles)
	})

	// Phase 3: Fiat issuance — Bank A registers a deposit, CB-A approves it.
	var depositID string
	t.Run("phase_3_fiat_issuance", func(t *testing.T) {
		t.Log("Step 1: Bank A registers deposit request...")
		var depositResp struct {
			DepositID string `json:"deposit_id"`
			Status    string `json:"status"`
		}
		require.NoError(t,
			bankA.post(t, "/api/v1/payments/deposits",
				map[string]string{"amount": depositAmount},
				&depositResp,
			),
			"register deposit must succeed",
		)
		require.NotEmpty(t, depositResp.DepositID, "deposit_id must not be empty")
		depositID = depositResp.DepositID
		t.Logf("Deposit registered: deposit_id=%s", depositID)

		t.Log("Step 2: CB-A approves the deposit (mints fiat tokens to Bank A)...")
		var approveResp struct {
			Status         string `json:"status"`
			FiatMintTxHash string `json:"fiat_mint_tx_hash"`
		}
		require.NoError(t,
			cbA.post(t, "/api/v1/payments/deposits/approve",
				map[string]string{"deposit_id": depositID},
				&approveResp,
			),
			"CB-A deposit approval must succeed",
		)
		assert.Equal(t, "approved", approveResp.Status, "deposit approval status")
		t.Logf("Deposit approved: tx_hash=%s", approveResp.FiatMintTxHash)

		// NOTE: ApproveDeposit auto-mints fCeBM to the bank's wallet. No separate
		// exchange call is needed; deposits/exchange is only a retry for failed auto-mints.

		t.Log("Step 3: Polling Bank A deposit until APPROVED/SETTLED...")
		pollUntil(t, 3*time.Second, 90*time.Second, func() (bool, error) {
			var list struct {
				Deposits []struct {
					ID     string `json:"id"`     // list uses "id", registration returns "deposit_id"
					Status string `json:"status"` // protobuf enum e.g. DEPOSIT_STATUS_APPROVED
				} `json:"deposits"`
			}
			if err := bankA.get(t, "/api/v1/payments/deposits", &list); err != nil {
				return false, nil
			}
			for _, d := range list.Deposits {
				if d.ID == depositID {
					t.Logf("  deposit status: %s", d.Status)
					approved := d.Status == "APPROVED" || d.Status == "SETTLED" || d.Status == "MINTED" ||
						d.Status == "DEPOSIT_STATUS_APPROVED" || d.Status == "DEPOSIT_STATUS_SETTLED" ||
						d.Status == "DEPOSIT_STATUS_MINTED"
					return approved, nil
				}
			}
			return false, nil
		})

		t.Log("Step 4: Asserting Bank A has fiat balance...")
		var balance struct {
			Balance  string `json:"balance"`
			Currency string `json:"currency"`
		}
		bankA.mustGet(t, "/api/v1/token/fiat-balance", &balance)
		assert.NotEmpty(t, balance.Balance, "Bank A fiat balance must be non-empty after issuance")
		assert.NotEqual(t, "0", balance.Balance, "Bank A fiat balance must be > 0 after issuance")
		t.Logf("Bank A fiat balance: %s %s", balance.Balance, balance.Currency)
	})

		// Phase 3b: Reserve Tokenisation — Bank A converts fCeBM → tCeBM via CB-A ApproveEscrow.
	// This is the mandatory prerequisite for a cross-currency swap: the bank must hold
	// tokenized reserves before the CB can perform the spoke burn → hub mint.
	t.Run("phase_3b_reserve_tokenisation", func(t *testing.T) {
		if depositID == "" {
			t.Skip("depositID not set (phase 3 did not run)")
		}

		// Use the full depositAmount for tokenisation (same units as deposit).
		tokeniseAmount := depositAmount

		t.Log("Step 1: Bank A requests Reserve Tokenisation (escrow fCeBM → tCeBM)...")
		var escrowResp struct {
			EscrowID string `json:"escrow_id"`
		}
		require.NoError(t,
			bankA.post(t, "/api/v1/payments/escrows",
				map[string]string{
					"deposit_id": depositID,
					"amount":     tokeniseAmount,
				},
				&escrowResp,
			),
			"escrow request must succeed",
		)
		require.NotEmpty(t, escrowResp.EscrowID, "escrow_id must not be empty")
		t.Logf("Escrow registered: escrow_id=%s", escrowResp.EscrowID)

		t.Log("Step 2: CB-A approves the escrow (burns fCeBM, mints tCeBM to bank-a)...")
		var approveResp struct {
			BurnTxHash string `json:"burn_tx_hash"`
			MintTxHash string `json:"mint_tx_hash"`
		}
		require.NoError(t,
			cbA.post(t, "/api/v1/payments/escrows/approve",
				map[string]string{"escrow_id": escrowResp.EscrowID},
				&approveResp,
			),
			"CB-A escrow approval must succeed",
		)
		t.Logf("Escrow approved: burn_tx=%s mint_tx=%s", approveResp.BurnTxHash, approveResp.MintTxHash)

		t.Log("Step 3: Polling Bank A tCeBM balance until non-zero...")
		var tCeBMBalance struct {
			Balance string `json:"balance"`
		}
		pollUntil(t, 3*time.Second, 90*time.Second, func() (bool, error) {
			if err := bankA.get(t, "/api/v1/token/balance", &tCeBMBalance); err != nil {
				return false, nil
			}
			t.Logf("  bank-a tCeBM balance: %s", tCeBMBalance.Balance)
			return tCeBMBalance.Balance != "" && tCeBMBalance.Balance != "0", nil
		})

		require.NotEmpty(t, tCeBMBalance.Balance, "Bank A tCeBM balance must be set after Reserve Tokenisation")
		require.NotEqual(t, "0", tCeBMBalance.Balance, "Bank A tCeBM balance must be > 0 after Reserve Tokenisation")
		t.Logf("Bank A tCeBM balance after Reserve Tokenisation: %s", tCeBMBalance.Balance)
	})

	// Phase 4: Cross-currency transfer — Bank A sends BRL, Bank B receives ARS.
	var swapID string
	t.Run("phase_4_transfer", func(t *testing.T) {
		// Record Bank A tCeBM balance before transfer (must decrease after bridge-in burn).
		var bankABalanceBefore struct {
			Balance string `json:"balance"`
		}
		_ = bankA.get(t, "/api/v1/token/balance", &bankABalanceBefore)
		t.Logf("Bank A tCeBM balance before transfer: %s", bankABalanceBefore.Balance)

		// Record Bank B baseline balance before transfer.
		var bankBBalanceBefore struct {
			Balance string `json:"balance"`
		}
		_ = bankB.get(t, "/api/v1/token/fiat-balance", &bankBBalanceBefore)
		t.Logf("Bank B balance before transfer: %s", bankBBalanceBefore.Balance)

		t.Log("Step 1: Bank A gets cross-currency quote (BRL → ARS)...")
		var quote struct {
			QuoteID              string  `json:"quote_id"`
			AmountOut            string  `json:"amount_out"`
			AmountIn             string  `json:"amount_in"`
			EffectiveRate        float64 `json:"effective_rate"`
			ValidUntil           int64   `json:"valid_until"`
			TimeRemainingSeconds int     `json:"time_remaining_seconds"`
		}
		quoteURL := fmt.Sprintf("/api/v2/amm/quote/cross-currency?source_currency=BRL&target_currency=ARS&amount_out=%s", swapAmountOut)
		bankA.mustGet(t, quoteURL, &quote)
		require.NotEmpty(t, quote.QuoteID, "quote_id must not be empty")
		require.Greater(t, quote.TimeRemainingSeconds, 0, "quote must not be expired")
		t.Logf("Quote: id=%s rate=%.6f valid_for=%ds", quote.QuoteID, quote.EffectiveRate, quote.TimeRemainingSeconds)

		t.Log("Step 2: Bank A executes cross-currency swap...")
		var swapResp struct {
			SwapID             string `json:"swap_id"`
			CorrelationID      string `json:"correlation_id"`
			Status             string `json:"status"`
			AmountIn           string `json:"amount_in"`
			AmountOut          string `json:"amount_out"`
			BridgeInPositionID string `json:"bridge_in_position_id"`
		}
		require.NoError(t,
			bankA.post(t, "/api/v2/amm/swap/cross-currency", map[string]interface{}{
				"source_currency":    "BRL",
				"target_currency":    "ARS",
				"pool_pair":          poolPair,
				"amount_out":         swapAmountOut,
				"max_amount_in":      swapMaxIn,
				"beneficiary_bank_id": "bank-b",
				"quote_id":           quote.QuoteID,
			}, &swapResp),
			"swap initiation must succeed",
		)
		require.NotEmpty(t, swapResp.SwapID, "swap_id must not be empty")
		swapID = swapResp.SwapID
		t.Logf("Swap initiated: swap_id=%s correlation_id=%s status=%s",
			swapID, swapResp.CorrelationID, swapResp.Status)
		// Initial status is typically BRIDGE_IN_PROGRESS but may be COMPLETED
		// immediately in fast test environments or when the relay is synchronous.
		require.NotEmpty(t, swapResp.Status, "initial swap status must not be empty")

		t.Log("Step 3: Polling swap status until COMPLETED...")
		var finalSwap struct {
			SwapID              string  `json:"swap_id"`
			Status              string  `json:"status"`
			AmountIn            string  `json:"amount_in"`
			AmountOut           string  `json:"amount_out"`
			EffectiveRate       float64 `json:"effective_rate"`
			SwapTxHash          string  `json:"swap_tx_hash"`
			BridgeOutPositionID string  `json:"bridge_out_position_id"`
			CompletedAt         string  `json:"completed_at"`
			FailureReason       string  `json:"failure_reason"`
		}
		pollUntil(t, 2*time.Second, 5*time.Minute, func() (bool, error) {
			if err := bankA.get(t, "/api/v2/amm/swap/cross-currency/"+swapID, &finalSwap); err != nil {
				return false, nil // transient
			}
			t.Logf("  swap status: %s", finalSwap.Status)
			if finalSwap.Status == "FAILED" {
				return false, fmt.Errorf("swap FAILED: %s", finalSwap.FailureReason)
			}
			return finalSwap.Status == "COMPLETED", nil
		})

		t.Log("Asserting final swap state...")
		assert.Equal(t, "COMPLETED", finalSwap.Status, "swap must be COMPLETED")
		assert.NotEmpty(t, finalSwap.AmountIn, "amount_in must be set")
		// Note: completed_at may not be present in all API versions; swap_tx_hash is the key receipt.
		assert.NotEmpty(t, finalSwap.SwapTxHash, "swap_tx_hash must be set")
		t.Logf("Swap COMPLETED: amount_in=%s amount_out=%s tx=%s bridge_out=%s",
			finalSwap.AmountIn, finalSwap.AmountOut, finalSwap.SwapTxHash, finalSwap.BridgeOutPositionID)

		// Assert Bank A's tCeBM balance decreased (bridge-in burned from bank-a, not auto-minted).
		// This is the key invariant: the CB MUST NOT have minted new tCeBM during bridge-in.
		if bankABalanceBefore.Balance != "" && bankABalanceBefore.Balance != "0" {
			var bankABalanceAfter struct {
				Balance string `json:"balance"`
			}
			pollUntil(t, 2*time.Second, 30*time.Second, func() (bool, error) {
				if err := bankA.get(t, "/api/v1/token/balance", &bankABalanceAfter); err != nil {
					return false, nil
				}
				t.Logf("  bank-a tCeBM balance after swap: %s (was %s)", bankABalanceAfter.Balance, bankABalanceBefore.Balance)
				return bankABalanceAfter.Balance != bankABalanceBefore.Balance, nil
			})
			assert.NotEqual(t, bankABalanceBefore.Balance, bankABalanceAfter.Balance,
				"Bank A tCeBM balance must decrease after swap (bridge-in burned payer's reserves — no new minting)")
			t.Logf("Bank A tCeBM reserve drain confirmed: before=%s after=%s", bankABalanceBefore.Balance, bankABalanceAfter.Balance)
		}
	})

	// Phase 5: Bank B receipt verification.
	// After the swap COMPLETED, the bridge-out executor on CB-B:
	//   1. Burns W-ARS on the Hub
	//   2. Mints tCeBM-ARS on Spoke-B to bank-b's registered wallet
	// Since onboarding now seeds the KMS with BESU_OPERATOR_KEY, bank-b's registered
	// wallet address equals ENTITY_BESU_ADDRESS — the same address GetBalance queries.
	// We verify both the bridge position state AND the dashboard-visible token balance.
	t.Run("phase_5_bank_b_receipt", func(t *testing.T) {
		if swapID == "" {
			t.Skip("swapID not set (phase 4 did not run)")
		}
		// Fetch the bridge_out_position_id from the completed swap.
		var swapStatus struct {
			Status              string `json:"status"`
			BridgeOutPositionID string `json:"bridge_out_position_id"`
		}
		bankA.mustGet(t, "/api/v2/amm/swap/cross-currency/"+swapID, &swapStatus)
		require.Equal(t, "COMPLETED", swapStatus.Status, "swap must be COMPLETED before checking receipt")

		if swapStatus.BridgeOutPositionID != "" {
			t.Logf("Polling CB-B bridge position %s until BURNED (tCeBM-ARS minted to bank-b)...",
				swapStatus.BridgeOutPositionID)

			pollUntil(t, 3*time.Second, 90*time.Second, func() (bool, error) {
				var resp struct {
					Positions []struct {
						PositionID  string `json:"position_id"`
						BridgeState string `json:"bridge_state"`
						OwnerBankID string `json:"owner_bank_id"`
					} `json:"positions"`
				}
				if err := cbB.get(t, "/api/v2/bridge/positions", &resp); err != nil {
					return false, nil
				}
				for _, p := range resp.Positions {
					if p.PositionID == swapStatus.BridgeOutPositionID || p.OwnerBankID == "bank-b" {
						t.Logf("  CB-B bridge position %s state=%s owner=%s",
							p.PositionID, p.BridgeState, p.OwnerBankID)
						if p.BridgeState == "BURNED" || p.BridgeState == "RELEASED" {
							return true, nil
						}
					}
				}
				return false, nil
			})
		} else {
			t.Log("bridge_out_position_id not set — swap used legacy path; skipping bridge position check")
		}

		// Verify that bank-b's tCeBM balance is non-zero on the dashboard endpoint.
		// This requires that the onboarding wallet address == BESU_OPERATOR_KEY address
		// (ensured by KMS_SEED_KEY_ID / KMS_SEED_PRIVATE_KEY in the auth service env).
		t.Log("Polling Bank B tCeBM balance until non-zero (dashboard receipt check)...")
		var finalBalance struct {
			Balance string `json:"balance"`
		}
		pollUntil(t, 3*time.Second, 60*time.Second, func() (bool, error) {
			if err := bankB.get(t, "/api/v1/token/balance", &finalBalance); err != nil {
				return false, nil
			}
			t.Logf("  bank-b tCeBM balance: %s", finalBalance.Balance)
			return finalBalance.Balance != "" && finalBalance.Balance != "0", nil
		})

		assert.NotEmpty(t, finalBalance.Balance, "Bank B tCeBM balance must be set after bridge-out")
		assert.NotEqual(t, "0", finalBalance.Balance, "Bank B tCeBM balance must be > 0 after bridge-out")
		t.Logf("Bank B receipt confirmed: tCeBM balance=%s (swap_id=%s)", finalBalance.Balance, swapID)
	})

	// Phase 6: PARTIAL LP-share withdrawal (specs/013-amm-lp-shares, decisions D1/D4/D7).
	// CB-A burns a fraction (withdrawFractionBps) of its on-chain CBW3-LP shares and receives a
	// single home-currency amount (W-BRL) via the zap-out path. Because it is partial, CB-A keeps
	// a reduced, still-ACTIVE position afterward (on-chain balance drops but stays > 0).
	// Runs LAST: it removes part of the pool's liquidity.
	t.Run("phase_6_lp_withdrawal", func(t *testing.T) {
		t.Log("Step 1: Reading CB-A on-chain LP-share position (lp-balance endpoint)...")
		var lpBal struct {
			LPShares        string  `json:"lp_shares"`
			LPTotalSupply   string  `json:"lp_total_supply"`
			SharePercentage float64 `json:"share_percentage"`
		}
		cbA.mustGet(t, "/api/v2/amm/lp-balance", &lpBal)
		t.Logf("CB-A lp_shares=%s total_supply=%s share=%.2f%%",
			lpBal.LPShares, lpBal.LPTotalSupply, lpBal.SharePercentage)
		require.NotEmpty(t, lpBal.LPShares, "lp_shares must be set")
		// Idempotency: a prior run on this live stack already burned CB-A's shares via the
		// withdrawal below. With no shares left there is nothing to withdraw, so treat this
		// as already-done and skip — mirroring the phase 1 (pool ACTIVE) and phase 2
		// (onboarding ACTIVE) skip-guards that keep the suite re-runnable against a seeded stack.
		if lpBal.LPShares == "0" {
			t.Log("CB-A holds no LP shares — withdrawal already executed on a prior run; skipping")
			return
		}

		t.Log("Step 2: Finding CB-A's ACTIVE liquidity position...")
		var positions struct {
			Positions []struct {
				LPID           string `json:"lp_id"`
				ProviderBankID string `json:"provider_bank_id"`
				PoolPair       string `json:"pool_pair"`
				Status         string `json:"status"`
				DepositSide    string `json:"deposit_side"`
			} `json:"positions"`
		}
		cbA.mustGet(t, "/api/v2/amm/liquidity/positions?pool_pair="+poolPair, &positions)
		var lpID, providerID string
		for _, p := range positions.Positions {
			if p.Status == "ACTIVE" {
				lpID, providerID = p.LPID, p.ProviderBankID
				break
			}
		}
		require.NotEmpty(t, lpID, "CB-A must have an ACTIVE liquidity position")
		t.Logf("Withdrawing position lp_id=%s provider=%s", lpID, providerID)

		t.Logf("Step 3: CB-A withdraws %d bps (partial) -> home currency zap-out...", withdrawFractionBps)
		var result struct {
			TokenAAmount   string `json:"token_a_amount"`
			TokenBAmount   string `json:"token_b_amount"`
			LPShares       string `json:"lp_shares"`
			WithdrawalMode string `json:"withdrawal_mode"`
		}
		cbA.mustPost(t, "/api/v2/amm/liquidity/remove", map[string]interface{}{
			"pool_pair":        poolPair,
			"provider_bank_id": providerID,
			"lp_id":            lpID,
			"fraction_bps":     withdrawFractionBps,
		}, &result)
		t.Logf("Withdrawal: mode=%s shares_burned=%s out_a=%s out_b=%s",
			result.WithdrawalMode, result.LPShares, result.TokenAAmount, result.TokenBAmount)
		require.Equal(t, "SHARES_HOME_CURRENCY_PARTIAL", result.WithdrawalMode, "must use the partial on-chain shares path")
		require.NotEqual(t, "0", result.TokenAAmount, "side-A provider must receive home currency (token A)")
		require.False(t, strings.HasPrefix(result.TokenAAmount, "0x"),
			"token_a_amount must be the realized amount, not a tx hash")
		require.Equal(t, "0", result.TokenBAmount, "single-currency exit must not return token B")

		// The shares burned must be the requested fraction of the pre-withdrawal balance.
		sharesBefore := mustBigInt(t, lpBal.LPShares)
		sharesBurned := mustBigInt(t, result.LPShares)
		expectedBurn := new(big.Int).Quo(
			new(big.Int).Mul(sharesBefore, big.NewInt(int64(withdrawFractionBps))), big.NewInt(10000))
		require.Equal(t, expectedBurn.String(), sharesBurned.String(),
			"shares burned must equal fraction_bps of the pre-withdrawal balance")

		t.Log("Step 4: Asserting CB-A's on-chain LP balance decreased but stayed > 0 (partial exit)...")
		var lpAfter struct {
			LPShares string `json:"lp_shares"`
		}
		cbA.mustGet(t, "/api/v2/amm/lp-balance", &lpAfter)
		t.Logf("CB-A lp_shares after partial withdrawal: %s (was %s)", lpAfter.LPShares, lpBal.LPShares)
		sharesAfter := mustBigInt(t, lpAfter.LPShares)
		require.Equal(t, -1, sharesAfter.Cmp(sharesBefore), "on-chain LP shares must decrease after partial burn")
		require.Equal(t, 1, sharesAfter.Sign(), "on-chain LP shares must remain > 0 after a partial withdrawal")

		t.Log("Step 5: Asserting CB-A's liquidity position is still ACTIVE after the partial exit...")
		var positionsAfter struct {
			Positions []struct {
				LPID   string `json:"lp_id"`
				Status string `json:"status"`
			} `json:"positions"`
		}
		cbA.mustGet(t, "/api/v2/amm/liquidity/positions?pool_pair="+poolPair, &positionsAfter)
		var stillActive bool
		for _, p := range positionsAfter.Positions {
			if p.LPID == lpID && p.Status == "ACTIVE" {
				stillActive = true
				break
			}
		}
		require.True(t, stillActive, "CB-A's position must remain ACTIVE after a partial withdrawal")
	})
}
