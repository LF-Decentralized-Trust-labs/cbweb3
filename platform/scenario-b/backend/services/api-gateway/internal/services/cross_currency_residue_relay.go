// SPDX-License-Identifier: Apache-2.0

// Package services provides CrossCurrencyResidueRelay for delegating the return of the
// unspent slippage buffer to the issuing Central Bank of the payer's own spoke.
//
// Step 1 (bridge-in) must move MaxAmountIn — the desired amount plus the slippage buffer —
// because the true cost is only known once the AMM swap runs. Step 2 consumes only the
// realized amount_in. The remainder is W-<source> sitting on the initiating gateway's Hub
// swap signer, and giving it back means burning W-<source> on the Hub and delivering
// tCeBM-<source> on the source spoke — both central-bank-only operations on that currency.
//
// So the return is delegated to the same CB that performed the bridge-in, over the same
// direct Go→Go channel (CENTRAL_BANK_API_URL, no Cacti hop: same jurisdiction).
//
// Unlike bridge-in, this call is fire-and-forward: the payment has already settled when it
// runs, so the orchestrator must not block on it and must not fail the swap if it errors.
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/relayauth"
)

// CrossCurrencyResidueRelayIface is implemented by CrossCurrencyResidueRelay.
type CrossCurrencyResidueRelayIface interface {
	// NotifyResidueReturn asks the issuing CB to return the unspent bridge-in buffer to the
	// payer. Returns the CB-side position_id of the return leg.
	NotifyResidueReturn(ctx context.Context, req CrossCurrencyResidueReturnRequest) (string, error)
}

// CrossCurrencyResidueReturnRequest is the payload sent to the issuing CB.
//
// It carries no amount. The CB derives the residue from its own bridge-in position and the
// on-chain LogSwap, so a caller cannot inflate what it gets back — the same trust model as
// the bridge-out endpoint (R2-CR-6).
type CrossCurrencyResidueReturnRequest struct {
	CorrelationID string `json:"correlation_id"`
	// SwapTxHash is the Hub AMM swap whose realized amount_in bounds the residue.
	SwapTxHash string `json:"swap_tx_hash"`
	// PoolPair tells the CB which pair's AMM must have emitted the LogSwap.
	PoolPair string `json:"pool_pair"`
	// BridgeInPositionID is the position the CB itself created in Step 1; its
	// mirrored_amount is the authoritative amount bridged in.
	BridgeInPositionID string `json:"bridge_in_position_id"`
	PayerBankID        string `json:"payer_bank_id"`
	SpokeIn            string `json:"spoke_in"`
}

// CrossCurrencyResidueRelay calls the issuing CB to perform the sovereign residue return.
type CrossCurrencyResidueRelay struct {
	centralBankURL  string // e.g. "http://api-gateway-central-bank-a:8080"
	relayAuthSecret string // X-Relay-Auth shared secret (legacy fallback)
	signer          *relayauth.Signer
	httpClient      *http.Client
}

// NewCrossCurrencyResidueRelay constructs the relay. The timeout is short compared to
// bridge-in's: the CB only verifies and enqueues, it does not wait for the release.
func NewCrossCurrencyResidueRelay(centralBankURL, relayAuthSecret string) *CrossCurrencyResidueRelay {
	return &CrossCurrencyResidueRelay{
		centralBankURL:  strings.TrimRight(centralBankURL, "/"),
		relayAuthSecret: relayAuthSecret,
		httpClient:      &http.Client{Timeout: 30 * time.Second},
	}
}

// WithSigner attaches a per-CB signer so residue requests carry an asymmetric signature
// (R2-CR-6), matching the bridge-in relay.
func (r *CrossCurrencyResidueRelay) WithSigner(s *relayauth.Signer) *CrossCurrencyResidueRelay {
	r.signer = s
	return r
}

// ResidueReturnPath is the CB endpoint that receives the delegation.
const ResidueReturnPath = "/internal/amm/cross-currency-residue-return"

// NotifyResidueReturn POSTs the residue-return request to the CB and returns the position_id.
func (r *CrossCurrencyResidueRelay) NotifyResidueReturn(ctx context.Context, req CrossCurrencyResidueReturnRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal residue-return request: %w", err)
	}

	url := r.centralBankURL + ResidueReturnPath
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Relay-Auth", r.relayAuthSecret)
	if r.signer != nil {
		headers, signErr := r.signer.HeadersFor(http.MethodPost, ResidueReturnPath, body, time.Now())
		if signErr != nil {
			return "", fmt.Errorf("sign residue-return request: %w", signErr)
		}
		for k, v := range headers {
			httpReq.Header.Set(k, v)
		}
	}

	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("central bank residue-return returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var out struct {
		Status     string `json:"status"`
		PositionID string `json:"position_id"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("decode residue-return response: %w", err)
	}
	return out.PositionID, nil
}
