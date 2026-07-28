// SPDX-License-Identifier: Apache-2.0

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
	"sort"
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
	watermarks   ports.RelayWatermarkStore
	logger       *slog.Logger
}

// NewCactiRelay creates a CactiRelay that calls the Cacti service at baseURL
// (e.g. "http://localhost:4000"). authSecret is sent as X-Relay-Auth on every request.
//
// watermarks persists the per-stream event cursor so the poller resumes from the
// last processed timestamp after a restart instead of resetting to time.Now()
// (which would silently drop events observed while the process was down — finding
// R2-H-11). It may be nil, in which case the poller falls back to time.Now() and
// no cursor is persisted.
func NewCactiRelay(baseURL string, authSecret string, watermarks ports.RelayWatermarkStore, logger *slog.Logger) *CactiRelay {
	return &CactiRelay{
		baseURL:      baseURL,
		authSecret:   authSecret,
		pollInterval: defaultPollInterval,
		httpClient:   &http.Client{Timeout: defaultHTTPTimeout},
		watermarks:   watermarks,
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
	// Track the last seen timestamp to avoid re-delivering events. Resume from the
	// persisted watermark so events observed while this process was down are not
	// skipped (finding R2-H-11); fall back to now when no watermark is available.
	lastSeen := c.loadWatermark(ctx, kind)
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			events, err := c.fetchEvents(ctx, kind, lastSeen)
			if err != nil {
				c.logger.Warn("cacti: poll events error", "kind", kind, "error", err)
				continue
			}
			// Advance the watermark only through events the handler actually processed. On the
			// first handler failure we stop advancing so the failed event (and everything after
			// it) is re-fetched on the next tick instead of being skipped forever — the watermark
			// must mean "delivered", not "fetched" (finding R2-H-11). Events are sorted ascending
			// so "everything after" is well defined even if the relay returns them out of order.
			sort.Slice(events, func(i, j int) bool { return events[i].ts < events[j].ts })
			advanced := false
			for _, e := range events {
				if err := handler(e.proof); err != nil {
					c.logger.Error("cacti: event handler error; halting watermark advance for retry",
						"kind", kind, "contractId", e.proof.ContractID, "error", err)
					break
				}
				if e.ts > lastSeen {
					lastSeen = e.ts
					advanced = true
				}
			}
			if advanced {
				c.saveWatermark(ctx, kind, lastSeen)
			}
		}
	}
}

// loadWatermark returns the resume cursor for kind: the persisted watermark when
// one exists, otherwise the current time (preserving the historical behaviour when
// no store is wired or nothing has been recorded yet).
func (c *CactiRelay) loadWatermark(ctx context.Context, kind string) int64 {
	if c.watermarks == nil {
		return time.Now().UnixMilli()
	}
	value, found, err := c.watermarks.GetWatermark(ctx, kind)
	if err != nil {
		c.logger.Warn("cacti: watermark load failed; starting from now", "kind", kind, "error", err)
		return time.Now().UnixMilli()
	}
	if !found {
		return time.Now().UnixMilli()
	}
	c.logger.Info("cacti: resuming from persisted watermark", "kind", kind, "sinceMs", value)
	return value
}

// saveWatermark persists the latest processed cursor for kind. Failures are logged
// but not fatal: a lost write only risks re-delivering already-handled events on the
// next restart, which downstream dedup absorbs.
func (c *CactiRelay) saveWatermark(ctx context.Context, kind string, value int64) {
	if c.watermarks == nil {
		return
	}
	if err := c.watermarks.SetWatermark(ctx, kind, value); err != nil {
		c.logger.Warn("cacti: watermark persist failed", "kind", kind, "sinceMs", value, "error", err)
	}
}

// relayEvent pairs a converted proof with the timestamp the relay assigned it, so the poller can
// advance its watermark to exactly the last successfully-handled event (finding R2-H-11).
type relayEvent struct {
	proof ports.InteroperabilityProof
	ts    int64
}

// fetchEvents retrieves events from the Cacti service newer than sinceMs.
// Returns each converted proof paired with its timestamp, and any error.
func (c *CactiRelay) fetchEvents(ctx context.Context, kind string, sinceMs int64) ([]relayEvent, error) {
	url := fmt.Sprintf("%s/api/v1/relay/events/%s?since=%d", c.baseURL, kind, sinceMs+1)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Relay-Auth", c.authSecret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, raw)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var events []relayEvent

	switch kind {
	case "settle":
		var raws []cactiSettleEvent
		if err := json.Unmarshal(raw, &raws); err != nil {
			return nil, err
		}
		for _, e := range raws {
			events = append(events, relayEvent{
				proof: ports.InteroperabilityProof{
					SourceChain:   e.Spoke,
					ContractID:    e.ContractID,
					EventName:     "LogHTLCClaimed",
					ProofPayload:  []byte(fmt.Sprintf(`{"secret":"%s","txHash":"%s","blockNumber":%d}`, e.Secret, e.TxHash, e.BlockNumber)),
					CorrelationID: e.TxHash,
				},
				ts: e.Timestamp,
			})
		}
	case "lock":
		var raws []cactiLockEvent
		if err := json.Unmarshal(raw, &raws); err != nil {
			return nil, err
		}
		for _, e := range raws {
			events = append(events, relayEvent{
				proof: ports.InteroperabilityProof{
					SourceChain:   e.Spoke,
					ContractID:    e.ContractID,
					EventName:     "LogHTLCLocked",
					HashLock:      e.HashLock,
					TimeLock:      e.TimeLock,
					ZetoLockRef:   e.ZetoLockRef,
					ProofPayload:  []byte(fmt.Sprintf(`{"sender":"%s","receiver":"%s","txHash":"%s","blockNumber":%d}`, e.Sender, e.Receiver, e.TxHash, e.BlockNumber)),
					CorrelationID: e.TxHash,
				},
				ts: e.Timestamp,
			})
		}
	}

	return events, nil
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
