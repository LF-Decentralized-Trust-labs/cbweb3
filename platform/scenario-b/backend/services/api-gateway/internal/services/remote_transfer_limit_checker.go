// SPDX-License-Identifier: Apache-2.0

package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/relayauth"
	"io"
	"log"
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
	// signer, when set, signs each request with this entity's own key so the CB can attribute the
	// call to a specific bank. Without it the only credential is authSecret — which is identical in
	// every entity, so any entity could forge these calls as any other, and check-and-deduct is the
	// CB's authoritative daily-limit gate.
	signer *relayauth.Signer
}

// NewRemoteTransferLimitChecker creates a RemoteTransferLimitChecker pointed at cbURL.
func NewRemoteTransferLimitChecker(cbURL, authSecret string, timeout time.Duration) *RemoteTransferLimitChecker {
	return &RemoteTransferLimitChecker{
		cbURL:      cbURL,
		authSecret: authSecret,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// WithSigner attaches the per-entity signer. The legacy shared secret is still sent alongside: the
// receiving CB accepts either during the migration, so a bank that signs and one that does not both
// keep working, and a CB that has not yet pinned this bank does not fail the payment closed.
func (r *RemoteTransferLimitChecker) WithSigner(s *relayauth.Signer) *RemoteTransferLimitChecker {
	r.signer = s
	return r
}

// sign attaches the signature headers for the given path and body, when a signer is configured.
// The signature covers method, path and body, so it must be computed for the exact path called —
// check-and-deduct and restore are different canonical strings.
func (r *RemoteTransferLimitChecker) sign(req *http.Request, path string, body []byte) {
	if r.signer == nil {
		return
	}
	headers, err := r.signer.HeadersFor(req.Method, path, body, time.Now())
	if err != nil {
		// Not fatal: the shared secret still authenticates the call during the migration. Failing
		// here would block a payment over a signing problem the receiver can still tolerate.
		log.Printf("[transfer-limit] could not sign %s: %v (falling back to the shared secret)", path, err)
		return
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
}

// The signature covers the PATH, so it must be the same string used to build the URL. Naming both
// from one constant is what keeps them from drifting.
const (
	transferLimitCheckPath   = "/internal/v2/transfer-limits/check-and-deduct"
	transferLimitRestorePath = "/internal/v2/transfer-limits/restore"
)

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
		r.cbURL+transferLimitCheckPath,
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("transfer limit pre-auth: failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Relay-Auth", r.authSecret)
	r.sign(req, transferLimitCheckPath, body)

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
		r.cbURL+transferLimitRestorePath,
		bytes.NewReader(body),
	)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Relay-Auth", r.authSecret)
	r.sign(req, transferLimitRestorePath, body)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close() // #nosec G104 -- response body discarded immediately; close error not actionable
}
