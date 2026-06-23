// SPDX-License-Identifier: Apache-2.0

package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RemoteTransferLimitChecker implements TransferLimitCheckerIface by calling the CB's
// internal /internal/v2/transfer-limits/* endpoints. Used by commercial bank gateways
// so enforcement always runs against the CB's authoritative DB (R1-10.1 Option A).
//
// Fail-closed: any non-2xx response (including CB unreachable) blocks the transfer.
type RemoteTransferLimitChecker struct {
	cbURL      string
	authSecret string
	httpClient *http.Client
}

// NewRemoteTransferLimitChecker creates a RemoteTransferLimitChecker pointed at cbURL.
func NewRemoteTransferLimitChecker(cbURL, authSecret string, timeout time.Duration) *RemoteTransferLimitChecker {
	return &RemoteTransferLimitChecker{
		cbURL:      cbURL,
		authSecret: authSecret,
		httpClient: &http.Client{Timeout: timeout},
	}
}

type remoteCheckRequest struct {
	PayerBankID string `json:"payer_bank_id"`
	Currency    string `json:"currency"`
	AmountHuman string `json:"amount_human"`
}

// CheckAndDeduct calls the CB's check-and-deduct endpoint. Fail-closed: any error blocks.
func (r *RemoteTransferLimitChecker) CheckAndDeduct(ctx context.Context, payerBankID, currency, amountHuman string) error {
	body, _ := json.Marshal(remoteCheckRequest{
		PayerBankID: payerBankID,
		Currency:    currency,
		AmountHuman: amountHuman,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		r.cbURL+"/internal/v2/transfer-limits/check-and-deduct",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("transfer limit pre-auth: failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Relay-Auth", r.authSecret)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("transfer limit pre-auth: CB unreachable (%s): %w", r.cbURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	// Propagate a structured limit-exceeded error so callers can surface a helpful message.
	if resp.StatusCode == http.StatusUnprocessableEntity {
		var errPayload struct {
			Error     string `json:"error"`
			ErrorCode string `json:"error_code"`
		}
		if json.Unmarshal(respBody, &errPayload) == nil && errPayload.ErrorCode == "TRANSFER_LIMIT_EXCEEDED" {
			return &ErrTransferLimitExceeded{
				PayerBankID: payerBankID,
				Currency:    currency,
			}
		}
	}

	return fmt.Errorf("transfer limit pre-auth: CB returned %d: %s", resp.StatusCode, string(respBody))
}

// Restore calls the CB's restore endpoint. Best-effort: errors are swallowed.
func (r *RemoteTransferLimitChecker) Restore(ctx context.Context, payerBankID, currency, amountHuman string) {
	body, _ := json.Marshal(remoteCheckRequest{
		PayerBankID: payerBankID,
		Currency:    currency,
		AmountHuman: amountHuman,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		r.cbURL+"/internal/v2/transfer-limits/restore",
		bytes.NewReader(body),
	)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Relay-Auth", r.authSecret)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close() // #nosec G104 -- response body discarded immediately; close error not actionable
}
