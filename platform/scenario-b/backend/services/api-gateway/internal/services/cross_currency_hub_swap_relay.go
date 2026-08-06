// SPDX-License-Identifier: Apache-2.0

// Package services provides CrossCurrencyHubSwapRelay for delegating the Hub AMM swap
// (Step 2) to the issuing Central Bank of the initiating bank's own spoke.
//
// The Hub AMM admits only verified Hub participants on both sides of a trade
// (onlyVerified(msg.sender) / onlyVerified(to)), and only central banks hold a Hub identity.
// Before this relay existed, the bank's own gateway ran the swap using the CB's private key —
// the sovereign key copied into the bank's container. This relay closes that: the bank sends
// a trigger and the CB executes the trade with its own signer, in its own process, exactly
// as it already does for bridge-in, bridge-out and residue-return.
//
// Synchronous by necessity: Step 3 (bridge-out) cannot start until the realized amount_in and
// the swap tx hash are known.
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/relayauth"
)

// HubSwapRelayIface is implemented by CrossCurrencyHubSwapRelay. The orchestrator uses it in
// Step 2 when the AMM swap must be executed by the issuing CB (sovereign model) instead of
// by this gateway.
type HubSwapRelayIface interface {
	// ExecuteHubSwap asks the issuing CB to run the Hub AMM swap funded by the bridge-in
	// position it created, and returns the realized cost and transaction hash.
	ExecuteHubSwap(ctx context.Context, req CrossCurrencyHubSwapRequest) (*SwapResult, error)
}

// CrossCurrencyHubSwapRequest is the payload sent to the issuing CB.
//
// It carries no token addresses and no signer address: the CB resolves the pair's AMM and
// tokens from its own on-chain PairRegistry, and it is the signer. MaxAmountIn is a ceiling
// the CB validates against the bridge-in position it minted — it is not an instruction the
// CB trusts.
type CrossCurrencyHubSwapRequest struct {
	CorrelationID      string `json:"correlation_id"`
	PayerBankID        string `json:"payer_bank_id"`
	BeneficiaryBankID  string `json:"beneficiary_bank_id,omitempty"`
	BridgeInPositionID string `json:"bridge_in_position_id"`
	PoolPair           string `json:"pool_pair"`
	TargetCurrency     string `json:"target_currency"`
	AmountOut          string `json:"amount_out"`
	MaxAmountIn        string `json:"max_amount_in"`
}

// CrossCurrencyHubSwapRelay calls the spoke CB to perform the sovereign Hub AMM swap.
type CrossCurrencyHubSwapRelay struct {
	centralBankURL  string // e.g. "http://api-gateway-central-bank-a:8080"
	relayAuthSecret string // X-Relay-Auth shared secret (legacy fallback)
	signer          *relayauth.Signer
	httpClient      *http.Client
}

// NewCrossCurrencyHubSwapRelay constructs the relay. The timeout covers one Hub transaction
// (gas estimation plus a QBFT block) with room for a busy node, and stays well under the
// bridge-in relay's 150s since no lifecycle polling happens on the CB side.
func NewCrossCurrencyHubSwapRelay(centralBankURL, relayAuthSecret string) *CrossCurrencyHubSwapRelay {
	return &CrossCurrencyHubSwapRelay{
		centralBankURL:  strings.TrimRight(centralBankURL, "/"),
		relayAuthSecret: relayAuthSecret,
		httpClient:      &http.Client{Timeout: 60 * time.Second},
	}
}

// WithSigner attaches a per-CB signer so hub-swap requests carry an asymmetric signature
// (R2-CR-6), matching the bridge-in and residue relays.
func (r *CrossCurrencyHubSwapRelay) WithSigner(s *relayauth.Signer) *CrossCurrencyHubSwapRelay {
	r.signer = s
	return r
}

// HubSwapPath is the CB endpoint that receives the delegation.
const HubSwapPath = "/internal/amm/cross-currency-hub-swap"

