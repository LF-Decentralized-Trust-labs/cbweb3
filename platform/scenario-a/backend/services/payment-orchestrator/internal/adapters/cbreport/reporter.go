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
	// backoff is per-instance so a test can zero it; production uses retryBackoff.
	backoff []time.Duration
}

// New builds a Reporter targeting the given Central Bank gateway base URL. The
// authSecret is sent as X-Relay-Auth so it clears the CB's relay-auth guard.
func New(baseURL, authSecret string) *Reporter {
	return &Reporter{
		client:     &http.Client{Timeout: 10 * time.Second},
		url:        strings.TrimRight(baseURL, "/") + path,
		authSecret: authSecret,
		backoff:    retryBackoff,
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

// maxAttempts and retryBackoff bound the delivery attempts for one leg.
//
// A report is the ONLY record of an incoming leg — the receiving bank's own
// orchestrator has none — so dropping it on the first transport hiccup silently
// removes a settled movement from the ledger. Three attempts cover the common
// case: the CB gateway restarting, or a connection reset. They do NOT make
// delivery durable; a CB down longer than the window, or this process exiting,
// still loses the report. Durable delivery needs an outbox and is deliberately
// not attempted here (see the card referenced in the PR).
const maxAttempts = 3

var retryBackoff = []time.Duration{2 * time.Second, 4 * time.Second}

// ReportSettledLeg POSTs the leg to the Central Bank, retrying a bounded number of
// times on failures that may be transient. It detaches from the caller context
// (settlement must not be undone by a slow report) but honors a bounded timeout
// per attempt via the HTTP client.
//
// A 4xx is NOT retried: the CB rejected the content, and repeating it cannot
// change that. Only transport failures and 5xx are worth another attempt.
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

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		permanent, err := r.postOnce(body)
		if err == nil {
			return nil
		}
		lastErr = err
		if permanent || attempt == maxAttempts {
			break
		}
		time.Sleep(r.backoff[attempt-1])
	}
	return lastErr
}

// postOnce performs a single delivery attempt. It reports whether the failure is
// permanent, so the caller can stop retrying something that cannot succeed.
func (r *Reporter) postOnce(body []byte) (permanent bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(body))
	if err != nil {
		return true, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if r.authSecret != "" {
		req.Header.Set("X-Relay-Auth", r.authSecret)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("central bank unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(resp.Body)
		// 4xx means the CB rejected this content; retrying sends the same bytes.
		perm := resp.StatusCode >= 400 && resp.StatusCode < 500
		return perm, fmt.Errorf("central bank returned %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return false, nil
}

// noSleep removes the retry backoff. Test-only: the backoff exists to spare a
// recovering central bank, and waiting six real seconds per case would make the
// suite slow enough that someone deletes the tests instead.
func (r *Reporter) noSleep() { r.backoff = []time.Duration{0, 0} }
