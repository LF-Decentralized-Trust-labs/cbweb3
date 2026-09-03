// SPDX-License-Identifier: Apache-2.0

//go:build e2e
// +build e2e

// Package residue_test verifies, against a live Scenario B stack, that the unspent slippage
// buffer of a cross-currency swap is returned to the payer.
//
// Step 1 (bridge-in) must move MaxAmountIn — the desired amount plus the slippage buffer —
// because the true cost is unknown until the AMM swap runs; Step 2 consumes only the realized
// amount_in. Without the residue return the difference strands on the Hub swap signer and the
// payer is over-debited on every swap.
//
// What this asserts:
//
//  1. the API reports residue_amount == max_amount_in − realized amount_in;
//  2. the payer's NET on-chain debit equals the realized amount_in, not the cap;
//  3. the Hub swap signer's wrapped-source balance returns to its pre-swap value, so
//     buffers do not accumulate across swaps;
//  4. the residue return is idempotent — replaying it enqueues no second burn.
//
// Deliberately env-driven: deployment topology (ports, token addresses, entity names) is not
// guessable, and hardcoding it would rot. The test SKIPS with an explicit list of what is
// missing rather than failing, so it is safe to run in any pipeline.
//
// Prerequisites in the target environment: an ACTIVE sovereign pair with liquidity, the payer
// bank holding tokenized reserves (>= max_amount_in), and the beneficiary bank registered as
// a participant at the beneficiary CB.
//
// Example (matches the toolkit's brazil/argentina sample topology):
//
//	E2E_PAYER_GW_URL=http://localhost:41646 \
//	E2E_CB_GW_URL=http://localhost:41645 \
//	E2E_KEYCLOAK_URL=http://localhost:40646 \
//	E2E_OIDC_CLIENT=bank-backend E2E_OIDC_SECRET=bank-backend-local-secret \
//	E2E_USERNAME=admin@itau.brasil.com E2E_PASSWORD=itau-bank-local \
//	E2E_HUB_RPC_URL=http://localhost:33845 E2E_SPOKE_RPC_URL=http://localhost:33646 \
//	E2E_W_SOURCE_TOKEN=0xde87af9156a223404885002669d3be239313ae33 \
//	E2E_SPOKE_TOKEN=0xa50a51c09a5c451c52bb714527e1974b686d8e77 \
//	E2E_HUB_SIGNER=0xFE3B557E8Fb62b89F4916B721be55cEb828dBd73 \
//	E2E_PAYER_WALLET=0x409e0f7a712B8f90e3a597e3258A2E2f04652e4d \
//	E2E_POOL_PAIR=W-BRL-W-ARS E2E_BENEFICIARY_BANK=bank-macro \
//	E2E_PAYER_BANK=bank-itau E2E_RELAY_SECRET=cbweb3-relay-shared-secret \
//	go test -tags e2e -v -timeout 20m ./...
//
// E2E_PAYER_BANK and E2E_RELAY_SECRET are optional: without them everything but the
// replay-protection probe runs.
package residue_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// config carries everything the test cannot infer about the target deployment.
type config struct {
	payerGW, cbGW              string
	keycloakURL, realm         string
	oidcClient, oidcSecret     string
	username, password         string
	hubRPC, spokeRPC           string
	wSourceToken, spokeToken   string
	hubSigner, payerWallet     string
	poolPair, sourceCur        string
	targetCur, beneficiaryBank string
	// payerBank and relaySecret are only needed for the optional replay probe, which calls
	// the issuing CB's internal endpoint directly.
	payerBank     string
	relaySecret   string
	amountOut     string
	maxMultiplier int64
	settleTimeout time.Duration
}