// ErrHubSwapNotOurs says the CB refused because this position's swap belongs to someone else: a
// delivery still trading, or an earlier attempt whose outcome is unknown and awaits reconciliation.
//
// It exists so this case is not rolled back. Reversing the bridge-in here would reclaim tokens that
// another delivery is spending, or that a possibly-broadcast transaction already spent — turning a
// benign duplicate delivery into a corrupted position.
var ErrHubSwapNotOurs = errors.New("hub swap for this position is owned by another attempt")

// ExecuteHubSwap POSTs the swap request to the CB and returns the realized outcome.
//
// A "duplicate" verdict is a success: the CB already executed this position's swap and
// replays the recorded result, so the orchestrator proceeds with the same facts instead of
// re-running a trade that is not idempotent on-chain.
func (r *CrossCurrencyHubSwapRelay) ExecuteHubSwap(ctx context.Context, req CrossCurrencyHubSwapRequest) (*SwapResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal hub-swap request: %w", err)
	}

	url := r.centralBankURL + HubSwapPath
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Relay-Auth", r.relayAuthSecret)
	if r.signer != nil {
		headers, signErr := r.signer.HeadersFor(http.MethodPost, HubSwapPath, body, time.Now())
		if signErr != nil {
			return nil, fmt.Errorf("sign hub-swap request: %w", signErr)
		}
		for k, v := range headers {
			httpReq.Header.Set(k, v)
		}
	}

	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusConflict {
		// The CB claims a position before trading, so a conflict means another delivery of THIS
		// delegation owns it. Distinguished from a plain failure because the caller must not roll
		// the bridge-in back: the tokens are being spent by that other delivery right now, and
		// reversing them would fight it.
		return nil, fmt.Errorf("%w: %s", ErrHubSwapNotOurs, strings.TrimSpace(string(respBody)))
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("central bank hub-swap returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var out struct {
		Status string `json:"status"`
		// SwapSenderAddress is the CB's own Hub address — the msg.sender of the trade and the
		// holder of the swap output. Step 3 must forward it so the beneficiary CB burns the
		// W-<target> from the right place.
		SwapSenderAddress string `json:"swap_sender_address"`
		SwapTxHash        string `json:"swap_tx_hash"`
		AmountIn          string `json:"amount_in"`
		Warning           string `json:"warning"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("decode hub-swap response: %w", err)
	}
	if out.SwapTxHash == "" || out.AmountIn == "" {
		// Without the tx hash and realized cost the orchestrator cannot run Step 3 or size
		// the residue. Treat a hollow success as a failure rather than proceeding on zeros.
		return nil, fmt.Errorf("central bank hub-swap returned no swap_tx_hash/amount_in (status=%q)", out.Status)
	}
	if out.SwapSenderAddress == "" {
		// The output sits on the CB's Hub address. Without it Step 3 would fall back to this
		// gateway's own signer — which in the delegated model holds nothing — and the
		// beneficiary CB would burn from the wrong address.
		return nil, fmt.Errorf("central bank hub-swap returned no swap_sender_address (status=%q, tx=%s)", out.Status, out.SwapTxHash)
	}
	if out.Warning != "" {
		// The CB executed the trade but could not record it. Proceeding is the atomicity-
		// preserving choice: the W-<source> is already spent, so failing here would guarantee a
		// half-settled payment (input consumed, output undelivered), whereas continuing delivers
		// it. The condition is surfaced, never swallowed — the CB logged it as CRITICAL and the
		// state travels back on the result so the swap record shows it.
		log.Printf("[correlation_id=%s] WARNING: CB executed the hub swap (tx=%s) but did not record it — reconciliation required before any replay: %s",
			req.CorrelationID, out.SwapTxHash, out.Warning)
	}
	return &SwapResult{
		TxHash:           out.SwapTxHash,
		AmountIn:         out.AmountIn,
		OrderID:          req.PayerBankID,
		State:            out.Status,
		ConfirmedAt:      time.Now(),
		HubSenderAddress: out.SwapSenderAddress,
	}, nil
}
