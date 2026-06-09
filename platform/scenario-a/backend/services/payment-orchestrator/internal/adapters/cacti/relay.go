// Package cacti provides the CactiRelay implementation of InteroperabilityPort.
// It connects the payment-orchestrator to the Cacti HTLC relay service running
// at interop/hub-and-spoke/cacti via its REST API.
package cacti

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

const (
	defaultPollInterval = 3 * time.Second
	defaultHTTPTimeout  = 10 * time.Second
)

// CactiRelay implements InteroperabilityPort by polling the Cacti HTLC relay
// service REST API. It is the production replacement for StubRelay.
type CactiRelay struct {
	baseURL      string
	authSecret   string
	pollInterval time.Duration
	httpClient   *http.Client
	logger       *slog.Logger
}

// NewCactiRelay creates a CactiRelay that calls the Cacti service at baseURL
// (e.g. "http://localhost:4000"). authSecret is sent as X-Relay-Auth on every request.
func NewCactiRelay(baseURL string, authSecret string, logger *slog.Logger) *CactiRelay {
	return &CactiRelay{
		baseURL:      baseURL,
		authSecret:   authSecret,
		pollInterval: defaultPollInterval,
		httpClient:   &http.Client{Timeout: defaultHTTPTimeout},
		logger:       logger,
	}
}

// ── InteroperabilityPort ──────────────────────────────────────────────────

// SubscribeLockEvents polls GET /api/v1/relay/events/lock on the Cacti service
// and calls handler for each new LogHTLCLocked event. The subscription runs
// until the context is cancelled.
func (c *CactiRelay) SubscribeLockEvents(ctx context.Context, handler func(ports.InteroperabilityProof) error) error {
	go c.pollEvents(ctx, "lock", handler)
	return nil
}

// SubscribeSettleEvents polls GET /api/v1/relay/events/settle on the Cacti
// service and calls handler for each new LogHTLCClaimed event.
func (c *CactiRelay) SubscribeSettleEvents(ctx context.Context, handler func(ports.InteroperabilityProof) error) error {
	go c.pollEvents(ctx, "settle", handler)
	return nil
}

// RelayProof POSTs a cross-chain proof to the Cacti service for forwarding.
func (c *CactiRelay) RelayProof(ctx context.Context, proof ports.InteroperabilityProof) (string, error) {
	body, err := json.Marshal(proofToRequest(proof))
	if err != nil {
		return "", fmt.Errorf("cacti relay proof marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/api/v1/relay/proof",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("cacti relay proof request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("cacti relay proof: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("cacti relay proof: unexpected status %d: %s", resp.StatusCode, raw)
	}

	var out struct {
		RelayTxID     string `json:"relay_tx_id"`
		CorrelationID string `json:"correlation_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("cacti relay proof decode: %w", err)
	}
	c.logger.Info("cacti: proof relayed", "relayTxId", out.RelayTxID, "correlationId", out.CorrelationID)
	return out.RelayTxID, nil
}

// VerifyProof GETs a proof from the Cacti service and returns true if found.
func (c *CactiRelay) VerifyProof(ctx context.Context, proof ports.InteroperabilityProof) (bool, error) {
	if proof.CorrelationID == "" {
		return false, fmt.Errorf("cacti verify proof: correlationId is required")
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/api/v1/relay/proof/"+proof.CorrelationID,
		nil,
	)
	if err != nil {
		return false, fmt.Errorf("cacti verify proof request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("cacti verify proof: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("cacti verify proof: unexpected status %d: %s", resp.StatusCode, raw)
	}

	var out struct {
		Verified bool `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, fmt.Errorf("cacti verify proof decode: %w", err)
	}
	return out.Verified, nil
}

// ── Internal polling loop ─────────────────────────────────────────────────

// cactiSettleEvent matches the JSON returned by GET /api/v1/relay/events/settle.
type cactiSettleEvent struct {
	Spoke       string `json:"spoke"`
	ContractID  string `json:"contractId"`
	Secret      string `json:"secret"`
	BlockNumber int64  `json:"blockNumber"`
	TxHash      string `json:"txHash"`
	Timestamp   int64  `json:"timestamp"` // Unix ms
}

// cactiLockEvent matches the JSON returned by GET /api/v1/relay/events/lock.
type cactiLockEvent struct {
	Spoke       string `json:"spoke"`
	ContractID  string `json:"contractId"`
	Sender      string `json:"sender"`
	Receiver    string `json:"receiver"`
	HashLock    string `json:"hashLock"`
	TimeLock    uint64 `json:"timeLock"`
	ZetoLockRef string `json:"zetoLockRef"`
	BlockNumber int64  `json:"blockNumber"`
	TxHash      string `json:"txHash"`
	Timestamp   int64  `json:"timestamp"` // Unix ms
}

func (c *CactiRelay) pollEvents(ctx context.Context, kind string, handler func(ports.InteroperabilityProof) error) {
	// Track the last seen timestamp to avoid re-delivering events.
	var lastSeen int64 = time.Now().UnixMilli()
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			proofs, since, err := c.fetchEvents(ctx, kind, lastSeen)
			if err != nil {
				c.logger.Warn("cacti: poll events error", "kind", kind, "error", err)
				continue
			}
			for _, p := range proofs {
				if err := handler(p); err != nil {
					c.logger.Error("cacti: event handler error", "kind", kind, "contractId", p.ContractID, "error", err)
				}
			}
			if since > lastSeen {
				lastSeen = since
			}
		}
	}
}