func loadConfig(t *testing.T) *config {
	t.Helper()
	c := &config{
		payerGW:         os.Getenv("E2E_PAYER_GW_URL"),
		cbGW:            os.Getenv("E2E_CB_GW_URL"),
		keycloakURL:     os.Getenv("E2E_KEYCLOAK_URL"),
		realm:           envOr("E2E_KEYCLOAK_REALM", "cbweb3"),
		oidcClient:      os.Getenv("E2E_OIDC_CLIENT"),
		oidcSecret:      os.Getenv("E2E_OIDC_SECRET"),
		username:        os.Getenv("E2E_USERNAME"),
		password:        os.Getenv("E2E_PASSWORD"),
		hubRPC:          os.Getenv("E2E_HUB_RPC_URL"),
		spokeRPC:        os.Getenv("E2E_SPOKE_RPC_URL"),
		wSourceToken:    os.Getenv("E2E_W_SOURCE_TOKEN"),
		spokeToken:      os.Getenv("E2E_SPOKE_TOKEN"),
		hubSigner:       os.Getenv("E2E_HUB_SIGNER"),
		payerWallet:     os.Getenv("E2E_PAYER_WALLET"),
		poolPair:        os.Getenv("E2E_POOL_PAIR"),
		sourceCur:       envOr("E2E_SOURCE_CURRENCY", "BRL"),
		targetCur:       envOr("E2E_TARGET_CURRENCY", "ARS"),
		beneficiaryBank: os.Getenv("E2E_BENEFICIARY_BANK"),
		payerBank:       os.Getenv("E2E_PAYER_BANK"),
		relaySecret:     os.Getenv("E2E_RELAY_SECRET"),
		amountOut:       envOr("E2E_AMOUNT_OUT", "5000000000000000000"),
		maxMultiplier:   3,
		settleTimeout:   3 * time.Minute,
	}
	if v := os.Getenv("E2E_MAX_MULTIPLIER"); v != "" {
		if n, ok := new(big.Int).SetString(v, 10); ok && n.Sign() > 0 {
			c.maxMultiplier = n.Int64()
		}
	}

	required := map[string]string{
		"E2E_PAYER_GW_URL":     c.payerGW,
		"E2E_CB_GW_URL":        c.cbGW,
		"E2E_KEYCLOAK_URL":     c.keycloakURL,
		"E2E_OIDC_CLIENT":      c.oidcClient,
		"E2E_USERNAME":         c.username,
		"E2E_PASSWORD":         c.password,
		"E2E_HUB_RPC_URL":      c.hubRPC,
		"E2E_SPOKE_RPC_URL":    c.spokeRPC,
		"E2E_W_SOURCE_TOKEN":   c.wSourceToken,
		"E2E_SPOKE_TOKEN":      c.spokeToken,
		"E2E_HUB_SIGNER":       c.hubSigner,
		"E2E_PAYER_WALLET":     c.payerWallet,
		"E2E_POOL_PAIR":        c.poolPair,
		"E2E_BENEFICIARY_BANK": c.beneficiaryBank,
	}
	var missing []string
	for k, v := range required {
		if strings.TrimSpace(v) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		t.Skipf("residue-return E2E skipped — no live stack configured; missing env: %s", strings.Join(missing, ", "))
	}
	// The slippage buffer must be wide enough to leave an observable residue. A multiplier of
	// 1 would make the test vacuous: it would pass whether or not the residue is returned.
	if c.maxMultiplier < 2 {
		t.Fatalf("E2E_MAX_MULTIPLIER must be >= 2 to leave an observable residue, got %d", c.maxMultiplier)
	}
	return c
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func TestResidueReturnedToPayer(t *testing.T) {
	c := loadConfig(t)
	token := login(t, c)

	// ── Quote ────────────────────────────────────────────────────────────────
	quote := struct {
		AmountIn string `json:"amount_in"`
		PoolPair string `json:"pool_pair"`
	}{}
	q := fmt.Sprintf("/api/v2/amm/quote/cross-currency?source_currency=%s&target_currency=%s&amount_out=%s&pool_pair=%s",
		c.sourceCur, c.targetCur, c.amountOut, url.QueryEscape(c.poolPair))
	getJSON(t, c.payerGW+q, token, &quote)
	quoted := mustInt(t, quote.AmountIn, "quoted amount_in")
	if quoted.Sign() <= 0 {
		t.Fatalf("quote returned a non-positive amount_in %q — is the pool seeded?", quote.AmountIn)
	}

	// A deliberately wide cap: the whole point is that the swap consumes far less than this.
	maxAmountIn := new(big.Int).Mul(quoted, big.NewInt(c.maxMultiplier))
	t.Logf("quote: amount_in=%s for amount_out=%s; submitting with max_amount_in=%s (%dx)",
		quoted, c.amountOut, maxAmountIn, c.maxMultiplier)

	// ── Baseline ─────────────────────────────────────────────────────────────
	hubBefore := balanceOf(t, c.hubRPC, c.wSourceToken, c.hubSigner)
	payerBefore := balanceOf(t, c.spokeRPC, c.spokeToken, c.payerWallet)
	t.Logf("baseline: hub signer wrapped-source=%s, payer spoke reserves=%s", hubBefore, payerBefore)
	if payerBefore.Cmp(maxAmountIn) < 0 {
		t.Skipf("payer holds %s tokenized reserves but the swap reserves %s — tokenize more before running",
			payerBefore, maxAmountIn)
	}

	// ── Swap ─────────────────────────────────────────────────────────────────
	var swap struct {
		SwapID             string `json:"swap_id"`
		Status             string `json:"status"`
		AmountIn           string `json:"amount_in"`
		SwapTxHash         string `json:"swap_tx_hash"`
		BridgeInPositionID string `json:"bridge_in_position_id"`
		ResidueAmount      string `json:"residue_amount"`
		ResiduePositionID  string `json:"residue_position_id"`
		ResidueStatus      string `json:"residue_status"`
	}
	postJSON(t, c.payerGW+"/api/v2/amm/swap/cross-currency", token, map[string]string{
		"source_currency":     c.sourceCur,
		"target_currency":     c.targetCur,
		"pool_pair":           c.poolPair,
		"amount_out":          c.amountOut,
		"max_amount_in":       maxAmountIn.String(),
		"beneficiary_bank_id": c.beneficiaryBank,
	}, &swap)

	if swap.Status != "COMPLETED" {
		t.Fatalf("expected COMPLETED swap, got %q (swap_id=%s)", swap.Status, swap.SwapID)
	}
	realized := mustInt(t, swap.AmountIn, "realized amount_in")

	// Guard against a vacuous run: if the swap happened to consume the whole cap there is no
	// residue to observe and the assertions below prove nothing.
	if realized.Cmp(maxAmountIn) >= 0 {
		t.Fatalf("swap consumed the whole cap (%s of %s) — nothing to return; widen E2E_MAX_MULTIPLIER",
			realized, maxAmountIn)
	}

	// ── (1) reported residue is exactly the unspent buffer ───────────────────
	wantResidue := new(big.Int).Sub(maxAmountIn, realized)
	gotResidue := mustInt(t, swap.ResidueAmount, "residue_amount")
	if gotResidue.Cmp(wantResidue) != 0 {
		t.Errorf("residue_amount = %s, want max_amount_in − amount_in = %s", gotResidue, wantResidue)
	}
	if swap.ResidueStatus != "RETURN_ENQUEUED" {
		t.Errorf("residue_status = %q, want RETURN_ENQUEUED", swap.ResidueStatus)
	}
	if swap.ResiduePositionID == "" {
		t.Error("residue_position_id is empty — the return leg was not created")
	}
	t.Logf("swap COMPLETED: realized=%s residue=%s residue_position=%s",
		realized, gotResidue, swap.ResiduePositionID)

	// ── (3) the buffer does not accumulate on the shared Hub signer ──────────
	// The relayer drives the burn asynchronously, so poll rather than assume.
	deadline := time.Now().Add(c.settleTimeout)
	var hubAfter *big.Int
	for {
		hubAfter = balanceOf(t, c.hubRPC, c.wSourceToken, c.hubSigner)
		if hubAfter.Cmp(hubBefore) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("hub signer wrapped-source balance did not return to its baseline within %s: before=%s after=%s (stranded %s)",
				c.settleTimeout, hubBefore, hubAfter, new(big.Int).Sub(hubAfter, hubBefore))
		}
		time.Sleep(3 * time.Second)
	}
	t.Logf("hub signer balance back to baseline %s — no accumulation", hubAfter)

	// ── (2) the payer's NET debit equals the realized cost ───────────────────
	payerAfter := balanceOf(t, c.spokeRPC, c.spokeToken, c.payerWallet)
	netDebit := new(big.Int).Sub(payerBefore, payerAfter)
	if netDebit.Cmp(realized) != 0 {
		t.Errorf("payer net debit = %s, want realized amount_in = %s (over-debit of %s; the uncorrected behaviour would debit %s)",
			netDebit, realized, new(big.Int).Sub(netDebit, realized), maxAmountIn)
	} else {
		t.Logf("payer net debit = %s == realized amount_in (cap was %s)", netDebit, maxAmountIn)
	}

	// ── (4) the return is idempotent ─────────────────────────────────────────
	if c.relaySecret == "" || c.payerBank == "" {
		t.Log("E2E_RELAY_SECRET / E2E_PAYER_BANK not set — skipping the replay-protection probe")
		return
	}
	var replay struct {
		Status     string `json:"status"`
		PositionID string `json:"position_id"`
	}
	postRelay(t, c.cbGW+"/internal/amm/cross-currency-residue-return", c.relaySecret, map[string]string{
		"correlation_id":        "e2e-replay-" + swap.SwapID,
		"swap_tx_hash":          swap.SwapTxHash,
		"pool_pair":             c.poolPair,
		"bridge_in_position_id": swap.BridgeInPositionID,
		"payer_bank_id":         c.payerBank,
	}, &replay)
	if replay.Status != "duplicate" {
		t.Errorf("replaying the residue return returned status %q, want \"duplicate\" — a second burn may have been enqueued", replay.Status)
	}
	if replay.PositionID != swap.ResiduePositionID {
		t.Errorf("replay returned position %q, want the original %q", replay.PositionID, swap.ResiduePositionID)
	}

	// The balance must still be at baseline: a replay must move nothing.
	if final := balanceOf(t, c.hubRPC, c.wSourceToken, c.hubSigner); final.Cmp(hubBefore) != 0 {
		t.Errorf("hub signer balance moved after the replay: %s (baseline %s)", final, hubBefore)
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────

var httpc = &http.Client{Timeout: 5 * time.Minute}

// login performs an OIDC password grant. The swap route requires a token carrying the
// commercial_bank role, which in the toolkit topology is granted to users, not to client
// service accounts — so a client-credentials grant is not sufficient.
func login(t *testing.T, c *config) string {
	t.Helper()
	form := url.Values{
		"grant_type": {"password"},
		"client_id":  {c.oidcClient},
		"username":   {c.username},
		"password":   {c.password},
	}
	if c.oidcSecret != "" {
		form.Set("client_secret", c.oidcSecret)
	}
	endpoint := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", strings.TrimRight(c.keycloakURL, "/"), c.realm)
	resp, err := httpc.PostForm(endpoint, form) //nolint:noctx
	if err != nil {
		t.Fatalf("token request to %s failed: %v", endpoint, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("token request returned HTTP %d: %s", resp.StatusCode, truncate(body))
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.AccessToken == "" {
		t.Fatalf("could not decode access_token from %s: %s", endpoint, truncate(body))
	}
	return out.AccessToken
}

// do sends req with the token as both Bearer header and access_token cookie, since the v2
// swap routes require the cookie while the read routes accept the header.
func do(t *testing.T, req *http.Request, token string, out interface{}) {
	t.Helper()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	}
	resp, err := httpc.Do(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", req.Method, req.URL, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("%s %s returned HTTP %d: %s", req.Method, req.URL, resp.StatusCode, truncate(body))
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			t.Fatalf("decode %s response: %v (body: %s)", req.URL, err, truncate(body))
		}
	}
}

func getJSON(t *testing.T, urlStr, token string, out interface{}) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		t.Fatalf("build GET %s: %v", urlStr, err)
	}
	do(t, req, token, out)
}

