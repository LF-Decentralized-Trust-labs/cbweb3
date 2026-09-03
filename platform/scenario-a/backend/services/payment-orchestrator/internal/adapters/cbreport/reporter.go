// SPDX-License-Identifier: Apache-2.0

// Package cbreport implements the SettlementReporter port over the Central Bank
// gateway's internal, relay-authenticated PvP-legs endpoint. It lets a settling
// orchestrator forward a completed PvP leg to the CB so the receiving bank can
// see the incoming credit on its statement.
package cbreport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

// path is the CB gateway internal endpoint that ingests settled PvP legs.
const path = "/internal/v1/payments/pvp-legs"

// Reporter posts settled PvP legs to the Central Bank gateway.
type Reporter struct {
	client     *http.Client
	url        string
	authSecret string
}

// New builds a Reporter targeting the given Central Bank gateway base URL. The
// authSecret is sent as X-Relay-Auth so it clears the CB's relay-auth guard.
func New(baseURL, authSecret string) *Reporter {
	return &Reporter{
		client:     &http.Client{Timeout: 10 * time.Second},
		url:        strings.TrimRight(baseURL, "/") + path,
		authSecret: authSecret,
	}
}

// legPayload is the JSON body accepted by the CB gateway.
type legPayload struct {
	TradeID    string `json:"trade_id"`
	ContractID string `json:"contract_id"`
	Sender     string `json:"sender"`
	Receiver   string `json:"receiver"`
	Amount     string `json:"amount"`
	SettledAt  string `json:"settled_at"` // RFC3339
}

// ReportSettledLeg POSTs the leg to the Central Bank. It detaches from the caller
// context (settlement must not be undone by a slow report) but honors a bounded
// timeout via the HTTP client.
func (r *Reporter) ReportSettledLeg(_ context.Context, leg ports.SettledLeg) error {
	body, err := json.Marshal(legPayload{
		TradeID:    leg.TradeID,
		ContractID: leg.ContractID,
		Sender:     leg.Sender,
		Receiver:   leg.Receiver,
		Amount:     leg.Amount,
		SettledAt:  leg.SettledAt.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("marshal settled leg: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if r.authSecret != "" {
		req.Header.Set("X-Relay-Auth", r.authSecret)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("central bank unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("central bank returned %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}
