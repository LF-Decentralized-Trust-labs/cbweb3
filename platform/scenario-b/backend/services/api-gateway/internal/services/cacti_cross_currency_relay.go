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

// CactiCrossCurrencyBridgeOutRequest is the payload sent to Cacti.
type CactiCrossCurrencyBridgeOutRequest struct {
	CorrelationID      string `json:"correlation_id"`
	SwapTxHash         string `json:"swap_tx_hash"`
	PoolPair           string `json:"pool_pair"`
	AmountOut          string `json:"amount_out"`
	BeneficiaryBankID  string `json:"beneficiary_bank_id"`
	SpokeOut           string `json:"spoke_out"`
	WrappedTargetToken string `json:"wrapped_target_token"`
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