func postJSON(t *testing.T, urlStr, token string, in, out interface{}) {
	t.Helper()
	payload, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal request for %s: %v", urlStr, err)
	}
	req, err := http.NewRequest(http.MethodPost, urlStr, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build POST %s: %v", urlStr, err)
	}
	req.Header.Set("Content-Type", "application/json")
	do(t, req, token, out)
}

// postRelay calls an internal CB endpoint guarded by the shared relay secret.
func postRelay(t *testing.T, urlStr, secret string, in, out interface{}) {
	t.Helper()
	payload, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal relay request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, urlStr, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build POST %s: %v", urlStr, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Relay-Auth", secret)
	do(t, req, "", out)
}

// balanceOf reads an ERC-20 balance via eth_call, so the assertion rests on chain state
// rather than on the API's own reporting.
func balanceOf(t *testing.T, rpcURL, token, holder string) *big.Int {
	t.Helper()
	addr := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(holder)), "0x")
	if len(addr) != 40 {
		t.Fatalf("holder %q is not a 20-byte address", holder)
	}
	data := "0x70a08231" + strings.Repeat("0", 24) + addr

	payload, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "id": 1, "method": "eth_call",
		"params": []interface{}{map[string]string{"to": token, "data": data}, "latest"},
	})
	resp, err := httpc.Post(rpcURL, "application/json", bytes.NewReader(payload)) //nolint:noctx
	if err != nil {
		t.Fatalf("eth_call to %s failed: %v", rpcURL, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var out struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode eth_call response: %v (body: %s)", err, truncate(body))
	}
	if out.Error != nil {
		t.Fatalf("eth_call balanceOf(%s) on %s: %s", holder, token, out.Error.Message)
	}
	v, ok := new(big.Int).SetString(strings.TrimPrefix(out.Result, "0x"), 16)
	if !ok {
		t.Fatalf("unparseable balance %q for %s", out.Result, holder)
	}
	return v
}

func mustInt(t *testing.T, s, what string) *big.Int {
	t.Helper()
	v, ok := new(big.Int).SetString(strings.TrimSpace(s), 10)
	if !ok {
		t.Fatalf("%s is not a base-10 integer: %q", what, s)
	}
	return v
}

func truncate(b []byte) string {
	const max = 400
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "…"
}
