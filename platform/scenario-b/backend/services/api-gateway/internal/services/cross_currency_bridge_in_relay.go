// Package services provides CrossCurrencyBridgeInRelay for delegating the bridge-in
// lock-mint to the issuing Central Bank of the initiating bank's own spoke.
//
// Minting wrapped sovereign money (W-<source>) on the Hub is a central-bank-only
// operation (CENTRAL_BANK_ROLE). A commercial bank must NOT mint it directly, so the
// cross-currency swap orchestrator (Step 1) delegates the lock-mint to its spoke's CB,
// reached directly via CENTRAL_BANK_API_URL (same jurisdiction — no Cacti hop needed).
//
// This is the bridge-in counterpart of CactiCrossCurrencyRelay (Step 3 bridge-out, which
// crosses jurisdictions to the foreign beneficiary CB via Cacti). Unlike bridge-out
// (fire-and-forward), bridge-in is synchronous: the CB endpoint enqueues the lock-mint and
// blocks until the position reaches ACTIVE, because the AMM swap (Step 2) cannot run until
// the W-<source> tokens exist on the Hub.
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

// CrossCurrencyBridgeInRelayIface is implemented by CrossCurrencyBridgeInRelay.
// The orchestrator uses this in Step 1 when the bridge-in lock-mint must be performed by
// the issuing CB (sovereign model) instead of the initiating gateway's own relayer.
type CrossCurrencyBridgeInRelayIface interface {
	// NotifyBridgeIn asks the issuing CB to lock-mint W-<source> on the Hub and waits
	// for the bridge position to reach ACTIVE. Returns the CB-side position_id.
	NotifyBridgeIn(ctx context.Context, req CrossCurrencyBridgeInRequest) (string, error)
}

// CrossCurrencyBridgeInRequest is the payload sent to the issuing CB.
type CrossCurrencyBridgeInRequest struct {
	CorrelationID  string `json:"correlation_id"`
	PayerBankID    string `json:"payer_bank_id"`
	SourceCurrency string `json:"source_currency"`
	Amount         string `json:"amount"`
	SpokeIn        string `json:"spoke_in"`
	// SwapSenderAddress is the initiating gateway's Hub swap signer — the CB mints W-<source>
	// to this address so the AMM swap (Step 2) can spend it. Symmetric with the bridge-out
	// SwapSenderAddress that tells CB-B where to burn W-<target> from.
	SwapSenderAddress string `json:"swap_sender_address,omitempty"`
}

// CrossCurrencyBridgeInRelay calls the spoke CB to perform the sovereign lock-mint.
type CrossCurrencyBridgeInRelay struct {
	centralBankURL  string // e.g. "http://api-gateway-central-bank-a:8080"
	relayAuthSecret string // X-Relay-Auth shared secret
	httpClient      *http.Client
}

// NewCrossCurrencyBridgeInRelay constructs the relay. The timeout must exceed the CB's
// bridge-in ACTIVE wait (120s) so the synchronous response is not cut short.
func NewCrossCurrencyBridgeInRelay(centralBankURL, relayAuthSecret string) *CrossCurrencyBridgeInRelay {
	return &CrossCurrencyBridgeInRelay{
		centralBankURL:  strings.TrimRight(centralBankURL, "/"),
		relayAuthSecret: relayAuthSecret,
		httpClient:      &http.Client{Timeout: 150 * time.Second},
	}
}

// NotifyBridgeIn POSTs the bridge-in request to the CB and returns the position_id.
func (r *CrossCurrencyBridgeInRelay) NotifyBridgeIn(ctx context.Context, req CrossCurrencyBridgeInRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal bridge-in request: %w", err)
	}

	url := r.centralBankURL + "/internal/amm/cross-currency-bridge-in"
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
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("central bank bridge-in returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var out struct {
		Status      string `json:"status"`
		PositionID  string `json:"position_id"`
		BridgeState string `json:"bridge_state"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("decode bridge-in response: %w", err)
	}
	return out.PositionID, nil
}
