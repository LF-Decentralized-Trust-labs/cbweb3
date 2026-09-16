// SPDX-License-Identifier: Apache-2.0

// Package services provides CactiCrossCurrencyRelay for notifying CB-B via Cacti (009).
//
// After CB-A's Hub AMM swap completes, the orquetrator calls this relay instead of
// directly enqueuing a bridge-out on CB-A's own payment-orchestrator.
// The relay POSTs to Cacti POST /api/v1/cross-currency/bridge-out, which in turn
// forwards to CB-B's internal gateway endpoint.
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
)

// CactiCrossCurrencyRelayIface is implemented by CactiCrossCurrencyRelay.
// The orquestrador uses this in Step 3 when cross-currency bridge-out should
// be handled by CB-B (sovereign model) instead of CB-A's local relayer.
type CactiCrossCurrencyRelayIface interface {
	// NotifyBridgeOut sends the bridge-out request to Cacti so it can route to CB-B.
	// Returns the position_id acknowledged by CB-B (or empty if async).
	NotifyBridgeOut(ctx context.Context, req CactiCrossCurrencyBridgeOutRequest) (string, error)
}

// BeneficiaryPreflightChecker is an OPTIONAL capability of a relay: asking the beneficiary's
// central bank whether a bank can receive, before any value moves.
//
// Separate from CactiCrossCurrencyRelayIface, and discovered with a type assertion, so a relay
// implementation that predates the check keeps working unchanged — it simply does not get the
// pre-flight, and the bridge-out retry still covers the failure. Widening the main interface
// would have forced every implementation and every test double to grow a method they do not use.
type BeneficiaryPreflightChecker interface {
	CheckBeneficiary(ctx context.Context, spokeOut, bankID string) BeneficiaryPreflight
}

// CactiCrossCurrencyBridgeOutRequest is the payload sent to Cacti.
type CactiCrossCurrencyBridgeOutRequest struct {
	CorrelationID      string `json:"correlation_id"`
	SwapTxHash         string `json:"swap_tx_hash"`
	PoolPair           string `json:"pool_pair"`
	AmountOut          string `json:"amount_out"`
	BeneficiaryBankID  string `json:"beneficiary_bank_id"`
	SpokeOut           string `json:"spoke_out"`
	WrappedTargetToken string `json:"wrapped_target_token"`
	// AmmAddress is the pair's on-chain AMM (dynamic per-pair model). The relay reads
	// isPaused() on it for its circuit-breaker gate; empty ⇒ relay fails safe.
	AmmAddress string `json:"amm_address,omitempty"`
	// SwapSenderAddress is the Hub address that received W-ARS from the AMM swap.
	// CB-B's executor burns from this address (CENTRAL_BANK_ROLE allows any-address burn).
	SwapSenderAddress string `json:"swap_sender_address,omitempty"`
}

// CactiCrossCurrencyRelay calls the Cacti relay to trigger CB-B bridge-out.
type CactiCrossCurrencyRelay struct {
	cactiURL        string // e.g. "http://host.docker.internal:4000"
	relayAuthSecret string // X-Relay-Auth shared secret
	httpClient      *http.Client
}

// NewCactiCrossCurrencyRelay constructs the relay.
func NewCactiCrossCurrencyRelay(cactiURL, relayAuthSecret string) *CactiCrossCurrencyRelay {
	return &CactiCrossCurrencyRelay{
		cactiURL:        strings.TrimRight(cactiURL, "/"),
		relayAuthSecret: relayAuthSecret,
		httpClient:      &http.Client{Timeout: 30 * time.Second},
	}
}

// NotifyBridgeOut POSTs the bridge-out notification to Cacti.
func (r *CactiCrossCurrencyRelay) NotifyBridgeOut(ctx context.Context, req CactiCrossCurrencyBridgeOutRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal bridge-out request: %w", err)
	}

	url := r.cactiURL + "/api/v1/cross-currency/bridge-out"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Relay-Auth", r.relayAuthSecret)

	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cacti returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var out struct {
		Status        string `json:"status"`
		CorrelationID string `json:"correlation_id"`
	}
	_ = json.Unmarshal(respBody, &out)
	return out.CorrelationID, nil
}

// BeneficiaryPreflight is the answer to "can this bank receive?", plus whether the question was
// answered at all.
//
// Answered is the field that matters. An unreachable peer and a definite refusal must lead to
// different decisions: only the refusal is a reason to stop a payment. Collapsing them into a
// bool would make a network blip look identical to an ineligible beneficiary, and the safest
// reading of that ambiguity — refuse — would let any central bank's downtime close the corridor.
type BeneficiaryPreflight struct {
	Answered bool
	Eligible bool
	Code     string
}

// CheckBeneficiary asks the beneficiary's central bank, through the relay, whether a bank can
// receive a delivery — before this gateway moves any value.
//
// It never returns an error. An unanswerable question is reported as Answered=false so the
// caller can proceed deliberately rather than by catching an error it might mishandle.
func (r *CactiCrossCurrencyRelay) CheckBeneficiary(ctx context.Context, spokeOut, bankID string) BeneficiaryPreflight {
	body, err := json.Marshal(map[string]string{"spoke_out": spokeOut, "bank_id": bankID})
	if err != nil {
		return BeneficiaryPreflight{}
	}
	endpoint := r.cactiURL + "/api/v1/cross-currency/beneficiary-check"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return BeneficiaryPreflight{}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Relay-Auth", r.relayAuthSecret)

	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return BeneficiaryPreflight{}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		// Includes the relay's own 400/502 and the peer's 503: the question did not get an
		// answer, which is not the same as an answer of "no".
		return BeneficiaryPreflight{}
	}
	var out struct {
		Eligible bool   `json:"eligible"`
		Code     string `json:"code"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return BeneficiaryPreflight{}
	}
	return BeneficiaryPreflight{Answered: true, Eligible: out.Eligible, Code: out.Code}
}
