// SPDX-License-Identifier: Apache-2.0

// Package relayer provides a client for the Hyperledger Cacti Relayer that orchestrates
// cross-chain bridging operations (Lock&Mint / Burn&Unlock) between Spokes and the Hub.
// It uses plain HTTP JSON and is safe to run as a no-op stub when CACTI_RELAYER_URL is
// empty (dev environments where the Relayer is mocked by the queue worker) — see FR-031,
// FR-039, FR-040.
package relayer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client interfaces with the Cacti Relayer for cross-chain bridging operations.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Config holds the connection parameters for the Relayer client.
type Config struct {
	BaseURL string
	Timeout time.Duration
}

// NewClient creates a new Relayer client. If cfg.BaseURL is empty the returned client is a
// no-op: all Submit* methods return nil immediately (useful in dev tryouts without Cacti).
func NewClient(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		httpClient: &http.Client{Timeout: timeout},
	}
}

// SubmitLockRequest carries the payload for a Lock&Mint notification.
type SubmitLockRequest struct {
	EventRef       string `json:"event_ref"`
	SourceChain    string `json:"source_chain"`
	TargetChain    string `json:"target_chain"`
	TokenAddress   string `json:"token_address"`
	Amount         string `json:"amount"`
	Sender         string `json:"sender"`
	Beneficiary    string `json:"beneficiary"`
	IdempotencyKey string `json:"idempotency_key"`
}

// SubmitBurnRequest carries the payload for a Burn&Unlock notification.
type SubmitBurnRequest struct {
	EventRef       string `json:"event_ref"`
	SourceChain    string `json:"source_chain"`
	TargetChain    string `json:"target_chain"`
	TokenAddress   string `json:"token_address"`
	Amount         string `json:"amount"`
	Sender         string `json:"sender"`
	Beneficiary    string `json:"beneficiary"`
	IdempotencyKey string `json:"idempotency_key"`
}

// SubmitLockEvent notifies the Relayer of a confirmed lock event on a Spoke (REQ-CAP-005).
func (c *Client) SubmitLockEvent(ctx context.Context, req SubmitLockRequest) error {
	return c.post(ctx, "/lock-mint", req)
}

// SubmitBurnEvent notifies the Relayer of a confirmed burn event on the Hub.
func (c *Client) SubmitBurnEvent(ctx context.Context, req SubmitBurnRequest) error {
	return c.post(ctx, "/burn-unlock", req)
}

// Health performs a GET /health on the Relayer. Returns nil if the endpoint is healthy.
func (c *Client) Health(ctx context.Context) error {
	if c.baseURL == "" {
		return nil
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(r)
	if err != nil {
		return fmt.Errorf("relayer health: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("relayer health status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) post(ctx context.Context, path string, body interface{}) error {
	if c.baseURL == "" {
		return nil
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal relayer payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("relayer %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("relayer %s returned %d: %s", path, resp.StatusCode, string(raw))
	}
	return nil
}

// Legacy signature kept for backwards compatibility with earlier wiring; forwards to the
// structured request with minimal data.
func (c *Client) SubmitLockEventRef(ctx context.Context, eventRef string) error {
	if eventRef == "" {
		return errors.New("eventRef is required")
	}
	return c.SubmitLockEvent(ctx, SubmitLockRequest{EventRef: eventRef, IdempotencyKey: eventRef})
}

// Legacy signature kept for backwards compatibility.
func (c *Client) SubmitBurnEventRef(ctx context.Context, eventRef string) error {
	if eventRef == "" {
		return errors.New("eventRef is required")
	}
	return c.SubmitBurnEvent(ctx, SubmitBurnRequest{EventRef: eventRef, IdempotencyKey: eventRef})
}