// fetchEvents retrieves events from the Cacti service newer than sinceMs.
// Returns converted proofs, the latest observed timestamp, and any error.
func (c *CactiRelay) fetchEvents(ctx context.Context, kind string, sinceMs int64) ([]ports.InteroperabilityProof, int64, error) {
	url := fmt.Sprintf("%s/api/v1/relay/events/%s?since=%d", c.baseURL, kind, sinceMs+1)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, sinceMs, err
	}
	req.Header.Set("X-Relay-Auth", c.authSecret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, sinceMs, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, sinceMs, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, raw)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, sinceMs, err
	}

	var proofs []ports.InteroperabilityProof
	var maxTs int64 = sinceMs

	switch kind {
	case "settle":
		var events []cactiSettleEvent
		if err := json.Unmarshal(raw, &events); err != nil {
			return nil, sinceMs, err
		}
		for _, e := range events {
			proofs = append(proofs, ports.InteroperabilityProof{
				SourceChain:   e.Spoke,
				ContractID:    e.ContractID,
				EventName:     "LogHTLCClaimed",
				ProofPayload:  []byte(fmt.Sprintf(`{"secret":"%s","txHash":"%s","blockNumber":%d}`, e.Secret, e.TxHash, e.BlockNumber)),
				CorrelationID: e.TxHash,
			})
			if e.Timestamp > maxTs {
				maxTs = e.Timestamp
			}
		}
	case "lock":
		var events []cactiLockEvent
		if err := json.Unmarshal(raw, &events); err != nil {
			return nil, sinceMs, err
		}
		for _, e := range events {
			proofs = append(proofs, ports.InteroperabilityProof{
				SourceChain:   e.Spoke,
				ContractID:    e.ContractID,
				EventName:     "LogHTLCLocked",
				HashLock:      e.HashLock,
				TimeLock:      e.TimeLock,
				ZetoLockRef:   e.ZetoLockRef,
				ProofPayload:  []byte(fmt.Sprintf(`{"sender":"%s","receiver":"%s","txHash":"%s","blockNumber":%d}`, e.Sender, e.Receiver, e.TxHash, e.BlockNumber)),
				CorrelationID: e.TxHash,
			})
			if e.Timestamp > maxTs {
				maxTs = e.Timestamp
			}
		}
	}

	return proofs, maxTs, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────

type proofRequest struct {
	CorrelationID string `json:"correlationId,omitempty"`
	SourceChain   string `json:"sourceChain"`
	ContractID    string `json:"contractId"`
	EventName     string `json:"eventName"`
	HashLock      string `json:"hashLock"`
	TimeLock      uint64 `json:"timeLock"`
	ZetoLockRef   string `json:"zetoLockRef"`
	ProofPayload  string `json:"proofPayload"` // base64
}

func proofToRequest(p ports.InteroperabilityProof) proofRequest {
	return proofRequest{
		CorrelationID: p.CorrelationID,
		SourceChain:   p.SourceChain,
		ContractID:    p.ContractID,
		EventName:     p.EventName,
		HashLock:      p.HashLock,
		TimeLock:      p.TimeLock,
		ZetoLockRef:   p.ZetoLockRef,
		ProofPayload:  string(p.ProofPayload),
	}
}
